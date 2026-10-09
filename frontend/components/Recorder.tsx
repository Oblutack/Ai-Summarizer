"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  RECORDING_BITS_PER_SECOND,
  RECORDING_MAX_BYTES,
  RECORDING_MAX_MINUTES,
  canRecordHere,
  formatClock,
  pickRecordingType,
  recordingFileName,
} from "../lib/recording";
import { useT } from "./I18nProvider";

interface RecorderProps {
  // Called with the finished recording, as a file the summarizer takes like any other.
  onRecorded: (file: File) => void;
}

function MicIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="9" y="2" width="6" height="12" rx="3" />
      <path d="M5 11a7 7 0 0 0 14 0M12 18v4" />
    </svg>
  );
}

type Phase = "idle" | "recording" | "blocked" | "failed";

// Records from the microphone, so a meeting needs no other app: the recording ends up in the same list as an
// attached file. Nothing is sent anywhere until the person presses Summarize.
export default function Recorder({ onRecorded }: RecorderProps) {
  const t = useT();
  // Whether this browser can record is only known in the browser, so the button appears after the first render.
  const [available, setAvailable] = useState(false);
  const [phase, setPhase] = useState<Phase>("idle");
  const [elapsed, setElapsed] = useState(0);
  const [notice, setNotice] = useState("");
  const recorder = useRef<MediaRecorder | null>(null);
  const stream = useRef<MediaStream | null>(null);
  const chunks = useRef<Blob[]>([]);
  const bytes = useRef(0);
  const startedAt = useRef(0);
  const discard = useRef(false);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => setAvailable(canRecordHere()), []);

  const release = useCallback(() => {
    if (timer.current) clearInterval(timer.current);
    timer.current = null;
    stream.current?.getTracks().forEach((track) => track.stop());
    stream.current = null;
  }, []);

  // Leaving the page (or the form) stops the microphone, and keeps nothing.
  useEffect(
    () => () => {
      discard.current = true;
      if (recorder.current && recorder.current.state !== "inactive") recorder.current.stop();
      release();
    },
    [release]
  );

  const finish = useCallback(
    (reason?: "limit") => {
      if (reason === "limit") setNotice(t("record.limit", { minutes: RECORDING_MAX_MINUTES }));
      if (recorder.current && recorder.current.state !== "inactive") recorder.current.stop();
    },
    [t]
  );

  const start = async () => {
    setNotice("");
    let media: MediaStream;
    try {
      media = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch {
      setPhase("blocked");
      return;
    }
    try {
      const type = pickRecordingType((candidate) => MediaRecorder.isTypeSupported(candidate));
      const instance = new MediaRecorder(media, {
        ...(type ? { mimeType: type } : {}),
        audioBitsPerSecond: RECORDING_BITS_PER_SECOND,
      });
      stream.current = media;
      recorder.current = instance;
      chunks.current = [];
      bytes.current = 0;
      discard.current = false;

      instance.ondataavailable = (event) => {
        if (event.data.size === 0) return;
        chunks.current.push(event.data);
        bytes.current += event.data.size;
        if (bytes.current >= RECORDING_MAX_BYTES) finish("limit");
      };
      instance.onstop = () => {
        const kept = !discard.current && chunks.current.length > 0;
        const mime = instance.mimeType || type || "audio/webm";
        const blob = new Blob(chunks.current, { type: mime });
        release();
        recorder.current = null;
        chunks.current = [];
        setPhase("idle");
        if (kept) onRecorded(new File([blob], recordingFileName(mime), { type: mime }));
      };
      instance.start(1000); // a piece every second, so the size can be watched and nothing is lost
      startedAt.current = Date.now();
      setElapsed(0);
      setPhase("recording");
      timer.current = setInterval(() => {
        const ms = Date.now() - startedAt.current;
        setElapsed(ms);
        if (ms >= RECORDING_MAX_MINUTES * 60_000) finish("limit");
      }, 250);
    } catch {
      media.getTracks().forEach((track) => track.stop());
      setPhase("failed");
    }
  };

  const stop = () => finish();
  const cancel = () => {
    discard.current = true;
    finish();
  };

  if (!available) return null;

  if (phase === "recording") {
    return (
      <div className="flex flex-wrap items-center gap-2 px-2 py-1" data-testid="recorder">
        <span className="sr-only" role="status">
          {t("record.recording")}
        </span>
        <span className="h-3 w-3 animate-pulse rounded-full bg-danger" aria-hidden="true" />
        <span className="font-semibold tabular-nums" data-testid="recording-time" aria-hidden="true">
          {formatClock(elapsed)}
        </span>
        <button type="button" onClick={stop} className="btn btn-primary btn-sm">
          {t("record.stop")}
        </button>
        <button type="button" onClick={cancel} className="btn btn-quiet btn-sm">
          {t("record.discard")}
        </button>
      </div>
    );
  }

  return (
    <>
      <button type="button" onClick={start} className="btn btn-quiet gap-2" data-testid="record-button">
        <MicIcon />
        {t("record.start")}
      </button>
      {phase === "blocked" && (
        <p className="px-2 text-sm font-medium text-danger" role="alert">
          {t("record.blocked")}
        </p>
      )}
      {phase === "failed" && (
        <p className="px-2 text-sm font-medium text-danger" role="alert">
          {t("record.failed")}
        </p>
      )}
      {notice && (
        <p className="px-2 text-sm font-medium text-accent" role="status">
          {notice}
        </p>
      )}
    </>
  );
}
