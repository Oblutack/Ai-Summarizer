import asyncio

import pytest
from fastapi.testclient import TestClient

import main


class FakeLLM:
    def __init__(self, reply="summary"):
        self.reply = reply
        self.prompts = []
        self.active = 0
        self.max_active = 0

    async def ainvoke(self, prompt):
        self.prompts.append(prompt)
        self.active += 1
        self.max_active = max(self.max_active, self.active)
        await asyncio.sleep(0.01)
        self.active -= 1
        return type("Msg", (), {"content": self.reply})()


@pytest.fixture
def llm(monkeypatch):
    fake = FakeLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    return fake


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def test_short_text_uses_single_prompt(llm):
    result = asyncio.run(main.process_summary("hello world", 100, 0))
    assert result == "summary"
    assert len(llm.prompts) == 1
    assert "about 100 words" in llm.prompts[0]


def test_page_limit_uses_map_reduce(llm):
    text = "word " * 5000  # several chunks
    asyncio.run(main.process_summary(text, 150, 2))
    assert len(llm.prompts) > 2
    assert "about 500 words" in llm.prompts[-1]


def test_long_text_without_page_limit_still_maps(llm):
    text = "word " * (main.SINGLE_SHOT_MAX_CHARS // 5 + 1000)
    asyncio.run(main.process_summary(text, 200, 0))
    assert len(llm.prompts) > 1
    assert "about 200 words" in llm.prompts[-1]


def test_llm_concurrency_is_capped(llm):
    asyncio.run(main.process_summary("word " * 20000, 150, 1))
    assert llm.max_active <= main.MAX_CONCURRENT_LLM_CALLS


def test_empty_model_reply_raises(monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: FakeLLM(reply="   "))
    with pytest.raises(ValueError):
        asyncio.run(main.process_summary("hello", 100, 0))


def test_summarize_text_endpoint(client, llm):
    r = client.post("/summarize-text?word_count=100", json={"text": "hello"})
    assert r.status_code == 200
    assert r.json() == {"summary": "summary"}


@pytest.mark.parametrize("query", ["word_count=5", "word_count=5000", "page_limit=-1", "page_limit=99"])
def test_invalid_options_rejected(client, llm, query):
    assert client.post(f"/summarize-text?{query}", json={"text": "hello"}).status_code == 422


def test_blank_and_oversized_text_rejected(client, llm):
    assert client.post("/summarize-text", json={"text": "  "}).status_code == 422
    too_long = {"text": "a" * (main.MAX_TEXT_CHARS + 1)}
    assert client.post("/summarize-text", json=too_long).status_code == 413


def test_llm_failure_is_hidden_from_client(client, monkeypatch):
    def boom():
        raise RuntimeError("secret api details")

    monkeypatch.setattr(main, "get_llm", boom)
    r = client.post("/summarize-text", json={"text": "hello"})
    assert r.status_code == 502
    assert "secret" not in r.text


def test_oversized_pdf_rejected(client, llm):
    big = b"%PDF-" + b"0" * (main.MAX_PDF_BYTES + 1)
    r = client.post("/summarize", files={"file": ("big.pdf", big, "application/pdf")})
    assert r.status_code == 413


def test_invalid_pdf_returns_422(client, llm):
    r = client.post("/summarize", files={"file": ("bad.pdf", b"not a pdf", "application/pdf")})
    assert r.status_code == 422
