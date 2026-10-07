import pytest
from fastapi.testclient import TestClient

import main
from retrieval import PAGE_BREAK


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


TEXT = PAGE_BREAK.join(["Intro page.", "The warranty lasts seven years from purchase."])


def test_proof_checks_sentences_and_needs_no_language_model(client, monkeypatch):
    def no_model():
        raise AssertionError("the proof check must never call the language model")

    monkeypatch.setattr(main, "get_llm", no_model)
    r = client.post("/proof", json={"summary": "The warranty lasts seven years.", "text": TEXT})
    assert r.status_code == 200
    body = r.json()
    (claim,) = [s for s in body["sentences"] if s["kind"] == "claim"]
    assert claim["support"] == "strong"
    assert claim["passages"][0]["page"] == 2
    assert body["found"] == 1 and body["notFound"] == 0 and body["verifiable"] is True


@pytest.mark.parametrize(
    ("payload", "status"),
    [
        ({"summary": "  ", "text": TEXT}, 422),
        ({"summary": "A claim.", "text": "   "}, 422),
        ({"summary": "x" * (main.MAX_PROOF_SUMMARY_CHARS + 1), "text": TEXT}, 413),
        ({"summary": "A claim.", "text": "x" * (main.MAX_MULTI_TEXT_CHARS + 20_000)}, 413),
        ({"summary": "A claim."}, 422),
    ],
)
def test_proof_validation(client, payload, status):
    assert client.post("/proof", json=payload).status_code == status
