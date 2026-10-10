"use client";
import { useEffect, useId, useRef, useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import type { Comparison } from "../lib/compare";
import { LANGUAGES } from "../lib/summaryOptions";
import type { Document } from "../types";
import ComparisonView from "./ComparisonView";
import { useT } from "./I18nProvider";
import { actionButton, fieldClass } from "./styles";

const CANDIDATES = 20;
const SEARCH_DELAY_MS = 250;

// Compares one saved document with another of the person's own: pick the other one, say which is newer, and read what
// changed. Nothing is saved: the comparison is worked out when it is asked for.
export default function ComparePanel({ doc }: { doc: Document }) {
  const t = useT();
  const ids = { search: useId(), pick: useId(), language: useId(), note: useId() };
  const [query, setQuery] = useState("");
  const [candidates, setCandidates] = useState<Document[]>([]);
  const [searching, setSearching] = useState(true);
  const [otherId, setOtherId] = useState("");
  const [thisIsNewer, setThisIsNewer] = useState(true);
  const [language, setLanguage] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [shown, setShown] = useState<{ result: Comparison; oldName: string; newName: string } | null>(null);
  // Only the newest search may fill the list.
  const latest = useRef(0);

  useEffect(() => {
    const ticket = ++latest.current;
    setSearching(true);
    const timer = setTimeout(async () => {
      try {
        const params = new URLSearchParams({ limit: String(CANDIDATES + 1) });
        if (query.trim()) params.set("q", query.trim());
        const response = await axios.get<Document[]>(`${API_URL}/documents?${params.toString()}`);
        if (ticket !== latest.current) return;
        // Only documents that kept their text can be compared, and not with themselves.
        setCandidates(response.data.filter((d) => d.ID !== doc.ID && d.hasContent).slice(0, CANDIDATES));
      } catch {
        if (ticket === latest.current) setCandidates([]);
      } finally {
        if (ticket === latest.current) setSearching(false);
      }
    }, query ? SEARCH_DELAY_MS : 0);
    return () => clearTimeout(timer);
  }, [query, doc.ID]);

  const other = candidates.find((d) => String(d.ID) === otherId);

  const compare = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!other) return;
    setBusy(true);
    setError("");
    setShown(null);
    const [older, newer] = thisIsNewer ? [other, doc] : [doc, other];
    try {
      const response = await axios.post<Comparison>(`${API_URL}/documents/compare`, { oldId: older.ID, newId: newer.ID, language });
      setShown({ result: response.data, oldName: older.Filename, newName: newer.Filename });
    } catch (err) {
      setError(apiError(err, t("compare.failed")));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4" data-testid="compare">
      <p className="text-sm text-ink/70">{t("compare.intro")}</p>
      <form onSubmit={compare} className="mt-3 space-y-4">
        <div className="flex flex-wrap items-end gap-4">
          <div className="flex min-w-[14rem] flex-1 flex-col gap-1">
            <label htmlFor={ids.search} className="label mb-0">
              {t("compare.searchLabel")}
            </label>
            <input id={ids.search} type="search" className={fieldClass} value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("compare.searchPlaceholder")} autoComplete="off" />
          </div>
          <div className="flex min-w-[14rem] flex-1 flex-col gap-1">
            <label htmlFor={ids.pick} className="label mb-0">
              {t("compare.pickLabel")}
            </label>
            <select id={ids.pick} className={fieldClass} value={otherId} onChange={(e) => setOtherId(e.target.value)} required aria-describedby={ids.note}>
              <option value="" disabled>
                {searching ? t("compare.searching") : "—"}
              </option>
              {candidates.map((d) => (
                <option key={d.ID} value={d.ID}>
                  {d.Filename} ({new Date(d.CreatedAt).toLocaleDateString()})
                </option>
              ))}
            </select>
          </div>
        </div>
        <p id={ids.note} className="muted text-sm" role="status">
          {!searching && candidates.length === 0 ? t("compare.noMatches") : ""}
        </p>

        <fieldset className="space-y-1">
          <legend className="label">{t("compare.directionLegend")}</legend>
          <label className="flex items-center gap-2">
            <input type="radio" name={`newer-${doc.ID}`} checked={thisIsNewer} onChange={() => setThisIsNewer(true)} />
            {t("compare.thisNewer")}
          </label>
          <label className="flex items-center gap-2">
            <input type="radio" name={`newer-${doc.ID}`} checked={!thisIsNewer} onChange={() => setThisIsNewer(false)} />
            {t("compare.thisOlder")}
          </label>
        </fieldset>

        <div className="flex flex-wrap items-end gap-4">
          <div className="flex flex-col gap-1">
            <label htmlFor={ids.language} className="label mb-0">
              {t("compare.languageLabel")}
            </label>
            <select id={ids.language} value={language} onChange={(e) => setLanguage(e.target.value)} className={fieldClass}>
              <option value="">{t("compare.sameLanguage")}</option>
              {LANGUAGES.map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </div>
          <button type="submit" disabled={busy || !other} className={actionButton}>
            {busy ? t("compare.running") : t("compare.run")}
          </button>
        </div>
      </form>

      {error && (
        <p className="mt-3 font-semibold text-danger" role="alert">
          {error}
        </p>
      )}
      {busy && (
        <p className="mt-3 text-sm font-semibold text-ink/70" role="status">
          {t("compare.running")}
        </p>
      )}
      {shown && <ComparisonView result={shown.result} oldName={shown.oldName} newName={shown.newName} />}
    </div>
  );
}
