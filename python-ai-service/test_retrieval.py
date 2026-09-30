from retrieval import select_context, tokenize


def test_short_text_is_returned_whole():
    assert select_context("tiny document", "anything", max_chars=1000) == "tiny document"


def test_long_text_selects_relevant_chunk_within_budget():
    filler = "Lorem ipsum dolor sit amet consectetur. " * 200
    needle = "The warranty period for the compressor is seven years from purchase."
    text = "\n\n".join([filler, filler, needle, filler, filler])

    context = select_context(text, "How long is the compressor warranty?", max_chars=3000)

    assert "seven years" in context
    assert len(context) <= 3000 + 50  # budget plus separators


def test_chunks_keep_document_order():
    a = "alpha unique token zebra. " * 80
    b = "filler words here. " * 120
    c = "gamma unique token zebra. " * 80
    context = select_context("\n\n".join([a, b, c]), "zebra token", max_chars=4000)
    assert context.index("alpha") < context.index("gamma")


def test_question_without_matches_still_returns_context():
    text = "some words " * 1000
    assert select_context(text, "qqqq", max_chars=2000)


def test_tokenize_drops_short_tokens_and_lowercases():
    assert tokenize("The AI is Big, ok?") == ["the", "big"]
