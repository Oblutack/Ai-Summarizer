import { LANGUAGES, SUMMARY_STYLES } from "./summaryOptions";

export interface SummaryPreferences {
  wordCount: number;
  style: string;
  language: string;
}

export const DEFAULT_PREFERENCES: SummaryPreferences = { wordCount: 150, style: "default", language: "English" };

// Must match the slider in SummaryOptions.
const MIN_WORDS = 50;
const MAX_WORDS = 500;
const KEY = "inkling.summaryPreferences";

type Store = Pick<Storage, "getItem" | "setItem">;

function browserStore(): Store | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage;
  } catch {
    return null; // blocked storage (private windows, strict settings)
  }
}

// Whatever is stored may be old, edited by hand or from another version, so every field is checked
// and anything unusable falls back to its default.
export function sanitizePreferences(raw: unknown): SummaryPreferences {
  const data = typeof raw === "object" && raw !== null ? (raw as Record<string, unknown>) : {};
  const { wordCount, style, language } = data;
  return {
    wordCount:
      typeof wordCount === "number" && Number.isInteger(wordCount) && wordCount >= MIN_WORDS && wordCount <= MAX_WORDS
        ? wordCount
        : DEFAULT_PREFERENCES.wordCount,
    style: SUMMARY_STYLES.some((s) => s.value === style) ? (style as string) : DEFAULT_PREFERENCES.style,
    language: (LANGUAGES as readonly unknown[]).includes(language) ? (language as string) : DEFAULT_PREFERENCES.language,
  };
}

export function loadPreferences(store: Store | null = browserStore()): SummaryPreferences {
  try {
    const stored = store?.getItem(KEY);
    return sanitizePreferences(stored ? JSON.parse(stored) : null);
  } catch {
    return DEFAULT_PREFERENCES;
  }
}

export function savePreferences(preferences: SummaryPreferences, store: Store | null = browserStore()): void {
  try {
    store?.setItem(KEY, JSON.stringify(preferences));
  } catch {
    // Remembering is a convenience: a full or blocked store must never get in the way.
  }
}
