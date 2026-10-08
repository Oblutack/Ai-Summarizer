"use client";
import { describeSource } from "../lib/citations";
import type { ChatSource } from "../types";
import { LOCALES } from "../lib/i18n";
import { useI18n } from "./I18nProvider";

interface SourceListProps {
  sources: ChatSource[];
  // The source currently expanded, if any.
  openId: number | null;
  onToggle: (id: number, open: boolean) => void;
  // Makes element ids unique when several chats are on the page.
  idPrefix: string;
  // Present when originals can be shown: opens one at the source's page.
  onOpenDocument?: (source: ChatSource) => void;
  // Whether this particular source has an original to open (default: yes, when onOpenDocument is given).
  canOpenDocument?: (source: ChatSource) => boolean;
}

// The passages an answer cites, each collapsed to a one-line label until opened.
export default function SourceList({
  sources,
  openId,
  onToggle,
  idPrefix,
  onOpenDocument,
  canOpenDocument,
}: SourceListProps) {
  const { t, language } = useI18n();
  if (sources.length === 0) return null;
  return (
    <div className="mt-2 border-t border-dashed border-ink/40 pt-2" data-testid="sources">
      <p className="uppercase text-sm tracking-widest text-ink/50">{t("sources.title")}</p>
      <ul className="space-y-1 mt-1">
        {sources.map((s) => {
          const where = describeSource(s, t, LOCALES[language]);
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
                  {where || t("sources.excerpt")}
                </summary>
                <blockquote className="mt-1 mb-2 border-l-2 border-ink/40 pl-3 text-base whitespace-pre-wrap">
                  {s.text}
                </blockquote>
                {onOpenDocument && s.page != null && (canOpenDocument?.(s) ?? true) && (
                  <button
                    type="button"
                    onClick={() => onOpenDocument(s)}
                    className="mb-2 rounded border-2 border-ink px-3 py-1 text-base uppercase hover:bg-ink hover:text-canvas"
                  >
                    {t("sources.open", { page: s.page })}
                  </button>
                )}
              </details>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
