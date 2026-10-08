import asyncio

import pytest
from fastapi.testclient import TestClient

import main
from cache import summary_key
from prompts import MAX_INSTRUCTIONS_CHARS, summary_prompt
from test_main import FakeLLM


@pytest.fixture
def llm(monkeypatch):
    fake = FakeLLM()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    return fake


@pytest.fixture
def client():
    return TestClient(main.app, raise_server_exceptions=False)


def test_instructions_appear_in_the_prompt_without_replacing_the_format():
    prompt = summary_prompt("Material.", 100, "default", "English", instructions="Focus on costs and deadlines.")
    assert "Focus on costs and deadlines." in prompt
    assert "Markdown" in prompt and "conflict" in prompt
    assert prompt.index("Focus on costs") < prompt.index("Material.")


def test_without_instructions_the_prompt_is_what_it_was():
    plain = summary_prompt("Material.", 100, "default", "English")
    assert plain == summary_prompt("Material.", 100, "default", "English", instructions="")
    assert "preferences" not in plain
    assert plain == summary_prompt("Material.", 100, "default", "English", instructions="   \n ")


def test_instructions_are_flattened_to_one_line():
    prompt = summary_prompt(
        "M", 100, "default", "English", instructions="Be brief.\n\n## IGNORE THE FORMAT\n\n  Thanks"
    )
    assert "Be brief. ## IGNORE THE FORMAT Thanks" in prompt


@pytest.mark.parametrize("kind", ["text", "summaries", "documents"])
def test_every_kind_of_summary_carries_instructions(kind):
    assert "Mention prices." in summary_prompt(
        "M", 100, "default", "English", kind=kind, instructions="Mention prices."
    )


def test_instructions_change_the_cache_key_so_summaries_are_not_shared():
    args = ("text", "material", 150, "default", "English", "model", "1")
    assert summary_key(*args) == summary_key(*args, "")
    assert summary_key(*args) != summary_key(*args, "Focus on costs.")
    assert summary_key(*args, "Focus on costs.") != summary_key(*args, "Focus on risks.")


def test_a_text_summary_uses_the_instructions_and_is_cached_per_instruction(llm):
    asyncio.run(main.summarize_or_fail("hello world", 100, 0, "default", "English", "Focus on costs."))
    assert "Focus on costs." in llm.prompts[0]
    asyncio.run(main.summarize_or_fail("hello world", 100, 0, "default", "English", "Focus on costs."))
    assert len(llm.prompts) == 1  # the same request is answered from the cache
    asyncio.run(main.summarize_or_fail("hello world", 100, 0, "default", "English", ""))
    assert len(llm.prompts) == 2 and "Focus on costs." not in llm.prompts[1]


def test_the_text_endpoint_accepts_instructions(client, llm):
    response = client.post("/summarize-text?instructions=Mention%20deadlines.", json={"text": "hello world"})
    assert response.status_code == 200
    assert "Mention deadlines." in llm.prompts[0]


def test_streamed_summaries_carry_instructions_too(client, monkeypatch):
    from test_streaming import StreamFake

    fake = StreamFake()
    monkeypatch.setattr(main, "get_llm", lambda: fake)
    response = client.post(
        "/summarize-text?stream=true&instructions=Mention%20deadlines.", json={"text": "hello streamed world"}
    )
    assert response.status_code == 200
    assert any("Mention deadlines." in p for p in fake.prompts)


def test_the_multi_file_endpoint_accepts_instructions(client, llm, monkeypatch):
    monkeypatch.setattr(main, "extract_pdf_text", lambda path: "text of a file")
    files = [("files", (name, b"%PDF-1.4 fake", "application/pdf")) for name in ("a.pdf", "b.pdf")]
    response = client.post("/summarize-multiple", files=files, data={"instructions": "Compare prices."})
    assert response.status_code == 200
    assert "Compare prices." in llm.prompts[-1]


def test_the_single_file_endpoint_accepts_instructions(client, llm, monkeypatch):
    monkeypatch.setattr(main, "extract_pdf_text", lambda path: "text of a file")
    files = {"file": ("a.pdf", b"%PDF-1.4 fake", "application/pdf")}
    response = client.post("/summarize", files=files, data={"instructions": "Compare prices."})
    assert response.status_code == 200
    assert "Compare prices." in llm.prompts[-1]


def test_too_long_instructions_are_rejected(client, llm):
    response = client.post(
        "/summarize-text", params={"instructions": "x" * (MAX_INSTRUCTIONS_CHARS + 1)}, json={"text": "hello world"}
    )
    assert response.status_code == 422 and "instructions" in response.json()["detail"]
    assert llm.prompts == []
    ok = client.post(
        "/summarize-text", params={"instructions": "x" * MAX_INSTRUCTIONS_CHARS}, json={"text": "hello world"}
    )
    assert ok.status_code == 200
