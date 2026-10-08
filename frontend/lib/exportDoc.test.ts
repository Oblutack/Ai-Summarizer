import { describe, expect, it } from "vitest";
import { wordDocument } from "./exportDoc";

describe("wordDocument", () => {
  it("builds a real Word file (a zip with a document part) from a summary", async () => {
    const { Packer } = await import("docx");
    const buffer = await Packer.toBuffer(
      await wordDocument(
        "Lease notes",
        "## Overview\nThe **tenant** pays *monthly*.\n\n- First\n  - Nested\n\n1. One\n2. Two\n\nClosing words."
      )
    );
    // A .docx is a zip archive: it starts with "PK" and names its parts in plain text.
    expect(buffer.subarray(0, 2).toString("latin1")).toBe("PK");
    const names = buffer.toString("latin1");
    expect(names).toContain("word/document.xml");
    expect(names).toContain("[Content_Types].xml");
  });

  it("copes with an empty summary", async () => {
    const { Packer } = await import("docx");
    const buffer = await Packer.toBuffer(await wordDocument("Empty", ""));
    expect(buffer.length).toBeGreaterThan(500);
  });
});
