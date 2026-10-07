"""Turns a document into a short two-host conversation, as a script of speaker turns.

The model writes the script; this module builds the prompt and, just as important, cleans up what
comes back. Models wrap JSON in code fences, add commentary around it, use their own speaker names
and sneak Markdown into speech, so none of that is trusted: the result is parsed tolerantly and then
normalized into a fixed shape the player can rely on.
"""

import json
import re

HOSTS = {"A": "Alex", "B": "Sam"}
MIN_TURNS = 4
MAX_TURNS = 30
MAX_TURN_CHARS = 700
# Each host must have at least this share of the turns, or it is a monologue with the odd interruption.
MIN_SHARE = 0.3
MAX_TITLE_CHARS = 120

# Documents longer than this are told to the hosts through their summary alone, to keep the prompt small.
FULL_TEXT_MAX_CHARS = 12_000


def podcast_prompt(summary: str, text: str, language: str | None) -> str:
    source = f"SUMMARY:\n{summary.strip()}"
    if text.strip() and len(text) <= FULL_TEXT_MAX_CHARS:
        source += f"\n\nFULL TEXT:\n{text.strip()}"
    spoken = (
        f"Write the whole conversation in {language}."
        if language
        else "Write the conversation in the language of the source material."
    )
    alex, sam = HOSTS["A"], HOSTS["B"]
    return f"""You write the script for a short, friendly podcast episode in which two hosts discuss a document.
The hosts are {alex} (curious, asks the questions a newcomer would ask) and {sam} (has read the document and
explains it clearly).

Rules:
- Use ONLY facts from the source material below. Do not add facts, examples, names or numbers that are
  not in it, and get every figure exactly right.
- The hosts take turns: {alex}, {sam}, {alex}, {sam} and so on, starting with {alex}. Every {alex} turn is a
  question or a reaction; every {sam} turn is an answer.
- Open with a one-line hook, cover the most important points in a natural back-and-forth, and end with a
  one-line takeaway.
- {MIN_TURNS * 3} to {MIN_TURNS * 4} turns in total. Each turn is one to three short sentences (under 60 words).
- Plain spoken language only: no Markdown, no bullet points, no stage directions, no sound effects, no emojis, no URLs.
- {spoken}

Reply in exactly this format and nothing else. The first line is the title, then one turn per line, each
starting with the host's name and a colon:
TITLE: a short episode title
{alex}: ...
{sam}: ...
{alex}: ...

SOURCE MATERIAL
---
{source}
---"""


_FENCE = re.compile(r"^\s*```(?:json)?\s*|\s*```\s*$", re.IGNORECASE)


def parse_script(raw: str) -> dict | None:
    """Reads the JSON object out of a model reply, tolerating code fences and surrounding commentary."""
    text = _FENCE.sub("", raw.strip())
    candidates = [text]
    start, end = text.find("{"), text.rfind("}")
    if start != -1 and end > start:
        candidates.append(text[start : end + 1])
    for candidate in candidates:
        try:
            data = json.loads(candidate)
        except ValueError:
            continue
        if isinstance(data, dict):
            return data
    return None


_TITLE_LINE = re.compile(r"^\s*(?:title|episode title)\s*[:：]\s*(.+?)\s*$", re.IGNORECASE)
_TURN_LINE = re.compile(
    r"^\s*[*_#>\-\s]*(alex|sam|host\s*[12ab]|speaker\s*[12ab])\s*[*_]*\s*[:：]\s*[*_]*\s*(.*)$", re.IGNORECASE
)


def parse_transcript(raw: str) -> dict | None:
    """Reads "TITLE: ..." followed by lines of "Alex: ..." and "Sam: ...". A line that starts with neither
    continues the previous turn, since models wrap long speech."""
    title = ""
    turns: list[dict] = []
    for line in _FENCE.sub("", raw.strip()).splitlines():
        if not line.strip():
            continue
        if not turns and not title and (match := _TITLE_LINE.match(line)):
            title = match.group(1)
            continue
        if match := _TURN_LINE.match(line):
            turns.append({"speaker": match.group(1), "text": match.group(2)})
        elif turns:
            turns[-1]["text"] += " " + line.strip()
    return {"title": title, "turns": turns} if turns else None


def parse_reply(raw: str) -> dict | None:
    """The script a model replied with, whether it wrote a transcript (what is asked for) or JSON."""
    return parse_transcript(raw) or parse_script(raw)


_MARKDOWN = re.compile(r"(\*\*|__|\*|_|`|#{1,6}\s|^\s*[-•]\s+)", re.MULTILINE)
_BRACKETED = re.compile(r"\[[^\]]*\]|\([^)]*(?:laugh|music|pause|sfx|sound)[^)]*\)", re.IGNORECASE)


def clean_line(text: str) -> str:
    """One turn as plain speech: no Markdown marks, no [stage directions], single spaces, bounded length."""
    text = _BRACKETED.sub("", text)
    text = _MARKDOWN.sub("", text)
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) > MAX_TURN_CHARS:
        text = text[:MAX_TURN_CHARS].rsplit(" ", 1)[0].rstrip(",;:") + "…"
    return text


def _speaker(value) -> str | None:
    """Maps the model's speaker label onto our two hosts ("A", "B")."""
    label = str(value or "").strip().lower()
    if label in ("a", "alex", "host a", "host 1", "1", "speaker a", "speaker 1"):
        return "A"
    if label in ("b", "sam", "host b", "host 2", "2", "speaker b", "speaker 2"):
        return "B"
    return None


def normalize_script(data: dict) -> dict | None:
    """The script in a fixed shape (title, turns of speaker A/B and plain text), or None when it is
    unusable: too short, one-sided, or not a list of turns at all."""
    turns_in = data.get("turns")
    if not isinstance(turns_in, list):
        return None
    turns = []
    for item in turns_in:
        if not isinstance(item, dict):
            continue
        speaker = _speaker(item.get("speaker"))
        text = clean_line(str(item.get("text") or ""))
        if speaker and text:
            turns.append({"speaker": speaker, "text": text})
        if len(turns) == MAX_TURNS:
            break
    if len(turns) < MIN_TURNS:
        return None
    # A conversation needs both voices throughout: models sometimes write one host's lines only, plus a
    # lone interruption from the other, which would sound like a lecture.
    smaller = min(sum(t["speaker"] == who for t in turns) for who in "AB")
    if smaller < 2 or smaller < MIN_SHARE * len(turns):
        return None
    title = clean_line(str(data.get("title") or ""))[:MAX_TITLE_CHARS] or "Podcast"
    return {"title": title, "turns": turns}
