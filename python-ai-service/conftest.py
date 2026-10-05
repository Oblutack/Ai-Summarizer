import pytest

import llm
import main


@pytest.fixture(autouse=True)
def isolated_state(monkeypatch):
    """Each test starts with a closed circuit breaker, no retired models and an empty summary cache,
    so state from one test can never leak into another."""
    monkeypatch.delenv("LLM_MODEL", raising=False)
    monkeypatch.delenv("LLM_FALLBACK_MODELS", raising=False)
    llm.reset_unavailable()
    main.summary_cache.clear()
    yield
    llm.reset_unavailable()
    main.summary_cache.clear()
