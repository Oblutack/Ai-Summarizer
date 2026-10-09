"""Reading scanned pages: a PDF that is only pictures of text gets its words from an OCR engine (Tesseract).

Why Tesseract and not a vision model: a language model asked to "read" a page will quietly fix, complete or invent
what it cannot make out, and the proof check (attribution.py) would then verify a summary against text that was
itself made up. An OCR engine only reports what it sees, runs here at no cost, and the page never leaves the server.

Only the pages that have no text are read this way, so a PDF with a few scanned pages among real ones keeps its own
text. Page boundaries stay (retrieval.PAGE_BREAK), so a citation still says "page 7".

The pages are drawn with PDFium (pypdfium2) at a size Tesseract reads well, and sent to the `tesseract` program one
at a time over standard input. Scanned pages are untrusted input and OCR is real work, so the number of pages, the
size of each picture and the time are all capped.
"""

import io
import logging
import os
import shutil
import subprocess
import time
from concurrent.futures import ThreadPoolExecutor
from typing import Optional

logger = logging.getLogger("ai-summarizer")

# Latin-script languages that are installed in the image. More languages in one run read each of them a little
# worse, so this is a choice (OCR_LANGUAGES, joined with "+" as Tesseract wants).
LANGUAGES = os.getenv("OCR_LANGUAGES", "eng+deu+fra+spa+ita+por+hrv")
# The most scanned pages read in one document, and the most time spent on all of them.
MAX_PAGES = int(os.getenv("OCR_MAX_PAGES", "40"))
TOTAL_SECONDS = float(os.getenv("OCR_TOTAL_SECONDS", "150"))
PAGE_TIMEOUT_SECONDS = 60.0
WORKERS = int(os.getenv("OCR_WORKERS", str(min(4, os.cpu_count() or 1))))

# About 200 dots per inch reads well and keeps a page small; PDF pages are 72 points to the inch.
DPI = 200
# No picture bigger than this on a side, whatever the page says its size is (a page can claim to be metres wide).
MAX_SIDE_PIXELS = 4200
# A page with fewer letters and digits than this has no text of its own.
MIN_CHARACTERS = 12


class OcrError(Exception):
    """A scanned page could not be read. `status` is the HTTP status to report, `message` is safe to show."""

    def __init__(self, status: int, message: str):
        super().__init__(message)
        self.status = status
        self.message = message


def available() -> bool:
    return shutil.which("tesseract") is not None


def characters(text: str) -> int:
    """How many letters and digits a text has."""
    return sum(1 for ch in text if ch.isalnum())


def needs_ocr(page_text: str) -> bool:
    """Whether a page has too little text of its own to be a real page of text (a picture of text, or an empty page)."""
    return characters(page_text) < MIN_CHARACTERS


def render_page(pdf: bytes, index: int) -> bytes:
    """One page of a PDF drawn as a grey PNG picture."""
    import pypdfium2 as pdfium

    document = pdfium.PdfDocument(pdf)
    try:
        page = document[index]
        try:
            width, height = page.get_size()
            scale = DPI / 72
            if max(width, height) * scale > MAX_SIDE_PIXELS:
                scale = MAX_SIDE_PIXELS / max(width, height)
            image = page.render(scale=scale).to_pil().convert("L")
        finally:
            page.close()
    finally:
        document.close()
    buffer = io.BytesIO()
    image.save(buffer, format="PNG")
    return buffer.getvalue()


def run_tesseract(
    png: bytes, languages: str, timeout: float, *options: str, must_succeed: bool = True
) -> "subprocess.CompletedProcess[bytes]":
    """Runs the tesseract program on a picture and returns what it finished with (its output is in `.stdout`)."""
    try:
        done = subprocess.run(
            ["tesseract", "stdin", "stdout", "-l", languages, *options],
            input=png,
            capture_output=True,
            timeout=timeout,
            # One thread per page: the pages are read side by side instead.
            env={**os.environ, "OMP_THREAD_LIMIT": "1"},
            check=False,
        )
    except FileNotFoundError as exc:
        raise OcrError(503, "Reading scanned pages is not available on this server.") from exc
    except subprocess.TimeoutExpired as exc:
        raise OcrError(504, "Reading a scanned page took too long.") from exc
    if must_succeed and done.returncode != 0:
        logger.error("tesseract failed (exit %s): %s", done.returncode, done.stderr[:300].decode("utf-8", "replace"))
        raise OcrError(422, "A scanned page could not be read. The file may be damaged.")
    return done


def read_picture(png: bytes, languages: str = LANGUAGES, timeout: float = PAGE_TIMEOUT_SECONDS) -> str:
    """The text Tesseract finds in a picture."""
    return run_tesseract(png, languages, timeout, "--psm", "3").stdout.decode("utf-8", "replace").strip()


def read_pages(pdf: bytes, indexes: list[int], languages: Optional[str] = None) -> dict[int, str]:
    """The text of the given pages (counting from 0), read side by side, in a limited time."""
    if len(indexes) > MAX_PAGES:
        raise OcrError(413, f"This PDF has {len(indexes)} scanned pages; at most {MAX_PAGES} can be read at once.")
    deadline = time.monotonic() + TOTAL_SECONDS
    chosen = languages or LANGUAGES

    def one(index: int) -> tuple[int, str]:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise OcrError(504, "Reading the scanned pages took too long. Try a PDF with fewer pages.")
        try:
            png = render_page(pdf, index)
        except OcrError:
            raise
        except Exception as exc:
            logger.warning("Could not draw page %s of a PDF: %s", index + 1, type(exc).__name__)
            raise OcrError(422, "A page of this PDF could not be drawn to be read. The file may be damaged.") from exc
        return index, read_picture(png, chosen, min(PAGE_TIMEOUT_SECONDS, remaining))

    with ThreadPoolExecutor(max_workers=max(1, WORKERS)) as pool:
        return dict(pool.map(one, indexes))
