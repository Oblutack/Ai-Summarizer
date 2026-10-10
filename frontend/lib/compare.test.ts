import { describe, expect, it } from "vitest";
import { comparisonToMarkdown, filterChanges, importanceCounts, type Change, type Comparison } from "./compare";

function change(over: Partial<Change>): Change {
  return {
    id: 1,
    kind: "changed",
    before: "Payment is due within 30 days.",
    after: "Payment is due within 60 days.",
    beforePage: 1,
    afterPage: 1,
    segments: null,
    numbers: { removed: [], added: [] },
    importance: "low",
    summary: "",
    impact: "",
    explained: false,
    ...over,
  };
}

const result: Comparison = {
  identical: false,
  counts: { changed: 1, added: 1, removed: 0, moved: 0 },
  bottomLine: "Payment terms changed and a duty was added.",
  explained: true,
  omitted: 0,
  changes: [
    change({ id: 1, importance: "high", summary: "The payment window doubles.", impact: "Slower cash.", numbers: { removed: ["30"], added: ["60"] } }),
    change({ id: 2, kind: "added", before: "", after: "Records are kept for seven years.\nAnd audited.", beforePage: null, afterPage: 3, importance: "medium" }),
  ],
};

describe("filterChanges", () => {
  const changes = [change({ id: 1, importance: "high" }), change({ id: 2, importance: "low" }), change({ id: 3, importance: "high" })];

  it("keeps everything for 'all'", () => {
    expect(filterChanges(changes, "all")).toEqual(changes);
  });

  it("keeps only one importance, in the order of the documents", () => {
    expect(filterChanges(changes, "high").map((c) => c.id)).toEqual([1, 3]);
    expect(filterChanges(changes, "medium")).toEqual([]);
  });
});

describe("importanceCounts", () => {
  it("counts each importance and the total", () => {
    const changes = [change({ importance: "high" }), change({ importance: "low" }), change({ importance: "high" })];
    expect(importanceCounts(changes)).toEqual({ all: 3, high: 2, medium: 0, low: 1 });
    expect(importanceCounts([])).toEqual({ all: 0, high: 0, medium: 0, low: 0 });
  });
});

describe("comparisonToMarkdown", () => {
  const markdown = comparisonToMarkdown(result, "Contract v1", "Contract v2");

  it("names the two documents and counts the changes", () => {
    expect(markdown).toContain("# Comparison: Contract v1 → Contract v2");
    expect(markdown).toContain("1 changed, 1 added, 0 removed, 0 moved.");
  });

  it("has the bottom line and every change with its quotes", () => {
    expect(markdown).toContain("## In short\n\nPayment terms changed and a duty was added.");
    expect(markdown).toContain("### 1. Changed (page 1), importance high");
    expect(markdown).toContain("**The payment window doubles.**");
    expect(markdown).toContain("Numbers: removed 30; added 60");
    expect(markdown).toContain("Before:\n\n> Payment is due within 30 days.");
    expect(markdown).toContain("After:\n\n> Payment is due within 60 days.");
    expect(markdown).toContain("### 2. Added (page 3), importance medium");
    expect(markdown).toContain("> Records are kept for seven years.\n> And audited.");
  });

  it("leaves out what a change does not have", () => {
    expect(markdown.split("### 2.")[1]).not.toContain("Before:");
    expect(markdown.split("### 2.")[1]).not.toContain("Numbers:");
  });

  it("shows pages as a move when they differ", () => {
    const moved = comparisonToMarkdown({ ...result, changes: [change({ beforePage: 2, afterPage: 5 })] }, "a", "b");
    expect(moved).toContain("(page 2 → page 5)");
  });

  it("says so when nothing differs", () => {
    const same = comparisonToMarkdown({ ...result, identical: true, changes: [] }, "a", "b");
    expect(same).toContain("No differences found");
    expect(same).not.toContain("## Changes");
  });

  it("mentions changes that are not listed", () => {
    expect(comparisonToMarkdown({ ...result, omitted: 12 }, "a", "b")).toContain("12 smaller changes are not listed.");
  });

  it("ends with one newline and has no runs of blank lines", () => {
    expect(markdown.endsWith("\n") && !markdown.endsWith("\n\n")).toBe(true);
    expect(markdown).not.toMatch(/\n{3,}/);
  });
});
