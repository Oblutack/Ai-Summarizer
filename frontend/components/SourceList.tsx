"use client";
import { describeSource } from "../lib/citations";
import type { ChatSource } from "../types";

interface SourceListProps {
  sources: ChatSource[];
  // The source currently expanded, if any.
  openId: number | null;
  onToggle: (id: number, open: boolean) => void;
  // Makes element ids unique when several chats are on the page.
  idPrefix: string;
}

// The passages an answer cites, each collapsed to a one-line label until opened.
export default function SourceList({ sources, openId, onToggle, idPrefix }: SourceListProps) {
  if (sources.length === 0) return null;
  return (
    <div className="mt-2 border-t border-dashed border-ink/40 pt-2" data-testid="sources">
      <p className="uppercase text-sm tracking-widest text-ink/50">Sources</p>
      <ul className="space-y-1 mt-1">
        {sources.map((s) => {
          const where = describeSource(s);
          return (
            <li key={s.id} id={`${idPrefix}-${s.id}`}>
              <details
                open={openId === s.id}
                onToggle={(e) => {
                  const isOpen = e.currentTarget.open;
                  if (isOpen !== (openId === s.id)) onToggle(s.id, isOpen);
                }}
              >
                <summary className="cursor-pointer text-base">
                  <span className="inline-flex items-center justify-center min-w-[1.5rem] h-6 px-1 mr-2 text-sm leading-none border border-ink rounded">
                    {s.id}
                  </span>
                  {where || "Excerpt"}
                </summary>
                <blockquote className="mt-1 mb-2 border-l-2 border-ink/40 pl-3 text-base whitespace-pre-wrap">
                  {s.text}
                </blockquote>
              </details>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
