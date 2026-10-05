import asyncio

import pytest
from fastapi.testclient import TestClient

import llm
import main


class ModelNotFound(Exception):
    code = "model_not_found"


class FakeClient:
    def __init__(self, model, calls):
        self.model, self.calls = model, calls

    async def ainvoke(self, prompt):
        self.calls.append(self.model)
        if self.model == "retired-model":
            raise ModelNotFound("The model does not exist")
        return type("Msg", (), {"content": f"answer from {self.model}"})()


@pytest.fixture(autouse=True)
def clean_llm_state(monkeypatch):
    monkeypatch.delenv("LLM_MODEL", raising=False)
    monkeypatch.delenv("LLM_FALLBACK_MODELS", raising=False)
    llm.reset_unavailable()
    yield
    llm.reset_unavailable()


def test_candidates_put_primary_first_and_drop_duplicates(monkeypatch):
    monkeypatch.setenv("LLM_MODEL", "a")
    monkeypatch.setenv("LLM_FALLBACK_MODELS", "b, a ,c,")
    assert llm.candidate_models() == ["a", "b", "c"]


def test_default_candidates():
    assert llm.candidate_models() == ["openai/gpt-oss-20b", "openai/gpt-oss-120b"]


def test_unavailable_models_are_skipped(monkeypatch):
    monkeypatch.setenv("LLM_MODEL", "a")
    monkeypatch.setenv("LLM_FALLBACK_MODELS", "b")
    llm.mark_unavailable("a")
    assert llm.active_model() == "b"
    llm.mark_unavailable("b")
    with pytest.raises(RuntimeError):
        llm.active_model()


def test_call_llm_falls_back_when_model_is_retired(monkeypatch):
    monkeypatch.setenv("LLM_MODEL", "retired-model")
    monkeypatch.setenv("LLM_FALLBACK_MODELS", "good-model")
    calls = []
    monkeypatch.setattr(main, "get_llm", lambda: FakeClient(llm.active_model(), calls))

    assert asyncio.run(main.call_llm("hi")) == "answer from good-model"
    assert calls == ["retired-model", "good-model"]
    # later calls go straight to the working model
    asyncio.run(main.call_llm("again"))
    assert calls[-1] == "good-model" and calls.count("retired-model") == 1


def test_call_llm_does_not_swallow_other_errors(monkeypatch):
    class Boom:
        async def ainvoke(self, prompt):
            raise ValueError("rate limited or something else")

    monkeypatch.setattr(main, "get_llm", lambda: Boom())
    with pytest.raises(ValueError):
        asyncio.run(main.call_llm("hi"))
    assert llm.active_model() == "openai/gpt-oss-20b"  # not marked unavailable


def test_call_llm_fails_when_every_model_is_retired(monkeypatch):
    monkeypatch.setenv("LLM_MODEL", "retired-model")
    monkeypatch.setenv("LLM_FALLBACK_MODELS", "retired-model")
    monkeypatch.setattr(main, "get_llm", lambda: FakeClient("retired-model", []))
    with pytest.raises(RuntimeError):
        asyncio.run(main.call_llm("hi"))


@pytest.mark.parametrize(
    "available,expected_active",
    [
        ({"openai/gpt-oss-20b", "openai/gpt-oss-120b"}, "openai/gpt-oss-20b"),
        ({"openai/gpt-oss-120b"}, "openai/gpt-oss-120b"),
    ],
)
def test_verify_models_picks_first_available(monkeypatch, available, expected_active):
    async def fake_list():
        return available

    monkeypatch.setattr(llm, "list_models", fake_list)
    assert asyncio.run(llm.verify_models()) == expected_active


def test_verify_models_returns_none_when_all_retired(monkeypatch):
    async def fake_list():
        return {"something-else"}

    monkeypatch.setattr(llm, "list_models", fake_list)
    assert asyncio.run(llm.verify_models()) is None


def test_provider_outage_does_not_mark_models_unavailable(monkeypatch):
    async def broken_list():
        raise ConnectionError("provider down")

    monkeypatch.setattr(llm, "list_models", broken_list)
    assert asyncio.run(llm.verify_models()) is None
    assert llm.active_model() == "openai/gpt-oss-20b"


def test_healthz_is_always_ok_even_without_a_key(monkeypatch):
    monkeypatch.delenv("GROQ_API_KEY", raising=False)
    assert TestClient(main.app).get("/healthz").json() == {"status": "ok"}


def test_readyz_requires_key_and_a_live_model(monkeypatch):
    client = TestClient(main.app)

    monkeypatch.delenv("GROQ_API_KEY", raising=False)
    assert client.get("/readyz").status_code == 503

    monkeypatch.setenv("GROQ_API_KEY", "test-key")

    async def ready():
        return "openai/gpt-oss-20b"

    async def not_ready():
        return None

    monkeypatch.setattr(main, "verify_models", ready)
    r = client.get("/readyz")
    assert r.status_code == 200 and r.json() == {"status": "ready", "model": "openai/gpt-oss-20b"}

    monkeypatch.setattr(main, "verify_models", not_ready)
    assert client.get("/readyz").status_code == 503


def test_content_text_handles_strings_and_content_blocks():
    from main import content_text

    assert content_text("  hello ") == "  hello "
    assert content_text(None) == ""
    assert content_text([]) == ""
    assert content_text(["a", {"type": "text", "text": "b"}, {"type": "image"}, {"text": 3}]) == "ab"
