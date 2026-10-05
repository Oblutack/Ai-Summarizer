"""Prometheus metrics: HTTP traffic, language-model latency and outcomes, token usage, cache and
circuit-breaker state.

Labels come from a small fixed set (route patterns, model names, outcomes), never from user input,
so the number of time series stays bounded.
"""

import time
from collections.abc import Callable, Iterable
from typing import Any

from prometheus_client import CollectorRegistry, Counter, Histogram
from prometheus_client.core import CounterMetricFamily, GaugeMetricFamily
from prometheus_client.metrics_core import Metric
from prometheus_client.process_collector import ProcessCollector

# Separate from the global default registry, so only what we publish is exposed and tests that
# import this module repeatedly don't collide.
registry = CollectorRegistry()
ProcessCollector(registry=registry)

http_requests = Counter(
    "http_requests_total",
    "HTTP requests served, by method, route pattern and status code.",
    ["method", "route", "status"],
    registry=registry,
)
http_duration = Histogram(
    "http_request_duration_seconds",
    "Time to serve an HTTP request, by method and route pattern. Streams count until they end.",
    ["method", "route"],
    buckets=(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120),
    registry=registry,
)

llm_requests = Counter(
    "llm_requests_total",
    "Calls to the language model, by model and outcome (ok, model_missing, provider_error, error, circuit_open).",
    ["model", "outcome"],
    registry=registry,
)
llm_duration = Histogram(
    "llm_request_duration_seconds",
    "Language model call time, by model and kind (invoke = whole reply, stream = until the last token).",
    ["model", "kind"],
    buckets=(0.25, 0.5, 1, 2, 4, 8, 16, 32, 64, 128),
    registry=registry,
)
llm_tokens = Counter(
    "llm_tokens_total",
    "Tokens used, by model and type (prompt or completion), as reported by the provider.",
    ["model", "type"],
    registry=registry,
)


def record_llm(model: str, kind: str, outcome: str, started: float) -> None:
    """Records one finished (or failed) model call. `started` is a time.perf_counter() reading."""
    llm_requests.labels(model=model, outcome=outcome).inc()
    llm_duration.labels(model=model, kind=kind).observe(time.perf_counter() - started)


def record_rejected(model: str) -> None:
    """Counts a call that was refused up front because the circuit breaker is open."""
    llm_requests.labels(model=model, outcome="circuit_open").inc()


def record_usage(model: str, usage: Any) -> None:
    """Counts the tokens the provider says a call used. `usage` is LangChain's usage_metadata
    (a dict with input_tokens and output_tokens), or None when the provider did not report it."""
    if not isinstance(usage, dict):
        return
    for key, kind in (("input_tokens", "prompt"), ("output_tokens", "completion")):
        count = usage.get(key)
        if isinstance(count, int) and count > 0:
            llm_tokens.labels(model=model, type=kind).inc(count)


class StateCollector:
    """Reads values that already live elsewhere (cache counters, circuit breaker) when scraped,
    instead of instrumenting those modules."""

    def __init__(self, cache: Callable[[], Any], breaker: Callable[[], Any]):
        self._cache = cache
        self._breaker = breaker

    def collect(self) -> Iterable[Metric]:
        cache, breaker = self._cache(), self._breaker()

        events = CounterMetricFamily(
            "summary_cache_lookups", "Summary cache lookups so far, by result.", labels=["result"]
        )
        events.add_metric(["hit"], cache.hits)
        events.add_metric(["miss"], cache.misses)
        yield events

        size = GaugeMetricFamily("summary_cache_entries", "Summaries currently held in the cache.")
        size.add_metric([], len(cache))
        yield size

        state = GaugeMetricFamily(
            "llm_circuit_open", "1 while the circuit breaker is open and model calls fail fast, else 0."
        )
        state.add_metric([], 1 if breaker.is_open else 0)
        yield state


class MetricsMiddleware:
    """Counts and times every HTTP request. Plain ASGI, so streamed responses are timed until
    their last byte."""

    def __init__(self, app):
        self.app = app

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        start = time.perf_counter()
        status = 500  # what the client gets if the app dies before responding

        async def send_and_note(message):
            nonlocal status
            if message["type"] == "http.response.start":
                status = message["status"]
            await send(message)

        try:
            await self.app(scope, receive, send_and_note)
        finally:
            # FastAPI records the matched route in the scope; unknown URLs have none, so scanners
            # cannot create a time series per probed path.
            route = getattr(scope.get("route"), "path", None) or "unmatched"
            http_requests.labels(method=scope["method"], route=route, status=str(status)).inc()
            http_duration.labels(method=scope["method"], route=route).observe(time.perf_counter() - start)
