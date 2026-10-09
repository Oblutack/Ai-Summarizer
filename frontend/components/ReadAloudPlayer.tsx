"use client";
import { useEffect, useMemo, useRef, useState } from "react";
import { useSpeechPlayer, type SpeechItem } from "../hooks/useSpeechPlayer";
import { toPlainText } from "../lib/markdownText";
import { useT } from "./I18nProvider";
import PlaybackControls from "./PlaybackControls";
import { textButton } from "./styles";

// The button that opens the reader. It stays hidden in browsers that cannot speak.
export function ReadAloudToggle({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const t = useT();
  const [supported, setSupported] = useState(false);
  useEffect(() => setSupported(typeof window !== "undefined" && "speechSynthesis" in window), []);
  if (!supported) return null;
  return (
    <button type="button" onClick={onToggle} className={textButton} aria-expanded={open}>
      {open ? t("readAloud.close") : t("readAloud.start")}
    </button>
  );
}

interface ReadAloudPlayerProps {
  // The summary as Markdown.
  markdown: string;
  onClose: () => void;
}

// Reads a summary aloud with one of the browser's own voices, paragraph by paragraph, with the controls of a
// player: pause, skip back and forward, a slider, the speed, the voice. It starts as soon as it opens.
export default function ReadAloudPlayer({ markdown, onClose }: ReadAloudPlayerProps) {
  const t = useT();
  const items: SpeechItem[] = useMemo(
    () =>
      toPlainText(markdown)
        .split("\n")
        .map((text) => text.trim())
        .filter(Boolean)
        .map((text) => ({ text, role: "read" as const })),
    [markdown]
  );
  const language = typeof navigator !== "undefined" ? navigator.language || "en" : "en";
  const player = useSpeechPlayer(items, language);

  // It starts by itself, once, when the person opened it.
  const started = useRef(false);
  useEffect(() => {
    if (!started.current && player.supported && items.length > 0) {
      started.current = true;
      player.play(0);
    }
  }, [player, items.length]);

  const current = player.index >= 0 ? items[player.index]?.text : "";

  return (
    <section aria-label={t("readAloud.panel")} className="mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4" data-testid="read-aloud">
      <div className="mb-3 flex items-center justify-between gap-2">
        <h4 className="font-sans text-lg font-semibold">{t("readAloud.panel")}</h4>
        <button type="button" onClick={onClose} className="btn btn-quiet btn-sm">
          {t("readAloud.close")}
        </button>
      </div>

      <PlaybackControls
        label={t("readAloud.panel")}
        status={player.status}
        index={player.index}
        count={items.length}
        rate={player.rate}
        remaining={player.remaining}
        disabled={!player.supported}
        onPlay={() => player.play(0)}
        onPause={player.pause}
        onResume={player.resume}
        onStop={player.stop}
        onPrevious={player.previous}
        onNext={player.next}
        onSeek={player.seek}
        onRate={player.setRate}
      />

      {player.voices.length > 1 && (
        <label className="mt-3 flex flex-wrap items-center gap-2 text-sm">
          <span className="font-semibold text-ink/80">{t("readAloud.voice")}</span>
          <select
            value={player.voiceOf("read")}
            onChange={(e) => player.setVoice("read", e.target.value)}
            className="h-9 max-w-[18rem] rounded-lg border border-ink/40 bg-surface px-2 text-sm"
          >
            {player.voices.map((voice) => (
              <option key={voice.voiceURI} value={voice.voiceURI}>
                {voice.name} ({voice.lang})
              </option>
            ))}
          </select>
        </label>
      )}

      {current && (
        <p className="reading mt-3 border-l-4 border-accent pl-3 text-base" data-testid="now-reading">
          <span className="mb-1 block text-xs font-semibold uppercase tracking-widest text-ink/70">{t("readAloud.nowReading")}</span>
          {current}
        </p>
      )}

      {player.failed && (
        <p className="mt-3 text-sm font-medium text-danger" role="alert">
          {t("readAloud.failed")}
        </p>
      )}
    </section>
  );
}
