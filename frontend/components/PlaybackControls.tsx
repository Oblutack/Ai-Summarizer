"use client";
import { useState } from "react";
import { PLAYBACK_RATES, minutesLeft } from "../lib/playback";
import type { PlayStatus } from "../hooks/useSpeechPlayer";
import { useT } from "./I18nProvider";

interface PlaybackControlsProps {
  // A name for the group of controls, for screen readers: "Reading aloud", "Podcast".
  label: string;
  status: PlayStatus;
  // The item being spoken (or -1), and how many there are.
  index: number;
  count: number;
  rate: number;
  // Seconds of speech left from the current item.
  remaining: number;
  disabled?: boolean;
  onPlay: () => void;
  onPause: () => void;
  onResume: () => void;
  onStop: () => void;
  onPrevious: () => void;
  onNext: () => void;
  onSeek: (index: number) => void;
  onRate: (rate: number) => void;
}

const icon = { viewBox: "0 0 24 24", className: "h-4 w-4", fill: "currentColor", "aria-hidden": true } as const;

const PlayIcon = () => (
  <svg {...icon}>
    <path d="M8 5v14l11-7z" />
  </svg>
);
const PauseIcon = () => (
  <svg {...icon}>
    <path d="M7 5h3.5v14H7zM13.5 5H17v14h-3.5z" />
  </svg>
);
const StopIcon = () => (
  <svg {...icon}>
    <path d="M6 6h12v12H6z" />
  </svg>
);
const PreviousIcon = () => (
  <svg {...icon}>
    <path d="M6 5h2v14H6zM20 5v14L9 12z" />
  </svg>
);
const NextIcon = () => (
  <svg {...icon}>
    <path d="M16 5h2v14h-2zM4 5v14l11-7z" />
  </svg>
);

// What a listener expects of a player, for anything that is spoken: skip back and forward, pause, jump to a place
// with the slider, see how much is left, and speed it up. The caller says what happens for each.
export default function PlaybackControls({
  label,
  status,
  index,
  count,
  rate,
  remaining,
  disabled,
  onPlay,
  onPause,
  onResume,
  onStop,
  onPrevious,
  onNext,
  onSeek,
  onRate,
}: PlaybackControlsProps) {
  const t = useT();
  // While the slider is being dragged it shows where it is, and the jump happens when it is let go.
  const [dragging, setDragging] = useState<number | null>(null);
  const position = dragging ?? Math.max(index, 0);
  const minutes = minutesLeft(remaining);

  const commit = () => {
    if (dragging !== null) onSeek(dragging);
    setDragging(null);
  };

  return (
    <div role="group" aria-label={label} className="flex flex-col gap-2" data-testid="playback-controls">
      <div className="flex flex-wrap items-center gap-2">
        <button type="button" onClick={onPrevious} disabled={disabled || status === "idle"} className="btn btn-secondary btn-sm" aria-label={t("playback.previous")} title={t("playback.previous")}>
          <PreviousIcon />
        </button>
        {status === "idle" && (
          <button type="button" onClick={onPlay} disabled={disabled} className="btn btn-primary btn-sm gap-2">
            <PlayIcon />
            {t("podcast.play")}
          </button>
        )}
        {status === "playing" && (
          <button type="button" onClick={onPause} className="btn btn-secondary btn-sm gap-2">
            <PauseIcon />
            {t("podcast.pause")}
          </button>
        )}
        {status === "paused" && (
          <button type="button" onClick={onResume} className="btn btn-primary btn-sm gap-2">
            <PlayIcon />
            {t("podcast.resume")}
          </button>
        )}
        <button type="button" onClick={onNext} disabled={disabled || status === "idle"} className="btn btn-secondary btn-sm" aria-label={t("playback.next")} title={t("playback.next")}>
          <NextIcon />
        </button>
        {status !== "idle" && (
          <button type="button" onClick={onStop} className="btn btn-secondary btn-sm gap-2">
            <StopIcon />
            {t("podcast.stop")}
          </button>
        )}
        <label className="flex items-center gap-2">
          <span className="text-sm font-semibold text-ink/80">{t("podcast.speed")}</span>
          <select
            value={rate}
            onChange={(e) => onRate(Number(e.target.value))}
            className="h-9 rounded-lg border border-ink/40 bg-surface px-2 text-sm"
          >
            {PLAYBACK_RATES.map((r) => (
              <option key={r} value={r}>
                {r}x
              </option>
            ))}
          </select>
        </label>
      </div>

      {count > 1 && (
        <div className="flex flex-wrap items-center gap-3">
          <input
            type="range"
            min={0}
            max={count - 1}
            step={1}
            value={position}
            disabled={disabled}
            onChange={(e) => setDragging(Number(e.target.value))}
            onPointerUp={commit}
            onKeyUp={commit}
            onBlur={commit}
            aria-label={t("playback.position")}
            aria-valuetext={t("playback.positionText", { n: position + 1, total: count })}
            className="min-w-[8rem] flex-1"
            data-testid="playback-position"
          />
          <span className="text-sm font-medium tabular-nums text-ink/80" data-testid="playback-progress">
            {position + 1} / {count}
          </span>
          {status !== "idle" && (
            <span className="text-sm text-ink/70" data-testid="playback-left">
              {minutes === 0 ? t("playback.leftLess") : t("playback.left", { n: minutes })}
            </span>
          )}
        </div>
      )}
    </div>
  );
}
