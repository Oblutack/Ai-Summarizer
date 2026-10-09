import io
import random
import subprocess

import numpy as np
import pytest
from fastapi.testclient import TestClient
from PIL import Image, ImageDraw, ImageEnhance, ImageFilter, ImageFont

import main
import ocr
import photos
from retrieval import PAGE_BREAK

LEASE = [
    "RESIDENTIAL LEASE AGREEMENT",
    "",
    "The tenant pays 950 euros per month,",
    "due on the fifth day of each month.",
    "",
    "Pets are allowed only with written",
    "permission of the landlord.",
    "Deposit: 1,900 euros.",
]
FACTS = ("950", "euros", "Pets", "1,900")


def page_of(lines, size=(1240, 1754), ink=30, paper=235):
    image = Image.new("L", size, paper)
    draw = ImageDraw.Draw(image)
    font = ImageFont.load_default(size=34)
    for i, line in enumerate(lines):
        draw.text((90, 140 + i * 56), line, fill=ink, font=font)
    return image


def photographed(page, angle=0.0, seed=3):
    """What a phone makes of a page: one side in shadow, a little blur, noise and a slight turn."""
    random.seed(seed)
    shade = Image.linear_gradient("L").resize(page.size).rotate(30)
    image = Image.composite(page, ImageEnhance.Brightness(page).enhance(0.55), shade)
    image = image.rotate(angle, expand=True, fillcolor=120).filter(ImageFilter.GaussianBlur(1.0))
    pixels = image.load()
    for _ in range(40000):
        x, y = random.randrange(image.width), random.randrange(image.height)
        pixels[x, y] = max(0, min(255, pixels[x, y] + random.randint(-40, 40)))
    return image


def encoded(image, fmt="JPEG", **options):
    buffer = io.BytesIO()
    image.save(buffer, format=fmt, **options)
    return buffer.getvalue()


# --- which files are photos ---


def test_photos_are_known_by_their_extension():
    for name in ("page.jpg", "PAGE.JPEG", "scan.Png", "web.webp", "my photo.final.jpg"):
        assert photos.is_image(name), name
    for name in ("page.pdf", "notes.docx", "a.gif", "a.heic", "jpg", "photo.jpg.pdf", "meeting.mp3"):
        assert not photos.is_image(name), name


# --- preparing a picture ---


def test_a_picture_is_turned_the_way_the_file_says():
    sideways = Image.new("RGB", (400, 200), "white")  # the phone saw a portrait page as landscape
    exif = Image.Exif()
    exif[0x0112] = 6  # rotate 90 clockwise to display
    out = photos.prepare(encoded(sideways, exif=exif), "p.jpg")
    assert out.height > out.width


def test_a_see_through_png_is_read_on_white_paper():
    image = Image.new("RGBA", (800, 800), (0, 0, 0, 0))
    ImageDraw.Draw(image).rectangle((100, 100, 200, 200), fill=(0, 0, 0, 255))
    out = photos.prepare(encoded(image, "PNG"), "p.png")
    assert out.mode == "L" and out.getpixel((5, 5)) == 255 and out.getpixel((150 * 2, 150 * 2)) < 20


def test_big_pictures_are_made_smaller_and_small_ones_bigger():
    big = photos.prepare(encoded(Image.new("L", (6000, 4000), 255), "PNG"), "big.png")
    assert max(big.size) == ocr.MAX_SIDE_PIXELS
    small = photos.prepare(encoded(Image.new("L", (600, 400), 255), "PNG"), "small.png")
    assert max(small.size) == 1600
    tiny = photos.prepare(encoded(Image.new("L", (100, 50), 255), "PNG"), "tiny.png")
    assert tiny.width == 300  # never more than three times bigger


def test_a_picture_with_too_many_pixels_is_refused_before_it_is_opened_up(monkeypatch):
    monkeypatch.setattr(photos, "MAX_PIXELS", 10_000)
    with pytest.raises(ocr.OcrError) as caught:
        photos.prepare(encoded(Image.new("L", (200, 200), 255), "PNG"), "huge.png")
    assert caught.value.status == 413 and "huge.png" in caught.value.message


def test_what_the_file_really_is_decides_not_its_name():
    for fmt in ("GIF", "BMP", "TIFF"):
        with pytest.raises(ocr.OcrError) as caught:
            photos.prepare(encoded(Image.new("L", (50, 50), 255), fmt), "pretend.jpg")
        assert caught.value.status == 422 and "JPG, PNG or WebP" in caught.value.message, fmt


def test_a_broken_picture_is_a_plain_error_that_tells_nothing_of_the_server():
    for junk in (b"", b"not a picture at all", encoded(Image.new("L", (50, 50), 255), "PNG")[:40]):
        with pytest.raises(ocr.OcrError) as caught:
            photos.prepare(junk, "broken.png")
        assert caught.value.status == 422 and "broken.png" in caught.value.message
        assert "PIL" not in caught.value.message and "Traceback" not in caught.value.message


# --- evening out the light ---


def test_a_shadow_over_the_page_is_removed():
    image = photographed(page_of(LEASE)).convert("L")
    flat = np.asarray(photos.even_out(image), dtype=float)
    # paper on the dark side and on the bright side end up equally white
    left, right = flat[300:1500, 15:65].mean(), flat[300:1500, -65:-15].mean()
    assert left > 215 and right > 215 and abs(left - right) < 25
    assert flat[0:10, :].mean() > 200
    before = np.asarray(image, dtype=float)
    assert abs(before[300:1500, 15:65].mean() - before[300:1500, -65:-15].mean()) > 25, (
        "the test photo really has a shadow"
    )


def test_light_text_on_a_dark_background_is_turned_into_dark_on_light():
    dark = page_of(LEASE, ink=235, paper=25)
    out = np.asarray(photos.even_out(dark))
    assert out.mean() > 200


# --- orientation ---


def fake_tesseract(monkeypatch, stdout=b"", stderr=b"", code=0):
    calls = []

    def run(png, languages, timeout, *options, must_succeed=True):
        calls.append((languages, options, must_succeed))
        return subprocess.CompletedProcess(["tesseract"], code, stdout, stderr)

    monkeypatch.setattr(ocr, "run_tesseract", run)
    return calls


def test_the_turn_tesseract_asks_for_is_read_from_its_report(monkeypatch):
    calls = fake_tesseract(
        monkeypatch, stdout=b"Page number: 0\nOrientation in degrees: 90\nRotate: 270\nOrientation confidence: 2.40\n"
    )
    assert photos.turn_needed(b"png", 10) == 270
    assert calls == [("osd", ("--psm", "0"), False)]


def test_a_guess_it_is_hardly_sure_of_is_ignored(monkeypatch):
    fake_tesseract(monkeypatch, stdout=b"Rotate: 90\nOrientation confidence: 0.20\n")
    assert photos.turn_needed(b"png", 10) == 0


def test_no_report_means_no_turn(monkeypatch):
    fake_tesseract(monkeypatch, stderr=b"Too few characters. Skipping this page\n", code=1)
    assert photos.turn_needed(b"png", 10) == 0


def test_a_missing_tesseract_is_still_reported(monkeypatch):
    def run(*args, **kwargs):
        raise ocr.OcrError(503, "Reading scanned pages is not available on this server.")

    monkeypatch.setattr(ocr, "run_tesseract", run)
    with pytest.raises(ocr.OcrError) as caught:
        photos.turn_needed(b"png", 10)
    assert caught.value.status == 503


# --- the words Tesseract found ---

HEADER = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n"


def word(block, par, line, n, conf, text):
    return f"5\t1\t{block}\t{par}\t{line}\t{n}\t0\t0\t10\t10\t{conf}\t{text}\n"


def test_the_words_are_put_back_into_lines_and_paragraphs_with_how_sure_they_were(monkeypatch):
    tsv = HEADER + "1\t1\t0\t0\t0\t0\t0\t0\t100\t100\t-1\t\n"
    tsv += (
        word(1, 1, 1, 1, 96.5, "The")
        + word(1, 1, 1, 2, 91, "tenant")
        + word(1, 1, 2, 1, 88, "pays")
        + word(1, 1, 2, 2, 20, "950")
    )
    tsv += word(2, 1, 1, 1, 95, "Pets") + word(2, 1, 1, 2, -1, "  ")
    fake_tesseract(monkeypatch, stdout=tsv.encode())
    text, sure = photos.read_words(b"png", "eng", 10)
    assert text == "The tenant\npays 950\n\nPets"
    assert sure == len("The") + len("tenant") + len("pays") + len("Pets"), "only words it was sure of count"


# --- two ways of reading a page ---


@pytest.fixture
def reading(monkeypatch):
    log = {"reads": 0, "turn": 0, "sizes": []}
    monkeypatch.setattr(photos, "prepare", lambda content, name: Image.new("L", (300, 200), 255))
    monkeypatch.setattr(photos, "even_out", lambda image: image)
    monkeypatch.setattr(photos, "turn_needed", lambda png, timeout: log["turn"])

    def read(png, languages, timeout):
        log["reads"] += 1
        log["sizes"].append(Image.open(io.BytesIO(png)).size)
        return log["answers"].pop(0)

    monkeypatch.setattr(photos, "read_words", read)
    return log


def test_an_upright_page_is_read_once(reading):
    reading["answers"] = [("The page text", 11)]
    assert photos.read_photo(b"x", "p.jpg", "eng", 30) == "The page text"
    assert reading["reads"] == 1


def test_a_page_the_detector_thinks_is_sideways_is_read_both_ways_and_the_surer_reading_wins(reading):
    reading["turn"] = 90
    reading["answers"] = [("garbage", 3), ("The real text", 40)]
    assert photos.read_photo(b"x", "p.jpg", "eng", 30) == "The real text"
    assert reading["sizes"] == [(300, 200), (200, 300)], "the second reading is of the turned page"


def test_a_wrong_guess_from_the_detector_does_no_harm(reading):
    reading["turn"] = 180
    reading["answers"] = [("The real text", 40), ("elaas", 2)]
    assert photos.read_photo(b"x", "p.jpg", "eng", 30) == "The real text"


def test_on_a_tie_the_photo_as_it_came_wins(reading):
    reading["turn"] = 90
    reading["answers"] = [("as it came", 10), ("turned", 10)]
    assert photos.read_photo(b"x", "p.jpg", "eng", 30) == "as it came"


def test_reading_a_photo_stops_when_the_time_is_up(reading):
    with pytest.raises(ocr.OcrError) as caught:
        photos.read_photo(b"x", "p.jpg", "eng", 0.0)
    assert caught.value.status == 504


def test_photos_are_read_side_by_side_in_order(monkeypatch):
    monkeypatch.setattr(photos, "read_photo", lambda content, name, languages, timeout: f"{name}:{content.decode()}")
    out = photos.read_photos([("a.jpg", b"1"), ("b.jpg", b"2"), ("c.jpg", b"3")])
    assert out == ["a.jpg:1", "b.jpg:2", "c.jpg:3"]


def test_too_many_photos_are_refused_before_any_work(monkeypatch):
    monkeypatch.setattr(ocr, "MAX_PAGES", 2)
    monkeypatch.setattr(photos, "read_photo", lambda *a: pytest.fail("no photo should be read"))
    with pytest.raises(ocr.OcrError) as caught:
        photos.read_photos([("a.jpg", b""), ("b.jpg", b""), ("c.jpg", b"")])
    assert caught.value.status == 413


# --- through the service ---


class Writer:
    async def ainvoke(self, prompt):
        self.prompt = prompt
        return type("Msg", (), {"content": "A summary."})()


@pytest.fixture
def client(monkeypatch):
    writer = Writer()
    seen = []
    monkeypatch.setattr(main, "get_llm", lambda: writer)
    monkeypatch.setattr(
        photos, "read_photos", lambda items: seen.append([n for n, _ in items]) or [f"Text of {n}" for n, _ in items]
    )
    c = TestClient(main.app, raise_server_exceptions=False)
    c.writer, c.seen = writer, seen
    return c


def upload(name, body=b"x", kind="image/jpeg"):
    return (name, body, kind)


def test_one_photo_is_summarized_when_allowed(client):
    r = client.post("/summarize", files={"file": upload("page.jpg")}, data={"ocr": "true"})
    assert r.status_code == 200 and r.json()["text"] == "Text of page.jpg"
    assert "Text of page.jpg" in client.writer.prompt


def test_a_photo_without_permission_says_to_sign_in(client):
    r = client.post("/summarize", files={"file": upload("page.jpg")})
    assert r.status_code == 422 and "Sign in" in r.json()["detail"]
    assert client.seen == []


def test_several_photos_are_pages_of_one_document(client):
    files = [("files", upload(n)) for n in ("p1.jpg", "p2.jpg", "p3.png")]
    r = client.post("/summarize-multiple", files=files, data={"ocr": "true"})
    assert r.status_code == 200
    assert client.seen == [["p1.jpg", "p2.jpg", "p3.png"]], "read together, in order"
    assert r.json()["filename"] == "p1.jpg (+2 more photos)"
    text = r.json()["text"]
    assert text.split(PAGE_BREAK) == ["Text of p1.jpg", "Text of p2.jpg", "Text of p3.png"]
    assert "=== " not in text, "one document, so no heading per document"
    assert "Text of p2.jpg" in client.writer.prompt


def test_photos_keep_their_place_among_other_files(client, monkeypatch):
    monkeypatch.setattr(main, "extract_upload_text", lambda name, content, allow_ocr=False: f"Words of {name}")
    files = [
        ("files", upload("a.pdf", kind="application/pdf")),
        ("files", upload("p1.jpg")),
        ("files", upload("b.pdf")),
        ("files", upload("p2.jpg")),
    ]
    r = client.post("/summarize-multiple", files=files, data={"ocr": "true"})
    assert r.status_code == 200
    text = r.json()["text"]
    assert text.index("=== a.pdf ===") < text.index("=== p1.jpg (+1 more photos) ===") < text.index("=== b.pdf ===")
    assert client.seen == [["p1.jpg", "p2.jpg"]]


def test_a_photo_problem_reaches_the_caller_as_a_plain_error(client, monkeypatch):
    def fail(items):
        raise ocr.OcrError(422, "Could not read page.jpg. It may be corrupted or not a real picture.")

    monkeypatch.setattr(photos, "read_photos", fail)
    r = client.post("/summarize", files={"file": upload("page.jpg")}, data={"ocr": "true"})
    assert r.status_code == 422 and "page.jpg" in r.json()["detail"]


def test_a_photo_with_no_text_says_so(client, monkeypatch):
    monkeypatch.setattr(photos, "read_photos", lambda items: [""])
    r = client.post("/summarize", files={"file": upload("blank.jpg")}, data={"ocr": "true"})
    assert r.status_code == 422 and "No readable text" in r.json()["detail"]


def test_a_photo_that_is_too_large_is_refused(client, monkeypatch):
    monkeypatch.setattr(main, "MAX_PDF_BYTES", 10)
    r = client.post("/summarize", files={"file": upload("big.jpg", b"x" * 50)}, data={"ocr": "true"})
    assert r.status_code == 413 and client.seen == []


# --- the real thing (needs the tesseract program: the Docker image and CI have it) ---

needs_tesseract = pytest.mark.skipif(not ocr.available(), reason="the tesseract program is not installed here")


def facts_in(text):
    return all(fact in text for fact in FACTS)


@needs_tesseract
def test_a_photo_of_a_page_taken_in_poor_light_is_read():
    text = photos.read_photos([("lease.jpg", encoded(photographed(page_of(LEASE), angle=1.5), quality=80))], "eng")[0]
    assert facts_in(text), text
    assert text.splitlines()[0].startswith("RESIDENTIAL"), "the first line stays first"


@needs_tesseract
@pytest.mark.parametrize("turn", [90, 180, 270])
def test_a_page_photographed_sideways_or_upside_down_is_read(turn):
    sideways = photographed(page_of(LEASE).rotate(turn, expand=True), angle=0.5)
    text = photos.read_photos([("lease.jpg", encoded(sideways, quality=80))], "eng")[0]
    assert facts_in(text), (turn, text)


@needs_tesseract
def test_a_phone_photo_that_is_stored_sideways_with_a_note_is_read():
    stored_sideways = photographed(page_of(LEASE), angle=0.5).rotate(90, expand=True)  # as the sensor saw it
    exif = Image.Exif()
    exif[0x0112] = 6  # "turn it 90 degrees clockwise to display"
    text = photos.read_photos([("IMG_0042.jpg", encoded(stored_sideways, quality=80, exif=exif))], "eng")[0]
    assert facts_in(text), text


@needs_tesseract
def test_a_screenshot_with_light_text_on_dark_is_read():
    dark = page_of(LEASE, ink=235, paper=25)
    text = photos.read_photos([("dark.png", encoded(dark, "PNG"))], "eng")[0]
    assert facts_in(text), text


@needs_tesseract
def test_a_clean_screenshot_is_read_as_well_as_before():
    text = photos.read_photos([("shot.png", encoded(page_of(LEASE, paper=255), "PNG"))], "eng")[0]
    assert facts_in(text), text


@needs_tesseract
def test_photos_of_two_pages_come_back_in_order_and_an_empty_page_stays_empty():
    pages = [
        photographed(page_of(["First page: the rent is 950 euros."])),
        Image.new("L", (900, 1200), 240),
        photographed(page_of(["Third page: pets are allowed."]), 1.0),
    ]
    first, second, third = photos.read_photos(
        [(f"{i}.jpg", encoded(p, quality=80)) for i, p in enumerate(pages)], "eng"
    )
    assert "950" in first and "First" in first
    assert second == ""
    assert "Third" in third and "pets" in third.lower()


@needs_tesseract
def test_two_photos_through_the_service_become_one_summarized_document(monkeypatch):
    writer = Writer()
    monkeypatch.setattr(main, "get_llm", lambda: writer)
    client = TestClient(main.app, raise_server_exceptions=False)
    files = [
        ("files", ("p1.jpg", encoded(photographed(page_of(LEASE)), quality=80), "image/jpeg")),
        ("files", ("p2.png", encoded(page_of(["The cat sat on the mat all day."], paper=255), "PNG"), "image/png")),
    ]
    r = client.post("/summarize-multiple", files=files, data={"ocr": "true"})
    assert r.status_code == 200, r.text
    text = r.json()["text"]
    assert "950" in text and "cat sat" in text and PAGE_BREAK in text
