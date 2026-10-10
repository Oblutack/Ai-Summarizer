import io
import subprocess
import time

import pytest
from fastapi.testclient import TestClient
from PIL import Image, ImageDraw, ImageFont

import main
import ocr
from extract import Fetched
from retrieval import PAGE_BREAK


def picture_of(text, size=(1240, 1754)):
    """A white page with the text written on it, as a scanner would give."""
    image = Image.new("RGB", size, "white")
    if text:
        draw = ImageDraw.Draw(image)
        font = ImageFont.load_default(size=44)
        for i, line in enumerate(text.split("|")):
            draw.text((80, 100 + i * 80), line, fill="black", font=font)
    return image


def scanned_pdf(pages, size=(1240, 1754), resolution=150):
    """A PDF that is only pictures: no text layer at all."""
    images = [picture_of(text, size) for text in pages]
    buffer = io.BytesIO()
    images[0].save(buffer, format="PDF", save_all=True, append_images=images[1:], resolution=resolution)
    return buffer.getvalue()


# --- deciding which pages need reading ---


def test_a_page_without_text_of_its_own_needs_reading():
    for empty in ("", "   \n\t ", "- - -", "12 .. 3"):
        assert ocr.needs_ocr(empty), repr(empty)
    assert not ocr.needs_ocr("A page with a real sentence on it.")
    assert not ocr.needs_ocr("0123456789ab")  # twelve letters and digits is enough


# --- drawing a page ---


def test_a_page_is_drawn_as_a_png_the_size_ocr_reads_well():
    png = ocr.render_page(scanned_pdf(["First page", "Second page"]), 1)
    assert png[:8] == b"\x89PNG\r\n\x1a\n"
    width, height = Image.open(io.BytesIO(png)).size
    # an A4 page at 200 dots an inch is about 1654 by 2339
    assert 1600 <= width <= 1700 and 2250 <= height <= 2400


def test_a_page_that_claims_to_be_huge_is_drawn_no_bigger_than_the_limit():
    # 5,000 by 7,000 points: ten times the size of a poster
    png = ocr.render_page(scanned_pdf(["x"], size=(5000, 7000), resolution=72), 0)
    assert max(Image.open(io.BytesIO(png)).size) <= ocr.MAX_SIDE_PIXELS


def test_a_page_that_does_not_exist_is_an_error_not_a_crash():
    with pytest.raises(RuntimeError):  # PDFium's own error: the caller turns it into a plain message
        ocr.render_page(scanned_pdf(["only"]), 5)


# --- running Tesseract ---


class Completed:
    def __init__(self, code=0, out=b"  Some words  \n", err=b""):
        self.returncode, self.stdout, self.stderr = code, out, err


def test_a_picture_is_sent_to_tesseract_over_standard_input_with_the_languages_and_limits(monkeypatch):
    seen = {}

    def fake_run(command, **kwargs):
        seen.update(command=command, **kwargs)
        return Completed()

    monkeypatch.setattr(ocr.subprocess, "run", fake_run)
    assert ocr.read_picture(b"PNG BYTES", "eng+deu", 12.5) == "Some words"
    assert seen["command"] == ["tesseract", "stdin", "stdout", "-l", "eng+deu", "--psm", "3"]
    assert seen["input"] == b"PNG BYTES" and seen["timeout"] == 12.5
    assert seen["env"]["OMP_THREAD_LIMIT"] == "1"
    assert seen["capture_output"] is True and seen["check"] is False


def test_tesseract_problems_become_plain_messages(monkeypatch):
    monkeypatch.setattr(
        ocr.subprocess, "run", lambda *a, **k: Completed(code=1, err=b"Error: secret=/etc/internal detail")
    )
    with pytest.raises(ocr.OcrError) as caught:
        ocr.read_picture(b"x")
    assert caught.value.status == 422 and "secret" not in caught.value.message and "/etc" not in caught.value.message

    def missing(*a, **k):
        raise FileNotFoundError("tesseract")

    monkeypatch.setattr(ocr.subprocess, "run", missing)
    with pytest.raises(ocr.OcrError) as caught:
        ocr.read_picture(b"x")
    assert caught.value.status == 503

    def slow(*a, **k):
        raise subprocess.TimeoutExpired("tesseract", 1)

    monkeypatch.setattr(ocr.subprocess, "run", slow)
    with pytest.raises(ocr.OcrError) as caught:
        ocr.read_picture(b"x")
    assert caught.value.status == 504


# --- reading several pages ---


def test_pages_read_side_by_side_come_back_under_their_own_numbers(monkeypatch):
    monkeypatch.setattr(ocr, "render_page", lambda pdf, index: str(index).encode())

    def read(png, languages, timeout):
        index = int(png)
        time.sleep(0.06 - index * 0.01)  # the later pages finish first
        return f"text of page {index}"

    monkeypatch.setattr(ocr, "read_picture", read)
    result = ocr.read_pages(b"%PDF", [0, 2, 4, 5])
    assert result == {0: "text of page 0", 2: "text of page 2", 4: "text of page 4", 5: "text of page 5"}


def test_too_many_scanned_pages_are_refused_before_any_work(monkeypatch):
    drawn = []
    monkeypatch.setattr(ocr, "MAX_PAGES", 3)
    monkeypatch.setattr(ocr, "render_page", lambda pdf, index: drawn.append(index) or b"x")
    with pytest.raises(ocr.OcrError) as caught:
        ocr.read_pages(b"%PDF", [0, 1, 2, 3])
    assert caught.value.status == 413 and "at most 3" in caught.value.message and not drawn


def test_reading_stops_when_the_time_is_up(monkeypatch):
    monkeypatch.setattr(ocr, "TOTAL_SECONDS", 0.0)
    monkeypatch.setattr(ocr, "render_page", lambda pdf, index: b"x")
    monkeypatch.setattr(ocr, "read_picture", lambda *a, **k: "never")
    with pytest.raises(ocr.OcrError) as caught:
        ocr.read_pages(b"%PDF", [0])
    assert caught.value.status == 504 and "too long" in caught.value.message


def test_a_page_that_cannot_be_drawn_is_a_plain_error(monkeypatch):
    def broken(pdf, index):
        raise RuntimeError("pdfium internals: /tmp/secret")

    monkeypatch.setattr(ocr, "render_page", broken)
    with pytest.raises(ocr.OcrError) as caught:
        ocr.read_pages(b"%PDF", [0])
    assert caught.value.status == 422 and "secret" not in caught.value.message


# --- putting the scanned pages into a document ---

TEXT = "A page with real text of its own, long enough to count."


@pytest.fixture
def reading(monkeypatch):
    calls = []
    monkeypatch.setattr(ocr, "available", lambda: True)
    monkeypatch.setattr(
        ocr, "read_pages", lambda pdf, indexes: calls.append(indexes) or {i: f"SCANNED {i + 1}" for i in indexes}
    )
    return calls


def test_only_the_blank_pages_are_read_and_the_others_keep_their_text(reading):
    text = PAGE_BREAK.join([TEXT, "", TEXT, "  ", TEXT])
    result = main.add_scanned_pages(b"%PDF", text, "mixed.pdf", allow_ocr=True)
    assert reading == [[1, 3]]
    assert result.split(PAGE_BREAK) == [TEXT, "SCANNED 2", TEXT, "SCANNED 4", TEXT]


def test_a_page_keeps_its_own_little_text_unless_reading_the_picture_finds_more(reading, monkeypatch):
    monkeypatch.setattr(
        ocr, "read_pages", lambda pdf, indexes: {1: "ok", 2: "A longer line that the scan really holds"}
    )
    text = PAGE_BREAK.join([TEXT, "Total 450", "Page 3"])
    result = main.add_scanned_pages(b"%PDF", text, "notes.pdf", allow_ocr=True).split(PAGE_BREAK)
    assert result == [TEXT, "Total 450", "A longer line that the scan really holds"]


def test_a_short_document_is_not_mistaken_for_a_scan(reading):
    for short in ("beta content", "Total: 450", "Hi there"):
        assert main.add_scanned_pages(b"%PDF", short, "short.pdf", allow_ocr=False) == short
    assert reading == []
    with pytest.raises(main.HTTPException):
        main.add_scanned_pages(b"%PDF", PAGE_BREAK.join(["", "1", ""]), "scan.pdf", allow_ocr=False)


def test_a_document_with_no_blank_page_is_left_alone(reading):
    text = PAGE_BREAK.join([TEXT, TEXT])
    assert main.add_scanned_pages(b"%PDF", text, "plain.pdf", allow_ocr=True) == text
    assert reading == []


def test_without_permission_nothing_is_read(reading):
    mixed = PAGE_BREAK.join([TEXT, ""])
    assert main.add_scanned_pages(b"%PDF", mixed, "mixed.pdf", allow_ocr=False) == mixed
    assert reading == []
    with pytest.raises(main.HTTPException) as caught:
        main.add_scanned_pages(b"%PDF", PAGE_BREAK.join(["", ""]), "scan.pdf", allow_ocr=False)
    assert caught.value.status_code == 422 and "scan.pdf" in caught.value.detail and "Sign in" in caught.value.detail
    assert reading == []


def test_without_tesseract_a_scan_stays_unread(monkeypatch):
    monkeypatch.setattr(ocr, "available", lambda: False)
    assert main.add_scanned_pages(b"%PDF", "", "scan.pdf", allow_ocr=True) == ""


def test_an_ocr_problem_reaches_the_caller_as_a_plain_error(monkeypatch):
    monkeypatch.setattr(ocr, "available", lambda: True)

    def fail(pdf, indexes):
        raise ocr.OcrError(413, "This PDF has 90 scanned pages; at most 40 can be read at once.")

    monkeypatch.setattr(ocr, "read_pages", fail)
    with pytest.raises(main.HTTPException) as caught:
        main.add_scanned_pages(b"%PDF", PAGE_BREAK.join(["", ""]), "big.pdf", allow_ocr=True)
    assert caught.value.status_code == 413 and "at most 40" in caught.value.detail


# --- through the service ---


class Writer:
    async def ainvoke(self, prompt):
        self.prompt = prompt
        return type("Msg", (), {"content": "A summary."})()


@pytest.fixture
def client(monkeypatch):
    writer = Writer()
    monkeypatch.setattr(main, "get_llm", lambda: writer)
    monkeypatch.setattr(main, "extract_pdf_text", lambda path: "")  # no text layer: every page is a picture
    monkeypatch.setattr(ocr, "available", lambda: True)
    monkeypatch.setattr(
        ocr, "read_pages", lambda pdf, indexes: dict.fromkeys(indexes, "Scanned lease. The rent is 950 euros.")
    )
    c = TestClient(main.app, raise_server_exceptions=False)
    c.writer = writer
    return c


def test_a_scanned_pdf_is_read_and_summarized_when_allowed(client):
    r = client.post("/summarize", files={"file": ("lease.pdf", b"%PDF", "application/pdf")}, data={"ocr": "true"})
    assert r.status_code == 200
    assert r.json()["text"] == "Scanned lease. The rent is 950 euros."
    assert "The rent is 950 euros" in client.writer.prompt


def test_a_scanned_pdf_without_permission_says_to_sign_in(client):
    r = client.post("/summarize", files={"file": ("lease.pdf", b"%PDF", "application/pdf")})
    assert r.status_code == 422 and "Sign in" in r.json()["detail"] and "lease.pdf" in r.json()["detail"]


def test_scanned_pdfs_can_be_combined_with_other_files(client, monkeypatch):
    files = [("files", ("lease.pdf", b"%PDF", "application/pdf")), ("files", ("notes.pdf", b"%PDF", "application/pdf"))]
    r = client.post("/summarize-multiple", files=files, data={"ocr": "true"})
    assert r.status_code == 200 and r.json()["text"].count("Scanned lease") == 2


def test_a_scanned_pdf_at_a_web_address_is_read_too(client, monkeypatch):
    async def fetch(url):
        return Fetched("pdf", b"%PDF-1.4", None, "https://files.example/scans/lease.pdf")

    monkeypatch.setattr(main, "fetch_page", fetch)
    allowed = client.post("/summarize-url?ocr=true", json={"url": "https://files.example/scans/lease.pdf"})
    assert allowed.status_code == 200 and "950 euros" in allowed.json()["text"]
    refused = client.post("/summarize-url", json={"url": "https://files.example/scans/lease.pdf"})
    assert refused.status_code == 422 and "Sign in" in refused.json()["detail"]


# --- the real thing (needs the tesseract program: the Docker image and CI have it) ---


@pytest.mark.skipif(not ocr.available(), reason="the tesseract program is not installed here")
def test_a_scanned_page_is_really_read():
    content = scanned_pdf(
        [
            "The tenant pays 950 euros|per month on the first day.",
            None,
            "Pets are allowed only|with written permission.",
        ]
    )
    text = main.extract_pdf_bytes(content, "scan.pdf", allow_ocr=True)
    pages = text.split(PAGE_BREAK)
    assert len(pages) == 3
    first = pages[0].lower()
    assert "tenant" in first and "950" in first and "euros" in first
    assert pages[1].strip() == ""  # an empty page stays empty
    third = pages[2].lower()
    assert "pets" in third and "permission" in third


@pytest.mark.skipif(not ocr.available(), reason="the tesseract program is not installed here")
def test_without_permission_a_real_scan_is_not_read():
    content = scanned_pdf(["The tenant pays 950 euros."])
    with pytest.raises(main.HTTPException) as caught:
        main.extract_pdf_bytes(content, "scan.pdf", allow_ocr=False)
    assert caught.value.status_code == 422


# --- how many scans are read at the same moment ---


@pytest.fixture
def one_place(monkeypatch):
    import threading

    monkeypatch.setattr(ocr, "_places", threading.BoundedSemaphore(1))
    monkeypatch.setattr(ocr, "QUEUE_SECONDS", 0.2)


def test_only_a_limited_number_of_documents_are_read_at_once(one_place, monkeypatch):
    import threading

    started, release = threading.Event(), threading.Event()

    def slow(pdf, index):
        started.set()
        release.wait(5)
        return b"x"

    monkeypatch.setattr(ocr, "render_page", slow)
    monkeypatch.setattr(ocr, "read_picture", lambda *a, **k: "text")

    first = {}
    worker = threading.Thread(target=lambda: first.update(ocr.read_pages(b"%PDF", [0])))
    worker.start()
    assert started.wait(5), "the first document is being read"

    # every place is taken: the next one waits a moment, then is told to come back
    with pytest.raises(ocr.OcrError) as caught:
        ocr.read_pages(b"%PDF", [0])
    assert caught.value.status == 503 and "busy" in caught.value.message

    release.set()
    worker.join(5)
    assert first == {0: "text"}, "the one being read was not harmed"
    # and the place is free again
    assert ocr.read_pages(b"%PDF", [0]) == {0: "text"}


def test_a_place_is_given_back_when_reading_fails(one_place, monkeypatch):
    def broken(pdf, index):
        raise RuntimeError("pdfium internals")

    monkeypatch.setattr(ocr, "render_page", broken)
    with pytest.raises(ocr.OcrError):
        ocr.read_pages(b"%PDF", [0])
    monkeypatch.setattr(ocr, "render_page", lambda pdf, index: b"x")
    monkeypatch.setattr(ocr, "read_picture", lambda *a, **k: "text")
    assert ocr.read_pages(b"%PDF", [0]) == {0: "text"}, "the failed job must not keep its place"


def test_photos_share_the_same_places(one_place, monkeypatch):
    import threading

    import photos

    started, release = threading.Event(), threading.Event()

    def slow(content, name, languages, timeout):
        started.set()
        release.wait(5)
        return "text"

    monkeypatch.setattr(photos, "read_photo", slow)
    worker = threading.Thread(target=lambda: photos.read_photos([("a.jpg", b"x")]))
    worker.start()
    assert started.wait(5)
    with pytest.raises(ocr.OcrError) as caught:
        photos.read_photos([("b.jpg", b"x")])
    assert caught.value.status == 503
    release.set()
    worker.join(5)


def test_a_busy_server_says_when_to_come_back(client, monkeypatch):
    def busy(pdf, indexes):
        raise ocr.OcrError(503, "Inkling is busy reading other scans right now. Please try again in a minute.")

    monkeypatch.setattr(ocr, "read_pages", busy)
    r = client.post("/summarize", files={"file": ("lease.pdf", b"%PDF", "application/pdf")}, data={"ocr": "true"})
    assert r.status_code == 503 and r.headers.get("retry-after") == "30" and "busy" in r.json()["detail"]
