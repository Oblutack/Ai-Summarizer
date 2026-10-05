import pytest
from fastapi.testclient import TestClient

import main

TOKEN = "shared-secret-for-tests"


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def test_without_a_configured_secret_everything_is_open(client, monkeypatch):
    monkeypatch.delenv("AI_SERVICE_TOKEN", raising=False)
    assert client.get("/options").status_code == 200


@pytest.mark.parametrize(
    "headers", [{}, {"Authorization": "Bearer wrong"}, {"Authorization": TOKEN}, {"Authorization": "Bearer "}]
)
def test_with_a_secret_requests_without_it_are_refused(client, monkeypatch, headers):
    monkeypatch.setenv("AI_SERVICE_TOKEN", TOKEN)
    for method, path in (("get", "/options"), ("post", "/summarize-text"), ("post", "/chat")):
        response = getattr(client, method)(path, headers=headers)
        assert response.status_code == 401, (method, path)
        assert response.headers["www-authenticate"] == "Bearer"
    assert "TOKEN" not in response.text.upper().replace("UNAUTHORIZED", "")


def test_the_right_secret_is_accepted(client, monkeypatch):
    monkeypatch.setenv("AI_SERVICE_TOKEN", TOKEN)
    response = client.get("/options", headers={"Authorization": f"Bearer {TOKEN}"})
    assert response.status_code == 200


def test_health_checks_stay_open_for_the_platform(client, monkeypatch):
    monkeypatch.setenv("AI_SERVICE_TOKEN", TOKEN)
    assert client.get("/healthz").status_code == 200


def test_metrics_keep_their_own_token_not_the_shared_secret(client, monkeypatch):
    monkeypatch.setenv("AI_SERVICE_TOKEN", TOKEN)
    monkeypatch.setenv("METRICS_TOKEN", "metrics-token")
    assert client.get("/metrics", headers={"Authorization": f"Bearer {TOKEN}"}).status_code == 401
    assert client.get("/metrics", headers={"Authorization": "Bearer metrics-token"}).status_code == 200


def test_refused_requests_are_still_counted_and_logged_by_the_outer_layers(client, monkeypatch):
    monkeypatch.setenv("AI_SERVICE_TOKEN", TOKEN)
    monkeypatch.setenv("METRICS_TOKEN", "metrics-token")
    client.post("/summarize-text", json={"text": "x"})
    body = client.get("/metrics", headers={"Authorization": "Bearer metrics-token"}).text
    assert 'status="401"' in body
