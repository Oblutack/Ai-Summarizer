"use client";
import { useEffect, useRef, useState } from "react";
import { PLAYBACK_RATES, clampRate, loadPlayback, savePlayback } from "../lib/playback";
import { formatClock } from "../lib/recording";
import { useT } from "./I18nProvider";

interface AudioPlayerProps {
  // Where the sound comes from: an address for the browser's player (a blob made from a file, usually).
  src: string;
  // What is playing, for screen readers: the file name.
  label: string;
  // Asks the player to jump to a time and play from there; a new nonce is a new request, so the same time can be asked again.
  seek?: { seconds: number; nonce: number } | null;
}

const SKIP_SECONDS = 10;

const icon = { viewBox: "0 0 24 24", className: "h-4 w-4", fill: "currentColor", "aria-hidden": true } as const;

// A recording's player: play and pause, ten seconds back and forward, a slider to go anywhere, and the speed. The
// speed is remembered and shared with the player that reads a summary aloud.
export default function AudioPlayer({ src, label, seek }: AudioPlayerProps) {
  const t = useT();
  const audio = useRef<HTMLAudioElement | null>(null);
  const [playing, setPlaying] = useState(false);
  const [time, setTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [rate, setRate] = useState(1);
  const [dragging, setDragging] = useState<number | null>(null);
  const [failed, setFailed] = useState(false);

  // The speed chosen last time, read after the first render so the page and the first client render agree.
  useEffect(() => {
    const saved = loadPlayback().rate;
    setRate(saved);
    if (audio.current) audio.current.playbackRate = saved;
  }, []);

  // Stop when the player goes away.
  useEffect(() => {
    const element = audio.current;
    return () => element?.pause();
  }, []);

  // A jump asked for from outside (a time in the transcript, a cited passage). If the recording has not loaded
  // yet, it is remembered and made as soon as the length is known.
  const pending = useRef<number | null>(null);
  const goTo = (seconds: number) => {
    const element = audio.current;
    if (!element) return;
    element.currentTime = seconds;
    setTime(seconds);
    void element.play().catch(() => undefined);
  };
  useEffect(() => {
    const element = audio.current;
    if (!seek || !element) return;
    if (element.readyState >= 1) goTo(seek.seconds);
    else pending.current = seek.seconds;
  }, [seek]);
  const applyPending = () => {
    if (pending.current === null) return;
    const seconds = pending.current;
    pending.current = null;
    goTo(seconds);
  };

  const knownDuration = Number.isFinite(duration) && duration > 0 ? duration : 0;

  const onMetadata = () => {
    const element = audio.current;
    if (!element) return;
    // A recording made in the browser has no length until it has been read to the end: the browser says Infinity.
    // Going to the far end makes it work the length out; the position goes back to the start afterwards.
    if (element.duration === Infinity) {
      const settle = () => {
        element.removeEventListener("timeupdate", settle);
        element.currentTime = 0;
        setDuration(element.duration);
        applyPending();
      };
      element.addEventListener("timeupdate", settle);
      element.currentTime = 1e101;
      return;
    }
    setDuration(element.duration);
    applyPending();
  };

  const jump = (seconds: number) => {
    const element = audio.current;
    if (!element) return;
    const limit = knownDuration || element.duration || seconds;
    element.currentTime = Math.min(Math.max(0, seconds), Number.isFinite(limit) ? limit : seconds);
    setTime(element.currentTime);
  };

  const toggle = () => {
    const element = audio.current;
    if (!element) return;
    if (element.paused) void element.play().catch(() => setFailed(true));
    else element.pause();
  };

  const changeRate = (value: number) => {
    const chosen = clampRate(value);
    setRate(chosen);
    if (audio.current) audio.current.playbackRate = chosen;
    savePlayback({ ...loadPlayback(), rate: chosen });
  };

  const shown = dragging ?? time;
  const clock = `${formatClock(shown * 1000)} / ${formatClock(knownDuration * 1000)}`;

  return (
    <div role="group" aria-label={label} className="flex flex-col gap-2" data-testid="audio-player">
      <audio
        ref={audio}
        src={src}
        preload="metadata"
        onLoadedMetadata={onMetadata}
        onDurationChange={() => {
          if (audio.current && Number.isFinite(audio.current.duration)) setDuration(audio.current.duration);
        }}
        onTimeUpdate={() => audio.current && setTime(audio.current.currentTime)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
        onError={() => setFailed(true)}
      />
      <div className="flex flex-wrap items-center gap-2">
        <button type="button" onClick={() => jump(time - SKIP_SECONDS)} className="btn btn-secondary btn-sm" aria-label={t("playback.back10")} title={t("playback.back10")}>
          <svg {...icon}>
            <path d="M12 5V2L7 6l5 4V7a5 5 0 1 1-5 5H5a7 7 0 1 0 7-7z" />
          </svg>
          <span className="ml-1 text-xs font-semibold" aria-hidden="true">10</span>
        </button>
        <button type="button" onClick={toggle} className="btn btn-primary btn-sm gap-2" data-testid="audio-toggle">
          {playing ? (
            <svg {...icon}>
              <path d="M7 5h3.5v14H7zM13.5 5H17v14h-3.5z" />
            </svg>
          ) : (
            <svg {...icon}>
              <path d="M8 5v14l11-7z" />
            </svg>
          )}
          {playing ? t("podcast.pause") : t("podcast.play")}
        </button>
        <button type="button" onClick={() => jump(time + SKIP_SECONDS)} className="btn btn-secondary btn-sm" aria-label={t("playback.forward10")} title={t("playback.forward10")}>
          <span className="mr-1 text-xs font-semibold" aria-hidden="true">10</span>
          <svg {...icon}>
            <path d="M12 5V2l5 4-5 4V7a5 5 0 1 0 5 5h2a7 7 0 1 1-7-7z" />
          </svg>
        </button>
        <span className="text-sm font-medium tabular-nums text-ink/80" data-testid="audio-time">
          {clock}
        </span>
        <label className="flex items-center gap-2">
          <span className="text-sm font-semibold text-ink/80">{t("podcast.speed")}</span>
          <select value={rate} onChange={(e) => changeRate(Number(e.target.value))} className="h-9 rounded-lg border border-ink/40 bg-surface px-2 text-sm">
            {PLAYBACK_RATES.map((r) => (
              <option key={r} value={r}>
                {r}x
              </option>
            ))}
          </select>
        </label>
      </div>
      <input
        type="range"
        min={0}
        max={Math.max(1, Math.floor(knownDuration))}
        step={1}
        value={Math.min(Math.floor(shown), Math.max(1, Math.floor(knownDuration)))}
        disabled={knownDuration === 0}
        onChange={(e) => setDragging(Number(e.target.value))}
        onPointerUp={() => {
          if (dragging !== null) jump(dragging);
          setDragging(null);
        }}
        onKeyUp={() => {
          if (dragging !== null) jump(dragging);
          setDragging(null);
        }}
        aria-label={t("playback.recordingPosition")}
        aria-valuetext={t("playback.positionTime", { now: formatClock(shown * 1000), total: formatClock(knownDuration * 1000) })}
        className="w-full"
        data-testid="audio-position"
      />
      {failed && (
        <p className="text-sm font-medium text-danger" aria-live="polite">
          {t("playback.audioFailed")}
        </p>
      )}
    </div>
  );
}
