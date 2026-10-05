"""Optional error reporting to Sentry. Does nothing unless SENTRY_DSN is set.

    SENTRY_DSN          where to send errors (from your Sentry project)
    SENTRY_ENVIRONMENT  e.g. production or staging (default: production)
    SENTRY_RELEASE      optional version label, such as a git commit

What is sent is deliberately narrow: unhandled errors and logged errors, with their stack traces.
Request bodies, headers, local variables and breadcrumbs are never attached, because in this
service they hold the documents people are summarizing.
"""

import logging
import os

import sentry_sdk
from sentry_sdk.types import Event, Hint

logger = logging.getLogger("ai-summarizer")


def scrub_event(event: Event, hint: Hint | None = None) -> Event:
    """Removes everything that could carry user data from an event before it is sent."""
    for key in ("request", "user", "breadcrumbs", "extra"):
        event.pop(key, None)  # type: ignore[misc]
    return event


def setup_error_reporting() -> bool:
    """Starts Sentry if SENTRY_DSN is set. Returns whether reporting is on. A bad DSN is logged and
    ignored: reporting is a convenience and must never stop the service from starting."""
    dsn = os.getenv("SENTRY_DSN", "")
    if not dsn:
        return False
    environment = os.getenv("SENTRY_ENVIRONMENT") or "production"
    try:
        sentry_sdk.init(
            dsn=dsn,
            environment=environment,
            release=os.getenv("SENTRY_RELEASE") or None,
            server_name="ai-service",
            send_default_pii=False,
            include_local_variables=False,  # locals can hold document text
            max_request_body_size="never",
            max_breadcrumbs=0,
            traces_sample_rate=0.0,
            before_send=scrub_event,
        )
    except Exception:
        logger.exception("Sentry is configured but could not start; errors will not be reported")
        return False
    logger.info("Error reporting enabled (Sentry, environment=%s)", environment)
    return True
