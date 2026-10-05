import pytest
from fastapi.testclient import TestClient
from prometheus_client.parser import text_string_to_metric_families

import llm
import main

TOKEN = "test-metrics-token"
AUTH = {"Authorization": f"Bearer {TOKEN}"}


class ProviderDown(Exception):
    status_code = 503


class ModelNotFound(Exception):
    code = "model_not_found"


class BadRequest(Exception):
    status_code = 400


def message(text="a summary", usage=None):
    return type("Msg", (), {"content": text, "usage_metadata": usage})()


class Fake:
    """Stands in for ChatOpenAI for both ainvoke and astream."""

    def __init__(self, reply="a summary", usage=None, error=None, pieces=("Hello ", "world")):
        self.reply, self.usage, self.error, self.pieces = reply, usage, error, pieces

    async def ainvoke(self, prompt):
        if self.error:
            raise self.error
        return message(self.reply, self.usage)

    async def astream(self, prompt):
        if self.error:
            raise self.error
        for i, piece in enumerate(self.pieces):
            last = i == len(self.pieces) - 1
            yield message(piece, self.usage if last else None)


@pytest.fixture
def client(monkeypatch):
    monkeypatch.setenv("METRICS_TOKEN", TOKEN)
    return TestClient(main.app, raise_server_exceptions=False)


def scrape(client):
    """Every series as {(name, labels-as-tuple): value}."""
    response = client.get("/metrics", headers=AUTH)
    assert response.status_code == 200
    series = {}
    for family in text_string_to_metric_families(response.text):
        for sample in family.samples:
            series[(sample.name, tuple(sorted(sample.labels.items())))] = sample.value
    return series


def value(series, name, **labels):
    return series.get((name, tuple(sorted(labels.items()))), 0.0)


def changed(before, after, name, **labels):
    return value(after, name, **labels) - value(before, name, **labels)


# ---- the endpoint ---------------------------------------------------------------------------


def test_metrics_are_off_unless_a_token_is_configured(monkeypatch):
    monkeypatch.delenv("METRICS_TOKEN", raising=False)
    assert TestClient(main.app).get("/metrics").status_code == 404


@pytest.mark.parametrize(
    "headers",
    [{}, {"Authorization": "Bearer nope"}, {"Authorization": TOKEN}, {"Authorization": "Bearer "}],
)
def test_metrics_need_the_bearer_token(client, headers):
    response = client.get("/metrics", headers=headers)
    assert response.status_code == 401
    assert response.headers["www-authenticate"] == "Bearer"


def test_metrics_are_served_with_the_token(client):
    response = client.get("/metrics", headers=AUTH)
    assert response.status_code == 200
    assert response.headers["content-type"].startswith("text/plain")


# ---- HTTP -----------------------------------------------------------------------------------


def test_requests_are_counted_by_route_pattern_not_by_url(client):
    before = scrape(client)
    client.get("/healthz")
    client.get("/some/scanner/probe/zq81x")
    after = scrape(client)

    assert changed(before, after, "http_requests_total", method="GET", route="/healthz", status="200") == 1
    assert changed(before, after, "http_requests_total", method="GET", route="unmatched", status="404") == 1
    assert not any("zq81x" in str(key) for key in after), "a raw URL leaked into a label"


# ---- the language model ---------------------------------------------------------------------


def test_a_successful_call_records_latency_and_tokens(client, monkeypatch):
    fake = Fake(usage={"input_tokens": 120, "output_tokens": 30, "total_tokens": 150})
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    model = llm.active_model()

    before = scrape(client)
    assert client.post("/summarize-text", json={"text": "some text to summarize"}).status_code == 200
    after = scrape(client)

    assert changed(before, after, "llm_requests_total", model=model, outcome="ok") == 1
    assert changed(before, after, "llm_request_duration_seconds_count", model=model, kind="invoke") == 1
    assert changed(before, after, "llm_tokens_total", model=model, type="prompt") == 120
    assert changed(before, after, "llm_tokens_total", model=model, type="completion") == 30


def test_streamed_calls_count_tokens_from_the_final_chunk(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: Fake(usage={"input_tokens": 50, "output_tokens": 7}))
    model = llm.active_model()

    before = scrape(client)
    client.post("/summarize-text?stream=true", json={"text": "some text to summarize"})
    after = scrape(client)

    assert changed(before, after, "llm_requests_total", model=model, outcome="ok") == 1
    assert changed(before, after, "llm_request_duration_seconds_count", model=model, kind="stream") == 1
    assert changed(before, after, "llm_tokens_total", model=model, type="prompt") == 50
    assert changed(before, after, "llm_tokens_total", model=model, type="completion") == 7


def test_missing_usage_is_not_an_error(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: Fake(usage=None))
    model = llm.active_model()
    before = scrape(client)
    assert client.post("/summarize-text", json={"text": "some text"}).status_code == 200
    after = scrape(client)
    assert changed(before, after, "llm_tokens_total", model=model, type="prompt") == 0


@pytest.mark.parametrize(
    ("error", "outcome"),
    [(ProviderDown(), "provider_error"), (BadRequest(), "error")],
)
def test_failures_are_classified(client, monkeypatch, error, outcome):
    monkeypatch.setattr(main, "get_llm", lambda: Fake(error=error))
    model = llm.active_model()
    before = scrape(client)
    assert client.post("/summarize-text", json={"text": "some text"}).status_code >= 400
    after = scrape(client)
    assert changed(before, after, "llm_requests_total", model=model, outcome=outcome) == 1


def test_a_retired_model_is_recorded_before_falling_back(client, monkeypatch):
    calls = []

    def get():
        calls.append(llm.active_model())
        return Fake(error=ModelNotFound()) if len(calls) == 1 else Fake()

    monkeypatch.setattr(main, "get_llm", get)
    retired = llm.active_model()
    before = scrape(client)
    assert client.post("/summarize-text", json={"text": "some text"}).status_code == 200
    after = scrape(client)
    assert changed(before, after, "llm_requests_total", model=retired, outcome="model_missing") == 1
    assert changed(before, after, "llm_requests_total", model=llm.active_model(), outcome="ok") == 1


def test_calls_refused_by_an_open_circuit_are_counted_and_the_state_is_visible(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: Fake(error=ProviderDown()))
    for _ in range(llm.breaker.threshold):
        client.post("/summarize-text", json={"text": "some text"})
    model = llm.active_model()

    before = scrape(client)
    assert client.post("/summarize-text", json={"text": "some text"}).status_code == 503
    after = scrape(client)

    assert changed(before, after, "llm_requests_total", model=model, outcome="circuit_open") == 1
    assert value(after, "llm_circuit_open") == 1.0


# ---- cache and breaker state ------------------------------------------------------------------


def test_cache_hits_and_misses_are_exposed(client, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: Fake())
    before = scrape(client)
    body = {"text": "the very same text twice"}
    client.post("/summarize-text", json=body)
    client.post("/summarize-text", json=body)
    after = scrape(client)

    assert changed(before, after, "summary_cache_lookups_total", result="miss") == 1
    assert changed(before, after, "summary_cache_lookups_total", result="hit") == 1
    assert value(after, "summary_cache_entries") == 1.0
    assert value(after, "llm_circuit_open") == 0.0


def test_a_client_that_disconnects_mid_stream_is_recorded_as_cancelled(client, monkeypatch):
    import asyncio

    monkeypatch.setattr(main, "get_llm", lambda: Fake(pieces=("one ", "two ", "three")))
    model = llm.active_model()
    before = scrape(client)

    async def read_one_piece_then_hang_up():
        stream = main.stream_llm("prompt")
        assert await stream.__anext__() == "one "
        await stream.aclose()  # what happens when the HTTP client goes away

    asyncio.run(read_one_piece_then_hang_up())
    after = scrape(client)
    assert changed(before, after, "llm_requests_total", model=model, outcome="cancelled") == 1
    assert changed(before, after, "llm_requests_total", model=model, outcome="ok") == 0
