import type { ChatSource } from "../types";
import { englishT, type Translate } from "./i18n";

// Prefix of the link target that marks a citation (the markdown renderer turns "[2]" into a link
// to "#cite-2", and a custom component renders that link as a small clickable chip).
export const CITATION_HREF = "#cite-";

// Turns citation markers such as "[2]" into markdown links, but only for sources that exist, so a
// stray "[7]" in the text is never made to look like a reference.
export function linkifyCitations(answer: string, sources: ChatSource[] | undefined): string {
  if (!sources || sources.length === 0) return answer;
  const known = new Set(sources.map((s) => s.id));
  return answer.replace(/\[(\d+)\]/g, (marker, digits: string) => {
    const id = Number(digits);
    return known.has(id) ? `[${id}](${CITATION_HREF}${id})` : marker;
  });
}

// The source number a link target points at, or null for an ordinary link.
export function citationId(href: string | undefined): number | null {
  if (!href || !href.startsWith(CITATION_HREF)) return null;
  const id = Number(href.slice(CITATION_HREF.length));
  return Number.isInteger(id) && id > 0 ? id : null;
}

// "8 Oct 2026": enough to tell apart documents with the same title (several pasted texts, say).
function shortDate(iso: string, locale: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleDateString(locale, { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
}

// Where a passage comes from, for people: "Page 2", "Pages 2-3", "report.pdf, page 2", or "" when
// nothing is known (pasted text, or documents saved before page numbers were kept). Answers drawn
// from the whole library also name the saved document and when it was saved.
export function describeSource(source: ChatSource, t: Translate = englishT, locale = "en-GB"): string {
  const parts: string[] = [];
  if (source.documentTitle) {
    const saved = source.savedAt ? shortDate(source.savedAt, locale) : "";
    parts.push(saved ? `${source.documentTitle} (${saved})` : source.documentTitle);
    if (source.document && source.document !== source.documentTitle) parts.push(source.document);
  } else if (source.document) {
    parts.push(source.document);
  }
  if (source.page != null) {
    const range = source.pageEnd != null && source.pageEnd !== source.page;
    const label = range ? t("source.pages", { a: source.page, b: source.pageEnd as number }) : t("source.page", { n: source.page });
    parts.push(parts.length > 0 ? label : label.charAt(0).toUpperCase() + label.slice(1));
  }
  return parts.join(", ");
}
