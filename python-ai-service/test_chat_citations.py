import pytest
from fastapi.testclient import TestClient

import main
from retrieval import PAGE_BREAK


class Replying:
    def __init__(self, reply):
        self.reply = reply
        self.prompts = []

    async def ainvoke(self, prompt):
        self.prompts.append(prompt)
        return type("Msg", (), {"content": self.reply, "usage_metadata": None})()


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def ask(client, monkeypatch, reply, text, **extra):
    fake = Replying(reply)
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    response = client.post("/chat", json={"text": text, "question": "What is the warranty?", **extra})
    return response, fake


def test_a_cited_answer_returns_the_passage_with_its_page(client, monkeypatch):
    text = PAGE_BREAK.join(["Intro page.", "The compressor warranty is seven years."])
    response, fake = ask(client, monkeypatch, "Seven years [2].", text)

    body = response.json()
    assert response.status_code == 200
    assert body["answer"] == "Seven years [2]."
    (source,) = body["sources"]
    assert source["id"] == 2 and "seven years" in source["text"]
    assert source["page"] == 2 and source["pageEnd"] == 2 and source["document"] is None
    assert PAGE_BREAK not in source["text"]
    assert "[1]" in fake.prompts[0] and "Cite your sources" in fake.prompts[0]


def test_the_prompt_labels_excerpts_with_page_and_file(client, monkeypatch):
    text = "=== manual.pdf ===\n" + PAGE_BREAK.join(["first page text", "second page text"])
    _, fake = ask(client, monkeypatch, "ok", text)
    assert "[1] (page 1 of manual.pdf)" in fake.prompts[0]
    assert "[2] (page 2 of manual.pdf)" in fake.prompts[0]


def test_only_cited_excerpts_are_returned(client, monkeypatch):
    long_text = ("filler sentence about nothing. " * 120 + "\n\n") * 6
    response, _ = ask(client, monkeypatch, "No citation in this answer.", long_text)
    assert response.json()["sources"] == []


def test_invented_numbers_are_removed_from_the_answer_and_not_returned(client, monkeypatch):
    response, _ = ask(client, monkeypatch, "Seven years [1] and also ten [42].", "The warranty is seven years.")
    body = response.json()
    assert body["answer"] == "Seven years [1] and also ten."
    assert [s["id"] for s in body["sources"]] == [1]


def test_old_citation_numbers_in_history_do_not_reach_the_model(client, monkeypatch):
    history = [
        {"role": "user", "content": "How long?"},
        {"role": "assistant", "content": "Seven years [7]."},
    ]
    _, fake = ask(client, monkeypatch, "ok", "The warranty is seven years.", history=history)
    assert "Assistant: Seven years." in fake.prompts[0]
    assert "[7]" not in fake.prompts[0]


def test_documents_without_page_info_still_cite(client, monkeypatch):
    response, fake = ask(client, monkeypatch, "Seven years [1].", "Pasted text: the warranty is seven years.")
    (source,) = response.json()["sources"]
    assert source["page"] is None and source["document"] is None
    assert "[1]\n" in fake.prompts[0], "no page label when there is no page information"


def test_pdf_extraction_keeps_page_breaks(tmp_path):
    from pypdf import PdfWriter

    path = tmp_path / "two.pdf"
    writer = PdfWriter()
    writer.add_blank_page(200, 200)
    writer.add_blank_page(200, 200)
    with open(path, "wb") as f:
        writer.write(f)
    assert main.extract_pdf_text(str(path)).count(PAGE_BREAK) == 1
