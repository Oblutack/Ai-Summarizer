from retrieval import PAGE_BREAK, Passage, document_headers, locate, select_passages


def pdf_text(*pages):
    return PAGE_BREAK.join(pages)


def test_short_document_is_one_numbered_passage():
    passages = select_passages("tiny document", "anything", max_chars=1000)
    assert passages == [Passage(id=1, text="tiny document", page=None, page_end=None, document=None)]


def test_passages_report_the_page_they_start_on():
    filler = "Lorem ipsum dolor sit amet consectetur. " * 60  # about 2,400 characters, more than one chunk
    text = pdf_text(filler, "The compressor warranty is seven years.", filler)
    passages = select_passages(text, "compressor warranty", max_chars=500)
    hit = next(p for p in passages if "seven years" in p.text)
    assert (hit.page, hit.page_end) == (2, 2)
    assert PAGE_BREAK not in hit.text, "form feeds are for locating pages, not for showing"


def test_every_passage_in_a_pdf_has_a_page_and_numbering_is_in_document_order():
    pages = [f"Page {i}. " + f"words about topic number {i}. " * 80 for i in range(1, 6)]
    passages = select_passages(pdf_text(*pages), "topic", max_chars=1_000_000)
    assert [p.id for p in passages] == list(range(1, len(passages) + 1))
    first_pages = [p.page for p in passages]
    assert all(p is not None for p in first_pages)
    assert first_pages == sorted(first_pages) and first_pages[0] == 1 and first_pages[-1] == 5


def test_passages_never_span_pages_so_citations_are_exact():
    text = pdf_text("a" * 100, "b" * 100, "c" * 100)
    passages = select_passages(text, "anything", max_chars=10_000)
    assert [(p.page, p.page_end) for p in passages] == [(1, 1), (2, 2), (3, 3)]


def test_a_long_page_is_split_but_stays_on_its_page():
    text = pdf_text("short first page", "long second page. " * 400, "short third page")
    passages = select_passages(text, "anything", max_chars=1_000_000)
    second = [p for p in passages if p.page == 2]
    assert len(second) > 1 and all(p.page_end == 2 for p in second)
    assert [p.page for p in passages] == sorted(p.page for p in passages)


def test_pages_without_text_are_skipped_but_still_counted():
    text = pdf_text("first page text", "", "third page text")  # the middle page is a scanned image
    passages = select_passages(text, "anything", max_chars=10_000)
    assert [(p.page, p.text) for p in passages] == [(1, "first page text"), (3, "third page text")]


def test_text_without_page_breaks_has_no_page_numbers():
    passages = select_passages("plain pasted text " * 50, "text", max_chars=10_000)
    assert all(p.page is None and p.page_end is None for p in passages)


def test_combined_files_report_the_file_and_restart_page_numbers():
    first = pdf_text("alpha " * 40, "alpha second page " * 40)
    second = pdf_text("beta " * 40, "beta second page " * 40)
    text = f"=== a.pdf ===\n{first}\n\n=== b.pdf ===\n{second}"
    passages = select_passages(text, "beta", max_chars=10_000)

    beta = [p for p in passages if "beta" in p.text]
    assert beta and all(p.document == "b.pdf" for p in beta)
    assert beta[0].page == 1, "page numbers restart within each file"
    alpha = [p for p in passages if p.document == "a.pdf"]
    assert alpha and alpha[0].page == 1


def test_locate_and_headers():
    text = "=== a.pdf ===\none\fpage two\n\n=== b.pdf ===\nother\fsecond"
    headers = document_headers(text)
    assert [name for _, name in headers] == ["a.pdf", "b.pdf"]
    assert locate(text, text.index("page two"), headers) == ("a.pdf", 2)
    assert locate(text, text.index("second"), headers) == ("b.pdf", 2)
    assert document_headers("=== not at the start") == []
    assert document_headers("pasted text with\n=== a line like a header ===\ninside") == []


def test_selection_still_prefers_relevant_passages_and_keeps_order():
    filler = "Lorem ipsum dolor sit amet consectetur. " * 200
    needle = "The warranty period for the compressor is seven years from purchase."
    text = pdf_text(filler, filler, needle, filler, filler)
    passages = select_passages(text, "How long is the compressor warranty?", max_chars=3000)
    assert any("seven years" in p.text for p in passages)
    assert sum(len(p.text) for p in passages) <= 3000 + 100
    assert [p.id for p in passages] == sorted(p.id for p in passages)
