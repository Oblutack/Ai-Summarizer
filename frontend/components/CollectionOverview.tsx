"use client";
import { useEffect, useRef, useState } from "react";
import Markdown from "markdown-to-jsx";
import { useAuth } from "../contexts/AuthContext";
import { API_URL } from "../lib/api";
import { createMarkdownOptions } from "../lib/markdown";
import { tidyMarkdown } from "../lib/markdownText";
import { loadPreferences } from "../lib/preferences";
import { readSummaryEvents } from "../lib/sse";
import CopyButton from "./CopyButton";
import { useT } from "./I18nProvider";

const markdownOptions = createMarkdownOptions();
// A briefing on several documents needs more room than a summary of one.
const OVERVIEW_WORDS = 300;

interface CollectionOverviewProps {
  // The collection (a tag) and how many documents are in it.
  tag: string;
  count: number;
}

// One briefing on a whole collection: what its documents say together, where they agree and where they
// differ. It streams in like a summary, is written from the saved summaries, and is not saved.
export default function CollectionOverview({ tag, count }: CollectionOverviewProps) {
  const t = useT();
  const { refresh } = useAuth();
  const [text, setText] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [written, setWritten] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  // Stop writing when the collection is changed or the page is left.
  useEffect(() => () => abortRef.current?.abort(), []);

  const write = async () => {
    const controller = new AbortController();
    abortRef.current = controller;
    setText("");
    setError("");
    setWritten(false);
    setLoading(true);
    try {
      const { style, language } = loadPreferences();
      const query = new URLSearchParams({ wordCount: String(OVERVIEW_WORDS), style, language, stream: "true" });
      const response = await fetch(`${API_URL}/library/overview?${query.toString()}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tag }),
        credentials: "include",
        signal: controller.signal,
      });
      if (!response.ok) {
        const body = await response.json().catch(() => null);
        if (response.status === 401 && body?.code === "unauthenticated") refresh();
        setError(body?.error || t("form.errGeneric"));
        return;
      }
      let outcome: "done" | "error" | undefined;
      await readSummaryEvents(response, (event) => {
        if (event.type === "delta") setText((previous) => previous + event.text);
        else if (event.type === "error") {
          setError(event.message || t("form.errGeneric"));
          outcome = "error";
        } else if (event.type === "done") outcome = "done";
      });
      if (outcome === "done") setWritten(true);
      else if (!outcome) setError(t("form.errInterrupted"));
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") setError(t("form.errCancelled"));
      else if (err instanceof TypeError) setError(t("form.errNetwork"));
      else setError(t("form.errUnexpected"));
    } finally {
      setLoading(false);
      abortRef.current = null;
    }
  };

  const hasText = text !== "";

  return (
    <div className="mt-4" data-testid="overview">
      <div className="flex flex-wrap items-center gap-2">
        {loading ? (
          <button type="button" onClick={() => abortRef.current?.abort()} className="btn btn-danger btn-sm">
            {t("common.cancel")}
          </button>
        ) : (
          <button type="button" onClick={write} className="btn btn-secondary btn-sm">
            {hasText || written ? t("library.overviewAgain") : t("library.overviewButton")}
          </button>
        )}
        {!hasText && !loading && <span className="muted text-sm">{t("library.overviewHint", { count })}</span>}
      </div>

      {error && (
        <p className="mt-3 text-base font-medium text-danger" role="alert">
          {error}
        </p>
      )}

      {(hasText || loading) && (
        <div className="mt-3 rounded-xl border border-ink/20 bg-canvas/50 p-4">
          <h3 className="mb-2 font-sans text-lg font-semibold">{t("library.overviewHeading", { tag })}</h3>
          <div className="reading" aria-busy={loading} data-testid="overview-text">
            <Markdown options={markdownOptions}>{tidyMarkdown(text)}</Markdown>
          </div>
          {loading && (
            <p className="mt-2 text-sm font-semibold text-ink/70" role="status">
              {t("stage.writing")}
            </p>
          )}
          {written && (
            <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
              <p className="muted text-sm">{t("library.overviewNote", { count })}</p>
              <CopyButton text={text} what={t("copy.whatSummary")} />
            </div>
          )}
        </div>
      )}
    </div>
  );
}
