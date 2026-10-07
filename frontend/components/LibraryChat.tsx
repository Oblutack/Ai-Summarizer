"use client";
import type { ChatSource } from "../types";
import ChatPanel from "./ChatPanel";

// Chat across everything the user has saved. The server says which document, and which stored PDF,
// each cited passage came from.
export default function LibraryChat() {
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
    <section aria-labelledby="library-heading" className="mb-12 border-2 border-ink rounded-lg p-6" data-testid="library">
      <h2 id="library-heading" className="text-3xl uppercase tracking-widest text-center">
        Ask All Your Documents
      </h2>
      <p className="text-center text-lg text-ink/70 mt-1">
        One question, searched across everything you have saved. Answers say which document and page they come from.
      </p>
      <ChatPanel
        path="/library/ask"
        idPrefix="library-source"
        emptyText="ASK ABOUT ANYTHING YOU HAVE SAVED..."
        assistantLabel="Your documents"
        viewerFor={viewerFor}
        className="mt-4 border-2 border-ink rounded-md p-2"
      />
    </section>
  );
}
