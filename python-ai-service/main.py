import asyncio
import hmac
import json
import logging
import os
import tempfile
import time
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager
from typing import Optional

from dotenv import load_dotenv
from fastapi import FastAPI, File, Form, HTTPException, Query, Request, UploadFile
from fastapi.responses import Response, StreamingResponse
from langchain_text_splitters import RecursiveCharacterTextSplitter
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest
from pydantic import BaseModel
from pypdf import PdfReader

from cache import from_env as cache_from_env
from cache import summary_key
from error_reporting import setup_error_reporting
from internal_auth import InternalAuthMiddleware
from llm import (
    ServiceUnavailable,
    active_model,
    breaker,
    candidate_models,
    counts_as_outage,
    get_llm,
    is_model_missing,
    mark_unavailable,
    verify_models,
)
from logging_setup import RequestContextMiddleware, configure_logging
from metrics import MetricsMiddleware, StateCollector, record_llm, record_rejected, record_usage, registry
from prompts import (
    DEFAULT_LANGUAGE,
    DEFAULT_STYLE,
    LANGUAGES,
    PROMPT_VERSION,
    STYLE_INSTRUCTIONS,
    chat_prompt,
    is_valid_language,
    is_valid_style,
    summary_prompt,
)
from retrieval import select_context

load_dotenv()
configure_logging()
setup_error_reporting()

logger = logging.getLogger("ai-summarizer")

MAX_PDF_BYTES = 10 * 1024 * 1024
MAX_TEXT_CHARS = 200_000
MAX_FILES = 5
MAX_MULTI_TEXT_CHARS = 400_000
MIN_WORDS, MAX_WORDS = 50, 1000
MAX_PAGE_LIMIT = 20
WORDS_PER_PAGE = 250

# Texts longer than this go through map-reduce even without a page limit, so a single
# prompt never exceeds the model's context or the provider's per-request token limit.
SINGLE_SHOT_MAX_CHARS = 24_000
CHUNK_SIZE = 4_000
CHUNK_OVERLAP = 200
# If the joined chunk summaries are still longer than this, they are summarized again.
REDUCE_MAX_CHARS = 24_000
# Cap parallel LLM calls so long documents don't trip the provider's rate limit.
MAX_CONCURRENT_LLM_CALLS = 3

# Chat limits: how much of the document is shown to the model per question, and how much
# conversation history and question text is accepted.
CHAT_CONTEXT_CHARS = 16_000
MAX_QUESTION_CHARS = 1_000
MAX_HISTORY_MESSAGES = 10
MAX_HISTORY_MESSAGE_CHARS = 4_000

SUMMARY_FAILED = "The language model failed to produce a summary. Please try again."
BUSY_MESSAGE = "The AI service is busy right now. Please try again in a minute."

summary_cache = cache_from_env()
# Cache and breaker state are read when Prometheus scrapes, so nothing is instrumented on the hot path.
registry.register(StateCollector(lambda: summary_cache, lambda: breaker))

ProgressCallback = Callable[[int, int], None]


@asynccontextmanager
async def lifespan(_: FastAPI):
    # Fail loudly in the logs at boot if the configured model has been retired.
    if os.getenv("GROQ_API_KEY"):
        model = await verify_models()
        if model:
            logger.info("Using language model %s", model)
    else:
        logger.error("GROQ_API_KEY is not set; summarization will fail")
    yield


app = FastAPI(title="AI Summarizer Service", lifespan=lifespan)
# Added innermost first: the secret is checked inside metrics and logging, so refused calls still show up in both.
app.add_middleware(InternalAuthMiddleware)
app.add_middleware(MetricsMiddleware)
app.add_middleware(RequestContextMiddleware)

_llm_slots: dict[asyncio.AbstractEventLoop, asyncio.Semaphore] = {}


class TextPayload(BaseModel):
    text: str


class ChatMessage(BaseModel):
    role: str
    content: str


class ChatPayload(BaseModel):
    text: str
    question: str
    history: Optional[list[ChatMessage]] = None


def _slots() -> asyncio.Semaphore:
    # A semaphore is bound to the loop that first awaits it, so keep one per running loop.
    return _llm_slots.setdefault(asyncio.get_running_loop(), asyncio.Semaphore(MAX_CONCURRENT_LLM_CALLS))


def to_http_error(exc: Exception, failure_message: str = SUMMARY_FAILED) -> HTTPException:
    """Turns any failure into a client-safe HTTP error; internals are logged, never returned."""
    if isinstance(exc, HTTPException):
        return exc
    if isinstance(exc, ServiceUnavailable):
        return HTTPException(503, BUSY_MESSAGE, headers={"Retry-After": "30"})
    logger.exception("Request failed", exc_info=exc)
    return HTTPException(502, failure_message)


def content_text(content: str | list[str | dict] | None) -> str:
    """The text of a model reply. Providers may return a plain string or a list of content blocks."""
    if not content:
        return ""
    if isinstance(content, str):
        return content
    parts = []
    for block in content:
        if isinstance(block, str):
            parts.append(block)
        elif isinstance(block, dict) and isinstance(block.get("text"), str):
            parts.append(block["text"])
    return "".join(parts)


async def call_llm(prompt: str) -> str:
    async with _slots():
        # If the provider says the model no longer exists, move to the next candidate and retry.
        for _ in candidate_models():
            model = active_model()
            if not breaker.allow():
                record_rejected(model)
                raise ServiceUnavailable("circuit open")
            started = time.perf_counter()
            try:
                response = await get_llm().ainvoke(prompt)
                breaker.success()
                record_llm(model, "invoke", "ok", started)
                record_usage(model, getattr(response, "usage_metadata", None))
                return content_text(response.content).strip()
            except Exception as exc:
                if is_model_missing(exc):
                    record_llm(model, "invoke", "model_missing", started)
                    mark_unavailable(model)
                    continue
                outage = counts_as_outage(exc)
                if outage:
                    breaker.failure()
                record_llm(model, "invoke", "provider_error" if outage else "error", started)
                raise
    raise RuntimeError("No configured language model is available")


async def stream_llm(prompt: str) -> AsyncIterator[str]:
    """Yields the model's reply piece by piece, with the same model fallback as call_llm."""
    async with _slots():
        for _ in candidate_models():
            model = active_model()
            if not breaker.allow():
                record_rejected(model)
                raise ServiceUnavailable("circuit open")
            started = False
            began = time.perf_counter()
            usage = None
            outcome = "cancelled"  # what it stays if the client disconnects and the stream is closed
            try:
                async for chunk in get_llm().astream(prompt):
                    usage = getattr(chunk, "usage_metadata", None) or usage
                    text = content_text(chunk.content)
                    if text:
                        started = True
                        yield text
                breaker.success()
                outcome = "ok"
                record_usage(model, usage)
                return
            except Exception as exc:
                # Only fall back if nothing was sent yet; a half-written answer can't be resumed.
                if is_model_missing(exc) and not started:
                    outcome = "model_missing"
                    mark_unavailable(model)
                    continue
                outage = counts_as_outage(exc)
                if outage:
                    breaker.failure()
                outcome = "provider_error" if outage else "error"
                raise
            finally:
                record_llm(model, "stream", outcome, began)
    raise RuntimeError("No configured language model is available")


@app.get("/")
def read_root():
    return {"message": "Python AI Service is running"}


@app.get("/healthz")
def healthz():
    """Liveness: the process is up. Deliberately independent of the LLM provider."""
    return {"status": "ok"}


@app.get("/metrics", include_in_schema=False)
def prometheus_metrics(request: Request):
    """Prometheus metrics. Off (404) unless METRICS_TOKEN is set, and then only for callers that
    present it as a bearer token: traffic patterns and usage are not for the public."""
    token = os.getenv("METRICS_TOKEN", "")
    if not token:
        raise HTTPException(404, "Not Found")
    given = request.headers.get("authorization", "")
    if not given.startswith("Bearer ") or not hmac.compare_digest(given[len("Bearer ") :], token):
        raise HTTPException(401, "Unauthorized", headers={"WWW-Authenticate": "Bearer"})
    return Response(generate_latest(registry), media_type=CONTENT_TYPE_LATEST)


@app.get("/readyz")
async def readyz():
    """Readiness: we hold an API key and at least one configured model exists at the provider."""
    if not os.getenv("GROQ_API_KEY"):
        raise HTTPException(503, "GROQ_API_KEY is not set")
    model = await verify_models()
    if model is None:
        raise HTTPException(503, "No usable language model (provider unreachable or models retired)")
    if breaker.is_open:
        raise HTTPException(503, "The language model circuit breaker is open")
    return {"status": "ready", "model": model}


@app.get("/options")
def options():
    return {"styles": list(STYLE_INSTRUCTIONS), "languages": LANGUAGES}


def validate_options(word_count: int, page_limit: int, style: str, language: str) -> None:
    if not MIN_WORDS <= word_count <= MAX_WORDS:
        raise HTTPException(422, f"word_count must be between {MIN_WORDS} and {MAX_WORDS}")
    if not 0 <= page_limit <= MAX_PAGE_LIMIT:
        raise HTTPException(422, f"page_limit must be between 0 and {MAX_PAGE_LIMIT}")
    if not is_valid_style(style):
        raise HTTPException(422, f"style must be one of: {', '.join(STYLE_INSTRUCTIONS)}")
    if not is_valid_language(language):
        raise HTTPException(422, "Unsupported language")


def extract_pdf_text(path: str) -> str:
    return "\n".join(page.extract_text() or "" for page in PdfReader(path).pages)


async def read_pdf_upload(file: UploadFile) -> str:
    """Reads an uploaded PDF and returns its text, raising client-safe HTTP errors."""
    content = await file.read(MAX_PDF_BYTES + 1)
    if len(content) > MAX_PDF_BYTES:
        raise HTTPException(413, f"{file.filename} is too large (max 10 MB)")

    temp_pdf_path = None
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as temp_pdf:
            temp_pdf.write(content)
            temp_pdf_path = temp_pdf.name
        try:
            return await asyncio.to_thread(extract_pdf_text, temp_pdf_path)
        except Exception as exc:
            logger.exception("Failed to parse PDF")
            raise HTTPException(
                422, f"Could not read {file.filename}. It may be corrupted or password-protected."
            ) from exc
    finally:
        if temp_pdf_path and os.path.exists(temp_pdf_path):
            os.unlink(temp_pdf_path)


def check_text(text: str) -> None:
    """Rejects input that can't be summarized, before any LLM work or streaming starts."""
    if not text.strip():
        raise HTTPException(422, "No text found to summarize. Scanned PDFs (images) are not supported.")
    if len(text) > MAX_TEXT_CHARS:
        raise HTTPException(413, f"Text is too long (max {MAX_TEXT_CHARS} characters)")


def check_documents(docs: list[tuple[str, str]]) -> None:
    for name, text in docs:
        if not text.strip():
            raise HTTPException(422, f"No text found in {name}. Scanned PDFs (images) are not supported.")
    if sum(len(text) for _, text in docs) > MAX_MULTI_TEXT_CHARS:
        raise HTTPException(413, f"The documents are too long together (max {MAX_MULTI_TEXT_CHARS} characters)")


# ---- Server-sent events -------------------------------------------------------------------
# Each event is one line of JSON: {"type": "status" | "delta" | "done" | "error", ...}.


def sse(event: dict) -> str:
    return f"data: {json.dumps(event, ensure_ascii=False)}\n\n"


CACHED_PIECE_CHARS = 160


async def summary_event_stream(
    prepare: Callable[[Optional[ProgressCallback]], Awaitable[str]],
    done_extra: Optional[dict] = None,
    cache_key: Optional[str] = None,
) -> AsyncIterator[str]:
    """Runs `prepare` (which may do map-reduce work and reports progress), then streams the
    final summary as it is generated. Failures become a final "error" event, since by the time
    they happen the HTTP status line has already been sent."""
    progress: asyncio.Queue = asyncio.Queue()
    task: Optional[asyncio.Task] = None
    try:
        yield sse({"type": "status", "stage": "preparing"})

        cached = summary_cache.get(cache_key) if cache_key else None
        if cached is not None:
            # Same input and options as before: replay the stored summary without calling the LLM.
            yield sse({"type": "status", "stage": "writing"})
            for i in range(0, len(cached), CACHED_PIECE_CHARS):
                yield sse({"type": "delta", "text": cached[i : i + CACHED_PIECE_CHARS]})
            yield sse({"type": "done", **(done_extra or {})})
            return

        task = asyncio.ensure_future(prepare(lambda done, total: progress.put_nowait((done, total))))
        while not task.done() or not progress.empty():
            try:
                done, total = await asyncio.wait_for(progress.get(), timeout=0.2)
            except TimeoutError:
                continue
            yield sse({"type": "status", "stage": "summarizing", "done": done, "total": total})
        prompt = task.result()

        yield sse({"type": "status", "stage": "writing"})
        pieces: list[str] = []
        async for piece in stream_llm(prompt):
            pieces.append(piece)
            yield sse({"type": "delta", "text": piece})
        if not pieces:
            raise ValueError("The model returned an empty or invalid summary.")
        if cache_key:
            summary_cache.put(cache_key, "".join(pieces))

        yield sse({"type": "done", **(done_extra or {})})
    except Exception as exc:
        error = to_http_error(exc)
        yield sse({"type": "error", "status": error.status_code, "message": error.detail})
    finally:
        if task and not task.done():
            task.cancel()


def sse_response(stream: AsyncIterator[str]) -> StreamingResponse:
    return StreamingResponse(
        stream,
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},  # no proxy buffering
    )


# ---- Endpoints ----------------------------------------------------------------------------


@app.post("/summarize")
async def summarize_file(
    file: UploadFile = File(...),
    word_count: int = Form(150),
    page_limit: int = Form(0),
    style: str = Form(DEFAULT_STYLE),
    language: str = Form(DEFAULT_LANGUAGE),
    stream: bool = Query(False),
):
    validate_options(word_count, page_limit, style, language)
    text = await read_pdf_upload(file)
    check_text(text)

    if stream:

        def prepare(progress):
            return prepare_summary_prompt(text, word_count, page_limit, style, language, progress)

        key = text_cache_key(text, word_count, page_limit, style, language)
        return sse_response(summary_event_stream(prepare, {"filename": file.filename, "text": text}, key))

    summary = await summarize_or_fail(text, word_count, page_limit, style, language)
    return {"filename": file.filename, "summary": summary, "text": text}


@app.post("/summarize-text")
async def summarize_text(
    payload: TextPayload,
    word_count: int = Query(150),
    page_limit: int = Query(0),
    style: str = Query(DEFAULT_STYLE),
    language: str = Query(DEFAULT_LANGUAGE),
    stream: bool = Query(False),
):
    validate_options(word_count, page_limit, style, language)
    check_text(payload.text)

    if stream:

        def prepare(progress):
            return prepare_summary_prompt(payload.text, word_count, page_limit, style, language, progress)

        key = text_cache_key(payload.text, word_count, page_limit, style, language)
        return sse_response(summary_event_stream(prepare, None, key))

    summary = await summarize_or_fail(payload.text, word_count, page_limit, style, language)
    return {"summary": summary}


@app.post("/summarize-multiple")
async def summarize_multiple(
    files: list[UploadFile] = File(...),
    word_count: int = Form(150),
    page_limit: int = Form(0),
    style: str = Form(DEFAULT_STYLE),
    language: str = Form(DEFAULT_LANGUAGE),
    stream: bool = Query(False),
):
    validate_options(word_count, page_limit, style, language)
    if not 1 <= len(files) <= MAX_FILES:
        raise HTTPException(422, f"Upload between 1 and {MAX_FILES} PDF files")

    docs = [(file.filename or "document.pdf", await read_pdf_upload(file)) for file in files]
    check_documents(docs)
    extra = {"filename": label_for(docs), "text": combine_documents(docs)}

    key = multi_cache_key(docs, word_count, page_limit, style, language)
    if stream:

        def prepare(progress):
            return prepare_multi_prompt(docs, word_count, page_limit, style, language, progress)

        return sse_response(summary_event_stream(prepare, extra, key))

    try:
        summary = summary_cache.get(key)
        if summary is None:
            summary = await process_multi_summary(docs, word_count, page_limit, style, language)
            summary_cache.put(key, summary)
    except Exception as exc:
        raise to_http_error(exc) from exc
    return {"filename": extra["filename"], "summary": summary, "text": extra["text"]}


@app.post("/chat")
async def chat(payload: ChatPayload):
    question = payload.question.strip()
    if not question:
        raise HTTPException(422, "Question is required")
    if len(question) > MAX_QUESTION_CHARS:
        raise HTTPException(422, f"Question is too long (max {MAX_QUESTION_CHARS} characters)")
    if not payload.text.strip():
        raise HTTPException(422, "This document has no text to chat with")
    if len(payload.text) > MAX_MULTI_TEXT_CHARS + 10_000:
        raise HTTPException(413, "Document is too long")

    history = [
        {"role": m.role, "content": m.content.strip()[:MAX_HISTORY_MESSAGE_CHARS]}
        for m in (payload.history or [])[-MAX_HISTORY_MESSAGES:]
        if m.role in ("user", "assistant") and m.content.strip()
    ]

    try:
        context = select_context(payload.text, question, CHAT_CONTEXT_CHARS)
        answer = await call_llm(chat_prompt(context, history, question))
    except Exception as exc:
        raise to_http_error(exc, "The language model failed to answer. Please try again.") from exc
    if not answer:
        raise HTTPException(502, "The language model returned an empty answer. Please try again.")
    return {"answer": answer}


# ---- Summarization pipeline ---------------------------------------------------------------


def label_for(docs: list[tuple[str, str]]) -> str:
    names = [name for name, _ in docs]
    if len(names) <= 2:
        return ", ".join(names)
    return f"{names[0]}, {names[1]} (+{len(names) - 2} more)"


def combine_documents(docs: list[tuple[str, str]]) -> str:
    return "\n\n".join(f"=== {name} ===\n{text}" for name, text in docs)


async def summarize_or_fail(text: str, word_count: int, page_limit: int, style: str, language: str) -> str:
    """Runs the summarizer, turning failures into client-safe HTTP errors."""
    check_text(text)
    key = text_cache_key(text, word_count, page_limit, style, language)
    cached = summary_cache.get(key)
    if cached is not None:
        return cached
    try:
        summary = await process_summary(text, word_count, page_limit, style, language)
    except Exception as exc:
        raise to_http_error(exc) from exc
    summary_cache.put(key, summary)
    return summary


def text_cache_key(text: str, word_count: int, page_limit: int, style: str, language: str) -> str:
    return summary_key(
        "text", text, target_words_for(word_count, page_limit), style, language, active_model_or_none(), PROMPT_VERSION
    )


def multi_cache_key(docs: list[tuple[str, str]], word_count: int, page_limit: int, style: str, language: str) -> str:
    return summary_key(
        "docs",
        combine_documents(docs),
        target_words_for(word_count, page_limit),
        style,
        language,
        active_model_or_none(),
        PROMPT_VERSION,
    )


def active_model_or_none() -> str:
    try:
        return active_model()
    except RuntimeError:
        return "none"


def split_text(text: str) -> list[str]:
    splitter = RecursiveCharacterTextSplitter(chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP)
    return [chunk.page_content for chunk in splitter.create_documents([text])]


async def condense(text: str, progress: Optional[ProgressCallback] = None) -> str:
    """Map step: summarize each chunk, repeating until the result fits in a single reduce prompt.

    `progress(done, total)` is called as each chunk of the current round finishes."""
    while True:
        chunks = split_text(text)
        finished = 0

        async def summarize_chunk(chunk: str, total: int) -> str:
            nonlocal finished
            result = await call_llm(
                "Summarize the following text concisely, focusing on the key points:\n\n---\n\n" + chunk
            )
            finished += 1
            if progress:
                progress(finished, total)
            return result

        results = await asyncio.gather(*(summarize_chunk(chunk, len(chunks)) for chunk in chunks))
        combined = "\n\n".join(r for r in results if r)
        if not combined:
            raise ValueError("Failed to generate intermediate summaries from the document.")
        # A single pass over a single chunk can't shrink further; stop to avoid looping forever.
        if len(combined) <= REDUCE_MAX_CHARS or len(chunks) == 1:
            return combined
        text = combined


def target_words_for(word_count: int, page_limit: int) -> int:
    return page_limit * WORDS_PER_PAGE if page_limit > 0 else word_count


async def prepare_summary_prompt(
    text: str,
    word_count: int,
    page_limit: int,
    style: str = DEFAULT_STYLE,
    language: str = DEFAULT_LANGUAGE,
    progress: Optional[ProgressCallback] = None,
) -> str:
    """Builds the final prompt, doing any map-reduce condensing it needs first."""
    target_words = target_words_for(word_count, page_limit)
    if page_limit > 0 or len(text) > SINGLE_SHOT_MAX_CHARS:
        return summary_prompt(await condense(text, progress), target_words, style, language, kind="summaries")
    return summary_prompt(text, target_words, style, language, kind="text")


async def process_summary(
    text: str,
    word_count: int,
    page_limit: int,
    style: str = DEFAULT_STYLE,
    language: str = DEFAULT_LANGUAGE,
) -> str:
    prompt = await prepare_summary_prompt(text, word_count, page_limit, style, language)
    summary = await call_llm(prompt)
    if not summary:
        raise ValueError("The model returned an empty or invalid summary.")
    return summary


async def prepare_multi_prompt(
    docs: list[tuple[str, str]],
    word_count: int,
    page_limit: int,
    style: str = DEFAULT_STYLE,
    language: str = DEFAULT_LANGUAGE,
    progress: Optional[ProgressCallback] = None,
) -> str:
    """Builds the combined prompt for several documents. Long documents are condensed first so
    the combined material fits in a single prompt and each document keeps its share of it."""

    async def prepare(name: str, text: str) -> str:
        body = await condense(text, progress) if len(text) > SINGLE_SHOT_MAX_CHARS // len(docs) else text
        return f"### {name}\n{body}"

    sections = await asyncio.gather(*(prepare(name, text) for name, text in docs))
    material = "\n\n".join(sections)
    if len(material) > REDUCE_MAX_CHARS:
        material = await condense(material, progress)

    return summary_prompt(material, target_words_for(word_count, page_limit), style, language, kind="documents")


async def process_multi_summary(
    docs: list[tuple[str, str]],
    word_count: int,
    page_limit: int,
    style: str = DEFAULT_STYLE,
    language: str = DEFAULT_LANGUAGE,
) -> str:
    prompt = await prepare_multi_prompt(docs, word_count, page_limit, style, language)
    summary = await call_llm(prompt)
    if not summary:
        raise ValueError("The model returned an empty or invalid summary.")
    return summary
