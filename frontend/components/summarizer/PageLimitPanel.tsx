"use client";
import { useT } from "../I18nProvider";

interface PageLimitPanelProps {
  value: string;
  onChange: (value: string) => void;
  onIncrement: () => void;
  onDecrement: () => void;
}

function Step({ label, onClick, children }: { label: string; onClick: () => void; children: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-lg border border-ink/40 bg-surface text-xl font-semibold hover:bg-ink/10"
    >
      {children}
    </button>
  );
}

// For long inputs a page limit replaces the word count: how many pages the summary may fill.
export default function PageLimitPanel({ value, onChange, onIncrement, onDecrement }: PageLimitPanelProps) {
  const t = useT();
  return (
    <div>
      <label htmlFor="page-limit" className="label">
        {t("form.pageLimit")}
      </label>
      <div className="flex items-center gap-2">
        <Step label={t("form.decrease")} onClick={onDecrement}>
          −
        </Step>
        <input
          id="page-limit"
          type="number"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t("form.pageLimitExample")}
          aria-label={t("form.pageLimitAria")}
          className="field w-20 text-center"
        />
        <Step label={t("form.increase")} onClick={onIncrement}>
          +
        </Step>
      </div>
      <p className="muted mt-1 text-sm">{t("form.pageApprox")}</p>
    </div>
  );
}
