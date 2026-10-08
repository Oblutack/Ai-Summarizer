"use client";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import {
  DEFAULT_LANGUAGE,
  LANGUAGE_KEY,
  resolveLanguage,
  translate,
  type MessageKey,
  type Params,
  type UiLanguage,
} from "../lib/i18n";

interface I18n {
  language: UiLanguage;
  setLanguage: (language: UiLanguage) => void;
  t: (key: MessageKey, params?: Params) => string;
}

const I18nContext = createContext<I18n>({
  language: DEFAULT_LANGUAGE,
  setLanguage: () => undefined,
  t: (key, params) => translate(DEFAULT_LANGUAGE, key, params),
});

function storedLanguage(): string | null {
  try {
    return localStorage.getItem(LANGUAGE_KEY);
  } catch {
    return null; // blocked storage
  }
}

// Holds the language of the interface. The server and the first render always use English, so they agree;
// the person's language is read right after, which is why a non-English page changes language a moment
// after it appears.
export default function I18nProvider({ children }: { children: React.ReactNode }) {
  const [language, setLanguageState] = useState<UiLanguage>(DEFAULT_LANGUAGE);

  useEffect(() => {
    setLanguageState(resolveLanguage(storedLanguage(), navigator.languages?.length ? navigator.languages : [navigator.language]));
  }, []);

  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);

  const setLanguage = useCallback((next: UiLanguage) => {
    setLanguageState(next);
    try {
      localStorage.setItem(LANGUAGE_KEY, next);
    } catch {
      // The choice only lasts for this visit when storage is blocked.
    }
  }, []);

  const value = useMemo<I18n>(
    () => ({ language, setLanguage, t: (key, params) => translate(language, key, params) }),
    [language, setLanguage]
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18n {
  return useContext(I18nContext);
}

// Shorthand for components that only need to translate.
export function useT() {
  return useContext(I18nContext).t;
}
