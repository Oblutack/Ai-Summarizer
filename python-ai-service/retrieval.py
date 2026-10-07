"""Dependency-free BM25 retrieval: picks the document passages relevant to a question, and says where
in the document each one came from so answers can cite their sources."""

import math
import re
from collections import Counter
from dataclasses import dataclass

from langchain_text_splitters import RecursiveCharacterTextSplitter

CHUNK_SIZE = 1_500
CHUNK_OVERLAP = 150
K1 = 1.5
B = 0.75

# PDF text keeps its page boundaries as form feeds (see main.extract_pdf_text); a form feed is the
# classic page-break character and reads as plain whitespace to everything else.
PAGE_BREAK = "\f"

# Several PDFs are combined as "=== name ===" headers followed by each file's text.
_DOCUMENT_HEADER = re.compile(r"^=== (.+) ===$", re.MULTILINE)

_TOKEN = re.compile(r"\w+", re.UNICODE)


def tokenize(text: str) -> list[str]:
    return [t for t in _TOKEN.findall(text.lower()) if len(t) > 2]


@dataclass(frozen=True)
class Passage:
    """A piece of the document, numbered for citation. Passages are numbered in document order."""

    id: int
    text: str
    page: int | None = None  # first page the passage touches (counted within its own file); None if unknown
    page_end: int | None = None
    document: str | None = None  # file name, when several files were combined


def _segments(text: str) -> list[tuple[int, int]]:
    """Start and end offsets of each page of each file. Chunks never cross a segment, so every
    passage lies on one page and a citation can name it exactly."""
    cuts = {0, len(text)}
    cuts.update(pos for pos, _ in document_headers(text))
    at = text.find(PAGE_BREAK)
    while at != -1:
        cuts.update((at, at + 1))
        at = text.find(PAGE_BREAK, at + 1)
    ordered = sorted(cuts)
    return list(zip(ordered, ordered[1:], strict=False))


def _chunks(text: str) -> list[tuple[str, int]]:
    """Splits into overlapping chunks, each with the offset where it starts in `text`. Pages (and files)
    are split first, so a chunk never spans two of them."""
    splitter = RecursiveCharacterTextSplitter(chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP, add_start_index=True)
    chunks = []
    for start, end in _segments(text):
        segment = text[start:end]
        if not segment.strip():  # a bare page break, or a page with no text (such as a scanned image)
            continue
        for d in splitter.create_documents([segment]):
            chunks.append((d.page_content, start + d.metadata["start_index"]))
    return chunks


def _rank(chunks: list[str], question: str) -> list[int]:
    """Chunk indexes from best to worst match for the question (ties keep document order)."""
    tokenized = [tokenize(c) for c in chunks]
    n = len(chunks)
    avg_len = sum(len(t) for t in tokenized) / n or 1
    doc_freq = Counter(term for tokens in tokenized for term in set(tokens))
    query_terms = set(tokenize(question))

    def score(tokens: list[str]) -> float:
        counts = Counter(tokens)
        total = 0.0
        for term in query_terms:
            tf = counts.get(term, 0)
            if not tf:
                continue
            idf = math.log(1 + (n - doc_freq[term] + 0.5) / (doc_freq[term] + 0.5))
            total += idf * tf * (K1 + 1) / (tf + K1 * (1 - B + B * len(tokens) / avg_len))
        return total

    return sorted(range(n), key=lambda i: (-score(tokenized[i]), i))


def _within_budget(chunks: list[str], ranked: list[int], max_chars: int) -> list[int]:
    """The best chunks that fit the character budget, in document order."""
    chosen: list[int] = []
    used = 0
    for i in ranked:
        if used + len(chunks[i]) > max_chars and chosen:
            continue
        chosen.append(i)
        used += len(chunks[i])
        if used >= max_chars:
            break
    return sorted(chosen)


def select_context(text: str, question: str, max_chars: int) -> str:
    """Returns the whole text if it fits, else the chunks that best match the question, in document order."""
    if len(text) <= max_chars:
        return text
    chunks = [c for c, _ in _chunks(text)]
    return "\n...\n".join(chunks[i] for i in _within_budget(chunks, _rank(chunks, question), max_chars))


def locate(text: str, offset: int, headers: list[tuple[int, str]] | None = None) -> tuple[str | None, int | None]:
    """The file name and page that the character at `offset` belongs to, or None where unknown."""
    if headers is None:
        headers = document_headers(text)
    document, section_start = None, 0
    for pos, name in headers:
        if pos > offset:
            break
        document, section_start = name, pos
    if PAGE_BREAK not in text:
        return document, None
    return document, text.count(PAGE_BREAK, section_start, offset) + 1


def document_headers(text: str) -> list[tuple[int, str]]:
    """Offsets and names of the "=== name ===" headers of a combined multi-file text (else none)."""
    if not text.startswith("=== "):
        return []
    return [(m.start(), m.group(1)) for m in _DOCUMENT_HEADER.finditer(text)]


def select_passages(text: str, question: str, max_chars: int) -> list[Passage]:
    """Numbered passages to answer from: all of the document if it fits, else the best matches.
    Each carries its page and file so the answer can cite it."""
    chunks = _chunks(text)
    contents = [c for c, _ in chunks]
    if len(text) <= max_chars:
        picked = list(range(len(chunks)))
    else:
        picked = _within_budget(contents, _rank(contents, question), max_chars)

    headers = document_headers(text)
    passages = []
    for number, i in enumerate(picked, start=1):
        content, start = chunks[i]
        document, page = locate(text, start, headers)
        page_end = page + content.count(PAGE_BREAK) if page is not None else None
        passages.append(
            Passage(
                id=number,
                text=content.replace(PAGE_BREAK, "\n").strip(),
                page=page,
                page_end=page_end,
                document=document,
            )
        )
    return passages
