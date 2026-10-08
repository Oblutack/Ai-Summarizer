import { describe, expect, it } from "vitest";
import { DEFAULT_PREFERENCES, loadPreferences, sanitizePreferences, savePreferences } from "./preferences";

function memoryStore(initial?: string) {
  let value = initial ?? null;
  return {
    getItem: () => value,
    setItem: (_key: string, next: string) => {
      value = next;
    },
  };
}

describe("preferences", () => {
  it("round-trips what was saved", () => {
    const store = memoryStore();
    savePreferences({ wordCount: 300, style: "bullets", language: "Spanish" }, store);
    expect(loadPreferences(store)).toEqual({ wordCount: 300, style: "bullets", language: "Spanish" });
  });

  it("uses the defaults when nothing was saved", () => {
    expect(loadPreferences(memoryStore())).toEqual(DEFAULT_PREFERENCES);
    expect(loadPreferences(null)).toEqual(DEFAULT_PREFERENCES);
  });

  it("falls back per field, keeping the valid ones", () => {
    expect(sanitizePreferences({ wordCount: 9999, style: "bullets", language: "Klingon" })).toEqual({
      wordCount: 150,
      style: "bullets",
      language: "English",
    });
  });

  it.each([null, 5, "text", [], { wordCount: "300" }, { wordCount: 99.5 }, { style: {} }])(
    "rejects unusable data: %j",
    (raw) => {
      expect(sanitizePreferences(raw)).toEqual(DEFAULT_PREFERENCES);
    }
  );

  it("survives broken JSON and storage that throws", () => {
    expect(loadPreferences(memoryStore("{broken"))).toEqual(DEFAULT_PREFERENCES);
    const hostile = {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("full");
      },
    };
    expect(loadPreferences(hostile)).toEqual(DEFAULT_PREFERENCES);
    expect(() => savePreferences(DEFAULT_PREFERENCES, hostile)).not.toThrow();
  });
});
