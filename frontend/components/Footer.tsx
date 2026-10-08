"use client";
import BrandMark from "./BrandMark";
import { useT } from "./I18nProvider";

const REPOSITORY = "https://github.com/Oblutack/Inkling";

export default function Footer() {
  const t = useT();
  return (
    <footer className="mt-20 border-t border-ink/15">
      <div className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-8 text-sm text-ink/70 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          <BrandMark className="h-7 w-7" />
          <p>
            <span className="font-semibold text-ink">Inkling</span> · {t("footer.tagline")}
          </p>
        </div>
        <a href={REPOSITORY} target="_blank" rel="noopener noreferrer" className="underline underline-offset-4 hover:text-ink">
          {t("footer.source")}
        </a>
      </div>
    </footer>
  );
}
