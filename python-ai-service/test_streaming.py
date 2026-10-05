import json

import pytest
from fastapi.testclient import TestClient

import llm
import main


class ModelNotFound(Exception):
    code = "model_not_found"


def chunk(text):
    return type("Chunk", (), {"content": text})()


class StreamFake:
    """Stands in for ChatOpenAI: ainvoke for map steps, astream for the final answer."""

    def __init__(self, pieces=("Hello ", "world"), fail_before=None, fail_after=None):
        self.pieces = pieces
        self.fail_before = fail_before  # exception raised before the first chunk
        self.fail_after = fail_after  # exception raised after the first chunk
        self.prompts = []

    async def ainvoke(self, prompt):
        self.prompts.append(prompt)
        return chunk("chunk summary")

    async def astream(self, prompt):
        self.prompts.append(prompt)
        if self.fail_before:
            raise self.fail_before
        for i, piece in enumerate(self.pieces):
            yield chunk(piece)
            if i == 0 and self.fail_after:
                raise self.fail_after


@pytest.fixture(autouse=True)
def clean_llm_state(monkeypatch):
    monkeypatch.delenv("LLM_MODEL", raising=False)
    monkeypatch.delenv("LLM_FALLBACK_MODELS", raising=False)
    llm.reset_unavailable()
    yield
    llm.reset_unavailable()


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def events(response):
    assert response.headers["content-type"].startswith("text/event-stream")
    out = []
    for block in response.text.split("\n\n"):
        if block.strip():
            assert block.startswith("data: "), block
            out.append(json.loads(block[len("data: ") :]))
    return out


def deltas(evts):
    return "".join(e["text"] for e in evts if e["type"] == "delta")


def test_text_stream_emits_status_deltas_and_done(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake())
    r = client.post("/summarize-text?stream=true", json={"text": "some text"})
    evts = events(r)

    assert evts[0] == {"type": "status", "stage": "preparing"}
    assert {"type": "status", "stage": "writing"} in evts
    assert deltas(evts) == "Hello world"
    assert evts[-1] == {"type": "done"}
    assert evts.index({"type": "status", "stage": "writing"}) < next(
        i for i, e in enumerate(evts) if e["type"] == "delta"
    )


def test_stream_and_plain_responses_use_the_same_prompt(client, monkeypatch):
    fake = StreamFake()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    client.post("/summarize-text?stream=true&word_count=120&style=bullets", json={"text": "some text"})
    main.summary_cache.clear()  # otherwise the second request is served from the cache
    client.post("/summarize-text?word_count=120&style=bullets", json={"text": "some text"})
    assert fake.prompts[0] == fake.prompts[1]


def test_long_text_reports_map_progress(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake())
    r = client.post("/summarize-text?stream=true&page_limit=1", json={"text": "word " * 5000})
    progress = [e for e in events(r) if e.get("stage") == "summarizing"]

    assert progress, "expected per-chunk progress events"
    assert all(1 <= e["done"] <= e["total"] for e in progress)
    assert progress[-1]["done"] == progress[-1]["total"]


def test_file_stream_done_event_carries_filename_and_source_text(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake())
    monkeypatch.setattr(main, "extract_pdf_text", lambda path: "extracted body")
    r = client.post("/summarize?stream=true", files={"file": ("a.pdf", b"x", "application/pdf")})
    done = events(r)[-1]
    assert done == {"type": "done", "filename": "a.pdf", "text": "extracted body"}


def test_multi_stream_done_event(client, monkeypatch):
    texts = iter(["alpha", "beta"])
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake())
    monkeypatch.setattr(main, "extract_pdf_text", lambda path: next(texts))
    files = [("files", ("a.pdf", b"x", "application/pdf")), ("files", ("b.pdf", b"x", "application/pdf"))]
    done = events(client.post("/summarize-multiple?stream=true", files=files))[-1]
    assert done["type"] == "done" and done["filename"] == "a.pdf, b.pdf"
    assert "=== a.pdf ===\nalpha" in done["text"]


def test_validation_errors_are_plain_http_errors_not_streams(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake())
    assert client.post("/summarize-text?stream=true", json={"text": "  "}).status_code == 422
    assert client.post("/summarize-text?stream=true&style=poem", json={"text": "hi"}).status_code == 422
    big = {"text": "a" * (main.MAX_TEXT_CHARS + 1)}
    assert client.post("/summarize-text?stream=true", json=big).status_code == 413


def test_failure_before_any_output_becomes_an_error_event_without_details(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake(fail_before=RuntimeError("secret provider detail")))
    evts = events(client.post("/summarize-text?stream=true", json={"text": "hi"}))
    assert evts[-1]["type"] == "error" and evts[-1]["status"] == 502
    assert "secret" not in json.dumps(evts)
    assert not any(e["type"] == "done" for e in evts)


def test_failure_midway_keeps_partial_text_but_ends_with_error(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake(fail_after=RuntimeError("dropped")))
    evts = events(client.post("/summarize-text?stream=true", json={"text": "hi"}))
    assert deltas(evts) == "Hello "
    assert evts[-1]["type"] == "error"


def test_empty_model_output_is_an_error_event(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake(pieces=("", "")))
    evts = events(client.post("/summarize-text?stream=true", json={"text": "hi"}))
    assert evts[-1]["type"] == "error"


def test_stream_falls_back_when_model_is_retired_before_output(client, monkeypatch):
    monkeypatch.setenv("LLM_MODEL", "retired-model")
    monkeypatch.setenv("LLM_FALLBACK_MODELS", "good-model")

    def get():
        return StreamFake(fail_before=ModelNotFound("gone")) if llm.active_model() == "retired-model" else StreamFake()

    monkeypatch.setattr(main, "get_llm", get)
    evts = events(client.post("/summarize-text?stream=true", json={"text": "hi"}))
    assert deltas(evts) == "Hello world" and evts[-1] == {"type": "done"}
    assert llm.active_model() == "good-model"


def test_non_ascii_text_is_not_escaped_awkwardly(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: StreamFake(pieces=("Čšž ", "日本")))
    r = client.post("/summarize-text?stream=true", json={"text": "hi"})
    assert "Čšž" in r.text and deltas(events(r)) == "Čšž 日本"
