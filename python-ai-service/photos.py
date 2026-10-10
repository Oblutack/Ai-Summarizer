"""Reading photos of pages: a picture taken with a phone is turned upright, evened out and read with the same
Tesseract that reads scanned PDFs (ocr.py).

A photo is harder than a scan: the phone may have been held sideways (most phones say so in the file, some do not),
one side of the page is in shadow, and the paper is not square to the camera. So before reading:

1. the picture is turned the way the file says, and made grey and no bigger than a scan page;
2. uneven light is evened out: each part is divided by the brightness of the paper around it, so a shadow no longer
   looks like ink (a screenshot with light text on a dark background is turned round first);
3. if Tesseract's orientation detection thinks the page is sideways or upside down, the page is read both ways and
   the way it is more sure about wins. Detection alone is not trusted: on blurry text it is often unsure.

Only what the pictures hold is returned, as text. Several photos are several pages, in the order they came.
"""

import io
import logging
import re
import time
import warnings
from concurrent.futures import ThreadPoolExecutor

import numpy as np
from PIL import Image, ImageFilter, ImageOps

import ocr

logger = logging.getLogger("ai-summarizer")

IMAGE_EXTENSIONS = (".jpg", ".jpeg", ".png", ".webp")
# Judged by what the file really is, not by its name.
IMAGE_FORMATS = ("JPEG", "PNG", "WEBP")

# A picture bigger than this many pixels is refused before it is opened up (a 48-megapixel phone is 48 million).
MAX_PIXELS = 150_000_000
# Words Tesseract is at least this sure about (out of 100) count when two ways of reading a page are compared.
CONFIDENT = 60
# The orientation guess has to be at least this sure before the page is also read turned.
MIN_ORIENTATION_CONFIDENCE = 0.5
# Pictures smaller than this on their longest side are made bigger: small text reads badly.
MIN_SIDE_PIXELS = 1600
MAX_UPSCALE = 3


def is_image(name: str) -> bool:
    return name.lower().endswith(IMAGE_EXTENSIONS)


def prepare(content: bytes, name: str) -> Image.Image:
    """The picture as an upright grey image of a size Tesseract reads well."""
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("ignore", Image.DecompressionBombWarning)
            picture: Image.Image = Image.open(io.BytesIO(content))
            if picture.format not in IMAGE_FORMATS:
                raise ocr.OcrError(422, f"{name} is not a JPG, PNG or WebP picture.")
            if picture.width * picture.height > MAX_PIXELS:
                raise ocr.OcrError(413, f"{name} is too large a picture to read ({picture.width}x{picture.height}).")
            if picture.format == "JPEG":
                picture.draft("L", (ocr.MAX_SIDE_PIXELS, ocr.MAX_SIDE_PIXELS))  # decode small, not then shrink
            picture = ImageOps.exif_transpose(picture)
            if picture.mode in ("RGBA", "LA", "P"):  # a see-through background is white paper
                picture = picture.convert("RGBA")
                flat = Image.new("RGBA", picture.size, "white")
                flat.alpha_composite(picture)
                picture = flat
            picture = picture.convert("L")
    except ocr.OcrError:
        raise
    except Exception as exc:
        logger.warning("Could not open a picture: %s", type(exc).__name__)
        raise ocr.OcrError(422, f"Could not read {name}. It may be corrupted or not a real picture.") from exc

    longest = max(picture.size)
    if longest > ocr.MAX_SIDE_PIXELS:
        picture.thumbnail((ocr.MAX_SIDE_PIXELS, ocr.MAX_SIDE_PIXELS), Image.Resampling.LANCZOS)
    elif longest < MIN_SIDE_PIXELS:
        scale = min(MAX_UPSCALE, MIN_SIDE_PIXELS / longest)
        picture = picture.resize(
            (round(picture.width * scale), round(picture.height * scale)), Image.Resampling.LANCZOS
        )
    return picture


def even_out(picture: Image.Image) -> Image.Image:
    """Removes shadows and uneven light, and stretches the contrast."""
    if np.asarray(picture).mean() < 110:  # light text on a dark background
        picture = ImageOps.invert(picture)
    small = picture.resize((max(1, picture.width // 8), max(1, picture.height // 8)))
    # The paper is the brightest thing around each spot; ink is thinner than the filter, so it drops out.
    paper = small.filter(ImageFilter.MaxFilter(9)).filter(ImageFilter.GaussianBlur(12))
    paper = paper.resize(picture.size, Image.Resampling.BILINEAR)
    flat = np.clip(
        np.asarray(picture, dtype=np.float32) / np.maximum(np.asarray(paper, dtype=np.float32), 1) * 255, 0, 255
    )
    return ImageOps.autocontrast(Image.fromarray(flat.astype(np.uint8)), cutoff=1)


def png_of(picture: Image.Image) -> bytes:
    buffer = io.BytesIO()
    picture.save(buffer, format="PNG")
    return buffer.getvalue()


def turn_needed(png: bytes, timeout: float) -> int:
    """How many degrees clockwise a page must be turned to stand upright, as far as Tesseract can tell (0 if it
    cannot tell)."""
    try:
        done = ocr.run_tesseract(png, "osd", timeout, "--psm", "0", must_succeed=False)
    except ocr.OcrError as exc:
        if exc.status == 503:
            raise
        return 0
    report = done.stdout.decode("utf-8", "replace") + done.stderr.decode("utf-8", "replace")
    turn = re.search(r"Rotate:\s*(\d+)", report)
    sure = re.search(r"Orientation confidence:\s*([\d.]+)", report)
    if not turn or not sure or float(sure.group(1)) < MIN_ORIENTATION_CONFIDENCE:
        return 0
    return int(turn.group(1)) % 360


def read_words(png: bytes, languages: str, timeout: float) -> tuple[str, int]:
    """(the text, how much of it Tesseract was sure of): the letters in words it was at least CONFIDENT about."""
    done = ocr.run_tesseract(png, languages, timeout, "--psm", "3", "tsv")
    lines: dict[tuple[str, str, str], list[str]] = {}
    sure = 0
    for row in done.stdout.decode("utf-8", "replace").splitlines()[1:]:
        cells = row.split("\t")
        if len(cells) < 12 or cells[0] != "5" or not cells[11].strip():
            continue
        lines.setdefault((cells[2], cells[3], cells[4]), []).append(cells[11].strip())
        try:
            if float(cells[10]) >= CONFIDENT:
                sure += len(cells[11].strip())
        except ValueError:
            pass
    # Lines of one paragraph stay together; a new paragraph (or block) starts after a blank line.
    paragraphs: dict[tuple[str, str], list[str]] = {}
    for (block, paragraph, _), words in lines.items():
        paragraphs.setdefault((block, paragraph), []).append(" ".join(words))
    text = "\n\n".join("\n".join(group) for group in paragraphs.values())
    return text, sure


def read_photo(content: bytes, name: str, languages: str, timeout: float) -> str:
    """The text on one photo."""
    started = time.monotonic()
    png = png_of(even_out(prepare(content, name)))
    options = [(png, 0)]
    turn = turn_needed(png, timeout)
    if turn:
        turned = Image.open(io.BytesIO(png)).rotate(-turn, expand=True, fillcolor=255)
        options.append((png_of(turned), turn))
    best_text, best_sure = "", -1
    for candidate, _ in options:
        left = timeout - (time.monotonic() - started)
        if left <= 0:
            raise ocr.OcrError(504, "Reading a photo took too long.")
        text, sure = read_words(candidate, languages, left)
        if sure > best_sure:  # on a tie the photo as it came wins
            best_text, best_sure = text, sure
    return best_text.strip()


def read_photos(items: list[tuple[str, bytes]], languages: str | None = None) -> list[str]:
    """The text of each (file name, picture), read side by side, in a limited time."""
    if len(items) > ocr.MAX_PAGES:
        raise ocr.OcrError(413, f"At most {ocr.MAX_PAGES} photos can be read at once.")
    with ocr.reading_place():
        return _read_all(items, languages)


def _read_all(items: list[tuple[str, bytes]], languages: str | None) -> list[str]:
    deadline = time.monotonic() + ocr.TOTAL_SECONDS
    chosen = languages or ocr.LANGUAGES

    def one(item: tuple[str, bytes]) -> str:
        left = deadline - time.monotonic()
        if left <= 0:
            raise ocr.OcrError(504, "Reading the photos took too long. Try fewer photos.")
        return read_photo(item[1], item[0], chosen, min(ocr.PAGE_TIMEOUT_SECONDS, left))

    with ThreadPoolExecutor(max_workers=max(1, ocr.WORKERS)) as pool:
        return list(pool.map(one, items))
