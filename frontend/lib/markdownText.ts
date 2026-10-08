// Reading the Markdown of a summary without rendering it: as plain text (to read aloud, to hand to a
// text field) and as a list of blocks (to build a Word document).

export type Inline = { text: string; bold?: boolean; italic?: boolean };

export type Block =
  | { kind: "heading"; level: 1 | 2 | 3; runs: Inline[] }
  | { kind: "paragraph"; runs: Inline[] }
  | { kind: "bullet"; runs: Inline[]; depth: number }
  | { kind: "numbered"; runs: Inline[]; number: number };

const HEADING = /^(#{1,6})\s+(.*)$/;
const BULLET = /^(\s*)[-*+•]\s+(.*)$/;
const NUMBERED = /^(\s*)(\d+)[.)]\s+(.*)$/;
const RULE = /^\s*([-*_=])(\s*\1){2,}\s*$/;
const UNDERLINE = /^\s*(=|-){3,}\s*$/;
const ATX_HEADING = /^\s*#{1,6}\s+\S/;
const CITATION_MARKS = /\s*(?:\[\d+\]|【\d+】)/g;

// **bold**, *italic* / _italic_, `code`, [text](url) -> text. Anything unclosed stays as typed.
export function parseInline(source: string): Inline[] {
  const runs: Inline[] = [];
  const pattern = /(\*\*|__)(.+?)\1|(\*|_)(.+?)\3|`([^`]+)`|\[([^\]]+)\]\([^)]*\)/g;
  let last = 0;
  for (const match of source.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > last) runs.push({ text: source.slice(last, index) });
    if (match[2] !== undefined) runs.push({ text: match[2], bold: true });
    else if (match[4] !== undefined) runs.push({ text: match[4], italic: true });
    else if (match[5] !== undefined) runs.push({ text: match[5] });
    else if (match[6] !== undefined) runs.push({ text: match[6] });
    last = index + match[0].length;
  }
  if (last < source.length) runs.push({ text: source.slice(last) });
  return runs.length ? runs : [{ text: "" }];
}

// Models sometimes write a heading as "# Title" and then underline it with ==== (or ----) as well. Left
// alone, the underline is read out as "equals equals" and some renderers show the "#" as typed, so a
// line of only = or - directly under a "#" heading is dropped.
export function tidyMarkdown(markdown: string): string {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  return lines.filter((line, i) => !(UNDERLINE.test(line) && i > 0 && ATX_HEADING.test(lines[i - 1]))).join("\n");
}

export function parseBlocks(markdown: string): Block[] {
  const blocks: Block[] = [];
  let paragraph: string[] = [];
  const flush = () => {
    if (paragraph.length) blocks.push({ kind: "paragraph", runs: parseInline(paragraph.join(" ")) });
    paragraph = [];
  };

  for (const raw of tidyMarkdown(markdown).split("\n")) {
    const line = raw.replace(/\s+$/, "");
    if (!line.trim() || RULE.test(line)) {
      flush();
      continue;
    }
    let m: RegExpMatchArray | null;
    if ((m = line.match(HEADING))) {
      flush();
      blocks.push({ kind: "heading", level: Math.min(m[1].length, 3) as 1 | 2 | 3, runs: parseInline(m[2]) });
    } else if ((m = line.match(BULLET))) {
      flush();
      blocks.push({ kind: "bullet", runs: parseInline(m[2]), depth: Math.min(Math.floor(m[1].length / 2), 3) });
    } else if ((m = line.match(NUMBERED))) {
      flush();
      blocks.push({ kind: "numbered", runs: parseInline(m[3]), number: Number(m[2]) });
    } else {
      paragraph.push(line.trim());
    }
  }
  flush();
  return blocks;
}

const text = (runs: Inline[]) => runs.map((r) => r.text).join("");

// The words of a summary as plain sentences, for a voice: no marks, headings end with a full stop so
// the voice pauses, and citation numbers like [2] are left out.
export function toPlainText(markdown: string): string {
  const lines = parseBlocks(markdown).map((block) => {
    const body = text(block.runs).replace(CITATION_MARKS, "").replace(/\s+/g, " ").trim();
    if (!body) return "";
    if (block.kind === "heading" && !/[.!?:]$/.test(body)) return body + ".";
    if ((block.kind === "bullet" || block.kind === "numbered") && !/[.!?:;]$/.test(body)) return body + ".";
    return body;
  });
  return lines.filter(Boolean).join("\n");
}

// A file name made from a title: letters, digits, dashes, never empty and never starting with a dot.
export function safeFileName(title: string, fallback = "summary"): string {
  const base = title
    .replace(/\.[A-Za-z0-9]{1,5}$/, "")
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^A-Za-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80)
    .replace(/-+$/, "");
  return base || fallback;
}

// A summary as a Markdown file: its title as a heading, then the summary as the model wrote it.
export function toMarkdownFile(title: string, summary: string): string {
  const heading = title.replace(/\s+/g, " ").trim();
  return `# ${heading}\n\n${summary.trim()}\n`;
}
