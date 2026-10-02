"""LLM client construction with model availability checks and automatic fallback.

Hosted providers retire models over time (llama-3.1-8b-instant did), so the configured model
is verified against the provider's model list, and calls fall over to the next candidate when
the provider reports the current one as missing.
"""
import logging
import os
import time
from typing import Optional

import httpx
from langchain_openai import ChatOpenAI

logger = logging.getLogger("ai-summarizer")

BASE_URL = "https://api.groq.com/openai/v1"
DEFAULT_MODEL = "openai/gpt-oss-20b"
DEFAULT_FALLBACKS = "openai/gpt-oss-120b"

TIMEOUT_SECONDS = 60
MAX_RETRIES = 5  # the OpenAI client backs off exponentially on 429/5xx
# gpt-oss models "think" before answering and those hidden tokens count against the output cap.
# Low effort plus a generous cap keeps them from exhausting it and returning empty content.
REASONING_EFFORT = os.getenv("LLM_REASONING_EFFORT", "low")
MAX_OUTPUT_TOKENS = 8192

MODEL_LIST_TTL_SECONDS = 60

_clients: dict[str, ChatOpenAI] = {}
_unavailable: set[str] = set()
_model_list_cache: tuple[float, set[str]] = (0.0, set())


def api_key() -> str:
    key = os.getenv("GROQ_API_KEY")
    if not key:
        raise RuntimeError("GROQ_API_KEY is not set")
    return key


def candidate_models() -> list[str]:
    """The configured model first, then the fallbacks, without duplicates."""
    primary = os.getenv("LLM_MODEL", DEFAULT_MODEL)
    fallbacks = os.getenv("LLM_FALLBACK_MODELS", DEFAULT_FALLBACKS).split(",")
    models: list[str] = []
    for model in [primary, *fallbacks]:
        model = model.strip()
        if model and model not in models:
            models.append(model)
    return models


def active_model() -> str:
    """The first candidate that has not been marked unavailable."""
    for model in candidate_models():
        if model not in _unavailable:
            return model
    raise RuntimeError("No configured language model is available")


def mark_unavailable(model: str) -> None:
    logger.error("Model %s is not available from the provider; switching to the next candidate", model)
    _unavailable.add(model)


def reset_unavailable() -> None:
    _unavailable.clear()
    _model_list_cache_clear()


def is_model_missing(exc: BaseException) -> bool:
    """True for the provider's "this model does not exist or you have no access" error."""
    return getattr(exc, "code", None) == "model_not_found"


def get_llm() -> ChatOpenAI:
    model = active_model()
    if model not in _clients:
        extra = {"reasoning_effort": REASONING_EFFORT} if "gpt-oss" in model else {}
        _clients[model] = ChatOpenAI(
            model=model,
            api_key=api_key(),
            base_url=BASE_URL,
            timeout=TIMEOUT_SECONDS,
            max_retries=MAX_RETRIES,
            max_tokens=MAX_OUTPUT_TOKENS,
            model_kwargs=extra,
        )
    return _clients[model]


def _model_list_cache_clear() -> None:
    global _model_list_cache
    _model_list_cache = (0.0, set())


async def list_models() -> set[str]:
    """Model ids the provider currently serves for this key (cached briefly)."""
    global _model_list_cache
    fetched_at, models = _model_list_cache
    if models and time.monotonic() - fetched_at < MODEL_LIST_TTL_SECONDS:
        return models

    async with httpx.AsyncClient(timeout=10) as client:
        response = await client.get(f"{BASE_URL}/models", headers={"Authorization": f"Bearer {api_key()}"})
        response.raise_for_status()
    models = {m["id"] for m in response.json().get("data", [])}
    _model_list_cache = (time.monotonic(), models)
    return models


async def verify_models() -> Optional[str]:
    """Checks the candidates against the provider's list and marks missing ones unavailable.

    Returns the model that will be used, or None if the provider could not be reached
    (in which case nothing is marked, since an outage is not the same as a retired model).
    """
    try:
        available = await list_models()
    except Exception as exc:
        logger.warning("Could not verify language models with the provider: %s", exc)
        return None

    for model in candidate_models():
        if model not in available:
            mark_unavailable(model)
    try:
        chosen = active_model()
    except RuntimeError:
        logger.critical("None of the configured models %s exist at the provider", candidate_models())
        return None
    if chosen != candidate_models()[0]:
        logger.warning("Primary model unavailable; using fallback %s", chosen)
    return chosen
