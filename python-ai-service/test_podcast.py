import json

import pytest
from fastapi.testclient import TestClient

import main
from podcast import (
    MAX_TURN_CHARS,
    MAX_TURNS,
    clean_line,
    normalize_script,
    parse_reply,
    parse_script,
    parse_transcript,
    podcast_prompt,
)

GOOD = {
    "title": "Atlas X200 in five minutes",
    "turns": [
        {"speaker": "A", "text": "So what is this manual about?"},
        {"speaker": "B", "text": "It covers the Atlas X200 compressor."},
        {"speaker": "A", "text": "How long is the warranty?"},
        {"speaker": "B", "text": "Seven years on the unit."},
        {"speaker": "A", "text": "And maintenance?"},
        {"speaker": "B", "text": "Change the filter every ninety days."},
    ],
}


# ---- the prompt ------------------------------------------------------------------------------


def test_the_prompt_demands_grounding_and_a_json_reply():
    prompt = podcast_prompt("The summary.", "The full text.", None)
    assert "ONLY facts from the source" in prompt and "exactly right" in prompt
    assert "take turns" in prompt and "starting with Alex" in prompt
    assert "TITLE: a short episode title" in prompt and "Alex: ..." in prompt and "Sam: ..." in prompt
    assert "SUMMARY:\nThe summary." in prompt and "FULL TEXT:\nThe full text." in prompt
    assert "language of the source material" in prompt


def test_long_documents_are_told_through_their_summary_alone():
    prompt = podcast_prompt("The summary.", "x" * 50_000, "Spanish")
    assert "FULL TEXT" not in prompt
    assert "in Spanish" in prompt


# ---- reading what the model sends back --------------------------------------------------------


@pytest.mark.parametrize(
    "wrap",
    [
        lambda s: s,
        lambda s: f"```json\n{s}\n```",
        lambda s: f"```\n{s}\n```",
        lambda s: f"Sure! Here is the script:\n{s}\nHope you like it.",
    ],
    ids=["plain", "json-fence", "bare-fence", "chatter-around"],
)
def test_the_json_is_found_however_it_is_wrapped(wrap):
    assert parse_script(wrap(json.dumps(GOOD))) == GOOD


@pytest.mark.parametrize("raw", ["", "no json here", "{broken", "[1, 2, 3]", '"just a string"'])
def test_replies_without_a_json_object_are_rejected(raw):
    assert parse_script(raw) is None


# ---- the transcript format -------------------------------------------------------------------

TRANSCRIPT = """TITLE: Atlas X200 in five minutes
Alex: So what is this manual about?
Sam: It covers the Atlas X200 compressor.
Alex: How long is the warranty?
Sam: Seven years on the unit.
Alex: And maintenance?
Sam: Change the filter every ninety days."""


def test_a_transcript_is_read_into_turns():
    data = parse_transcript(TRANSCRIPT)
    assert data["title"] == "Atlas X200 in five minutes"
    assert [(t["speaker"], t["text"]) for t in data["turns"]][:2] == [
        ("Alex", "So what is this manual about?"),
        ("Sam", "It covers the Atlas X200 compressor."),
    ]
    assert normalize_script(data)["turns"][0] == {"speaker": "A", "text": "So what is this manual about?"}


@pytest.mark.parametrize(
    "wrap",
    [
        lambda s: s,
        lambda s: "```" + chr(10) + s + chr(10) + "```",
        lambda s: s.replace("Alex:", "**Alex:**").replace("Sam:", "**Sam:**"),
        lambda s: s.replace("Alex:", "ALEX:").replace("Sam:", "sam :"),
        lambda s: s.replace("Alex:", "Host 1:").replace("Sam:", "Host 2:"),
        lambda s: s.replace("Alex:", "- Alex:").replace("Sam:", "- Sam:"),
        lambda s: s.replace("Alex:", "Alex" + chr(0xFF1A)).replace("Sam:", "Sam" + chr(0xFF1A)),
    ],
    ids=["plain", "fenced", "bold-names", "case-and-spacing", "host-numbers", "bulleted", "fullwidth-colon"],
)
def test_transcript_variants_are_all_understood(wrap):
    script = normalize_script(parse_transcript(wrap(TRANSCRIPT)))
    assert script is not None
    assert [t["speaker"] for t in script["turns"]] == ["A", "B", "A", "B", "A", "B"]
    assert script["turns"][1]["text"] == "It covers the Atlas X200 compressor."


def test_a_wrapped_line_continues_the_turn_before_it():
    raw = chr(10).join(["TITLE: t", "Alex: First part", "of one long question?", "Sam: An answer."])
    data = parse_transcript(raw)
    assert data["turns"][0]["text"] == "First part of one long question?"
    assert len(data["turns"]) == 2


def test_a_missing_title_is_fine_and_text_before_the_first_turn_is_ignored():
    data = parse_transcript(chr(10).join(["Sure, here you go:", "Alex: Hello?", "Sam: Hi."]))
    assert data["title"] == ""
    assert [t["text"] for t in data["turns"]] == ["Hello?", "Hi."]


@pytest.mark.parametrize("raw", ["", "no names here at all", "TITLE: only a title"])
def test_text_without_turns_is_not_a_transcript(raw):
    assert parse_transcript(raw) is None


def test_either_format_is_accepted():
    assert parse_reply(json.dumps(GOOD)) == GOOD
    assert parse_reply(TRANSCRIPT)["turns"][0]["speaker"] == "Alex"
    assert parse_reply("nothing useful") is None


# ---- normalizing ------------------------------------------------------------------------------


def test_a_good_script_keeps_its_shape():
    assert normalize_script(GOOD) == GOOD


def test_speaker_names_map_onto_the_two_hosts():
    data = {
        "title": "t",
        "turns": [
            {"speaker": "Alex", "text": "One."},
            {"speaker": "SAM", "text": "Two."},
            {"speaker": "host 1", "text": "Three."},
            {"speaker": "Host 2", "text": "Four."},
        ],
    }
    assert [t["speaker"] for t in normalize_script(data)["turns"]] == ["A", "B", "A", "B"]


def test_markdown_and_stage_directions_are_removed_from_speech():
    assert clean_line("**Wow**, that's `big`! [laughs] (soft music) # Really?") == "Wow, that's big! Really?"
    assert clean_line("- a bullet\n   spread   over lines") == "a bullet spread over lines"


def test_a_long_turn_is_cut_at_a_word_boundary():
    line = clean_line("word " * 400)
    assert len(line) <= MAX_TURN_CHARS + 1 and line.endswith("…") and not line.endswith(" …")


def test_unknown_speakers_and_empty_lines_are_dropped():
    data = {
        "title": "t",
        "turns": GOOD["turns"] + [{"speaker": "Narrator", "text": "Hi"}, {"speaker": "A", "text": "  "}],
    }
    assert len(normalize_script(data)["turns"]) == len(GOOD["turns"])


def test_the_number_of_turns_is_capped():
    turns = [{"speaker": "AB"[i % 2], "text": f"Line {i}."} for i in range(100)]
    assert len(normalize_script({"title": "t", "turns": turns})["turns"]) == MAX_TURNS


@pytest.mark.parametrize(
    "data",
    [
        {"title": "t"},
        {"title": "t", "turns": "not a list"},
        {"title": "t", "turns": [{"speaker": "A", "text": "Only one line."}]},
        {"title": "t", "turns": [{"speaker": "A", "text": f"Line {i}."} for i in range(8)]},  # one voice only
        {  # one host only interrupts once: a lecture, not a conversation
            "title": "t",
            "turns": [{"speaker": "A", "text": "A question?"}]
            + [{"speaker": "B", "text": f"Answer {i}."} for i in range(9)],
        },
        {"title": "t", "turns": ["strings", 3, None]},
    ],
    ids=["no-turns", "not-a-list", "too-short", "one-sided", "lopsided", "garbage"],
)
def test_unusable_scripts_are_rejected(data):
    assert normalize_script(data) is None


def test_a_missing_title_gets_a_default():
    assert normalize_script({"turns": GOOD["turns"]})["title"] == "Podcast"


# ---- the endpoint -----------------------------------------------------------------------------


class Replying:
    def __init__(self, *replies):
        self.replies = list(replies)
        self.prompts = []

    async def ainvoke(self, prompt):
        self.prompts.append(prompt)
        reply = self.replies.pop(0) if len(self.replies) > 1 else self.replies[0]
        return type("Msg", (), {"content": reply, "usage_metadata": None})()


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def make(client, monkeypatch, *replies, **body):
    fake = Replying(*replies)
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    payload = {"summary": "A summary of the atlas manual.", "text": "The atlas manual text.", **body}
    return client.post("/podcast", json=payload), fake


def test_a_transcript_reply_gives_a_clean_script(client, monkeypatch):
    response, _ = make(client, monkeypatch, TRANSCRIPT)
    body = response.json()
    assert response.status_code == 200
    assert body["title"] == "Atlas X200 in five minutes"
    assert [t["speaker"] for t in body["turns"]] == ["A", "B", "A", "B", "A", "B"]


def test_the_endpoint_returns_a_clean_script(client, monkeypatch):
    response, fake = make(client, monkeypatch, "```json\n" + json.dumps(GOOD) + "\n```")
    assert response.status_code == 200
    assert response.json() == GOOD
    assert "atlas manual" in fake.prompts[0]


def test_an_unusable_first_reply_is_retried(client, monkeypatch):
    response, fake = make(client, monkeypatch, "sorry, no", json.dumps(GOOD))
    assert response.status_code == 200 and response.json() == GOOD
    assert len(fake.prompts) == 2


def test_replies_that_stay_unusable_give_a_clean_error_after_a_few_tries(client, monkeypatch):
    response, fake = make(client, monkeypatch, "nope", "still nope", "never")
    assert response.status_code == 502
    assert "usable" in response.json()["detail"].lower()
    assert len(fake.prompts) == main.PODCAST_ATTEMPTS


def test_a_one_sided_script_is_retried_until_a_balanced_one_comes(client, monkeypatch):
    lopsided = {"title": "t", "turns": [{"speaker": "B", "text": f"Line {i}."} for i in range(10)]}
    response, fake = make(client, monkeypatch, json.dumps(lopsided), json.dumps(GOOD))
    assert response.status_code == 200 and response.json() == GOOD
    assert len(fake.prompts) == 2


def test_the_language_can_be_chosen(client, monkeypatch):
    _, fake = make(client, monkeypatch, json.dumps(GOOD), language="German")
    assert "in German" in fake.prompts[0]


@pytest.mark.parametrize(
    ("override", "status"),
    [
        ({"summary": "  "}, 422),
        ({"language": "Klingon"}, 422),
        ({"text": "x" * (main.MAX_MULTI_TEXT_CHARS + 20_000)}, 413),
    ],
    ids=["blank-summary", "unknown-language", "huge-text"],
)
def test_podcast_validation(client, monkeypatch, override, status):
    response, _ = make(client, monkeypatch, json.dumps(GOOD), **override)
    assert response.status_code == status
