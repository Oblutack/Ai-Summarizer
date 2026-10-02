import json

import pytest
from fastapi.testclient import TestClient

import main
from cache import SummaryCache, summary_key


class Clock:
    def __init__(self):
        self.now = 1000.0

    def __call__(self):
        return self.now


def test_get_and_put():
    c = SummaryCache(max_entries=4, ttl_seconds=60)
    assert c.get("k") is None
    c.put("k", "summary")
    assert c.get("k") == "summary"
    assert (c.hits, c.misses) == (1, 1)


def test_entries_expire():
    clock = Clock()
    c = SummaryCache(max_entries=4, ttl_seconds=60, clock=clock)
    c.put("k", "summary")
    clock.now += 59
    assert c.get("k") == "summary"
    clock.now += 2
    assert c.get("k") is None and len(c) == 0


def test_least_recently_used_entry_is_evicted():
    c = SummaryCache(max_entries=2, ttl_seconds=60)
    c.put("a", "1")
    c.put("b", "2")
    c.get("a")  # a is now more recent than b
    c.put("c", "3")
    assert c.get("b") is None and c.get("a") == "1" and c.get("c") == "3"


def test_disabled_cache_stores_nothing():
    for c in (SummaryCache(max_entries=0, ttl_seconds=60), SummaryCache(max_entries=4, ttl_seconds=0)):
        c.put("k", "v")
        assert c.get("k") is None and len(c) == 0


def test_empty_and_oversized_values_are_not_stored():
    c = SummaryCache(max_entries=4, ttl_seconds=60)
    c.put("empty", "")
    c.put("huge", "x" * 200_000)
    assert len(c) == 0


def test_key_changes_with_every_input():
    base = dict(kind="text", material="hello", target_words=100, style="default", language="English", model="m", prompt_version="1")
    keys = {summary_key(**base)}
    for field, value in [("kind", "docs"), ("material", "hello!"), ("target_words", 101), ("style", "bullets"),
                         ("language", "German"), ("model", "m2"), ("prompt_version", "2")]:
        keys.add(summary_key(**{**base, field: value}))
    assert len(keys) == 8, "every option must change the key"
    assert summary_key(**base) == summary_key(**base), "and the key must be stable"


def test_key_does_not_contain_the_text():
    assert "secret document" not in summary_key("text", "secret document", 100, "default", "English", "m", "1")


def test_field_boundaries_cannot_collide():
    a = summary_key("text", "x", 100, "ab", "c", "m", "1")
    b = summary_key("text", "x", 100, "a", "bc", "m", "1")
    assert a != b


# ---- through the API -------------------------------------------------------------------------


class CountingLLM:
    def __init__(self, reply="the summary"):
        self.reply, self.calls = reply, 0

    async def ainvoke(self, prompt):
        self.calls += 1
        return type("M", (), {"content": self.reply})()

    async def astream(self, prompt):
        self.calls += 1
        for piece in ("the ", "summary"):
            yield type("M", (), {"content": piece})()


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def stream_text(response):
    return "".join(
        json.loads(b[6:])["text"] for b in response.text.split("\n\n") if b.startswith("data: ") and '"delta"' in b
    )


def test_repeating_a_request_skips_the_llm(client, monkeypatch):
    fake = CountingLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    body = {"text": "some document text"}

    first = client.post("/summarize-text?word_count=100", json=body).json()
    second = client.post("/summarize-text?word_count=100", json=body).json()

    assert first == second == {"summary": "the summary"}
    assert fake.calls == 1


@pytest.mark.parametrize("query", ["word_count=200", "style=bullets", "language=German", "page_limit=1"])
def test_different_options_are_not_served_from_cache(client, monkeypatch, query):
    fake = CountingLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    body = {"text": "some document text"}
    client.post("/summarize-text?word_count=100", json=body)
    before = fake.calls
    client.post(f"/summarize-text?{query}", json=body)
    assert fake.calls > before


def test_different_text_is_not_served_from_cache(client, monkeypatch):
    fake = CountingLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    client.post("/summarize-text", json={"text": "document one"})
    client.post("/summarize-text", json={"text": "document two"})
    assert fake.calls == 2


def test_streamed_summary_is_cached_and_replayed(client, monkeypatch):
    fake = CountingLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    body = {"text": "some document text"}

    first = client.post("/summarize-text?stream=true", json=body)
    second = client.post("/summarize-text?stream=true", json=body)

    assert stream_text(first) == stream_text(second) == "the summary"
    assert fake.calls == 1
    assert second.text.rstrip().endswith('"type": "done"}')


def test_stream_and_plain_requests_share_the_cache(client, monkeypatch):
    fake = CountingLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    body = {"text": "some document text"}
    client.post("/summarize-text?stream=true", json=body)
    assert client.post("/summarize-text", json=body).json() == {"summary": "the summary"}
    assert fake.calls == 1


def test_cached_file_summary_still_returns_the_source_text(client, monkeypatch):
    fake = CountingLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    monkeypatch.setattr(main, "extract_pdf_text", lambda path: "extracted body")
    files = {"file": ("a.pdf", b"x", "application/pdf")}
    client.post("/summarize", files=files)
    second = client.post("/summarize", files=files).json()
    assert fake.calls == 1
    assert second == {"filename": "a.pdf", "summary": "the summary", "text": "extracted body"}


def test_multi_document_summaries_are_cached(client, monkeypatch):
    fake = CountingLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    monkeypatch.setattr(main, "extract_pdf_text", lambda path: "same text")
    files = [("files", ("a.pdf", b"x", "application/pdf")), ("files", ("b.pdf", b"x", "application/pdf"))]
    client.post("/summarize-multiple", files=files)
    client.post("/summarize-multiple", files=files)
    assert fake.calls == 1


def test_failed_summaries_are_never_cached(client, monkeypatch):
    class Failing(CountingLLM):
        async def ainvoke(self, prompt):
            self.calls += 1
            raise RuntimeError("boom")

    fake = Failing()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    body = {"text": "some document text"}
    assert client.post("/summarize-text", json=body).status_code == 502
    assert client.post("/summarize-text", json=body).status_code == 502
    assert fake.calls == 2 and len(main.summary_cache) == 0


def test_empty_model_output_is_not_cached(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: CountingLLM(reply="   "))
    client.post("/summarize-text", json={"text": "some document text"})
    assert len(main.summary_cache) == 0
