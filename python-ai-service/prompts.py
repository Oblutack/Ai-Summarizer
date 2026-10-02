"""Prompt construction for summaries and document chat."""

# Bump when prompt wording changes so cached summaries made with the old wording are not reused.
PROMPT_VERSION = "1"

DEFAULT_STYLE = "default"
DEFAULT_LANGUAGE = "English"

STYLE_INSTRUCTIONS = {
    "default": (
        "Use headings, bullet points, and bold text where appropriate to structure the key information."
    ),
    "bullets": (
        "Write only concise bullet points, one sentence each, grouped under short headings. "
        "Do not write paragraphs."
    ),
    "brief": (
        "Write an executive brief with these sections: '## Overview' (2-3 sentences), "
        "'## Key Points' (bullets), and '## Implications' (bullets). Keep the tone professional."
    ),
    "simple": (
        "Explain it in plain language a curious 12-year-old could follow: short sentences, "
        "everyday words, no jargon (explain any unavoidable term briefly)."
    ),
    "takeaways": (
        "Write '## Key Takeaways' as a numbered list, then '## Action Items' as a bullet list "
        "of concrete next steps. If the text has no action items, write 'None'."
    ),
}

LANGUAGES = [
    "English",
    "Spanish",
    "French",
    "German",
    "Italian",
    "Portuguese",
    "Dutch",
    "Polish",
    "Turkish",
    "Russian",
    "Serbian",
    "Croatian",
    "Bosnian",
    "Chinese",
    "Japanese",
]


def is_valid_style(style: str) -> bool:
    return style in STYLE_INSTRUCTIONS


def is_valid_language(language: str) -> bool:
    return language in LANGUAGES


def _language_line(language: str) -> str:
    return f"\n        Write the entire output in {language}." if language != DEFAULT_LANGUAGE else ""


def summary_prompt(material: str, target_words: int, style: str, language: str, kind: str = "text") -> str:
    """Builds the final summarization prompt.

    kind: "text" for raw text, "summaries" for already-condensed chunk summaries,
    "documents" for several labelled documents that must be summarized together.
    """
    if kind == "summaries":
        task = f"Condense and combine the following summaries into a single, well-structured text of about {target_words} words."
    elif kind == "documents":
        task = (
            f"Write one combined summary of about {target_words} words covering all of the documents below. "
            "Mention the document names when a point comes from a specific one, and call out notable "
            "overlaps or differences between them."
        )
    else:
        task = f"Provide a summary of the following text in about {target_words} words."

    return f"""{task}
        **Format the entire output strictly as Markdown.**
        {STYLE_INSTRUCTIONS[style]}{_language_line(language)}

        ---

        {material}"""


def chat_prompt(context: str, history: list[dict], question: str) -> str:
    turns = "\n".join(
        f"{'User' if m['role'] == 'user' else 'Assistant'}: {m['content']}" for m in history
    )
    history_block = f"Conversation so far:\n{turns}\n\n" if turns else ""
    return f"""You answer questions about a document. Use only the document excerpts below.
If the answer is not in the excerpts, say you could not find it in the document; do not guess.
Answer in the same language as the user's question, be concise, and format with Markdown when helpful.

Document excerpts:
---
{context}
---

{history_block}User: {question}
Assistant:"""
