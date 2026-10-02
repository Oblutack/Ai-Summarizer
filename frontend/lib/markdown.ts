// Shared Markdown styling so summaries look the same in the form, the saved cards and the PDF export.
// `size` adds an extra text-size class to body elements (the saved cards use larger text).
export function createMarkdownOptions(size = "") {
  const cell = "border border-ink/40 px-2 py-1 text-left align-top";
  return {
    overrides: {
      h1: {
        props: { className: "text-2xl md:text-3xl font-bold my-4 break-after-avoid" },
      },
      h2: {
        props: { className: "text-xl md:text-2xl font-bold my-3 break-after-avoid" },
      },
      p: { props: { className: `mb-4 ${size}`.trim() } },
      ul: { props: { className: `list-disc list-inside mb-4 ml-4 ${size}`.trim() } },
      ol: { props: { className: `list-decimal list-inside mb-4 ml-4 ${size}`.trim() } },
      table: { props: { className: "border-collapse my-4 text-lg" } },
      th: { props: { className: cell } },
      td: { props: { className: cell } },
      strong: { props: { className: "text-ink" } },
    },
  };
}
