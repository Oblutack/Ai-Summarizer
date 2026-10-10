"use client";
import { reasonKey, type FieldResult, type FieldSpec, type Row } from "../lib/extraction";
import { useT } from "./I18nProvider";

// One cell: the value found, whether it checks out, and (on opening it) the exact quote it was read from.
function Cell({ result }: { result: FieldResult | undefined }) {
  const t = useT();
  if (!result || !result.found) {
    return (
      <td className="px-3 py-2 align-top text-ink/60" data-testid="cell" data-state="missing">
        <span aria-hidden="true">—</span>
        <span className="sr-only">{t("extract.notFound")}</span>
      </td>
    );
  }
  const key = reasonKey(result.reason);
  const reason = key ? t(key) : result.reason;
  return (
    <td className="px-3 py-2 align-top" data-testid="cell" data-state={result.verified ? "verified" : "unverified"}>
      <details>
        <summary className="cursor-pointer break-words">
          {result.value}{" "}
          <span aria-hidden="true" className={result.verified ? "text-accent" : "text-danger"}>
            {result.verified ? "✓" : "⚠"}
          </span>
          <span className="sr-only">{result.verified ? t("extract.verified") : t("extract.unverified", { reason })}</span>
        </summary>
        <div className="mt-1 space-y-1 border-l-4 border-ink/30 pl-2 text-sm">
          {result.quote && (
            <p className="whitespace-pre-wrap">
              <span className="label mr-1">{t("extract.quote")}:</span>“{result.quote}”
              {result.page ? <span className="muted"> ({t("extract.page", { n: result.page })})</span> : null}
            </p>
          )}
          {!result.verified && <p className="font-medium text-danger">{t("extract.unverified", { reason })}</p>}
        </div>
      </details>
    </td>
  );
}

// The extracted data as a table: a row for each document, a column for each field.
export default function ExtractTable({ rows, fields }: { rows: Row[]; fields: FieldSpec[] }) {
  const t = useT();
  return (
    <div className="overflow-x-auto rounded-lg border border-ink/20" role="region" aria-label={t("extract.resultTitle")} tabIndex={0}>
      <table className="w-full border-collapse text-left" data-testid="extract-table">
        <caption className="sr-only">{t("extract.resultTitle")}</caption>
        <thead>
          <tr className="border-b border-ink/30 bg-surface">
            <th scope="col" className="px-3 py-2 font-semibold">
              {t("extract.colDocument")}
            </th>
            {fields.map((f) => (
              <th key={f.name} scope="col" className="px-3 py-2 font-semibold">
                {f.name}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.id} className="border-b border-ink/15 last:border-0" data-testid="extract-row">
              <th scope="row" className="max-w-[16rem] break-words px-3 py-2 align-top font-medium">
                {row.name}
              </th>
              {row.error || !row.result ? (
                <td colSpan={Math.max(fields.length, 1)} className="px-3 py-2 font-medium text-danger" role="alert">
                  {t("extract.rowFailed", { message: row.error ?? t("extract.failed") })}
                </td>
              ) : (
                fields.map((f, i) => <Cell key={f.name} result={row.result?.fields[i]} />)
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
