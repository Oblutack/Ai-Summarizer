import asyncio
import json
import logging
import os
import tempfile
from contextlib import asynccontextmanager
from typing import AsyncIterator, Awaitable, Callable, Optional

from dotenv import load_dotenv
from fastapi import FastAPI, File, Form, HTTPException, Query, UploadFile
from fastapi.responses import StreamingResponse
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_community.document_loaders import PyPDFLoader
from pydantic import BaseModel

from llm import active_model, candidate_models, get_llm, is_model_missing, mark_unavailable, verify_models
from prompts import (
    DEFAULT_LANGUAGE,
    DEFAULT_STYLE,
    LANGUAGES,
    STYLE_INSTRUCTIONS,
    chat_prompt,
    is_valid_language,
    is_valid_style,
    summary_prompt,
)
from retrieval import select_context

load_dotenv()

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


async def call_llm(prompt: str) -> str:
    async with _slots():
        # If the provider says the model no longer exists, move to the next candidate and retry.
        for _ in candidate_models():
            model = active_model()
            try:
                response = await get_llm().ainvoke(prompt)
                return (response.content or "").strip()
            except Exception as exc:
                if not is_model_missing(exc):
                    raise
                mark_unavailable(model)
    raise RuntimeError("No configured language model is available")


async def stream_llm(prompt: str) -> AsyncIterator[str]:
    """Yields the model's reply piece by piece, with the same model fallback as call_llm."""
    async with _slots():
        for _ in candidate_models():
            model = active_model()
            started = False
            try:
                async for chunk in get_llm().astream(prompt):
                    text = chunk.content or ""
                    if text:
                        started = True
                        yield text
                return
            except Exception as exc:
                # Only fall back if nothing was sent yet; a half-written answer can't be resumed.
                if started or not is_model_missing(exc):
                    raise
                mark_unavailable(model)
    raise RuntimeError("No configured language model is available")


@app.get("/")
def read_root():
    return {"message": "Python AI Service is running"}


@app.get("/healthz")
def healthz():
    """Liveness: the process is up. Deliberately independent of the LLM provider."""
    return {"status": "ok"}


@app.get("/readyz")
async def readyz():
    """Readiness: we hold an API key and at least one configured model exists at the provider."""
    if not os.getenv("GROQ_API_KEY"):
        raise HTTPException(503, "GROQ_API_KEY is not set")
    model = await verify_models()
    if model is None:
        raise HTTPException(503, "No usable language model (provider unreachable or models retired)")
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
    return "\n".join(doc.page_content for doc in PyPDFLoader(path).load())


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
        except Exception:
            logger.exception("Failed to parse PDF")
            raise HTTPException(
                422, f"Could not read {file.filename}. It may be corrupted or password-protected."
            )
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


async def summary_event_stream(
    prepare: Callable[[Optional[ProgressCallback]], Awaitable[str]],
    done_extra: Optional[dict] = None,
) -> AsyncIterator[str]:
    """Runs `prepare` (which may do map-reduce work and reports progress), then streams the
    final summary as it is generated. Failures become a final "error" event, since by the time
    they happen the HTTP status line has already been sent."""
    progress: asyncio.Queue = asyncio.Queue()
    task: Optional[asyncio.Task] = None
    try:
        yield sse({"type": "status", "stage": "preparing"})

        task = asyncio.create_task(prepare(lambda done, total: progress.put_nowait((done, total))))
        while not task.done() or not progress.empty():
            try:
                done, total = await asyncio.wait_for(progress.get(), timeout=0.2)
            except asyncio.TimeoutError:
                continue
            yield sse({"type": "status", "stage": "summarizing", "done": done, "total": total})
        prompt = task.result()

        yield sse({"type": "status", "stage": "writing"})
        wrote_anything = False
        async for piece in stream_llm(prompt):
            wrote_anything = True
            yield sse({"type": "delta", "text": piece})
        if not wrote_anything:
            raise ValueError("The model returned an empty or invalid summary.")

        yield sse({"type": "done", **(done_extra or {})})
    except HTTPException as exc:
        yield sse({"type": "error", "status": exc.status_code, "message": exc.detail})
    except Exception:
        logger.exception("Streaming summarization failed")
        yield sse({"type": "error", "status": 502, "message": SUMMARY_FAILED})
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
        prepare = lambda progress: prepare_summary_prompt(text, word_count, page_limit, style, language, progress)
        return sse_response(summary_event_stream(prepare, {"filename": file.filename, "text": text}))

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
        prepare = lambda progress: prepare_summary_prompt(payload.text, word_count, page_limit, style, language, progress)
        return sse_response(summary_event_stream(prepare))

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

    if stream:
        prepare = lambda progress: prepare_multi_prompt(docs, word_count, page_limit, style, language, progress)
        return sse_response(summary_event_stream(prepare, extra))

    try:
        summary = await process_multi_summary(docs, word_count, page_limit, style, language)
    except HTTPException:
        raise
    except Exception:
        logger.exception("Multi-document summarization failed")
        raise HTTPException(502, SUMMARY_FAILED)
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
    except Exception:
        logger.exception("Chat failed")
        raise HTTPException(502, "The language model failed to answer. Please try again.")
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
    try:
        return await process_summary(text, word_count, page_limit, style, language)
    except HTTPException:
        raise
    except Exception:
        logger.exception("Summarization failed")
        raise HTTPException(502, SUMMARY_FAILED)


def split_text(text: str) -> list[str]:
    splitter = RecursiveCharacterTextSplitter(chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP)
    return [chunk.page_content for chunk in splitter.create_documents([text])]


async def condense(text: str, progress: Optional[ProgressCallback] = None) -> str:
    """Map step: summarize each chunk, repeating until the result fits in a single reduce prompt.

    `progress(done, total)` is called as each chunk of the current round finishes."""
    while True:
        chunks = split_text(text)
        finished = 0

        async def summarize_chunk(chunk: str) -> str:
            nonlocal finished
            result = await call_llm(
                "Summarize the following text concisely, focusing on the key points:\n\n---\n\n" + chunk
            )
            finished += 1
            if progress:
                progress(finished, len(chunks))
            return result

        results = await asyncio.gather(*(summarize_chunk(chunk) for chunk in chunks))
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
