"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { toPlainText } from "../lib/markdownText";
import { pickVoices, splitForSpeech } from "../lib/podcast";
import { useT } from "./I18nProvider";
import { textButton } from "./styles";

interface ReadAloudButtonProps {
  // The summary as Markdown.
  markdown: string;
}

// Reads a summary aloud with one of the browser's own voices (nothing is sent anywhere). It stays
// hidden in browsers that cannot speak.
export default function ReadAloudButton({ markdown }: ReadAloudButtonProps) {
  const t = useT();
  const [supported, setSupported] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [error, setError] = useState("");
  // Bumped to make the callbacks of an earlier reading do nothing once it has been stopped.
  const run = useRef(0);

  useEffect(() => setSupported(typeof window !== "undefined" && "speechSynthesis" in window), []);

  const stop = useCallback(() => {
    run.current++;
    window.speechSynthesis?.cancel();
    setPlaying(false);
  }, []);

  // Stop talking when the summary goes away.
  useEffect(
    () => () => {
      run.current++;
      if (typeof window !== "undefined") window.speechSynthesis?.cancel();
    },
    []
  );

  const play = () => {
    const synth = window.speechSynthesis;
    const chunks = splitForSpeech(toPlainText(markdown).replace(/\n/g, " "));
    if (!synth || chunks.length === 0) return;
    synth.cancel();
    const id = ++run.current;
    const [voice] = pickVoices(synth.getVoices(), navigator.language || "en");
    setError("");
    setPlaying(true);

    const speak = (i: number) => {
      if (run.current !== id) return;
      if (i >= chunks.length) {
        setPlaying(false);
        return;
      }
      const utterance = new SpeechSynthesisUtterance(chunks[i]);
      if (voice) utterance.voice = voice as SpeechSynthesisVoice;
      utterance.onend = () => speak(i + 1);
      utterance.onerror = (event) => {
        if (run.current !== id || event.error === "interrupted" || event.error === "canceled") return;
        run.current++;
        setPlaying(false);
        setError(t("readAloud.failed"));
      };
      synth.speak(utterance);
    };
    speak(0);
  };

  if (!supported) return null;
  return (
    <>
      <button type="button" onClick={playing ? stop : play} className={textButton} aria-pressed={playing}>
        {playing ? t("readAloud.stop") : t("readAloud.start")}
      </button>
      {error && (
        <span className="text-sm font-medium text-danger" role="alert">
          {error}
        </span>
      )}
    </>
  );
}
