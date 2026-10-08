"use client";
import { useT } from "../I18nProvider";

interface PageLimitPanelProps {
  value: string;
  onChange: (value: string) => void;
  onIncrement: () => void;
  onDecrement: () => void;
}

function Arrow({ points, label, onClick }: { points: string; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="text-ink h-4 w-4 flex items-center justify-center hover:opacity-70"
    >
      <svg viewBox="0 0 10 10" className="w-full h-full fill-current" aria-hidden="true">
        <polygon points={points} />
      </svg>
    </button>
  );
}

export default function PageLimitPanel({ value, onChange, onIncrement, onDecrement }: PageLimitPanelProps) {
  const t = useT();
  return (
    <div className="w-full lg:w-48 flex-shrink-0 mt-8 lg:mt-0 lg:pt-24 text-xl md:text-2xl font-bebas">
      <h3 className="uppercase tracking-widest text-center mb-2">{t("form.pageLimit")}</h3>
      <div className="relative p-1 border-2 border-ink rounded-md">
        <div className="border border-dashed border-ink/50 rounded-sm">
          <input
            type="number"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            placeholder={t("form.pageLimitExample")}
            aria-label={t("form.pageLimitAria")}
            className="w-full p-2 bg-transparent focus:outline-none text-center"
          />
        </div>
        <div className="absolute right-2 top-1/2 -translate-y-1/2 flex flex-col space-y-1">
          <Arrow points="5 2, 8 8, 2 8" label={t("form.increase")} onClick={onIncrement} />
          <Arrow points="5 8, 2 2, 8 2" label={t("form.decrease")} onClick={onDecrement} />
        </div>
      </div>
      <p className="text-center text-base mt-2 text-ink/70">{t("form.pageApprox")}</p>
    </div>
  );
}
