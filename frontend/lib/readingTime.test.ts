import { describe, expect, it } from "vitest";
import { countWords, describeLength, readingMinutes } from "./readingTime";

describe("countWords", () => {
  it("counts words and ignores lone markdown marks", () => {
    expect(countWords("## Overview\n\n- One two\n* three\n> four — five")).toBe(6);
  });

  it("handles empty and blank text", () => {
    expect(countWords("")).toBe(0);
    expect(countWords("  \n\t ")).toBe(0);
  });

  it("counts numbers and other scripts", () => {
    expect(countWords("Revenue grew 12% in 2026")).toBe(5);
    expect(countWords("Hola señor, ¿cómo está?")).toBe(4);
  });
});

describe("readingMinutes", () => {
  it.each([
    [1, 1],
    [200, 1],
    [299, 1],
    [300, 2],
    [1000, 5],
  ])("%i words take %i min", (words, minutes) => {
    expect(readingMinutes(words)).toBe(minutes);
  });
});

describe("describeLength", () => {
  it("describes length and time", () => {
    expect(describeLength("one")).toBe("1 word · 1 min read");
    expect(describeLength("word ".repeat(1500))).toBe("1,500 words · 8 min read");
  });

  it("says nothing for empty text", () => {
    expect(describeLength("")).toBe("");
  });
});
