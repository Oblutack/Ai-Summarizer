"""Turns text into vectors, so documents can be searched by meaning and not only by their words.

A small model runs inside this service (no extra paid service, nothing leaves the machine). It is
optional: when it is switched off or cannot load, callers get EmbeddingsUnavailable and the gateway
falls back to keyword search.
"""

import base64
import logging
import os
import threading

import numpy as np

logger = logging.getLogger("ai-summarizer")

DEFAULT_MODEL = "BAAI/bge-small-en-v1.5"

# A floor for how similar a passage must be to count as "about" the question. It is deliberately low:
# on a test set the scores of questions the library can answer and ones it cannot overlap (page-sized
# passages score lower than short ones), so a high value throws away correct passages, which is worse
# than sending a weak one for the language model to dismiss. It only removes clearly unrelated text.
# Scores are not comparable between models, so each known model has its own value. An unknown model
# gets 0 (no floor) unless EMBEDDING_MIN_SCORE says otherwise.
KNOWN_MIN_SCORES = {
    "BAAI/bge-small-en-v1.5": 0.48,
    "thenlper/gte-base": 0.72,
}

MAX_TEXTS = 128
MAX_TEXT_CHARS = 6_000
KINDS = ("passage", "query")
# Inference is CPU-bound: a couple at a time keeps the other endpoints responsive.
MAX_CONCURRENT_EMBEDDINGS = 2


class EmbeddingsUnavailable(Exception):
    """Embeddings are switched off, or the model could not be loaded."""


def enabled() -> bool:
    return os.getenv("EMBEDDINGS", "on").strip().lower() not in ("off", "0", "false", "no")


def model_name() -> str:
    return os.getenv("EMBEDDING_MODEL", "").strip() or DEFAULT_MODEL


def min_score() -> float:
    raw = os.getenv("EMBEDDING_MIN_SCORE", "").strip()
    if raw:
        try:
            return float(raw)
        except ValueError:
            logger.warning("EMBEDDING_MIN_SCORE is not a number; using the model's default")
    return KNOWN_MIN_SCORES.get(model_name(), 0.0)


_load_lock = threading.Lock()
_slots = threading.BoundedSemaphore(MAX_CONCURRENT_EMBEDDINGS)
_models: dict[str, object] = {}


def _model():
    name = model_name()
    if name in _models:
        return _models[name]
    with _load_lock:
        if name not in _models:
            try:
                from fastembed import TextEmbedding

                _models[name] = TextEmbedding(model_name=name)
            except Exception as exc:  # a missing package, an unknown model name, no network for the download
                logger.error("could not load the embedding model", extra={"model": name, "error": str(exc)})
                raise EmbeddingsUnavailable(f"The embedding model {name} could not be loaded") from exc
        return _models[name]


def embed(texts: list[str], kind: str) -> list[bytes]:
    """One vector per text, as unit-length little-endian float32 bytes (a dot product is the cosine)."""
    if not enabled():
        raise EmbeddingsUnavailable("Embeddings are turned off")
    model = _model()
    clipped = [t[:MAX_TEXT_CHARS] for t in texts]
    with _slots:
        try:
            produced = model.query_embed(clipped) if kind == "query" else model.passage_embed(clipped)
            matrix = np.array(list(produced), dtype=np.float32)
        except Exception as exc:
            logger.error("embedding failed", extra={"error": str(exc)})
            raise EmbeddingsUnavailable("Embedding failed") from exc
    norms = np.linalg.norm(matrix, axis=1, keepdims=True)
    unit = matrix / np.where(norms == 0, 1, norms)
    return [row.astype("<f4").tobytes() for row in unit]


def encode(vector: bytes) -> str:
    return base64.b64encode(vector).decode("ascii")
