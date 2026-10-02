"use client";
import { LANGUAGES, SUMMARY_STYLES } from "../../lib/summaryOptions";

interface SummaryOptionsProps {
  wordCount: number;
  onWordCountChange: (value: number) => void;
  // The word count is replaced by a page limit for long inputs.
  wordCountDisabled: boolean;
  style: string;
  onStyleChange: (value: string) => void;
  language: string;
  onLanguageChange: (value: string) => void;
}

const selectClass =
  "bg-canvas border-2 border-ink rounded-md px-3 py-1 focus:outline-none cursor-pointer";

export default function SummaryOptions({
  wordCount,
  onWordCountChange,
  wordCountDisabled,
  style,
  onStyleChange,
  language,
  onLanguageChange,
}: SummaryOptionsProps) {
  const dim = wordCountDisabled ? "opacity-50" : "";

  return (
    <>
      <div className="w-full flex flex-col md:flex-row justify-center items-center md:space-x-4">
        <label htmlFor="word-count" className={`uppercase tracking-widest ${dim}`}>
          Summary Word Count:
        </label>
        <div className="flex flex-col items-center my-2 md:my-0">
          <input
            id="word-count"
            type="range"
            disabled={wordCountDisabled}
            min="50"
            max="500"
            step="10"
            value={wordCount}
            onChange={(e) => onWordCountChange(Number(e.target.value))}
            className="w-60 disabled:opacity-50 disabled:cursor-not-allowed"
          />
          <div
            aria-hidden="true"
            className="w-60 flex justify-between px-1 -mt-1 text-ink opacity-40 text-xs"
          >
            {Array.from({ length: 11 }, (_, i) => (
              <span key={i}>|</span>
            ))}
          </div>
        </div>
        <span className={`w-28 text-center tracking-widest ${dim}`}>{wordCount} Words</span>
      </div>

      <div className="w-full flex flex-col md:flex-row justify-center items-center gap-4 md:gap-8">
        <label className="flex items-center gap-3 uppercase tracking-widest">
          Style
          <select value={style} onChange={(e) => onStyleChange(e.target.value)} className={selectClass}>
            {SUMMARY_STYLES.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-3 uppercase tracking-widest">
          Language
          <select
            value={language}
            onChange={(e) => onLanguageChange(e.target.value)}
            className={selectClass}
          >
            {LANGUAGES.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
        </label>
      </div>
    </>
  );
}
