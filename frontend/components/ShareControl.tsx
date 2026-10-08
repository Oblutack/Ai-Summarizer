"use client";
import { useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import CopyButton from "./CopyButton";
import { useT } from "./I18nProvider";
import { textButton } from "./styles";

interface ShareControlProps {
  documentId: number;
  // The current public link token, if the summary is shared.
  token?: string;
  onChange: (token: string | undefined) => void;
}

// Turns a public, read-only link to a summary on or off. The link shows the summary and its title only.
export default function ShareControl({ documentId, token, onChange }: ShareControlProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const t = useT();

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (err) {
      setError(apiError(err, t("share.failed")));
    } finally {
      setBusy(false);
    }
  };

  const share = () =>
    run(async () => {
      const response = await axios.post<{ token: string }>(`${API_URL}/documents/${documentId}/share`);
      onChange(response.data.token);
    });

  const unshare = () =>
    run(async () => {
      await axios.delete(`${API_URL}/documents/${documentId}/share`);
      onChange(undefined);
    });

  const link = token && typeof window !== "undefined" ? `${window.location.origin}/s/${token}` : "";

  return (
    <div className="contents" data-testid="share">
      {!token ? (
        <button type="button" onClick={share} disabled={busy} className={textButton}>
          {busy ? t("share.making") : t("share.make")}
        </button>
      ) : (
        <div className="mt-2 w-full rounded-xl border border-ink/20 bg-canvas/50 p-4">
          <p className="text-sm text-ink/70">
            {t("share.notice")}
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <input
              readOnly
              value={link}
              aria-label={t("share.linkAria")}
              onFocus={(e) => e.currentTarget.select()}
              className="field min-w-0 flex-1 text-sm"
            />
            <CopyButton text={link} what={t("copy.whatLink")} variant="link" />
            <button type="button" onClick={unshare} disabled={busy} className={textButton}>
              {t("share.stop")}
            </button>
          </div>
        </div>
      )}
      {error && (
        <p className="text-sm font-medium text-danger" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
