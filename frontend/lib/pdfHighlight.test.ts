import { describe, expect, it } from "vitest";
import { compact, findHighlight, type TextPiece } from "./pdfHighlight";

// A page the way PDF.js reports it: one piece per run of text, lines ending with hasEOL.
const page = (...lines: string[]): TextPiece[] => lines.map((str) => ({ str, hasEOL: true }));

const PASSAGE = "Replace the air filter every 90 days or after 500 operating hours, whichever comes first.";

describe("compact", () => {
  it("ignores case, spacing, soft hyphens and ligatures", () => {
    expect(compact("The  Of­fice\n ﬁle")).toBe("theofficefile");
  });
});

describe("findHighlight", () => {
  it("highlights the pieces that hold the passage", () => {
    const pieces = page("Maintenance", "Replace the air filter every 90 days", "or after 500 operating hours,", "whichever comes first.", "Drain the tank weekly.");
    expect(findHighlight(pieces, PASSAGE)).toEqual([1, 2, 3]);
  });

  it("copes with different line breaks and spacing than the server's reader", () => {
    const pieces = page("Replace the air fil", "ter every 90 days or after", "500 operating  hours, which", "ever comes first.");
    expect(findHighlight(pieces, PASSAGE)).toEqual([0, 1, 2, 3]);
  });

  it("finds a passage that starts with text missing from the page (a heading)", () => {
    const pieces = page("Other text on the page", "Replace the air filter every 90 days or after 500 operating hours, whichever comes first.");
    expect(findHighlight(pieces, `Maintenance Schedule ${PASSAGE}`)).toEqual([1]);
  });

  it("does not highlight beyond the passage", () => {
    const pieces = page("Replace the air filter every 90 days or after 500 operating hours, whichever comes first.", "An unrelated line afterwards.");
    expect(findHighlight(pieces, PASSAGE)).toEqual([0]);
  });

  it("returns nothing when the passage is not on the page", () => {
    expect(findHighlight(page("Completely different words about something else entirely."), PASSAGE)).toEqual([]);
  });

  it("returns nothing for empty or tiny input", () => {
    expect(findHighlight([], PASSAGE)).toEqual([]);
    expect(findHighlight(page("some text"), "")).toEqual([]);
    expect(findHighlight(page("some text"), "ab")).toEqual([]);
    expect(findHighlight(page(""), PASSAGE)).toEqual([]);
  });

  it("finds short passages exactly", () => {
    const pieces = page("Warranty", "seven years", "from purchase");
    expect(findHighlight(pieces, "seven years")).toEqual([1]);
  });

  it("works when the page text has ligatures the passage spells out", () => {
    const pieces = page("The ﬁlter must be cleaned weekly and replaced often.");
    expect(findHighlight(pieces, "The filter must be cleaned weekly")).toEqual([0]);
  });
});
