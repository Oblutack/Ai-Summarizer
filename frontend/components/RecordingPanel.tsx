"use client";
import { useEffect, useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import type { FileInfo } from "../types";
import AudioPlayer from "./AudioPlayer";
import { useT } from "./I18nProvider";
import OriginalText from "./OriginalText";

interface RecordingPanelProps {
  documentId: number;
  file: FileInfo;
  // A jump asked for from outside (a cited passage in the chat). A new object each time.
  seek?: { seconds: number; nonce: number } | null;
}

// A saved recording: the player, and under it the transcript, whose times start the recording from that place.
export default function RecordingPanel({ documentId, file, seek }: RecordingPanelProps) {
  const t = useT();
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  const [asked, setAsked] = useState<{ seconds: number; nonce: number } | null>(null);

  // Whatever was asked from outside, and whatever is clicked here, ends up as one request to the player.
  useEffect(() => {
    if (seek) setAsked(seek);
  }, [seek]);

  // The recording is fetched when the panel opens (it can be several megabytes), as a blob made in the browser.
  useEffect(() => {
    let cancelled = false;
    let made = "";
    axios
      .get<Blob>(`${API_URL}/documents/${documentId}/files/${file.id}`, { responseType: "blob" })
      .then((response) => {
        if (cancelled) return;
        made = URL.createObjectURL(response.data);
        setUrl(made);
      })
      .catch((err) => !cancelled && setError(apiError(err, t("doc.recordingFailed"))));
    return () => {
      cancelled = true;
      if (made) URL.revokeObjectURL(made);
    };
  }, [documentId, file.id, t]);

  return (
    <section aria-label={t("doc.recording")} className="mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4" data-testid="recording-panel">
      <h4 className="mb-3 font-sans text-lg font-semibold">{t("doc.recording")}</h4>
      {error && (
        <p className="text-base font-medium text-danger" role="alert">
          {error}
        </p>
      )}
      {!url && !error && <p className="muted text-sm">{t("doc.recordingLoading")}</p>}
      {url && <AudioPlayer src={url} label={file.name} seek={asked} />}
      <OriginalText documentId={documentId} transcript onSeek={(seconds) => setAsked({ seconds, nonce: Date.now() })} />
    </section>
  );
}
