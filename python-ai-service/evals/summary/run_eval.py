"""Measures how faithful the summaries are, with the real model.

It summarizes made-up documents whose facts are known (cases.py) several times each, exactly the way the
service does, and scores every summary (summary_quality.py): did it keep the facts, did it make the mistakes
this kind of document invites, did it invent a figure, does it have the format and language that were asked
for. The headline number is the share of "clean" runs.

Use it before and after changing a summary prompt, the model, its reasoning effort or its temperature. It
calls the model, so it needs GROQ_API_KEY (read from the project's .env) and counts every call against that key:
it prints how many it is about to make and refuses more than --max-calls (150) unless you add --yes. Run it from python-ai-service/ :

    python evals/summary/run_eval.py                          # every case, 3 samples each: about 100 model calls
    python evals/summary/run_eval.py --samples 5 --label prompt-v2
    python evals/summary/run_eval.py --cases lease,deck       # only some cases
    python evals/summary/run_eval.py --failures               # also print every summary that was not clean
    python evals/summary/run_eval.py --save-baseline          # keep this run as the reference
    python evals/summary/run_eval.py --compare results/a.json results/b.json   # no model call
    python evals/summary/run_eval.py --rescore results/a.json   # score saved summaries again; no model call

Settings under test come from the environment, as in the service: LLM_MODEL, LLM_REASONING_EFFORT,
LLM_TEMPERATURE ("default" for the provider's own). Each result file records them, so two files can be compared knowing what differed.
"""

import argparse
import asyncio
import json
import os
import sys
import time
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
SERVICE = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path[:0] = [SERVICE, HERE]

from dotenv import load_dotenv  # noqa: E402

load_dotenv(os.path.join(SERVICE, "..", ".env"))
os.environ.setdefault("LOG_LEVEL", "WARNING")

import cases as corpus  # noqa: E402
import llm  # noqa: E402
import main  # noqa: E402
import summary_quality as sq  # noqa: E402

RESULTS = os.path.join(HERE, "results")
BASELINE = os.path.join(HERE, "baseline.json")


async def summarize(case, variant) -> str:
    """One summary of a case, made the way the service makes it."""
    if case.kind == "single":
        return await main.process_summary(case.docs[0][1], variant.word_count, 0, variant.style, variant.language)
    if case.kind == "multi":
        return await main.process_multi_summary(list(case.docs), variant.word_count, 0, variant.style, variant.language)
    prompt = await main.prepare_overview_prompt(case.subject, list(case.docs), variant.word_count, variant.style, variant.language)
    summary = await main.call_llm(prompt)
    if not summary:
        raise ValueError("the model returned an empty summary")
    return summary


async def run_one(semaphore, case, variant, sample):
    async with semaphore:
        started = time.monotonic()
        try:
            summary = await summarize(case, variant)
        except Exception as exc:  # a failed call is reported, and left out of the rates
            return {"case": case.id, "variant": label_of(variant), "sample": sample, "error": f"{type(exc).__name__}: {exc}"[:300]}
        score = sq.score_summary(
            summary,
            case.text,
            case.facts,
            case.traps,
            style=variant.style,
            language=variant.language,
            target_words=variant.word_count,
            allowed_numbers=case.allowed_numbers,
        )
        return {
            "case": case.id,
            "variant": label_of(variant),
            "sample": sample,
            "seconds": round(time.monotonic() - started, 1),
            "summary": summary,
            "score": score.as_dict(),
            "_score": score,
        }


def calls_for(case) -> int:
    """About how many model calls one summary of a case takes: one, or for a long document one for each chunk
    of it, plus the passes that combine them, plus the final summary. Retries after a rate limit add more."""
    if case.kind != "single" or len(case.docs[0][1]) <= main.SINGLE_SHOT_MAX_CHARS:
        return 1
    chunks = -(-len(case.docs[0][1]) // (main.CHUNK_SIZE - main.CHUNK_OVERLAP))
    return chunks + 2


def label_of(variant) -> str:
    return f"{variant.style}/{variant.language}/{variant.word_count}w"


async def run(selected, samples, concurrency):
    semaphore = asyncio.Semaphore(concurrency)
    jobs = [
        run_one(semaphore, case, variant, sample)
        for case in selected
        for variant in case.variants
        for sample in range(1, samples + 1)
    ]
    return await asyncio.gather(*jobs)


def settings() -> dict:
    return {
        "model": main.active_model(),
        "reasoningEffort": os.getenv("LLM_REASONING_EFFORT", "low"),
        "temperature": "provider default" if llm.TEMPERATURE in ("", "default") else llm.TEMPERATURE,
        "promptVersion": main.PROMPT_VERSION,
    }


def summarize_results(results) -> dict:
    """Rates per case and variant, per case, and overall."""
    good = [r for r in results if "score" in r]
    groups = {"variants": defaultdict(list), "cases": defaultdict(list), "all": []}
    for r in good:
        groups["variants"][f"{r['case']} {r['variant']}"].append(r["_score"])
        groups["cases"][r["case"]].append(r["_score"])
        groups["all"].append(r["_score"])
    return {
        "overall": sq.aggregate(groups["all"]),
        "cases": {k: sq.aggregate(v) for k, v in groups["cases"].items()},
        "variants": {k: sq.aggregate(v) for k, v in groups["variants"].items()},
        "errors": [r for r in results if "error" in r],
    }


def table(rows: dict, title: str) -> str:
    head = f"{title:<34} {'runs':>4} {'clean':>6} {'recall':>7} {'traps':>6} {'numbers':>8} {'format':>7} {'lang':>5} {'length':>7}"
    lines = [head, "-" * len(head)]

    def pct(value):
        return "   -" if value is None else f"{value:>4.0%}"

    for name, a in rows.items():
        lines.append(
            f"{name:<34} {a['runs']:>4} {pct(a['clean']):>6} {pct(a['recall']):>7} {pct(a['trapRate']):>6} "
            f"{pct(a['inventedNumberRate']):>8} {pct(a['formatProblemRate']):>7} {pct(a['languageOk']):>5} {a['lengthRatio']:>6.1f}x"
        )
    return "\n".join(lines)


def what_went_wrong(results) -> str:
    """The most common reasons a run was not clean, so the next change can aim at them."""
    reasons = defaultdict(int)
    for r in results:
        if "score" not in r or r["score"]["clean"]:
            continue
        s = r["score"]
        for name in s["trapsHit"]:
            reasons[f"trap: {name}  [{r['case']}]"] += 1
        for number in s["inventedNumbers"]:
            reasons[f"invented number {number}  [{r['case']}]"] += 1
        for problem in s["formatProblems"]:
            reasons[f"format: {problem}"] += 1
        if s["languageOk"] is False:
            reasons[f"wrong language ({s['language']})  [{r['case']}]"] += 1
        if s["recall"] < sq.MIN_RECALL:
            for fact in s["factsMissed"]:
                reasons[f"missed fact: {fact}  [{r['case']}]"] += 1
    ranked = sorted(reasons.items(), key=lambda kv: -kv[1])[:15]
    return "\n".join(f"  {count:>2}x  {reason}" for reason, count in ranked) or "  (every run was clean)"


def public(results) -> list:
    return [{k: v for k, v in r.items() if k != "_score"} for r in results]


def compare(path_a: str, path_b: str) -> int:
    a, b = (json.load(open(p, encoding="utf-8")) for p in (path_a, path_b))
    print(f"A: {os.path.basename(path_a)}  {a['settings']}\nB: {os.path.basename(path_b)}  {b['settings']}\n")
    keys = ["clean", "recall", "trapRate", "inventedNumberRate", "formatProblemRate", "languageOk", "lengthOk"]
    print(f"{'':<34} " + " ".join(f"{k:>18}" for k in keys[:5]))
    for name in ["overall", *sorted(a["summary"]["cases"])]:
        ra = a["summary"]["overall"] if name == "overall" else a["summary"]["cases"].get(name)
        rb = b["summary"]["overall"] if name == "overall" else b["summary"]["cases"].get(name)
        if not ra or not rb:
            continue
        cells = []
        for k in keys[:5]:
            x, y = ra.get(k), rb.get(k)
            cells.append("-" if x is None or y is None else f"{x:.0%} -> {y:.0%}")
        print(f"{name:<34} " + " ".join(f"{c:>18}" for c in cells))
    worse = sq.regressions(b["summary"]["overall"], a["summary"]["overall"])
    print("\nB is worse than A on:", "; ".join(worse) if worse else "nothing beyond the tolerance")
    return 1 if worse else 0


def rescore(path: str) -> int:
    """Scores the summaries saved in a result file again with the current scoring, calling no model. Use it after
    improving cases.py or summary_quality.py, to see what the same summaries would have scored."""
    with open(path, encoding="utf-8") as f:
        saved = json.load(f)
    by_id = {c.id: c for c in corpus.CASES}
    variants = {label_of(v): v for c in corpus.CASES for v in c.variants}
    results = []
    for r in saved["results"]:
        if "summary" not in r:
            continue
        case, variant = by_id[r["case"]], variants[r["variant"]]
        score = sq.score_summary(
            r["summary"], case.text, case.facts, case.traps, style=variant.style, language=variant.language,
            target_words=variant.word_count, allowed_numbers=case.allowed_numbers,
        )  # fmt: skip
        results.append({**{k: v for k, v in r.items() if k != "score"}, "score": score.as_dict(), "_score": score})
    summary = summarize_results(results)
    print(table(summary["variants"], "case and variant"))
    print()
    print(table({"OVERALL": summary["overall"]}, ""))
    print("\nwhy runs were not clean:\n" + what_went_wrong(results))
    payload = {**{k: saved[k] for k in ("label", "time", "settings", "samples")}, "summary": summary | {"errors": 0}, "results": public(results)}
    out = path.replace(".json", "-rescored.json")
    with open(out, "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=1)
    print(f"\nsaved {os.path.relpath(out, SERVICE)}")
    return 0


def main_cli() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--samples", type=int, default=3, help="summaries per case and variant (default 3)")
    parser.add_argument("--cases", default="", help="comma-separated case ids (default: all)")
    parser.add_argument("--concurrency", type=int, default=3, help="model calls at once (default 3)")
    parser.add_argument("--label", default="", help="name for the result file")
    parser.add_argument("--failures", action="store_true", help="print every summary that was not clean")
    parser.add_argument("--max-calls", type=int, default=150, help="refuse a run that would make more model calls than this")
    parser.add_argument("--yes", action="store_true", help="go ahead with a run above --max-calls")
    parser.add_argument("--save-baseline", action="store_true", help="write this run as baseline.json")
    parser.add_argument("--baseline", default="", help="compare with this result file and fail on a regression")
    parser.add_argument("--compare", nargs=2, metavar=("A", "B"), help="compare two result files; calls no model")
    parser.add_argument("--rescore", metavar="FILE", help="score a saved result file again with the current scoring; calls no model")
    args = parser.parse_args()

    if args.compare:
        return compare(*args.compare)
    if args.rescore:
        return rescore(args.rescore)
    if not os.getenv("GROQ_API_KEY"):
        print("GROQ_API_KEY is not set (put it in the project's .env)")
        return 2

    wanted = {c.strip() for c in args.cases.split(",") if c.strip()}
    selected = [c for c in corpus.CASES if not wanted or c.id in wanted]
    unknown = wanted - {c.id for c in corpus.CASES}
    if unknown:
        print(f"unknown cases: {', '.join(sorted(unknown))}; known: {', '.join(c.id for c in corpus.CASES)}")
        return 2

    total = sum(len(c.variants) for c in selected) * args.samples
    calls = sum(len(c.variants) * calls_for(c) for c in selected) * args.samples
    print(f"{len(selected)} cases, {total} summaries, about {calls} model calls, settings: {settings()}")
    if calls > args.max_calls and not args.yes:
        print(
            f"\nThat is more than {args.max_calls} calls, each one counted against your provider key. Use fewer samples or "
            "--cases, or add --yes to go ahead."
        )
        return 2
    started = time.monotonic()
    results = asyncio.run(run(selected, args.samples, args.concurrency))
    summary = summarize_results(results)

    print(f"\ndone in {time.monotonic() - started:.0f}s; {len(summary['errors'])} failed calls\n")
    print(table(summary["variants"], "case and variant"))
    print()
    print(table({"OVERALL": summary["overall"]}, ""))
    print("\nwhy runs were not clean:\n" + what_went_wrong(results))
    for e in summary["errors"][:5]:
        print(f"\nERROR {e['case']} {e['variant']}: {e['error']}")

    if args.failures:
        for r in results:
            if "score" in r and not r["score"]["clean"]:
                s = r["score"]
                print(f"\n--- {r['case']} {r['variant']} #{r['sample']}  traps={s['trapsHit']} numbers={s['inventedNumbers']} "
                      f"format={s['formatProblems']} missed={s['factsMissed']} lang={s['language']}\n{r['summary']}")

    payload = {
        "label": args.label,
        "time": time.strftime("%Y-%m-%d %H:%M:%S"),
        "settings": settings(),
        "samples": args.samples,
        "summary": {k: v for k, v in summary.items() if k != "errors"} | {"errors": len(summary["errors"])},
        "results": public(results),
    }
    os.makedirs(RESULTS, exist_ok=True)
    name = f"{time.strftime('%Y%m%d-%H%M%S')}-{args.label or 'run'}.json"
    path = os.path.join(RESULTS, name)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=1)
    print(f"\nsaved {os.path.relpath(path, SERVICE)}")
    if args.save_baseline:
        with open(BASELINE, "w", encoding="utf-8") as f:
            json.dump({k: payload[k] for k in ("label", "time", "settings", "samples", "summary")}, f, ensure_ascii=False, indent=1)
        print("saved evals/summary/baseline.json")

    if args.baseline:
        with open(args.baseline, encoding="utf-8") as f:
            reference = json.load(f)
        worse = sq.regressions(summary["overall"], reference["summary"]["overall"])
        print("\nagainst the baseline:", "; ".join(worse) if worse else "no regression")
        return 1 if worse else 0
    return 0


if __name__ == "__main__":
    sys.exit(main_cli())
