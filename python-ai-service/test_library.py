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


PASSAGES = [
    {"id": 1, "text": "Bananas are rich in potassium.", "page": 1, "pageEnd": 1, "document": "fruit.pdf"},
    {"id": 2, "text": "The lease ends on 1 March.", "page": 4, "pageEnd": 4, "document": "Rental agreement"},
]


def ask(client, monkeypatch, reply, **body):
    fake = Replying(reply)
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    payload = {"question": "What do bananas contain?", "passages": PASSAGES, **body}
    return client.post("/ask", json=payload), fake


# ---- /ask ------------------------------------------------------------------------------------


def test_the_answer_cites_passages_from_different_documents(client, monkeypatch):
    response, fake = ask(client, monkeypatch, "Potassium [1], and the lease ends in March [2].")
    body = response.json()
    assert response.status_code == 200
    assert [s["id"] for s in body["sources"]] == [1, 2]
    assert body["sources"][1]["document"] == "Rental agreement" and body["sources"][1]["page"] == 4
    prompt = fake.prompts[0]
    assert "[1] (page 1 of fruit.pdf)" in prompt and "[2] (page 4 of Rental agreement)" in prompt
    assert "different documents" in prompt, "the model is told the excerpts come from several documents"


def test_only_cited_passages_come_back_and_invented_numbers_are_removed(client, monkeypatch):
    response, _ = ask(client, monkeypatch, "Potassium [1] and something else [9].")
    body = response.json()
    assert body["answer"] == "Potassium [1] and something else."
    assert [s["id"] for s in body["sources"]] == [1]


def test_old_citation_numbers_in_the_history_are_dropped(client, monkeypatch):
    history = [{"role": "user", "content": "Hi"}, {"role": "assistant", "content": "Hello there [7]."}]
    _, fake = ask(client, monkeypatch, "ok", history=history)
    assert "Assistant: Hello there." in fake.prompts[0] and "[7]" not in fake.prompts[0]


@pytest.mark.parametrize(
    ("override", "status"),
    [
        ({"question": "  "}, 422),
        ({"passages": []}, 422),
        ({"passages": [{"id": 1, "text": "a"}, {"id": 1, "text": "b"}]}, 422),
        ({"passages": [{"id": 0, "text": "a"}]}, 422),
        ({"passages": [{"id": i, "text": "x"} for i in range(1, main.MAX_LIBRARY_PASSAGES + 2)]}, 413),
        ({"question": "x" * (main.MAX_QUESTION_CHARS + 1)}, 422),
    ],
)
def test_ask_validation(client, monkeypatch, override, status):
    response, _ = ask(client, monkeypatch, "ok", **override)
    assert response.status_code == status


def test_oversized_passages_are_trimmed_not_trusted(client, monkeypatch):
    big = {"id": 1, "text": "y" * (main.MAX_LIBRARY_PASSAGE_CHARS * 3)}
    _, fake = ask(client, monkeypatch, "ok", passages=[big])
    assert "y" * (main.MAX_LIBRARY_PASSAGE_CHARS + 1) not in fake.prompts[0]


def test_a_failing_model_gives_a_clean_error(client, monkeypatch):
    class Boom:
        async def ainvoke(self, prompt):
            raise RuntimeError("secret internals")

    monkeypatch.setattr(main, "get_llm", lambda: Boom())
    response = client.post("/ask", json={"question": "q?", "passages": PASSAGES})
    assert response.status_code >= 500
    assert "secret internals" not in response.text


# ---- /passages -------------------------------------------------------------------------------


def test_passages_come_back_with_their_pages(client):
    text = PAGE_BREAK.join(["First page words.", "Second page words."])
    response = client.post("/passages", json={"text": text})
    assert response.status_code == 200
    found = response.json()["passages"]
    assert [(p["page"], p["pageEnd"]) for p in found] == [(1, 1), (2, 2)]
    assert found[1]["text"] == "Second page words." and found[0]["document"] is None


def test_passages_name_the_file_of_combined_documents(client):
    text = "=== a.pdf ===\nAlpha text.\n\n=== b.pdf ===\nBeta text."
    found = client.post("/passages", json={"text": text}).json()["passages"]
    assert [p["document"] for p in found] == ["a.pdf", "b.pdf"]


@pytest.mark.parametrize(
    ("text", "status"),
    [
        pytest.param("   ", 422, id="blank"),
        pytest.param("x" * (main.MAX_MULTI_TEXT_CHARS + 20_000), 413, id="too-long"),
    ],
)
def test_passages_validation(client, text, status):
    assert client.post("/passages", json={"text": text}).status_code == status
