"""Measures how well document search finds the right passage for a question.

It compares the keyword search the app has always had (Postgres full-text search, run through the
compose database) with embedding models, and with the two combined the way the gateway combines them.
Use it before changing the embedding model, the similarity floor or the fusion weights.

Run from python-ai-service/ with the service's requirements installed and the compose database up:

    python evals/search/run_eval.py --hard                       # compare methods on the full library
    python evals/search/run_eval.py --hard --paragraphs          # the same with paragraph-sized passages
    python evals/search/run_eval.py --hard --thresholds          # where to put the similarity floor
    python evals/search/run_eval.py --hard --models BAAI/bge-small-en-v1.5,thenlper/gte-base

Questions have a kind: kw (shares words with the answer), para (same meaning, other words), ind (a
conceptual question), hard (a nearby document could fool the search), lang (another language) and xl
(question and document in different languages). Scores: @1 and @3 are the share of questions whose
right passage is ranked first or within the first three; MRR is the mean of 1 / rank.
"""

import os
import re
import subprocess
import sys
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
SERVICE = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path[:0] = [SERVICE, HERE]

import corpus  # noqa: E402
import corpus_hard  # noqa: E402
import numpy as np  # noqa: E402
from retrieval import select_passages  # noqa: E402

DEFAULT_MODELS = ["BAAI/bge-small-en-v1.5"]
# What the gateway uses (go-api/controllers/embeddings.go): meaning leads, keyword has a small say.
FUSION_WEIGHTS = (1.0, 0.15)
FUSION_K = 10


def build_library(hard: bool, paragraphs: bool):
    docs = dict(corpus.DOCS)
    questions = list(corpus.QUESTIONS)
    if hard:
        docs.update(corpus_hard.DISTRACTORS)
        docs.update(corpus_hard.FOREIGN)
        questions += corpus_hard.QUESTIONS
    passages = []  # (id, document, text)
    for name, text in docs.items():
        if paragraphs:  # a finer cut, like a long document with many passages
            pieces = [x.strip() for x in text.split("\n\n") if len(x.strip()) > 60]
        else:  # exactly as the app cuts documents
            pieces = [p.text for p in select_passages(text, "", len(text) + 1)]
        for t in pieces:
            passages.append((len(passages) + 1, name, t))
    gold = []
    for _, doc, phrase, _ in questions:
        ids = {i for i, d, t in passages if d == doc and phrase.lower() in t.lower()}
        assert ids, f"gold phrase not found in any passage: {doc!r} {phrase!r}"
        gold.append(ids)
    return docs, questions, passages, gold


# ---- keyword search: the same query building and SQL as go-api/controllers/libraryController.go ---------------
QUERY_WORD = re.compile(r"[\w]+", re.UNICODE)


def search_query(question: str, max_terms: int = 12) -> str:
    seen, terms = set(), []
    for w in QUERY_WORD.findall(question.lower()):
        if len(w) < 2 or w in seen or "_" in w:
            continue
        seen.add(w)
        terms.append(w + ":*" if len(w) >= 4 else w)
        if len(terms) == max_terms:
            break
    return " | ".join(terms)


def psql(sql: str) -> str:
    out = subprocess.run(
        ["docker", "exec", "-i", "summarizer-db", "psql", "-U", "user", "-d", "summarizer_test", "-At", "-F", "|", "-q", "-v", "ON_ERROR_STOP=1"],
        input=sql,
        capture_output=True,
        text=True,
        encoding="utf-8",
    )
    if out.returncode != 0:
        raise RuntimeError(out.stderr)
    return out.stdout


def load_table(passages) -> None:
    rows = ",".join(f"({i},$d${d}$d$,$t${t}$t$)" for i, d, t in passages)
    psql(
        "DROP TABLE IF EXISTS eval_passages;"
        "CREATE TABLE eval_passages(id int primary key, doc text, text text,"
        " tsv tsvector GENERATED ALWAYS AS (to_tsvector('english', text)) STORED);"
        f"INSERT INTO eval_passages(id, doc, text) VALUES {rows};"
    )


def keyword_ranking(query: str) -> list[int]:
    """Passage ids best first (English stemming first, then plain words), as the app orders them."""
    if not query:
        return []
    for config, vector in (("english", "tsv"), ("simple", "to_tsvector('simple', text)")):
        out = psql(
            f"WITH q AS (SELECT to_tsquery('{config}', $q${query}$q$) AS query) "
            f"SELECT id FROM eval_passages, q WHERE {vector} @@ q.query "
            f"ORDER BY ts_rank_cd({vector}, q.query) DESC, id LIMIT 60;"
        )
        ids = [int(x) for x in out.split()]
        if ids:
            return ids
    return []


# ---- embeddings ---------------------------------------------------------------------------------------------------
def embed_all(model_name: str, passages, queries):
    from fastembed import TextEmbedding

    model = TextEmbedding(model_name=model_name)
    docs = np.array(list(model.passage_embed([t for _, _, t in passages])))
    qs = np.array(list(model.query_embed(queries)))
    docs /= np.linalg.norm(docs, axis=1, keepdims=True)
    qs /= np.linalg.norm(qs, axis=1, keepdims=True)
    return qs @ docs.T  # one row of similarities per question


def ranking_from(sims_row, passages) -> list[int]:
    return [passages[j][0] for j in np.argsort(-sims_row)]


def fuse(by_meaning: list[int], by_keyword: list[int]) -> list[int]:
    """Reciprocal rank fusion with the gateway's weights."""
    score: dict[int, float] = defaultdict(float)
    for weight, ranking in zip(FUSION_WEIGHTS, (by_meaning, by_keyword), strict=True):
        for pos, pid in enumerate(ranking):
            score[pid] += weight / (FUSION_K + pos + 1)
    return [pid for pid, _ in sorted(score.items(), key=lambda kv: (-kv[1], kv[0]))]


# ---- scoring ----------------------------------------------------------------------------------------------------------
KIND_ORDER = ("kw", "para", "ind", "hard", "lang", "xl")


def report(name: str, questions, gold, rankings) -> None:
    rows: dict[str, list[float]] = defaultdict(lambda: [0, 0, 0, 0.0])  # @1, @3, n, reciprocal rank
    for (_, _, _, kind), wanted, ranking in zip(questions, gold, rankings, strict=True):
        pos = next((i + 1 for i, pid in enumerate(ranking) if pid in wanted), None)
        for key in (kind, "all"):
            r = rows[key]
            r[2] += 1
            if pos:
                r[0] += pos <= 1
                r[1] += pos <= 3
                r[3] += 1.0 / pos
    line = f"{name:<44}"
    for key in [k for k in KIND_ORDER if k in rows] + ["all"]:
        at1, at3, n, rr = rows[key]
        line += f" | {key}({int(n)}) @1 {at1 / n:4.0%} @3 {at3 / n:4.0%} MRR {rr / n:.2f}"
    print(line)


def compare(models, hard: bool, paragraphs: bool) -> None:
    _, questions, passages, gold = build_library(hard, paragraphs)
    print(f"{len(passages)} passages, {len(questions)} questions\n")
    load_table(passages)
    try:
        by_keyword = [keyword_ranking(search_query(q)) for q, *_ in questions]
        report("keyword search (Postgres full-text)", questions, gold, by_keyword)
        for m in models:
            sims = embed_all(m, passages, [q for q, *_ in questions])
            by_meaning = [ranking_from(row, passages) for row in sims]
            short = m.split("/")[-1]
            report(f"embeddings: {short}", questions, gold, by_meaning)
            report(f"  + keyword, as the gateway combines them", questions, gold, [fuse(a, b) for a, b in zip(by_meaning, by_keyword, strict=True)])
        print("\nQuestions keyword search misses in its top three:")
        for (q, _, _, kind), wanted, ranking in zip(questions, gold, by_keyword, strict=True):
            if not any(p in wanted for p in ranking[:3]):
                print(f"  [{kind}] {q}")
    finally:
        psql("DROP TABLE IF EXISTS eval_passages;")


def thresholds(models, hard: bool, paragraphs: bool) -> None:
    """The best similarity for questions the library can answer, versus ones it cannot."""
    _, questions, passages, _ = build_library(hard, paragraphs)
    from fastembed import TextEmbedding

    for name in models:
        model = TextEmbedding(model_name=name)
        docs = np.array(list(model.passage_embed([t for _, _, t in passages])))
        docs /= np.linalg.norm(docs, axis=1, keepdims=True)

        def best(texts):
            q = np.array(list(model.query_embed(texts)))
            q /= np.linalg.norm(q, axis=1, keepdims=True)
            return (q @ docs.T).max(axis=1)

        answerable = best([q for q, *_ in questions])
        unanswerable = best(corpus_hard.UNANSWERABLE)
        print(f"\n{name}")
        print(f"  answerable   best score: min {answerable.min():.3f}  p10 {np.percentile(answerable, 10):.3f}  median {np.median(answerable):.3f}")
        print(f"  unanswerable best score: median {np.median(unanswerable):.3f}  p90 {np.percentile(unanswerable, 90):.3f}  max {unanswerable.max():.3f}")
        for floor in (0.40, 0.45, 0.48, 0.50, 0.52, 0.55, 0.58):
            print(f"  floor {floor:.2f}: answerable kept {np.mean(answerable >= floor):4.0%}   unanswerable wrongly kept {np.mean(unanswerable >= floor):4.0%}")


def main() -> None:
    args = sys.argv[1:]
    models = args[args.index("--models") + 1].split(",") if "--models" in args else DEFAULT_MODELS
    hard, paragraphs = "--hard" in args, "--paragraphs" in args
    if "--thresholds" in args:
        thresholds(models, hard, paragraphs)
    else:
        compare(models, hard, paragraphs)


if __name__ == "__main__":
    main()
