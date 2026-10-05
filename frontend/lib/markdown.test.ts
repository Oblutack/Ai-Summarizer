import { describe, expect, it } from "vitest";
import { createMarkdownOptions } from "./markdown";

describe("createMarkdownOptions", () => {
  it("styles the elements a summary uses", () => {
    const { overrides } = createMarkdownOptions();
    for (const tag of ["h1", "h2", "p", "ul", "ol", "table", "th", "td", "strong"] as const) {
      expect(overrides[tag].props.className).toBeTruthy();
    }
  });

  it("adds the size class to body text only", () => {
    const { overrides } = createMarkdownOptions("text-xl");
    expect(overrides.p.props.className).toContain("text-xl");
    expect(overrides.ul.props.className).toContain("text-xl");
    expect(overrides.h1.props.className).not.toContain("text-xl");
  });

  it("leaves no stray whitespace without a size", () => {
    const { overrides } = createMarkdownOptions();
    expect(overrides.p.props.className).toBe(overrides.p.props.className.trim());
  });
});
