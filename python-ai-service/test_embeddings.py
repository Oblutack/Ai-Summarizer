import base64
import os
import struct

import numpy as np
import pytest
from fastapi.testclient import TestClient

import embeddings
import main


class FakeModel:
    """Vectors from letters, so equal texts match and the passage and query paths differ visibly."""

    dim = 8

    def __init__(self):
        self.calls = []

    def _vector(self, text, shift):
        v = np.zeros(self.dim)
        for ch in text.lower():
            v[(ord(ch) + shift) % self.dim] += 1
        return v * 3.0  # deliberately not unit length

    def passage_embed(self, texts):
        self.calls.append(("passage", list(texts)))
        return (self._vector(t, 0) for t in texts)

    def query_embed(self, texts):
        self.calls.append(("query", list(texts)))
        return (self._vector(t, 0) for t in texts)


@pytest.fixture
def fake(monkeypatch):
    monkeypatch.delenv("EMBEDDINGS", raising=False)
    monkeypatch.delenv("EMBEDDING_MODEL", raising=False)
    monkeypatch.delenv("EMBEDDING_MIN_SCORE", raising=False)
    model = FakeModel()
    monkeypatch.setattr(embeddings, "_models", {embeddings.DEFAULT_MODEL: model})
    return model


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def decode(b64):
    raw = base64.b64decode(b64)
    return np.array(struct.unpack(f"<{len(raw) // 4}f", raw))


# ---- the vectors -------------------------------------------------------------------------------


def test_vectors_are_unit_length_little_endian_float32(fake):
    vectors = embeddings.embed(["hello world", "another text"], "passage")
    assert len(vectors) == 2 and all(len(v) == fake.dim * 4 for v in vectors)
    for v in vectors:
        assert np.isclose(np.linalg.norm(np.frombuffer(v, dtype="<f4")), 1.0, atol=1e-5)


def test_the_dot_product_of_equal_texts_is_one_and_of_different_texts_less(fake):
    a, b, c = (np.frombuffer(v, dtype="<f4") for v in embeddings.embed(["aaaa", "aaaa", "zzzz"], "passage"))
    assert np.isclose(a @ b, 1.0, atol=1e-5)
    assert a @ c < 0.9


def test_queries_and_passages_take_their_own_path(fake):
    embeddings.embed(["a question"], "query")
    embeddings.embed(["a passage"], "passage")
    assert [kind for kind, _ in fake.calls] == ["query", "passage"]


def test_very_long_texts_are_clipped(fake):
    embeddings.embed(["x" * (embeddings.MAX_TEXT_CHARS + 500)], "passage")
    assert len(fake.calls[0][1][0]) == embeddings.MAX_TEXT_CHARS


def test_a_zero_vector_does_not_divide_by_zero(monkeypatch, fake):
    monkeypatch.setattr(FakeModel, "_vector", lambda self, text, shift: np.zeros(self.dim))
    vector = np.frombuffer(embeddings.embed(["anything"], "passage")[0], dtype="<f4")
    assert not np.isnan(vector).any()


# ---- settings -------------------------------------------------------------------------------------


@pytest.mark.parametrize("value", ["off", "OFF", "0", "false", "no", " No "])
def test_embeddings_can_be_switched_off(monkeypatch, fake, value):
    monkeypatch.setenv("EMBEDDINGS", value)
    assert not embeddings.enabled()
    with pytest.raises(embeddings.EmbeddingsUnavailable):
        embeddings.embed(["text"], "passage")


def test_the_defaults(monkeypatch):
    monkeypatch.delenv("EMBEDDINGS", raising=False)
    monkeypatch.delenv("EMBEDDING_MODEL", raising=False)
    monkeypatch.delenv("EMBEDDING_MIN_SCORE", raising=False)
    assert embeddings.enabled()
    assert embeddings.model_name() == embeddings.DEFAULT_MODEL
    assert embeddings.min_score() == embeddings.KNOWN_MIN_SCORES[embeddings.DEFAULT_MODEL]


def test_each_known_model_has_its_own_cutoff_and_unknown_ones_have_none(monkeypatch):
    monkeypatch.delenv("EMBEDDING_MIN_SCORE", raising=False)
    monkeypatch.setenv("EMBEDDING_MODEL", "thenlper/gte-base")
    assert embeddings.min_score() == embeddings.KNOWN_MIN_SCORES["thenlper/gte-base"]
    monkeypatch.setenv("EMBEDDING_MODEL", "some/unknown-model")
    assert embeddings.min_score() == 0.0


def test_the_cutoff_can_be_overridden_and_a_bad_value_is_ignored(monkeypatch):
    monkeypatch.setenv("EMBEDDING_MIN_SCORE", "0.42")
    assert embeddings.min_score() == 0.42
    monkeypatch.setenv("EMBEDDING_MIN_SCORE", "high")
    assert embeddings.min_score() == embeddings.KNOWN_MIN_SCORES[embeddings.DEFAULT_MODEL]


def test_a_model_that_cannot_load_is_reported_as_unavailable(monkeypatch):
    monkeypatch.setattr(embeddings, "_models", {})
    monkeypatch.setenv("EMBEDDING_MODEL", "no/such-model")
    with pytest.raises(embeddings.EmbeddingsUnavailable):
        embeddings.embed(["text"], "passage")


def test_a_failure_while_embedding_is_reported_as_unavailable(monkeypatch, fake):
    def boom(self, texts):
        raise RuntimeError("onnx exploded")

    monkeypatch.setattr(FakeModel, "passage_embed", boom)
    with pytest.raises(embeddings.EmbeddingsUnavailable):
        embeddings.embed(["text"], "passage")


# ---- the endpoint -----------------------------------------------------------------------------------


def test_the_endpoint_returns_vectors_and_what_the_gateway_needs_to_know(client, fake):
    response = client.post("/embed", json={"texts": ["one", "two"], "kind": "passage"})
    body = response.json()
    assert response.status_code == 200
    assert body["model"] == embeddings.DEFAULT_MODEL
    assert body["dim"] == fake.dim
    assert body["minScore"] == embeddings.KNOWN_MIN_SCORES[embeddings.DEFAULT_MODEL]
    assert len(body["vectors"]) == 2 and decode(body["vectors"][0]).shape == (fake.dim,)


def test_the_kind_defaults_to_passage(client, fake):
    client.post("/embed", json={"texts": ["one"]})
    assert fake.calls[0][0] == "passage"


@pytest.mark.parametrize(
    ("body", "status"),
    [
        ({"texts": ["a"], "kind": "document"}, 422),
        ({"texts": []}, 422),
        ({"texts": ["fine", "  "]}, 422),
        ({"texts": ["x"] * (embeddings.MAX_TEXTS + 1)}, 413),
    ],
    ids=["bad-kind", "no-texts", "blank-text", "too-many"],
)
def test_the_endpoint_validates(client, fake, body, status):
    assert client.post("/embed", json=body).status_code == status


def test_the_endpoint_says_503_when_embeddings_are_off(client, fake, monkeypatch):
    monkeypatch.setenv("EMBEDDINGS", "off")
    response = client.post("/embed", json={"texts": ["one"]})
    assert response.status_code == 503 and "off" in response.json()["detail"]


# ---- the real model (slow, needs the model files): RUN_EMBEDDING_MODEL_TESTS=1 pytest test_embeddings.py ------


@pytest.mark.skipif(os.getenv("RUN_EMBEDDING_MODEL_TESTS") != "1", reason="loads the real embedding model")
def test_the_real_model_finds_a_passage_by_meaning(monkeypatch):
    monkeypatch.delenv("EMBEDDINGS", raising=False)
    monkeypatch.delenv("EMBEDDING_MODEL", raising=False)
    monkeypatch.setattr(embeddings, "_models", {})
    passages = [
        "Cats and dogs are not allowed without written permission from the landlord.",
        "The monthly rent is 950 euros and is due on the first day of every month.",
        "Heating and water are included in the rent.",
    ]
    docs = np.array([np.frombuffer(v, dtype="<f4") for v in embeddings.embed(passages, "passage")])
    query = np.frombuffer(embeddings.embed(["Can I keep a pet in the flat?"], "query")[0], dtype="<f4")
    assert int(np.argmax(docs @ query)) == 0
    assert (docs @ query).max() >= embeddings.min_score()
