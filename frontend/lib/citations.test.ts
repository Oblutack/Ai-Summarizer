import { describe, expect, it } from "vitest";
import type { ChatSource } from "../types";
import { citationId, describeSource, linkifyCitations } from "./citations";

const source = (over: Partial<ChatSource> = {}): ChatSource => ({
  id: 1,
  text: "passage",
  page: null,
  pageEnd: null,
  ...over,
});

describe("linkifyCitations", () => {
  it("links markers that match a source", () => {
    expect(linkifyCitations("Seven years [1].", [source({ id: 1 })])).toBe("Seven years [1](#cite-1).");
  });

  it("links several markers in a row", () => {
    const sources = [source({ id: 1 }), source({ id: 3 })];
    expect(linkifyCitations("Agreed [1][3].", sources)).toBe("Agreed [1](#cite-1)[3](#cite-3).");
  });

  it("leaves markers without a matching source as plain text", () => {
    expect(linkifyCitations("Maybe [7] and [1].", [source({ id: 1 })])).toBe("Maybe [7] and [1](#cite-1).");
  });

  it("does nothing without sources, so old answers and pasted text stay as they are", () => {
    expect(linkifyCitations("Plain [1] text", undefined)).toBe("Plain [1] text");
    expect(linkifyCitations("Plain [1] text", [])).toBe("Plain [1] text");
  });

  it("does not touch other bracketed text", () => {
    expect(linkifyCitations("list[a] and [1.5] and [ 1 ]", [source({ id: 1 })])).toBe("list[a] and [1.5] and [ 1 ]");
  });
});

describe("citationId", () => {
  it("reads the number from a citation link", () => {
    expect(citationId("#cite-12")).toBe(12);
  });

  it("returns null for anything else", () => {
    for (const href of [undefined, "", "https://example.com", "#cite-", "#cite-x", "#cite-0", "#cite-1.5", "#other"]) {
      expect(citationId(href)).toBeNull();
    }
  });
});

describe("describeSource", () => {
  it("describes a page, a range, and a file", () => {
    expect(describeSource(source({ page: 2, pageEnd: 2 }))).toBe("Page 2");
    expect(describeSource(source({ page: 2, pageEnd: 3 }))).toBe("Pages 2-3");
    expect(describeSource(source({ page: 2, pageEnd: null }))).toBe("Page 2");
    expect(describeSource(source({ page: 4, pageEnd: 4, document: "report.pdf" }))).toBe("report.pdf, page 4");
    expect(describeSource(source({ document: "report.pdf" }))).toBe("report.pdf");
  });

  it("is empty when nothing is known", () => {
    expect(describeSource(source())).toBe("");
  });
});
