"use client";
import { useT } from "./I18nProvider";

// The first thing a keyboard user can reach: jump past the header to the page itself.
export default function SkipLink() {
  const t = useT();
  return (
    <a
      href="#main"
      className="sr-only z-50 rounded-lg bg-accent px-4 py-2 font-semibold text-accent-fg focus:not-sr-only focus:fixed focus:left-4 focus:top-4"
    >
      {t("nav.skip")}
    </a>
  );
}
