"""Shared-secret protection for the AI service.

Locally and in docker compose the service sits on a private network that only the Go API can reach,
so no secret is needed. On hosting platforms where it ends up on a public URL, set AI_SERVICE_TOKEN
(the same value on the Go API): every request except the health check must then carry it as
`Authorization: Bearer <token>`. Without it, anyone who found the URL could spend the model quota.
"""

import hmac
import json
import os

# Reachable without the secret: platforms probe the health check, and /metrics has its own token.
OPEN_PATHS = frozenset({"/healthz", "/metrics"})


class InternalAuthMiddleware:
    def __init__(self, app):
        self.app = app

    async def __call__(self, scope, receive, send):
        token = os.getenv("AI_SERVICE_TOKEN", "")
        if scope["type"] != "http" or not token or scope["path"] in OPEN_PATHS:
            await self.app(scope, receive, send)
            return

        headers = {k.decode("latin-1").lower(): v.decode("latin-1") for k, v in scope.get("headers", [])}
        given = headers.get("authorization", "")
        if given.startswith("Bearer ") and hmac.compare_digest(given[len("Bearer ") :], token):
            await self.app(scope, receive, send)
            return

        body = json.dumps({"detail": "Unauthorized"}).encode()
        await send(
            {
                "type": "http.response.start",
                "status": 401,
                "headers": [
                    (b"content-type", b"application/json"),
                    (b"content-length", str(len(body)).encode()),
                    (b"www-authenticate", b"Bearer"),
                ],
            }
        )
        await send({"type": "http.response.body", "body": body})
