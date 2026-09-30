import asyncio
import logging
import os
import tempfile
from functools import lru_cache

from dotenv import load_dotenv
from fastapi import FastAPI, File, Form, HTTPException, Query, UploadFile
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_community.document_loaders import PyPDFLoader
from langchain_openai import ChatOpenAI
from pydantic import BaseModel

load_dotenv()

logger = logging.getLogger("ai-summarizer")

MAX_PDF_BYTES = 10 * 1024 * 1024
MAX_TEXT_CHARS = 200_000
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
LLM_TIMEOUT_SECONDS = 60
LLM_MAX_RETRIES = 5  # the OpenAI client backs off exponentially on 429/5xx

app = FastAPI(title="AI Summarizer Service")

_llm_slots: dict[asyncio.AbstractEventLoop, asyncio.Semaphore] = {}


class TextPayload(BaseModel):
    text: str


@lru_cache(maxsize=1)
def get_llm() -> ChatOpenAI:
    api_key = os.getenv("GROQ_API_KEY")
    if not api_key:
        raise RuntimeError("GROQ_API_KEY is not set")
    return ChatOpenAI(
        model="llama-3.1-8b-instant",
        api_key=api_key,
        base_url="https://api.groq.com/openai/v1",
        timeout=LLM_TIMEOUT_SECONDS,
        max_retries=LLM_MAX_RETRIES,
    )


async def call_llm(prompt: str) -> str:
    # A semaphore is bound to the loop that first awaits it, so keep one per running loop.
    loop = asyncio.get_running_loop()
    slots = _llm_slots.setdefault(loop, asyncio.Semaphore(MAX_CONCURRENT_LLM_CALLS))
    async with slots:
        response = await get_llm().ainvoke(prompt)
    return (response.content or "").strip()


@app.get("/")
def read_root():
    return {"message": "Python AI Service is running"}


def validate_options(word_count: int, page_limit: int) -> None:
    if not MIN_WORDS <= word_count <= MAX_WORDS:
        raise HTTPException(422, f"word_count must be between {MIN_WORDS} and {MAX_WORDS}")
    if not 0 <= page_limit <= MAX_PAGE_LIMIT:
        raise HTTPException(422, f"page_limit must be between 0 and {MAX_PAGE_LIMIT}")


def extract_pdf_text(path: str) -> str:
    return "\n".join(doc.page_content for doc in PyPDFLoader(path).load())


@app.post("/summarize")
async def summarize_file(
    file: UploadFile = File(...),
    word_count: int = Form(150),
    page_limit: int = Form(0),
):
    validate_options(word_count, page_limit)

    content = await file.read(MAX_PDF_BYTES + 1)
    if len(content) > MAX_PDF_BYTES:
        raise HTTPException(413, "PDF is too large (max 10 MB)")

    temp_pdf_path = None
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as temp_pdf:
            temp_pdf.write(content)
            temp_pdf_path = temp_pdf.name

        try:
            text = await asyncio.to_thread(extract_pdf_text, temp_pdf_path)
        except Exception:
            logger.exception("Failed to parse PDF")
            raise HTTPException(422, "Could not read this PDF. It may be corrupted or password-protected.")

        summary = await summarize_or_fail(text, word_count, page_limit)
        return {"filename": file.filename, "summary": summary}
    finally:
        if temp_pdf_path and os.path.exists(temp_pdf_path):
            os.unlink(temp_pdf_path)


@app.post("/summarize-text")
async def summarize_text(
    payload: TextPayload,
    word_count: int = Query(150),
    page_limit: int = Query(0),
):
    validate_options(word_count, page_limit)
    summary = await summarize_or_fail(payload.text, word_count, page_limit)
    return {"summary": summary}


async def summarize_or_fail(text: str, word_count: int, page_limit: int) -> str:
    """Runs the summarizer, turning failures into client-safe HTTP errors."""
    if not text.strip():
        raise HTTPException(422, "No text found to summarize. Scanned PDFs (images) are not supported.")
    if len(text) > MAX_TEXT_CHARS:
        raise HTTPException(413, f"Text is too long (max {MAX_TEXT_CHARS} characters)")

    try:
        return await process_summary(text, word_count, page_limit)
    except HTTPException:
        raise
    except Exception:
        logger.exception("Summarization failed")
        raise HTTPException(502, "The language model failed to produce a summary. Please try again.")


def split_text(text: str) -> list[str]:
    splitter = RecursiveCharacterTextSplitter(chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP)
    return [chunk.page_content for chunk in splitter.create_documents([text])]


async def condense(text: str) -> str:
    """Map step: summarize each chunk, repeating until the result fits in a single reduce prompt."""
    while True:
        chunks = split_text(text)
        results = await asyncio.gather(
            *(
                call_llm(
                    "Summarize the following text concisely, focusing on the key points:\n\n---\n\n" + chunk
                )
                for chunk in chunks
            )
        )
        combined = "\n\n".join(r for r in results if r)
        if not combined:
            raise ValueError("Failed to generate intermediate summaries from the document.")
        # A single pass over a single chunk can't shrink further; stop to avoid looping forever.
        if len(combined) <= REDUCE_MAX_CHARS or len(chunks) == 1:
            return combined
        text = combined


async def process_summary(text: str, word_count: int, page_limit: int) -> str:
    if page_limit > 0 or len(text) > SINGLE_SHOT_MAX_CHARS:
        target_words = page_limit * WORDS_PER_PAGE if page_limit > 0 else word_count
        combined = await condense(text)
        prompt = f"""Condense and combine the following summaries into a single, well-structured text of about {target_words} words.
        **Format the entire output strictly as Markdown.**
        Use headings (#, ##), bullet points (*), and bold text (**) to organize the information clearly and improve readability.

        ---

        {combined}"""
    else:
        prompt = f"""Provide a summary of the following text in about {word_count} words.
        **Format the output strictly as Markdown.**
        Use headings, bullet points, and bold text where appropriate to structure the key information.

        ---

        {text}"""

    summary = await call_llm(prompt)
    if not summary:
        raise ValueError("The model returned an empty or invalid summary.")
    return summary
