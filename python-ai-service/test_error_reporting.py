import pytest
import sentry_sdk

import error_reporting


@pytest.fixture(autouse=True)
def no_leftover_client():
    yield
    sentry_sdk.get_client().close()
    sentry_sdk.init(dsn="")  # back to a disabled client (an empty DSN ignores the environment)


def test_nothing_starts_without_a_dsn(monkeypatch):
    monkeypatch.delenv("SENTRY_DSN", raising=False)
    assert error_reporting.setup_error_reporting() is False
    assert not sentry_sdk.get_client().is_active()


def test_a_bad_dsn_does_not_stop_the_service(monkeypatch):
    monkeypatch.setenv("SENTRY_DSN", "this is not a dsn")
    assert error_reporting.setup_error_reporting() is False


def test_a_valid_dsn_starts_reporting_with_privacy_options(monkeypatch):
    monkeypatch.setenv("SENTRY_DSN", "https://public@example.invalid/1")
    monkeypatch.setenv("SENTRY_ENVIRONMENT", "staging")
    assert error_reporting.setup_error_reporting() is True

    options = sentry_sdk.get_client().options
    assert options["environment"] == "staging"
    assert options["send_default_pii"] is False
    assert options["include_local_variables"] is False
    assert options["max_request_body_size"] == "never"
    assert options["traces_sample_rate"] == 0.0


def test_events_are_scrubbed_of_user_data():
    event = {
        "message": "boom",
        "request": {"url": "http://x/summarize-text", "data": "CONFIDENTIAL DOCUMENT"},
        "user": {"ip_address": "203.0.113.9"},
        "breadcrumbs": {"values": [{"message": "typed text"}]},
        "extra": {"prompt": "secret"},
        "tags": {"request_id": "abc12345"},
    }
    cleaned = error_reporting.scrub_event(event)
    for key in ("request", "user", "breadcrumbs", "extra"):
        assert key not in cleaned
    assert cleaned["message"] == "boom"
    assert cleaned["tags"] == {"request_id": "abc12345"}
