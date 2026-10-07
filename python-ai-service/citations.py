"""Citations in chat answers: the model is shown numbered excerpts and cites them as [1], [2], ...

Models do not always follow the format. Some write "[1, 2]", some cite a number that does not exist,
and gpt-oss tends to use its own style, 【2】 or 【2†source】. Whatever comes back is normalized here,
so every marker the user sees is a plain [n] that points at a real excerpt."""

import re

_OPEN, _CLOSE = r"[\[【]", r"[\]】]"
# One or more numbers, optionally followed by a "†source"-style note, inside either kind of bracket.
_MARKER = re.compile(rf"{_OPEN}(\d+(?:\s*,\s*\d+)*)(?:†[^\]】]*)?{_CLOSE}")
# A marker, with the space before it, as left in earlier answers that are replayed as history.
_MARKER_WITH_SPACE = re.compile(rf"[ \t]*{_OPEN}\d+(?:\s*,\s*\d+)*(?:†[^\]】]*)?{_CLOSE}")


def clean_citations(answer: str, valid_ids: set[int]) -> tuple[str, list[int]]:
    """Keeps only markers that refer to real excerpts, writing "[1, 2]" and "【1】" as "[1][2]" and "[1]".
    Returns the cleaned answer and the excerpt numbers cited, in order of first use."""
    cited: list[int] = []

    def fix(match: re.Match) -> str:
        ids = [int(n) for n in re.split(r"\s*,\s*", match.group(1))]
        good = [n for n in dict.fromkeys(ids) if n in valid_ids]
        cited.extend(n for n in good if n not in cited)
        return "".join(f"[{n}]" for n in good)

    cleaned = _MARKER.sub(fix, answer)
    # Dropping an invalid marker can leave a stray space before punctuation: "fact [9]." -> "fact ."
    cleaned = re.sub(r"[ \t]+([.,;:!?])", r"\1", cleaned)
    return cleaned, cited


def strip_citations(text: str) -> str:
    """Removes citation markers. Earlier answers are replayed as conversation history, and their
    numbers would not match the excerpts numbered for the new question."""
    return _MARKER_WITH_SPACE.sub("", text)
