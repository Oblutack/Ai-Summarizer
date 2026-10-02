import asyncio

import pytest
from fastapi.testclient import TestClient

import llm
import main
from llm import CircuitBreaker, ServiceUnavailable, counts_as_outage


class Clock:
    def __init__(self):
        self.now = 100.0

    def __call__(self):
        return self.now


def test_opens_after_threshold_consecutive_failures():
    b = CircuitBreaker(threshold=3, cooldown=30, clock=Clock())
    for _ in range(2):
        b.failure()
        assert b.allow()
    b.failure()
    assert b.is_open and not b.allow()


def test_a_success_resets_the_failure_count():
    b = CircuitBreaker(threshold=3, cooldown=30, clock=Clock())
    b.failure()
    b.failure()
    b.success()
    b.failure()
    b.failure()
    assert not b.is_open


def test_half_open_lets_exactly_one_probe_through():
    clock = Clock()
    b = CircuitBreaker(threshold=1, cooldown=30, clock=clock)
    b.failure()
    clock.now += 29
    assert not b.allow()
    clock.now += 2
    assert b.allow(), "first request after the cooldown is the probe"
    assert not b.allow(), "others keep failing fast while the probe is in flight"


def test_successful_probe_closes_and_failed_probe_reopens():
    clock = Clock()
    b = CircuitBreaker(threshold=1, cooldown=30, clock=clock)
    b.failure()
    clock.now += 31
    assert b.allow()
    b.success()
    assert not b.is_open and b.allow()

    b.failure()
    clock.now += 31
    assert b.allow()
    b.failure()  # probe failed
    assert b.is_open and not b.allow()
    clock.now += 31
    assert b.allow(), "and the cooldown restarts, so another probe comes later"


@pytest.mark.parametrize(
    "status,outage",
    [(500, True), (502, True), (503, True), (429, True), (408, True),
     (400, False), (401, False), (403, False), (404, False), (422, False)],
)
def test_only_provider_side_statuses_count_as_outages(status, outage):
    exc = Exception("x")
    exc.status_code = status
    assert counts_as_outage(exc) is outage


def test_errors_without_a_status_count_as_outages():
    assert counts_as_outage(ConnectionError("down")) and counts_as_outage(TimeoutError())


class Failing:
    def __init__(self, status=None):
        self.calls, self.status = 0, status

    async def ainvoke(self, prompt):
        self.calls += 1
        exc = RuntimeError("provider error")
        if self.status is not None:
            exc.status_code = self.status
        raise exc

    async def astream(self, prompt):
        self.calls += 1
        raise RuntimeError("provider error")
        yield  # pragma: no cover


@pytest.fixture
def small_breaker(monkeypatch):
    b = CircuitBreaker(threshold=3, cooldown=30, clock=Clock())
    monkeypatch.setattr(llm, "breaker", b)
    monkeypatch.setattr(main, "breaker", b)
    return b


def test_call_llm_fails_fast_once_the_circuit_is_open(monkeypatch, small_breaker):
    fake = Failing(status=503)
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    for _ in range(3):
        with pytest.raises(RuntimeError):
            asyncio.run(main.call_llm("hi"))
    assert fake.calls == 3 and small_breaker.is_open

    with pytest.raises(ServiceUnavailable):
        asyncio.run(main.call_llm("hi"))
    assert fake.calls == 3, "an open circuit must not call the provider"


def test_client_errors_do_not_open_the_circuit(monkeypatch, small_breaker):
    monkeypatch.setattr(main, "get_llm", lambda: Failing(status=400))
    for _ in range(10):
        with pytest.raises(RuntimeError):
            asyncio.run(main.call_llm("hi"))
    assert not small_breaker.is_open


def test_streaming_failures_also_open_the_circuit(monkeypatch, small_breaker):
    monkeypatch.setattr(main, "get_llm", lambda: Failing())

    async def drain():
        async for _ in main.stream_llm("hi"):
            pass

    for _ in range(3):
        with pytest.raises(RuntimeError):
            asyncio.run(drain())
    with pytest.raises(ServiceUnavailable):
        asyncio.run(drain())


def test_api_returns_503_with_retry_after_when_open(monkeypatch, small_breaker):
    fake = Failing(status=503)
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    client = TestClient(main.app, raise_server_exceptions=False)
    for i in range(3):
        client.post("/summarize-text", json={"text": f"doc {i}"})  # distinct text, so none is cached

    r = client.post("/summarize-text", json={"text": "another doc"})
    assert r.status_code == 503 and r.headers["retry-after"] == "30"
    assert "busy" in r.json()["detail"]
    assert fake.calls == 3


def test_stream_reports_busy_as_an_error_event(monkeypatch, small_breaker):
    monkeypatch.setattr(main, "get_llm", lambda: Failing(status=503))
    client = TestClient(main.app, raise_server_exceptions=False)
    for i in range(3):
        client.post("/summarize-text", json={"text": f"doc {i}"})
    r = client.post("/summarize-text?stream=true", json={"text": "another doc"})
    assert '"status": 503' in r.text and "busy" in r.text


def test_cached_summaries_still_work_while_the_circuit_is_open(monkeypatch, small_breaker):
    class Ok:
        async def ainvoke(self, prompt):
            return type("M", (), {"content": "cached answer"})()

    monkeypatch.setattr(main, "get_llm", lambda: Ok())
    client = TestClient(main.app, raise_server_exceptions=False)
    body = {"text": "a document we already summarized"}
    assert client.post("/summarize-text", json=body).status_code == 200

    for _ in range(3):
        small_breaker.failure()
    assert small_breaker.is_open
    r = client.post("/summarize-text", json=body)
    assert r.status_code == 200 and r.json() == {"summary": "cached answer"}


def test_readyz_reports_an_open_circuit(monkeypatch, small_breaker):
    monkeypatch.setenv("GROQ_API_KEY", "k")

    async def ready():
        return "openai/gpt-oss-20b"

    monkeypatch.setattr(main, "verify_models", ready)
    client = TestClient(main.app)
    assert client.get("/readyz").status_code == 200
    for _ in range(3):
        small_breaker.failure()
    assert client.get("/readyz").status_code == 503
