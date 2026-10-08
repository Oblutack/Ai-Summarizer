"use client";
import { UI_LANGUAGES, isUiLanguage } from "../lib/i18n";
import { useI18n } from "./I18nProvider";

// Chooses the language of the interface. Each language is named in itself, so it can be found
// whichever language is showing.
export default function LanguageSwitcher() {
  const { language, setLanguage, t } = useI18n();
  return (
    <select
      value={language}
      onChange={(e) => isUiLanguage(e.target.value) && setLanguage(e.target.value)}
      aria-label={t("nav.language")}
      className="h-11 cursor-pointer rounded-lg border border-ink/30 bg-transparent px-2 text-sm font-medium hover:bg-ink/10"
    >
      {UI_LANGUAGES.map((l) => (
        <option key={l.code} value={l.code} className="bg-canvas text-ink">
          {l.label}
        </option>
      ))}
    </select>
  );
}
