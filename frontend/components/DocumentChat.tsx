"use client";
import type { ChatSource, FileInfo } from "../types";
import { fileForPassage } from "../lib/proof";
import ChatPanel from "./ChatPanel";
import { useT } from "./I18nProvider";

interface DocumentChatProps {
  documentId: number;
  // The original PDFs kept with this document, if any: they make cited pages openable.
  files?: FileInfo[];
  // A question to put in the box, for example from selected text. A new object each time.
  prefill?: { text: string };
}

// Chat about one saved document.
export default function DocumentChat({ documentId, files, prefill }: DocumentChatProps) {
  const t = useT();
  const viewerFor = (source: ChatSource) => {
    const file = fileForPassage(files, source);
    return file && source.page != null ? { documentId, file, page: source.page, passage: source.text } : undefined;
  };
  return (
    <ChatPanel
      path={`/documents/${documentId}/chat`}
      idPrefix={`source-${documentId}`}
      emptyText={t("chat.documentEmpty")}
      assistantLabel={t("chat.documentLabel")}
      viewerFor={viewerFor}
      testId="document-chat"
      suggestionsPath={`/documents/${documentId}/suggestions`}
      prefill={prefill}
    />
  );
}
