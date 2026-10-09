"use client";
import { LANGUAGES, SUMMARY_STYLES } from "../../lib/summaryOptions";
import { useT } from "../I18nProvider";

interface SummaryOptionsProps {
  wordCount: number;
  onWordCountChange: (value: number) => void;
  // The word count is replaced by a page limit for long inputs.
  wordCountDisabled: boolean;
  style: string;
  onStyleChange: (value: string) => void;
  language: string;
  onLanguageChange: (value: string) => void;
  // The page limit control, placed next to the others when it applies.
  children?: React.ReactNode;
}

// How the summary should be: its length, style and language, in one row.
export default function SummaryOptions({
  wordCount,
  onWordCountChange,
  wordCountDisabled,
  style,
  onStyleChange,
  language,
  onLanguageChange,
  children,
}: SummaryOptionsProps) {
  const dim = wordCountDisabled ? "opacity-50" : "";
  const t = useT();

  return (
    <div className="grid gap-x-6 gap-y-4 sm:grid-cols-2 lg:grid-cols-[1.3fr_1fr_1fr]">
      <div>
        <div className={`flex items-baseline justify-between gap-3 ${dim}`}>
          <label htmlFor="word-count" className="label mb-0">
            {t("form.wordCount")}
          </label>
          {/* With files attached the page limit sets the length, so the number would only be a dimmed, low-contrast distraction. */}
          {!wordCountDisabled && <span className="text-sm font-semibold tabular-nums">{t("form.words", { n: wordCount })}</span>}
        </div>
        <input
          id="word-count"
          type="range"
          disabled={wordCountDisabled}
          min="50"
          max="500"
          step="10"
          value={wordCount}
          onChange={(e) => onWordCountChange(Number(e.target.value))}
          className="mt-1"
        />
      </div>

      <div>
        <label htmlFor="summary-style" className="label">
          {t("form.style")}
        </label>
        <select id="summary-style" value={style} onChange={(e) => onStyleChange(e.target.value)} className="field">
          {SUMMARY_STYLES.map((s) => (
            <option key={s.value} value={s.value}>
              {t(`style.${s.value}`)}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label htmlFor="summary-language" className="label">
          {t("form.language")}
        </label>
        <select id="summary-language" value={language} onChange={(e) => onLanguageChange(e.target.value)} className="field">
          {LANGUAGES.map((l) => (
            <option key={l} value={l}>
              {t(`lang.${l}`)}
            </option>
          ))}
        </select>
      </div>

      {children}
    </div>
  );
}
