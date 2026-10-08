// Shared Markdown styling so summaries look the same in the form, the saved cards, the shared page and
// the PDF export. Headings use the interface face; the words themselves are set in the reading face by
// the container (the `reading` class). `size` adds an extra text-size class to body elements.
export function createMarkdownOptions(size = "") {
  const cell = "border border-ink/30 px-3 py-1.5 text-left align-top";
  const body = (base: string) => `${base} ${size}`.trim();
  return {
    // A summary is the model's writing about the document, not a web page: "<iostream>" in a C++ summary is
    // a library name to show as typed, not an HTML tag to build.
    disableParsingRawHTML: true,
    overrides: {
      h1: { props: { className: "mb-2 mt-6 break-after-avoid font-sans text-2xl font-bold tracking-normal first:mt-0" } },
      h2: { props: { className: "mb-2 mt-5 break-after-avoid font-sans text-xl font-bold tracking-normal first:mt-0" } },
      h3: { props: { className: "mb-1 mt-4 break-after-avoid font-sans text-lg font-semibold first:mt-0" } },
      p: { props: { className: body("mb-3") } },
      ul: { props: { className: body("mb-3 list-disc space-y-1 pl-6") } },
      ol: { props: { className: body("mb-3 list-decimal space-y-1 pl-6") } },
      blockquote: { props: { className: "mb-3 border-l-4 border-ink/30 pl-4 italic" } },
      table: { props: { className: "my-4 border-collapse text-base" } },
      th: { props: { className: cell } },
      td: { props: { className: cell } },
      strong: { props: { className: "font-semibold" } },
    },
  };
}
