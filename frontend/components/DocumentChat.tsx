"use client";
import type { ChatSource, FileInfo } from "../types";
import { fileForPassage } from "../lib/proof";
import ChatPanel from "./ChatPanel";

interface DocumentChatProps {
  documentId: number;
  // The original PDFs kept with this document, if any: they make cited pages openable.
  files?: FileInfo[];
}

// Chat about one saved document.
export default function DocumentChat({ documentId, files }: DocumentChatProps) {
  const viewerFor = (source: ChatSource) => {
    const file = fileForPassage(files, source);
    return file && source.page != null ? { documentId, file, page: source.page, passage: source.text } : undefined;
  };
  return (
    <ChatPanel
      path={`/documents/${documentId}/chat`}
      idPrefix={`source-${documentId}`}
      emptyText="ASK ANYTHING ABOUT THIS DOCUMENT..."
      assistantLabel="Document"
      viewerFor={viewerFor}
      testId="document-chat"
    />
  );
}
