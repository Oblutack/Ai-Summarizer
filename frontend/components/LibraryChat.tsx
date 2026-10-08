"use client";
import { useEffect, useState } from "react";
import type { ChatSource } from "../types";
import ChatPanel from "./ChatPanel";
import CollectionOverview from "./CollectionOverview";
import { useT } from "./I18nProvider";

interface LibraryChatProps {
  // The tags the person has used and how many documents carry each. A tag is a collection: its documents
  // can be asked about as a group.
  tags: { tag: string; count: number }[];
  // The tag the saved list is filtered by, if any: asking follows it.
  activeTag: string;
}

// Chat across everything the user has saved, or across one collection. The server says which document,
// and which stored PDF, each cited passage came from.
export default function LibraryChat({ tags, activeTag }: LibraryChatProps) {
  const t = useT();
  const [scope, setScope] = useState(activeTag);

  // Choosing a tag to filter the list also points the question at that group.
  useEffect(() => setScope(activeTag), [activeTag]);
  // A collection that no longer exists (its last document was untagged or deleted) falls back to everything.
  const known = scope === "" || tags.some((entry) => entry.tag === scope);
  const effectiveScope = known ? scope : "";
  const selected = tags.find((entry) => entry.tag === effectiveScope);

  const viewerFor = (source: ChatSource) =>
    source.documentId != null && source.fileId != null && source.page != null
      ? {
          documentId: source.documentId,
          file: { id: source.fileId, name: source.fileName ?? "document.pdf" },
          page: source.page,
          passage: source.text,
        }
      : undefined;

  return (
    <section aria-labelledby="library-heading" className="card mb-8" data-testid="library">
      <h2 id="library-heading" className="text-2xl md:text-3xl">
        {selected ? t("library.headingScoped", { tag: selected.tag }) : t("library.heading")}
      </h2>
      <p className="muted mt-1">{t("library.intro")}</p>

      {tags.length > 0 ? (
        <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-2">
          <label htmlFor="library-scope" className="label mb-0">
            {t("library.scope")}
          </label>
          <select
            id="library-scope"
            value={effectiveScope}
            onChange={(e) => setScope(e.target.value)}
            className="field w-auto max-w-full"
          >
            <option value="">{t("library.scopeAll")}</option>
            {tags.map((entry) => (
              <option key={entry.tag} value={entry.tag}>
                {entry.tag} ({entry.count})
              </option>
            ))}
          </select>
        </div>
      ) : (
        <p className="muted mt-3 text-sm">{t("library.tagTip")}</p>
      )}

      {/* A briefing on the whole group, once it has at least two documents to compare. */}
      {selected && selected.count >= 2 && <CollectionOverview key={`overview-${selected.tag}`} tag={selected.tag} count={selected.count} />}

      {/* A new conversation for each group: an answer about one collection should not carry into another. */}
      <ChatPanel
        key={`chat-${effectiveScope}`}
        path="/library/ask"
        idPrefix="library-source"
        emptyText={selected ? t("library.emptyScoped", { tag: selected.tag, count: selected.count }) : t("library.empty")}
        assistantLabel={selected ? selected.tag : t("library.label")}
        viewerFor={viewerFor}
        extraBody={effectiveScope ? { tag: effectiveScope } : undefined}
        className="mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4"
      />
    </section>
  );
}
