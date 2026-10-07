"use client";
import type { ReactNode } from "react";
import { citationId } from "../lib/citations";

interface CitationLinkProps {
  href?: string;
  children?: ReactNode;
  // Called with the source number when a citation chip is clicked.
  onSelect: (id: number) => void;
}

// How the markdown renderer draws links inside a chat answer. A link to "#cite-2" is a citation and
// becomes a small button that opens that source; any other link stays an ordinary link.
export default function CitationLink({ href, children, onSelect }: CitationLinkProps) {
  const id = citationId(href);
  if (id === null) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className="underline">
        {children}
      </a>
    );
  }
  return (
    <button
      type="button"
      onClick={() => onSelect(id)}
      aria-label={`Show source ${id}`}
      className="inline-flex items-center justify-center min-w-[1.5rem] h-6 px-1 mx-[2px] align-baseline text-sm leading-none border border-ink rounded hover:bg-ink hover:text-canvas"
    >
      {id}
    </button>
  );
}
