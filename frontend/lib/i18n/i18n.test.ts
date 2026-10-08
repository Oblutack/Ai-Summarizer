import { describe, expect, it } from "vitest";
import { bs } from "./bs";
import { de } from "./de";
import { en } from "./en";
import { es } from "./es";
import { fr } from "./fr";
import { LOCALES, UI_LANGUAGES, englishT, isUiLanguage, resolveLanguage, translate, type MessageKey } from "./index";

const translations = { es, de, fr, bs };
const keys = Object.keys(en) as MessageKey[];

function placeholders(text: string): string[] {
  return [...text.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
}

describe("the dictionaries", () => {
  it("have a message for every key, with no empty ones", () => {
    for (const [code, table] of Object.entries(translations)) {
      const record = table as Record<string, string>;
      expect(Object.keys(record).sort(), code).toEqual([...keys].sort());
      for (const key of keys) expect(record[key]?.trim(), `${code}: ${key}`).toBeTruthy();
    }
  });

  it("use exactly the placeholders of the English message", () => {
    for (const [code, table] of Object.entries(translations)) {
      for (const key of keys) {
        expect(placeholders((table as Record<string, string>)[key]), `${code}: ${key}`).toEqual(placeholders(en[key]));
      }
    }
  });

  it("really are translated: most messages differ from English in every language", () => {
    for (const [code, table] of Object.entries(translations)) {
      const same = keys.filter((k) => (table as Record<string, string>)[k] === en[k]).length;
      expect(same / keys.length, code).toBeLessThan(0.15);
    }
  });

  it("know every language the interface offers", () => {
    expect(UI_LANGUAGES.map((l) => l.code)).toEqual(["en", ...Object.keys(translations)]);
    for (const l of UI_LANGUAGES) expect(LOCALES[l.code]).toBeTruthy();
  });
});

describe("translate", () => {
  it("fills in placeholders", () => {
    expect(translate("en", "quiz.score", { score: 2, total: 3 })).toBe("You got 2 of 3 right.");
    expect(translate("de", "quiz.progress", { n: 1, total: 10 })).toBe("Frage 1 von 10");
  });

  it("leaves an unknown placeholder visible rather than printing undefined", () => {
    expect(translate("en", "quiz.progress", { n: 1 })).toBe("Question 1 of {total}");
  });

  it("falls back to English for a missing message", () => {
    const original = es["nav.language"];
    (es as Record<string, string>)["nav.language"] = "";
    try {
      expect(translate("es", "nav.language")).toBe("Language");
    } finally {
      (es as Record<string, string>)["nav.language"] = original;
    }
  });

  it("gives the English wording to code outside components", () => {
    expect(englishT("form.summarize")).toBe("Summarize");
    expect(englishT("source.pages", { a: 2, b: 3 })).toBe("pages 2-3");
  });
});

describe("resolveLanguage", () => {
  it("prefers the person's own choice", () => {
    expect(resolveLanguage("fr", ["de-DE"])).toBe("fr");
  });

  it("ignores a stored value that is not a language we have", () => {
    expect(resolveLanguage("klingon", ["es-MX"])).toBe("es");
    expect(isUiLanguage("klingon")).toBe(false);
  });

  it("follows the browser's languages in order, by their base language", () => {
    expect(resolveLanguage(null, ["ja-JP", "de-AT", "en"])).toBe("de");
    expect(resolveLanguage(null, ["FR-ca"])).toBe("fr");
  });

  it("reads Croatian and Serbian speakers' browsers as Bosnian", () => {
    expect(resolveLanguage(null, ["hr-HR"])).toBe("bs");
    expect(resolveLanguage(null, ["sr-RS"])).toBe("bs");
    expect(resolveLanguage(null, ["bs"])).toBe("bs");
  });

  it("falls back to English", () => {
    expect(resolveLanguage(null, [])).toBe("en");
    expect(resolveLanguage(null, ["ja-JP", "zh-CN"])).toBe("en");
  });
});
