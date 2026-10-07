import { describe, expect, it } from "vitest";
import type { FileInfo, ProofResult, ProofSentence } from "../types";
import { fileForPassage, numberWarnings, proofHeadline, supportLabel, supportMark } from "./proof";

const result = (over: Partial<ProofResult> = {}): ProofResult => ({
  sentences: [],
  claims: 13,
  found: 12,
  partly: 0,
  notFound: 0,
  verifiable: true,
  ...over,
});

const sentence = (over: Partial<ProofSentence> = {}): ProofSentence => ({
  text: "A claim.",
  kind: "claim",
  support: "strong",
  coverage: 1,
  missingNumbers: [],
  elsewhereNumbers: [],
  passages: [],
  ...over,
});

describe("proofHeadline", () => {
  it("reports what was found, and only mentions the rest when there is some", () => {
    expect(proofHeadline(result())).toBe("12 of 13 statements found in the document");
    expect(proofHeadline(result({ found: 10, partly: 2, notFound: 1 }))).toBe(
      "10 of 13 statements found in the document · 2 partly found · 1 not found"
    );
  });
});

describe("supportLabel and supportMark", () => {
  it("name every level, and use a different shape for each", () => {
    expect(supportLabel("strong")).toBe("Found in the document");
    expect(supportLabel("weak")).toBe("Partly found");
    expect(supportLabel("none")).toBe("Not found");
    expect(new Set([supportMark("strong"), supportMark("weak"), supportMark("none")]).size).toBe(3);
  });
});

describe("numberWarnings", () => {
  it("is empty when the numbers check out", () => {
    expect(numberWarnings(sentence())).toEqual([]);
  });

  it("separates invented numbers from misplaced ones", () => {
    const warnings = numberWarnings(sentence({ missingNumbers: ["10", "12"], elsewhereNumbers: ["7"] }));
    expect(warnings).toEqual([
      "Not in the document: 10, 12",
      "Only found elsewhere in the document, not beside this wording: 7",
    ]);
  });
});

describe("fileForPassage", () => {
  const a: FileInfo = { id: 1, name: "a.pdf", size: 1 };
  const b: FileInfo = { id: 2, name: "b.pdf", size: 1 };

  it("uses the only file when there is one", () => {
    expect(fileForPassage([a], {})).toBe(a);
    expect(fileForPassage([a], { document: null })).toBe(a);
  });

  it("matches by name when several files were combined", () => {
    expect(fileForPassage([a, b], { document: "b.pdf" })).toBe(b);
  });

  it("does not guess among several files, and has nothing without any", () => {
    expect(fileForPassage([a, b], {})).toBeUndefined();
    expect(fileForPassage([a, b], { document: "c.pdf" })).toBeUndefined();
    expect(fileForPassage([], {})).toBeUndefined();
    expect(fileForPassage(undefined, { document: "a.pdf" })).toBeUndefined();
  });
});
