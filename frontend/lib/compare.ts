// Comparing two versions of a document: what the server sends back, and what the page does with it.

export type ChangeKind = "added" | "removed" | "changed" | "moved";
export type Importance = "high" | "medium" | "low";
export type SegmentKind = "eq" | "del" | "ins";

export interface Change {
  id: number;
  kind: ChangeKind;
  // The exact text of the older and the newer document (empty on the side where there is none).
  before: string;
  after: string;
  beforePage: number | null;
  afterPage: number | null;
  // For a changed passage: the words that stayed, went and came, in order.
  segments: [SegmentKind, string][] | null;
  numbers: { removed: string[]; added: string[] };
  importance: Importance;
  // What changed and why it could matter, written by the AI (or a plain description when it said nothing usable).
  summary: string;
  impact: string;
  explained: boolean;
}

export interface Comparison {
  identical: boolean;
  counts: Record<ChangeKind, number>;
  changes: Change[];
  bottomLine: string;
  explained: boolean;
  omitted: number;
}

export const IMPORTANCE_ORDER: Importance[] = ["high", "medium", "low"];

export type ImportanceFilter = Importance | "all";

// The changes to show: all of them, or those of one importance. They keep the order of the documents.
export function filterChanges(changes: Change[], filter: ImportanceFilter): Change[] {
  return filter === "all" ? changes : changes.filter((change) => change.importance === filter);
}

// How many changes there are of each importance, for the filter buttons.
export function importanceCounts(changes: Change[]): Record<ImportanceFilter, number> {
  const counts: Record<ImportanceFilter, number> = { all: changes.length, high: 0, medium: 0, low: 0 };
  for (const change of changes) counts[change.importance] += 1;
  return counts;
}

const KIND_LABEL: Record<ChangeKind, string> = { changed: "Changed", added: "Added", removed: "Removed", moved: "Moved" };

function quote(text: string): string {
  return text
    .split(/\r?\n/)
    .map((line) => `> ${line}`)
    .join("\n");
}

function pages(change: Change): string {
  const before = change.beforePage ? `page ${change.beforePage}` : "";
  const after = change.afterPage ? `page ${change.afterPage}` : "";
  if (before && after) return before === after ? ` (${before})` : ` (${before} → ${after})`;
  return before || after ? ` (${before || after})` : "";
}

// The comparison as a Markdown report, for pasting into an email or a document. The words are English labels: a copied
// report is a document of its own, not part of the page.
export function comparisonToMarkdown(result: Comparison, oldName: string, newName: string): string {
  const lines = [`# Comparison: ${oldName} → ${newName}`, ""];
  if (result.identical) {
    lines.push("No differences found: the two documents say the same, apart from capital letters, spacing and punctuation.");
    return lines.join("\n") + "\n";
  }
  const { changed, added, removed, moved } = result.counts;
  lines.push(`${changed} changed, ${added} added, ${removed} removed, ${moved} moved.`, "");
  if (result.bottomLine) lines.push("## In short", "", result.bottomLine, "");
  lines.push("## Changes", "");
  result.changes.forEach((change, index) => {
    lines.push(`### ${index + 1}. ${KIND_LABEL[change.kind]}${pages(change)}, importance ${change.importance}`, "");
    if (change.summary) lines.push(`**${change.summary}**`, "");
    if (change.impact) lines.push(change.impact, "");
    if (change.numbers.removed.length || change.numbers.added.length) {
      const parts = [];
      if (change.numbers.removed.length) parts.push(`removed ${change.numbers.removed.join(", ")}`);
      if (change.numbers.added.length) parts.push(`added ${change.numbers.added.join(", ")}`);
      lines.push(`Numbers: ${parts.join("; ")}`, "");
    }
    if (change.before) lines.push("Before:", "", quote(change.before), "");
    if (change.after) lines.push("After:", "", quote(change.after), "");
  });
  if (result.omitted > 0) lines.push(`${result.omitted} smaller changes are not listed.`, "");
  return lines.join("\n").replace(/\n{3,}/g, "\n\n").trimEnd() + "\n";
}
