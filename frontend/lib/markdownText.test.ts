import { describe, expect, it } from "vitest";
import { parseBlocks, parseInline, safeFileName, tidyMarkdown, toMarkdownFile, toPlainText } from "./markdownText";

describe("parseInline", () => {
  it("finds bold, italic, code and links", () => {
    expect(parseInline("plain **bold** and *soft* with `code` and [a link](https://x.example) end")).toEqual([
      { text: "plain " },
      { text: "bold", bold: true },
      { text: " and " },
      { text: "soft", italic: true },
      { text: " with " },
      { text: "code" },
      { text: " and " },
      { text: "a link" },
      { text: " end" },
    ]);
  });

  it("leaves an unclosed mark as typed", () => {
    expect(parseInline("a **b and c")).toEqual([{ text: "a **b and c" }]);
  });

  it("never returns nothing", () => {
    expect(parseInline("")).toEqual([{ text: "" }]);
  });
});

describe("parseBlocks", () => {
  const sample = `# Title

## Overview
Some **important** text
that wraps.

- First point
  - Nested point
* Second point

1. One
2) Two

---
Closing words.`;

  it("reads headings, paragraphs, bullets and numbered items", () => {
    const blocks = parseBlocks(sample);
    expect(blocks.map((b) => b.kind)).toEqual([
      "heading",
      "heading",
      "paragraph",
      "bullet",
      "bullet",
      "bullet",
      "numbered",
      "numbered",
      "paragraph",
    ]);
    expect(blocks[0]).toMatchObject({ kind: "heading", level: 1 });
    expect(blocks[1]).toMatchObject({ kind: "heading", level: 2 });
    expect(blocks[4]).toMatchObject({ kind: "bullet", depth: 1 });
    expect(blocks[7]).toMatchObject({ kind: "numbered", number: 2 });
  });

  it("joins the lines of one paragraph", () => {
    const paragraph = parseBlocks(sample)[2];
    expect(paragraph.kind === "paragraph" && paragraph.runs.map((r) => r.text).join("")).toBe(
      "Some important text that wraps."
    );
  });

  it("caps deep headings at level 3 and accepts Windows line breaks", () => {
    expect(parseBlocks("###### Deep\r\n\r\nText")[0]).toMatchObject({ kind: "heading", level: 3 });
  });

  it("returns nothing for empty text", () => {
    expect(parseBlocks("")).toEqual([]);
    expect(parseBlocks("\n  \n---\n")).toEqual([]);
  });
});

describe("toPlainText", () => {
  it("makes sentences a voice can read", () => {
    const plain = toPlainText("## Overview\nThe **warranty** lasts 24 months [2].\n\n- Keep the receipt\n- Call support!");
    expect(plain).toBe("Overview.\nThe warranty lasts 24 months.\nKeep the receipt.\nCall support!");
  });

  it("drops citation marks of both kinds", () => {
    expect(toPlainText("Seven years 【3】 on the unit [1][2]")).toBe("Seven years on the unit");
  });

  it("is empty for empty text", () => {
    expect(toPlainText("")).toBe("");
  });
});

describe("safeFileName", () => {
  it.each([
    ["Lease for Harbour Street", "Lease-for-Harbour-Street"],
    ["atlas-manual.pdf", "atlas-manual"],
    ["Café résumé: 100% done!", "Cafe-resume-100-done"],
    ["../../etc/passwd", "etc-passwd"],
    ["日本語", "summary"],
    ["", "summary"],
    ["   ", "summary"],
  ])("%j -> %j", (title, expected) => {
    expect(safeFileName(title)).toBe(expected);
  });

  it("is short enough for any file system", () => {
    expect(safeFileName("word ".repeat(100)).length).toBeLessThanOrEqual(80);
  });
});

describe("toMarkdownFile", () => {
  it("puts the title on top of the summary", () => {
    expect(toMarkdownFile("  My   notes ", "\n## Overview\nText\n\n")).toBe("# My notes\n\n## Overview\nText\n");
  });
});

describe("underlined headings", () => {
  const underlined = "# Dynamic Arrays: The Basics\n=====================\n\nVectors grow when needed.\n\n## Why\n---\nBecause.";

  it("drops a line of = or - right under a # heading", () => {
    expect(tidyMarkdown(underlined)).toBe("# Dynamic Arrays: The Basics\n\nVectors grow when needed.\n\n## Why\nBecause.");
  });

  it("keeps a rule that stands on its own", () => {
    expect(tidyMarkdown("One.\n\n---\n\nTwo.")).toBe("One.\n\n---\n\nTwo.");
  });

  it("is not read out as 'equals'", () => {
    const spoken = toPlainText(underlined);
    expect(spoken).not.toContain("=");
    expect(spoken.split("\n")[0]).toBe("Dynamic Arrays: The Basics.");
  });
});
