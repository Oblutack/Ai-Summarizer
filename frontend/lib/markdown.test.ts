import Markdown from "markdown-to-jsx";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
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

describe("text that looks like HTML", () => {
  const render = (source: string) => renderToStaticMarkup(createElement(Markdown, { options: createMarkdownOptions() }, source));

  it("shows a C++ header name as typed and keeps the text after it", () => {
    const html = render("The program includes <iostream> for output.\n\nThen it prints a line.");
    expect(html).toContain("&lt;iostream&gt;");
    expect(html).not.toMatch(/<iostream/);
    expect(html).toContain("Then it prints a line.");
  });

  it("does not turn script or image tags into elements", () => {
    const html = render('Hello <script>alert(1)</script> <img src="x" onerror="alert(1)"> there');
    expect(html).not.toMatch(/<script|<img/);
    expect(html).toContain("there");
  });
});
