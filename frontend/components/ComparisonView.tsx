"use client";
import { useMemo, useState } from "react";
import { comparisonToMarkdown, filterChanges, IMPORTANCE_ORDER, importanceCounts, type Change, type Comparison, type ImportanceFilter } from "../lib/compare";
import type { MessageKey, Params } from "../lib/i18n";
import CopyButton from "./CopyButton";
import { useT } from "./I18nProvider";

type Translate = (key: MessageKey, params?: Params) => string;

const KIND_KEY = {
  changed: "compare.kindChanged",
  added: "compare.kindAdded",
  removed: "compare.kindRemoved",
  moved: "compare.kindMoved",
} as const;

const IMPORTANCE_KEY = {
  high: "compare.impHigh",
  medium: "compare.impMedium",
  low: "compare.impLow",
} as const;

const FILTER_KEY = {
  all: "compare.filterAll",
  high: "compare.filterHigh",
  medium: "compare.filterMedium",
  low: "compare.filterLow",
} as const;

// A label and a border colour per importance. The words carry the meaning; the colour only helps.
const IMPORTANCE_LOOK = {
  high: "border-danger text-danger",
  medium: "border-accent text-accent",
  low: "border-ink/40 text-ink/70",
} as const;

function pageNote(change: Change, t: Translate): string {
  const { beforePage: before, afterPage: after } = change;
  if (before && after) return before === after ? t("compare.pageSame", { n: before }) : t("compare.pageMoved", { before, after });
  if (after) return t("compare.pageNew", { n: after });
  if (before) return t("compare.pageOld", { n: before });
  return "";
}

// Text with the words that went struck through and the words that came underlined. Both are also said aloud
// ("removed:", "added:") so that nothing depends on seeing the colours.
function Redline({ segments }: { segments: [string, string][] }) {
  const t = useT();
  return (
    <p className="whitespace-pre-wrap leading-relaxed" data-testid="redline">
      {segments.map(([kind, text], i) =>
        kind === "del" ? (
          <del key={i} className="bg-danger/15 line-through decoration-2">
            <span className="sr-only">{t("compare.srRemoved")}</span>
            {text}
          </del>
        ) : kind === "ins" ? (
          <ins key={i} className="bg-accent/20 underline decoration-2 underline-offset-2">
            <span className="sr-only">{t("compare.srAdded")}</span>
            {text}
          </ins>
        ) : (
          <span key={i}>{text}</span>
        )
      )}
    </p>
  );
}

function Quote({ label, text, tone }: { label: string; text: string; tone: "old" | "new" }) {
  return (
    <div className={`border-l-4 pl-3 ${tone === "old" ? "border-danger/60" : "border-accent/60"}`}>
      <p className="label mb-0.5">{label}</p>
      <p className="whitespace-pre-wrap leading-relaxed">{text}</p>
    </div>
  );
}

function ChangeItem({ change }: { change: Change }) {
  const t = useT();
  const page = pageNote(change, t);
  const { removed, added } = change.numbers;
  return (
    <li className="space-y-2 rounded-lg border border-ink/20 bg-surface p-3" data-testid="change" data-kind={change.kind} data-importance={change.importance}>
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="chip pointer-events-none">{t(KIND_KEY[change.kind])}</span>
        <span className={`rounded-full border px-2 py-0.5 font-semibold ${IMPORTANCE_LOOK[change.importance]}`}>{t(IMPORTANCE_KEY[change.importance])}</span>
        {page && <span className="muted">{page}</span>}
      </div>
      {change.suspicious && (
        <p className="text-sm font-medium text-danger" data-testid="change-suspicious">
          {t("compare.suspiciousChange")}
        </p>
      )}
      {change.summary && <p className="font-semibold">{change.summary}</p>}
      {change.impact && <p className="text-ink/80">{change.impact}</p>}
      {(removed.length > 0 || added.length > 0) && (
        <p className="text-sm text-ink/80" data-testid="change-numbers">
          {removed.length > 0 && <span className="mr-3">{t("compare.numbersRemoved", { list: removed.join(", ") })}</span>}
          {added.length > 0 && <span>{t("compare.numbersAdded", { list: added.join(", ") })}</span>}
        </p>
      )}
      {change.kind === "changed" && change.segments ? (
        <Redline segments={change.segments} />
      ) : (
        <div className="space-y-2">
          {change.before && <Quote label={t("compare.before")} text={change.before} tone="old" />}
          {change.after && <Quote label={t("compare.after")} text={change.after} tone="new" />}
        </div>
      )}
    </li>
  );
}

// What a comparison found: the bottom line, then every change with its exact quotes, filterable by importance.
export default function ComparisonView({ result, oldName, newName }: { result: Comparison; oldName: string; newName: string }) {
  const t = useT();
  const [filter, setFilter] = useState<ImportanceFilter>("all");
  const counts = useMemo(() => importanceCounts(result.changes), [result.changes]);
  const shown = filterChanges(result.changes, filter);

  if (result.identical) {
    return (
      <p className="mt-4 rounded-lg border border-ink/20 bg-surface p-3" role="status" data-testid="compare-identical">
        {t("compare.identical")}
      </p>
    );
  }
  const { changed, added, removed, moved } = result.counts;
  return (
    <section className="mt-4 space-y-4" aria-label={t("compare.heading", { old: oldName, new: newName })} data-testid="comparison">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-xl font-bold">{t("compare.heading", { old: oldName, new: newName })}</h3>
          <p className="muted text-sm">{t("compare.counts", { changed, added, removed, moved })}</p>
        </div>
        <CopyButton text={() => comparisonToMarkdown(result, oldName, newName)} what={t("compare.what")} />
      </div>

      {result.bottomLine && (
        <div className="rounded-lg border border-accent/50 bg-surface p-3" data-testid="bottom-line">
          <p className="label mb-1">{t("compare.inShort")}</p>
          <p className="leading-relaxed">{result.bottomLine}</p>
        </div>
      )}
      {result.suspicious && (
        <p className="rounded-lg border border-danger/60 bg-surface p-3 font-medium" role="note" data-testid="compare-suspicious">
          {t("compare.suspiciousBanner")}
        </p>
      )}
      {!result.explained && (
        <p className="text-sm text-ink/80" role="note">
          {t("compare.unexplained")}
        </p>
      )}

      <div role="group" aria-label={t("compare.filterLegend")} className="flex flex-wrap gap-2">
        {(["all", ...IMPORTANCE_ORDER] as const).map((level) => (
          <button
            key={level}
            type="button"
            onClick={() => setFilter(level)}
            aria-pressed={filter === level}
            className={`btn btn-sm ${filter === level ? "btn-primary" : "btn-secondary"}`}
          >
            {t(FILTER_KEY[level], { n: counts[level] })}
          </button>
        ))}
      </div>

      {shown.length === 0 ? (
        <p className="muted">{t("compare.nothingHere")}</p>
      ) : (
        <ol className="space-y-3" aria-label={t("compare.heading", { old: oldName, new: newName })}>
          {shown.map((change) => (
            <ChangeItem key={change.id} change={change} />
          ))}
        </ol>
      )}
      {result.omitted > 0 && <p className="muted text-sm">{t("compare.omitted", { n: result.omitted })}</p>}
    </section>
  );
}
