import json
import logging

import pytest
from fastapi.testclient import TestClient

import logging_setup
import main
from logging_setup import JsonFormatter, TextFormatter, request_id_var, resolve_request_id


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


class Capture(logging.Handler):
    """Formats each record at emit time, while the request's id is still in the context."""

    def __init__(self, formatter):
        super().__init__(logging.DEBUG)
        self.setFormatter(formatter)
        self.lines = []

    def emit(self, record):
        self.lines.append(self.format(record))


@pytest.fixture
def logs():
    handler = Capture(JsonFormatter())
    root = logging.getLogger()
    root.addHandler(handler)
    old_level = root.level
    root.setLevel(logging.DEBUG)
    yield [handler]
    root.removeHandler(handler)
    root.setLevel(old_level)


def entries(handler):
    return [json.loads(line) for line in handler.lines]


def access_lines(handler):
    return [e for e in entries(handler) if e["logger"] == "ai-summarizer.access"]


def test_every_response_carries_a_request_id(client):
    r = client.get("/healthz")
    assert len(r.headers["x-request-id"]) == 16
    assert client.get("/healthz").headers["x-request-id"] != r.headers["x-request-id"]


def test_well_formed_incoming_id_is_reused(client):
    r = client.get("/healthz", headers={"X-Request-ID": "trace-abc_123.xyz"})
    assert r.headers["x-request-id"] == "trace-abc_123.xyz"


@pytest.mark.parametrize("bad", ["short", "has spaces in it", "x" * 200, "<script>alert(1)</script>"])
def test_unsafe_incoming_ids_are_replaced(client, bad):
    got = client.get("/healthz", headers={"X-Request-ID": bad}).headers["x-request-id"]
    assert got != bad and len(got) == 16


def test_resolve_request_id():
    assert resolve_request_id("valid-id-12345") == "valid-id-12345"
    assert resolve_request_id(None) != resolve_request_id(None)
    assert len(resolve_request_id("no")) == 16


def test_access_line_has_the_expected_fields(client, logs):
    client.get("/healthz", headers={"X-Request-ID": "req-12345678"})
    [line] = access_lines(logs[0])
    assert line["msg"] == "request" and line["request_id"] == "req-12345678"
    assert line["method"] == "GET" and line["path"] == "/healthz" and line["status"] == 200
    assert line["level"] == "INFO" and line["service"] == "ai-service"
    assert isinstance(line["duration_ms"], int) and "time" in line


def test_access_line_never_includes_the_query_string_or_body(client, logs, monkeypatch):
    monkeypatch.setattr(main, "get_llm", lambda: type("L", (), {"ainvoke": staticmethod(lambda p: None)})())
    client.post("/summarize-text?word_count=100&secret=TOPSECRET", json={"text": "CONFIDENTIAL DOCUMENT TEXT"})
    # httpx (the test client) logs its own outgoing URL; only the service's own loggers matter here.
    app_lines = [json.dumps(e) for e in entries(logs[0]) if not e["logger"].startswith("httpx")]
    assert app_lines, "the service should have logged something"
    blob = "\n".join(app_lines)
    assert "TOPSECRET" not in blob and "CONFIDENTIAL" not in blob


def test_access_levels_follow_the_status(client, logs):
    client.get("/healthz")  # 200
    client.get("/does-not-exist")  # 404
    client.post("/summarize-text", json={"text": "   "})  # 422
    levels = {(e["path"], e["status"]): e["level"] for e in access_lines(logs[0])}
    assert levels[("/healthz", 200)] == "INFO"
    assert levels[("/does-not-exist", 404)] == "WARNING"
    assert levels[("/summarize-text", 422)] == "WARNING"


def test_server_errors_are_logged_at_error_level(client, logs, monkeypatch):
    class Boom:
        async def ainvoke(self, prompt):
            raise RuntimeError("provider exploded")

    monkeypatch.setattr(main, "get_llm", lambda: Boom())
    r = client.post("/summarize-text", json={"text": "some text"})
    assert r.status_code == 502
    [line] = access_lines(logs[0])
    assert line["level"] == "ERROR" and line["status"] == 502


def test_application_logs_carry_the_request_id_and_the_traceback(client, logs, monkeypatch):
    class Boom:
        async def ainvoke(self, prompt):
            raise RuntimeError("provider exploded")

    monkeypatch.setattr(main, "get_llm", lambda: Boom())
    r = client.post("/summarize-text", json={"text": "some text"}, headers={"X-Request-ID": "req-abcdefgh"})

    failure = [e for e in entries(logs[0]) if e["msg"] == "Request failed"]
    assert failure, "the handler should have logged the failure"
    assert failure[0]["request_id"] == "req-abcdefgh" == r.headers["x-request-id"]
    assert "provider exploded" in failure[0]["exception"]
    assert "provider exploded" not in r.text, "but the client never sees internals"


def test_streaming_requests_get_the_id_header_and_one_access_line(client, logs, monkeypatch):
    class Streamer:
        async def astream(self, prompt):
            for piece in ("a", "b"):
                yield type("M", (), {"content": piece})()

    monkeypatch.setattr(main, "get_llm", lambda: Streamer())
    r = client.post(
        "/summarize-text?stream=true", json={"text": "some text"}, headers={"X-Request-ID": "req-stream-1234"}
    )
    assert r.headers["x-request-id"] == "req-stream-1234"
    [line] = access_lines(logs[0])
    assert line["status"] == 200 and line["request_id"] == "req-stream-1234"


def test_request_id_is_cleared_after_the_request(client):
    client.get("/healthz", headers={"X-Request-ID": "req-12345678"})
    assert request_id_var.get() == "-"


def test_json_formatter_output_is_valid_json_with_unicode_and_exceptions():
    record = logging.LogRecord("ai-summarizer", logging.ERROR, __file__, 1, "Čšž 日本 failed", None, None)
    try:
        raise ValueError("bad")
    except ValueError:
        import sys

        record.exc_info = sys.exc_info()
    token = request_id_var.set("req-unicode-1")
    try:
        out = json.loads(JsonFormatter().format(record))
    finally:
        request_id_var.reset(token)
    assert out["msg"] == "Čšž 日本 failed" and out["request_id"] == "req-unicode-1"
    assert "ValueError: bad" in out["exception"]


def test_text_formatter_includes_the_request_id():
    record = logging.LogRecord("ai-summarizer", logging.INFO, __file__, 1, "hello", None, None)
    token = request_id_var.set("req-text-12345")
    try:
        assert "[req-text-12345]" in TextFormatter().format(record)
    finally:
        request_id_var.reset(token)


def test_configure_logging_honors_format_and_level(monkeypatch):
    root = logging.getLogger()
    saved_handlers, saved_level = root.handlers[:], root.level
    try:
        monkeypatch.setenv("LOG_FORMAT", "text")
        monkeypatch.setenv("LOG_LEVEL", "warning")
        logging_setup.configure_logging()
        assert isinstance(root.handlers[0].formatter, TextFormatter) and root.level == logging.WARNING

        monkeypatch.setenv("LOG_FORMAT", "json")
        monkeypatch.setenv("LOG_LEVEL", "debug")
        logging_setup.configure_logging()
        assert isinstance(root.handlers[0].formatter, JsonFormatter) and root.level == logging.DEBUG
    finally:
        root.handlers, root.level = saved_handlers, saved_level
