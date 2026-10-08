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
    <div className="mt-3 border-t border-ink/15 pt-2" data-testid="sources">
      <p className="text-xs font-semibold uppercase tracking-widest text-ink/70">{t("sources.title")}</p>
      <ul className="mt-1 space-y-1">
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
                <summary className="cursor-pointer text-sm">
                  <span className="mr-2 inline-flex h-6 min-w-[1.5rem] items-center justify-center rounded border border-ink/50 px-1 text-xs leading-none">
                    {s.id}
                  </span>
                  {where || t("sources.excerpt")}
                </summary>
                <blockquote className="mb-2 mt-1 whitespace-pre-wrap border-l-2 border-accent/60 pl-3 text-sm">
                  {s.text}
                </blockquote>
                {onOpenDocument && s.page != null && (canOpenDocument?.(s) ?? true) && (
                  <button
                    type="button"
                    onClick={() => onOpenDocument(s)}
                    className="btn btn-secondary btn-sm mb-2"
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
