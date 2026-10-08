"use client";
import type { ChatSource } from "../types";
import ChatPanel from "./ChatPanel";
import { useT } from "./I18nProvider";

// Chat across everything the user has saved. The server says which document, and which stored PDF,
// each cited passage came from.
export default function LibraryChat() {
  const t = useT();
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
        {t("library.heading")}
      </h2>
      <p className="muted mt-1">
        {t("library.intro")}
      </p>
      <ChatPanel
        path="/library/ask"
        idPrefix="library-source"
        emptyText={t("library.empty")}
        assistantLabel={t("library.label")}
        viewerFor={viewerFor}
        className="mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4"
      />
    </section>
  );
}
