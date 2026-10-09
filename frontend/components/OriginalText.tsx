"use client";
import { useEffect, useMemo, useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import { splitTimeMark } from "../lib/playback";
import CopyButton from "./CopyButton";
import { useT } from "./I18nProvider";

interface OriginalTextProps {
  documentId: number;
  // True for a recording: its text is the transcript.
  transcript: boolean;
  // When the recording can be played, a click on a time mark starts it from there.
  onSeek?: (seconds: number) => void;
}

// The text a saved summary was made from: for a recording, what was said, with the time of each paragraph. It is
// fetched only when opened (it can be long) and only the owner can ask for it.
export default function OriginalText({ documentId, transcript, onSeek }: OriginalTextProps) {
  const t = useT();
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    axios
      .get<{ text: string }>(`${API_URL}/documents/${documentId}/text`)
      .then((response) => !cancelled && setText(response.data.text))
      .catch((err) => !cancelled && setError(apiError(err, t("doc.textFailed"))));
    return () => {
      cancelled = true;
    };
  }, [documentId, t]);

  // Paragraphs are separated by a blank line (and a page of a recording by a form feed).
  const paragraphs = useMemo(() => (text === null ? [] : text.split(/\n{2,}|\f/).filter((p) => p.trim())), [text]);

  const label = transcript ? t("doc.transcript") : t("doc.originalText");

  return (
    <section aria-label={label} className="mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4" data-testid="original-text">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h4 className="font-sans text-lg font-semibold">{label}</h4>
        {text !== null && <CopyButton text={text} what={label.toLowerCase()} />}
      </div>
      {error && (
        <p className="text-base font-medium text-danger" role="alert">
          {error}
        </p>
      )}
      {text === null && !error && <p className="muted text-sm">{t("doc.textLoading")}</p>}
      {text !== null && (
        // A long text scrolls inside its box; tabIndex lets a keyboard reach it to scroll.
        <div className="reading max-h-96 space-y-3 overflow-y-auto pr-1 text-base" tabIndex={0} data-testid="original-text-body">
          {paragraphs.map((paragraph, i) => {
            const mark = onSeek ? splitTimeMark(paragraph) : null;
            return mark ? (
              <p key={i} className="whitespace-pre-wrap">
                <button
                  type="button"
                  onClick={() => onSeek?.(mark.seconds)}
                  className="mr-1 rounded border border-accent/50 px-1.5 text-sm font-semibold tabular-nums text-accent hover:bg-accent/10"
                  aria-label={t("sources.playFrom", { time: mark.clock })}
                  title={t("sources.playFrom", { time: mark.clock })}
                  data-testid="time-mark"
                >
                  {mark.clock}
                </button>{" "}
                {mark.rest}
              </p>
            ) : (
              <p key={i} className="whitespace-pre-wrap">
                {paragraph}
              </p>
            );
          })}
        </div>
      )}
    </section>
  );
}
