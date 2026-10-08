import { parseBlocks, safeFileName, toMarkdownFile, type Inline } from "./markdownText";

export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  // The browser has started the download by now; the address can go.
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

export function downloadMarkdown(title: string, summary: string): void {
  const blob = new Blob([toMarkdownFile(title, summary)], { type: "text/markdown;charset=utf-8" });
  downloadBlob(blob, `${safeFileName(title)}.md`);
}

// The Word library is large, so it is only loaded when someone asks for a Word file.
export async function wordDocument(title: string, summary: string) {
  const { AlignmentType, Document, HeadingLevel, Paragraph, TextRun } = await import("docx");
  type Heading = (typeof HeadingLevel)[keyof typeof HeadingLevel];

  const runs = (inline: Inline[]) => inline.map((r) => new TextRun({ text: r.text, bold: r.bold, italics: r.italic }));
  const headings: Record<1 | 2 | 3, Heading> = {
    1: HeadingLevel.HEADING_2,
    2: HeadingLevel.HEADING_3,
    3: HeadingLevel.HEADING_4,
  };

  const children = [new Paragraph({ heading: HeadingLevel.TITLE, children: [new TextRun(title.replace(/\s+/g, " ").trim())] })];
  for (const block of parseBlocks(summary)) {
    if (block.kind === "heading") {
      children.push(new Paragraph({ heading: headings[block.level], children: runs(block.runs), spacing: { before: 240 } }));
    } else if (block.kind === "bullet") {
      children.push(new Paragraph({ bullet: { level: block.depth }, children: runs(block.runs) }));
    } else if (block.kind === "numbered") {
      children.push(
        new Paragraph({
          indent: { left: 360, hanging: 360 },
          alignment: AlignmentType.START,
          children: [new TextRun(`${block.number}.\t`), ...runs(block.runs)],
          tabStops: [{ type: "left", position: 360 }],
        })
      );
    } else {
      children.push(new Paragraph({ children: runs(block.runs), spacing: { after: 160 } }));
    }
  }
  return new Document({ title, creator: "Inkling", sections: [{ children }] });
}

export async function downloadWord(title: string, summary: string): Promise<void> {
  const { Packer } = await import("docx");
  const blob = await Packer.toBlob(await wordDocument(title, summary));
  downloadBlob(blob, `${safeFileName(title)}.docx`);
}
