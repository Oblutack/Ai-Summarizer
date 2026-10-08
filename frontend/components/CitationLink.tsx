"use client";
import type { ReactNode } from "react";
import { citationId } from "../lib/citations";
import { useT } from "./I18nProvider";

interface CitationLinkProps {
  href?: string;
  children?: ReactNode;
  // Called with the source number when a citation chip is clicked.
  onSelect: (id: number) => void;
}

// How the markdown renderer draws links inside a chat answer. A link to "#cite-2" is a citation and
// becomes a small button that opens that source; any other link stays an ordinary link.
export default function CitationLink({ href, children, onSelect }: CitationLinkProps) {
  const t = useT();
  const id = citationId(href);
  if (id === null) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className="text-accent underline underline-offset-2">
        {children}
      </a>
    );
  }
  return (
    <button
      type="button"
      onClick={() => onSelect(id)}
      aria-label={t("citation.show", { id })}
      className="mx-[2px] inline-flex h-6 min-w-[1.5rem] items-center justify-center rounded border border-accent/60 px-1 align-baseline font-sans text-xs font-semibold leading-none text-accent hover:bg-accent hover:text-accent-fg"
    >
      {id}
    </button>
  );
}
