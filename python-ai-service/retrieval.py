"""Dependency-free BM25 retrieval used to pick relevant document chunks for chat."""

import math
import re
from collections import Counter

from langchain_text_splitters import RecursiveCharacterTextSplitter

CHUNK_SIZE = 1_500
CHUNK_OVERLAP = 150
K1 = 1.5
B = 0.75

_TOKEN = re.compile(r"\w+", re.UNICODE)


def tokenize(text: str) -> list[str]:
    return [t for t in _TOKEN.findall(text.lower()) if len(t) > 2]


def select_context(text: str, question: str, max_chars: int) -> str:
    """Returns the whole text if it fits, else the chunks that best match the question, in document order."""
    if len(text) <= max_chars:
        return text

    splitter = RecursiveCharacterTextSplitter(chunk_size=CHUNK_SIZE, chunk_overlap=CHUNK_OVERLAP)
    chunks = [c.page_content for c in splitter.create_documents([text])]
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

    ranked = sorted(range(n), key=lambda i: (-score(tokenized[i]), i))  # ties keep document order

    chosen: list[int] = []
    used = 0
    for i in ranked:
        if used + len(chunks[i]) > max_chars and chosen:
            continue
        chosen.append(i)
        used += len(chunks[i])
        if used >= max_chars:
            break

    return "\n...\n".join(chunks[i] for i in sorted(chosen))
