import { bs } from "./bs";
import { de } from "./de";
import { en, type MessageKey } from "./en";
import { es } from "./es";
import { fr } from "./fr";

// The languages the interface itself can be shown in. (Summaries can be written in more: see summaryOptions.)
// Each has one dictionary file with every message; English is the source and the fallback.
export const UI_LANGUAGES = [
  { code: "en", label: "English" },
  { code: "es", label: "Español" },
  { code: "de", label: "Deutsch" },
  { code: "fr", label: "Français" },
  { code: "bs", label: "Bosanski" },
] as const;

export type UiLanguage = (typeof UI_LANGUAGES)[number]["code"];
export type { MessageKey };

export const LANGUAGE_KEY = "inkling.language";
export const DEFAULT_LANGUAGE: UiLanguage = "en";

const dictionaries: Record<UiLanguage, Record<MessageKey, string>> = { en, es, de, fr, bs };

export function isUiLanguage(value: unknown): value is UiLanguage {
  return UI_LANGUAGES.some((l) => l.code === value);
}

export type Params = Record<string, string | number>;
export type Translate = (key: MessageKey, params?: Params) => string;

// A message in the given language, with {name} placeholders filled in. A key missing from a language
// falls back to English, so a half-finished translation never shows an empty label.
export function translate(language: UiLanguage, key: MessageKey, params?: Params): string {
  const template = dictionaries[language][key] || en[key];
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (whole, name: string) => (name in params ? String(params[name]) : whole));
}

// The translator for code that is not a component (and for tests): English.
export const englishT: Translate = (key, params) => translate("en", key, params);

// The locale used to format dates in each language.
export const LOCALES: Record<UiLanguage, string> = { en: "en-GB", es: "es-ES", de: "de-DE", fr: "fr-FR", bs: "bs-BA" };

// Which language to use: the person's own choice if they made one, else the first language the browser
// asks for that we have (de-AT -> de), else English.
export function resolveLanguage(stored: string | null, preferred: readonly string[]): UiLanguage {
  if (isUiLanguage(stored)) return stored;
  for (const tag of preferred) {
    const base = tag.toLowerCase().split("-")[0];
    // Croatian and Serbian readers can read the Bosnian interface.
    const mapped = base === "hr" || base === "sr" ? "bs" : base;
    if (isUiLanguage(mapped)) return mapped;
  }
  return DEFAULT_LANGUAGE;
}
