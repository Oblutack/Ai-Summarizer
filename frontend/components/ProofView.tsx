"use client";
import { describeSource } from "../lib/citations";
import { fileForPassage, numberWarnings, proofHeadline, supportLabel, supportMark } from "../lib/proof";
import type { FileInfo, ProofPassage, ProofResult } from "../types";

interface ProofViewProps {
  result: ProofResult;
  // The original PDFs kept with the document, if any: they make the evidence openable.
  files?: FileInfo[];
  onOpenDocument: (file: FileInfo, passage: ProofPassage) => void;
}

// A summary laid out sentence by sentence, each marked by how well the original document backs it,
// and each opening to show the passage it was matched with.
export default function ProofView({ result, files, onOpenDocument }: ProofViewProps) {
  return (
    <div data-testid="proof" className="text-xl">
      <p className="font-bold" data-testid="proof-headline">
        {proofHeadline(result)}
      </p>

      {!result.verifiable && (
        <p className="mt-1 text-lg" role="status">
          Hardly any wording matches, so this summary is probably in a different language than the document. The
          check compares words, so it cannot tell here.
        </p>
      )}

      <p className="mt-1 text-base text-ink/70">
        <span aria-hidden="true">● </span>found &nbsp;
        <span aria-hidden="true">◐ </span>partly found &nbsp;
        <span aria-hidden="true">○ </span>not found. This compares wording, not meaning: &ldquo;partly&rdquo; or
        &ldquo;not found&rdquo; means look closer, not that it is wrong.
      </p>

      <ul className="mt-3 space-y-2">
        {result.sentences.map((s, i) => {
          if (s.kind === "heading" || s.support === null) {
            return (
              <li key={i} className="pt-2 text-xl font-bold tracking-wide">
                {s.text}
              </li>
            );
          }
          const warnings = numberWarnings(s);
          return (
            <li key={i} data-support={s.support}>
              <details>
                <summary className="cursor-pointer">
                  <span
                    role="img"
                    aria-label={supportLabel(s.support)}
                    className={s.support === "none" ? "mr-2 text-red-600" : "mr-2"}
                  >
                    {supportMark(s.support)}
                  </span>
                  {s.text}
                  {warnings.length > 0 && (
                    <span className="ml-2 text-base text-red-600" aria-hidden="true">
                      &#9888;
                    </span>
                  )}
                </summary>
                <div className="ml-6 mt-1 space-y-2 text-base">
                  <p className="text-ink/70">{supportLabel(s.support)}</p>
                  {warnings.map((w) => (
                    <p key={w} className="text-red-600" role="note">
                      &#9888; {w}
                    </p>
                  ))}
                  {s.passages.length === 0 && <p>No passage of the document matches this sentence.</p>}
                  {s.passages.map((p) => {
                    const file = fileForPassage(files, p);
                    return (
                      <div key={p.id}>
                        <p className="uppercase tracking-widest text-sm text-ink/50">
                          {describeSource(p) || "Excerpt"}
                        </p>
                        <blockquote className="border-l-2 border-ink/40 pl-3 whitespace-pre-wrap">{p.text}</blockquote>
                        {file && p.page != null && (
                          <button
                            type="button"
                            onClick={() => onOpenDocument(file, p)}
                            className="mt-1 rounded border-2 border-ink px-3 py-1 text-base uppercase hover:bg-ink hover:text-canvas"
                          >
                            Open page {p.page} in the document
                          </button>
                        )}
                      </div>
                    );
                  })}
                </div>
              </details>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
