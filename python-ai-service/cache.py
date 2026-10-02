"""A small in-memory cache for finished summaries.

Summarizing the same text with the same options is deterministic enough that repeating the LLM
call is wasted time and tokens (people re-click, retry after a network blip, or re-run the same
document in another style tab). Entries expire and the cache is bounded, so memory stays small.
The cache is per process: it is an optimization, never a source of truth.
"""
import hashlib
import os
import time
from collections import OrderedDict
from typing import Callable, Optional

DEFAULT_MAX_ENTRIES = 256
DEFAULT_TTL_SECONDS = 3600
MAX_SUMMARY_CHARS = 100_000  # don't hold unusually large values


class SummaryCache:
    def __init__(
        self,
        max_entries: int = DEFAULT_MAX_ENTRIES,
        ttl_seconds: float = DEFAULT_TTL_SECONDS,
        clock: Callable[[], float] = time.monotonic,
    ):
        self.max_entries = max_entries
        self.ttl_seconds = ttl_seconds
        self._clock = clock
        self._items: "OrderedDict[str, tuple[float, str]]" = OrderedDict()
        self.hits = 0
        self.misses = 0

    @property
    def enabled(self) -> bool:
        return self.max_entries > 0 and self.ttl_seconds > 0

    def get(self, key: str) -> Optional[str]:
        if not self.enabled:
            return None
        entry = self._items.get(key)
        if entry is None or self._clock() - entry[0] > self.ttl_seconds:
            self._items.pop(key, None)
            self.misses += 1
            return None
        self._items.move_to_end(key)  # most recently used
        self.hits += 1
        return entry[1]

    def put(self, key: str, value: str) -> None:
        if not self.enabled or not value or len(value) > MAX_SUMMARY_CHARS:
            return
        self._items[key] = (self._clock(), value)
        self._items.move_to_end(key)
        while len(self._items) > self.max_entries:
            self._items.popitem(last=False)  # evict the least recently used

    def clear(self) -> None:
        self._items.clear()
        self.hits = self.misses = 0

    def __len__(self) -> int:
        return len(self._items)


def summary_key(kind: str, material: str, target_words: int, style: str, language: str, model: str, prompt_version: str) -> str:
    """Hashes everything that can change the output; the (possibly huge) text is never stored."""
    h = hashlib.sha256()
    for part in (kind, str(target_words), style, language, model, prompt_version):
        h.update(part.encode("utf-8"))
        h.update(b"\x00")
    h.update(material.encode("utf-8"))
    return h.hexdigest()


def from_env() -> SummaryCache:
    return SummaryCache(
        max_entries=int(os.getenv("SUMMARY_CACHE_SIZE", DEFAULT_MAX_ENTRIES)),
        ttl_seconds=float(os.getenv("SUMMARY_CACHE_TTL_SECONDS", DEFAULT_TTL_SECONDS)),
    )
