"use client";
import { useEffect, useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import CopyButton from "./CopyButton";
import { useT } from "./I18nProvider";

interface OriginalTextProps {
  documentId: number;
  // True for a recording: its text is the transcript.
  transcript: boolean;
}

// The text a saved summary was made from: for a recording, what was said, with the time of each paragraph. It is
// fetched only when opened (it can be long) and only the owner can ask for it.
export default function OriginalText({ documentId, transcript }: OriginalTextProps) {
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
        <div className="reading max-h-96 overflow-y-auto whitespace-pre-wrap pr-1 text-base" tabIndex={0} data-testid="original-text-body">
          {text}
        </div>
      )}
    </section>
  );
}
