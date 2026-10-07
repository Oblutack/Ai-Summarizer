import type { FileInfo, ProofResult, ProofSentence, Support } from "../types";

// "12 of 13 statements found in the document · 1 partly found"
export function proofHeadline(result: ProofResult): string {
  const parts = [`${result.found} of ${result.claims} statements found in the document`];
  if (result.partly > 0) parts.push(`${result.partly} partly found`);
  if (result.notFound > 0) parts.push(`${result.notFound} not found`);
  return parts.join(" · ");
}

export function supportLabel(support: Support): string {
  return support === "strong" ? "Found in the document" : support === "weak" ? "Partly found" : "Not found";
}

// The marker drawn beside a sentence. Shape, not just colour, carries the meaning.
export function supportMark(support: Support): string {
  return support === "strong" ? "●" : support === "weak" ? "◐" : "○";
}

// Warnings about the numbers in a sentence, which are the likeliest thing for a summary to get wrong.
export function numberWarnings(sentence: ProofSentence): string[] {
  const warnings: string[] = [];
  if (sentence.missingNumbers.length > 0) {
    warnings.push(`Not in the document: ${sentence.missingNumbers.join(", ")}`);
  }
  if (sentence.elsewhereNumbers.length > 0) {
    warnings.push(`Only found elsewhere in the document, not beside this wording: ${sentence.elsewhereNumbers.join(", ")}`);
  }
  return warnings;
}

// The stored original a cited passage belongs to: by name when several files were combined, else the only one.
export function fileForPassage(files: FileInfo[] | undefined, passage: { document?: string | null }): FileInfo | undefined {
  if (!files || files.length === 0) return undefined;
  if (passage.document) return files.find((f) => f.name === passage.document);
  return files.length === 1 ? files[0] : undefined;
}
