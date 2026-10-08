import type { FileInfo, ProofResult, ProofSentence, Support } from "../types";
import { englishT, type Translate } from "./i18n";

// "12 of 13 statements found in the document · 1 partly found"
export function proofHeadline(result: ProofResult, t: Translate = englishT): string {
  const parts = [t("proof.headline", { found: result.found, claims: result.claims })];
  if (result.partly > 0) parts.push(t("proof.partly", { n: result.partly }));
  if (result.notFound > 0) parts.push(t("proof.notFound", { n: result.notFound }));
  return parts.join(" · ");
}

export function supportLabel(support: Support, t: Translate = englishT): string {
  return t(support === "strong" ? "proof.strong" : support === "weak" ? "proof.weak" : "proof.none");
}

// The marker drawn beside a sentence. Shape, not just colour, carries the meaning.
export function supportMark(support: Support): string {
  return support === "strong" ? "●" : support === "weak" ? "◐" : "○";
}

// Warnings about the numbers in a sentence, which are the likeliest thing for a summary to get wrong.
export function numberWarnings(sentence: ProofSentence, t: Translate = englishT): string[] {
  const warnings: string[] = [];
  if (sentence.missingNumbers.length > 0) {
    warnings.push(t("proof.missingNumbers", { numbers: sentence.missingNumbers.join(", ") }));
  }
  if (sentence.elsewhereNumbers.length > 0) {
    warnings.push(t("proof.elsewhere", { numbers: sentence.elsewhereNumbers.join(", ") }));
  }
  return warnings;
}

// The stored original a cited passage belongs to: by name when several files were combined, else the only one.
export function fileForPassage(files: FileInfo[] | undefined, passage: { document?: string | null }): FileInfo | undefined {
  if (!files || files.length === 0) return undefined;
  if (passage.document) return files.find((f) => f.name === passage.document);
  return files.length === 1 ? files[0] : undefined;
}
