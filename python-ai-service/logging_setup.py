"""Structured logging and per-request ids for the AI service.

Every log line carries the id of the request that caused it. The Go API forwards its own id in the
X-Request-ID header, so one id finds the whole story across both services.
"""
import json
import logging
import os
import re
import secrets
import sys
import time
from contextvars import ContextVar
from typing import Optional

REQUEST_ID_HEADER = "x-request-id"

# Accept ids from callers only if they are short and boring, so they are safe to log and echo.
_VALID_REQUEST_ID = re.compile(r"^[A-Za-z0-9._-]{8,64}$")

request_id_var: ContextVar[str] = ContextVar("request_id", default="-")

access_logger = logging.getLogger("ai-summarizer.access")


class JsonFormatter(logging.Formatter):
    """One JSON object per line, ready for a log aggregator."""

    def format(self, record: logging.LogRecord) -> str:
        entry = {
            "time": self.formatTime(record, "%Y-%m-%dT%H:%M:%S") + f".{int(record.msecs):03d}Z",
            "level": record.levelname,
            "service": "ai-service",
            "logger": record.name,
            "msg": record.getMessage(),
            "request_id": request_id_var.get(),
        }
        # Structured fields passed via `extra={"fields": {...}}`
        entry.update(getattr(record, "fields", {}) or {})
        if record.exc_info:
            entry["exception"] = self.formatException(record.exc_info)
        return json.dumps(entry, ensure_ascii=False, default=str)


class TextFormatter(logging.Formatter):
    def __init__(self):
        super().__init__("%(asctime)s %(levelname)-7s [%(request_id)s] %(name)s: %(message)s")

    def format(self, record: logging.LogRecord) -> str:
        record.request_id = request_id_var.get()
        return super().format(record)


def configure_logging() -> None:
    """LOG_LEVEL: debug | info (default) | warning | error.  LOG_FORMAT: json (default) | text."""
    level = getattr(logging, os.getenv("LOG_LEVEL", "info").upper(), logging.INFO)
    formatter = TextFormatter() if os.getenv("LOG_FORMAT", "json").lower() == "text" else JsonFormatter()

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(formatter)

    root = logging.getLogger()
    root.handlers = [handler]
    root.setLevel(level)

    # Route uvicorn's own messages through the same formatter. Its access log is replaced by ours.
    for name in ("uvicorn", "uvicorn.error"):
        lg = logging.getLogger(name)
        lg.handlers = []
        lg.propagate = True
    logging.getLogger("uvicorn.access").disabled = True


def new_request_id() -> str:
    return secrets.token_hex(8)


def resolve_request_id(incoming: Optional[str]) -> str:
    return incoming if incoming and _VALID_REQUEST_ID.match(incoming) else new_request_id()


class RequestContextMiddleware:
    """Pure ASGI middleware (not BaseHTTPMiddleware, which interferes with streaming responses).

    Adopts or creates the request id, makes it available to every log line made while handling the
    request, returns it in the response headers, and writes one access line per request. The
    access line carries the path but never the query string or headers.
    """

    def __init__(self, app):
        self.app = app

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        headers = {k.decode("latin-1").lower(): v.decode("latin-1") for k, v in scope.get("headers", [])}
        request_id = resolve_request_id(headers.get(REQUEST_ID_HEADER))
        token = request_id_var.set(request_id)

        start = time.perf_counter()
        status = 500  # what the client gets if the app dies before responding

        async def send_with_id(message):
            nonlocal status
            if message["type"] == "http.response.start":
                status = message["status"]
                message.setdefault("headers", [])
                message["headers"] = [*message["headers"], (REQUEST_ID_HEADER.encode(), request_id.encode())]
            await send(message)

        try:
            await self.app(scope, receive, send_with_id)
        finally:
            level = logging.ERROR if status >= 500 else logging.WARNING if status >= 400 else logging.INFO
            access_logger.log(
                level,
                "request",
                extra={
                    "fields": {
                        "method": scope["method"],
                        "path": scope["path"],
                        "status": status,
                        "duration_ms": round((time.perf_counter() - start) * 1000),
                    }
                },
            )
            request_id_var.reset(token)
