import json

import pytest
from fastapi.testclient import TestClient

import main
from prompts import summary_prompt


class Writer:
    """Stands in for the model: remembers what it was asked and answers with a fixed overview."""

    def __init__(self, text="The overview."):
        self.text = text
        self.prompts = []

    async def ainvoke(self, prompt):
        self.prompts.append(prompt)
        return type("Msg", (), {"content": self.text})()

    async def astream(self, prompt):
        self.prompts.append(prompt)
        for piece in self.text.split(" "):
            yield type("Chunk", (), {"content": piece + " "})()


@pytest.fixture
def writer(monkeypatch):
    fake = Writer()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    return fake


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def documents(n=3, text="A short summary of the document."):
    return [{"name": f"Lecture {i}", "text": f"{text} Number {i}."} for i in range(1, n + 1)]


def events(response):
    return [json.loads(line[6:]) for line in response.text.splitlines() if line.startswith("data: ")]


def test_overview_is_written_from_the_summaries_of_the_documents(client, writer):
    r = client.post("/overview", json={"name": "Biology course", "documents": documents()})
    assert r.status_code == 200
    assert r.json() == {"filename": "Overview: Biology course", "summary": "The overview."}

    prompt = writer.prompts[-1]
    assert "“Biology course”" in prompt
    for name in ("### Lecture 1", "### Lecture 2", "### Lecture 3"):
        assert name in prompt
    assert "where the documents agree" in prompt and "differ or contradict" in prompt
    assert "about 300 words" in prompt  # the overview default is longer than a one-document summary


def test_overview_honours_length_style_and_language(client, writer):
    client.post("/overview?word_count=500&style=bullets&language=German", json={"documents": documents(2)})
    prompt = writer.prompts[-1]
    assert "about 500 words" in prompt and "German" in prompt and "bullet" in prompt.lower()


def test_overview_carries_standing_instructions(client, writer):
    client.post("/overview?instructions=Keep%20it%20formal", json={"documents": documents(2)})
    assert "Keep it formal" in writer.prompts[-1]


def test_overview_streams_and_names_the_overview_in_the_done_event(client, writer):
    r = client.post("/overview?stream=true", json={"name": "Biology course", "documents": documents(2)})
    assert r.status_code == 200 and r.headers["content-type"].startswith("text/event-stream")
    sent = events(r)
    assert "".join(e["text"] for e in sent if e["type"] == "delta").strip() == "The overview."
    assert sent[-1] == {"type": "done", "filename": "Overview: Biology course"}


def test_overview_without_a_name_is_still_named(client, writer):
    assert client.post("/overview", json={"documents": documents(2)}).json()["filename"] == "Overview"


@pytest.mark.parametrize("count", [0, 1, main.MAX_OVERVIEW_DOCUMENTS + 1])
def test_overview_needs_between_two_and_twenty_documents(client, writer, count):
    r = client.post("/overview", json={"documents": documents(count)})
    assert r.status_code == 422 and "between 2 and 20" in r.json()["detail"]
    assert not writer.prompts


def test_overview_rejects_an_empty_document_and_bad_options(client, writer):
    empty = documents(2) + [{"name": "Blank", "text": "   "}]
    r = client.post("/overview", json={"documents": empty})
    assert r.status_code == 422 and "Blank" in r.json()["detail"]
    assert client.post("/overview?style=poem", json={"documents": documents(2)}).status_code == 422
    assert client.post("/overview?word_count=5", json={"documents": documents(2)}).status_code == 422
    assert client.post("/overview", json={"name": "x"}).status_code == 422


def test_overview_names_documents_that_have_no_name(client, writer):
    client.post("/overview", json={"documents": [{"name": "  ", "text": "One."}, {"name": "", "text": "Two."}]})
    assert "### Document 1" in writer.prompts[-1] and "### Document 2" in writer.prompts[-1]


def test_a_second_identical_overview_is_answered_from_the_cache(client, writer):
    body = {"name": "Biology course", "documents": documents(2)}
    client.post("/overview", json=body)
    client.post("/overview", json=body)
    assert len(writer.prompts) == 1


def test_a_changed_document_is_not_answered_from_the_cache(client, writer):
    docs = documents(2)
    client.post("/overview", json={"documents": docs})
    docs[1]["text"] += " A new sentence."
    client.post("/overview", json={"documents": docs})
    assert len(writer.prompts) == 2


def test_very_long_material_is_condensed_before_the_briefing(client, writer, monkeypatch):
    seen = []

    async def condense(text, progress=None):
        seen.append(len(text))
        return "CONDENSED MATERIAL"

    monkeypatch.setattr(main, "condense", condense)
    long_summary = "A long summary sentence about the topic. " * 200  # 8,000 characters each
    r = client.post("/overview", json={"documents": documents(5, long_summary)})
    assert r.status_code == 200
    assert seen and seen[0] > main.REDUCE_MAX_CHARS
    assert "CONDENSED MATERIAL" in writer.prompts[-1]


def test_a_model_failure_is_a_plain_error(client, monkeypatch):
    class Broken:
        async def ainvoke(self, prompt):
            raise RuntimeError("boom")

    monkeypatch.setattr(main, "get_llm", lambda: Broken())
    r = client.post("/overview", json={"documents": documents(2)})
    assert r.status_code >= 500 and "boom" not in r.text


def test_collection_prompt_without_a_name_has_no_empty_quotes():
    prompt = summary_prompt("### A\nx", 200, "default", "English", kind="collection")
    assert "called" not in prompt.split("---")[0]
