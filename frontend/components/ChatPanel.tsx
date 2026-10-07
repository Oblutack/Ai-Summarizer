"use client";
import { useEffect, useRef, useState } from "react";
import axios from "axios";
import Markdown from "markdown-to-jsx";
import dynamic from "next/dynamic";
import type { ChatMessage, ChatSource } from "../types";
import { API_URL, apiError } from "../lib/api";
import { linkifyCitations } from "../lib/citations";
import CitationLink from "./CitationLink";
import SourceList from "./SourceList";

const MAX_QUESTION_CHARS = 1000;
const HISTORY_SENT = 10;

// The PDF viewer is large; it is only fetched when someone opens a page.
const PdfViewer = dynamic(() => import("./PdfViewer"), { ssr: false });

// An original PDF page a source can be opened at.
export interface ViewerTarget {
  documentId: number;
  file: { id: number; name: string };
  page: number;
  passage: string;
}

interface ChatPanelProps {
  // Where a question is posted, relative to the API (e.g. "/documents/3/chat").
  path: string;
  // Makes DOM ids unique when several chats are on the page.
  idPrefix: string;
  // Shown before the first question.
  emptyText: string;
  // Who the answers come from, as labelled above each one.
  assistantLabel: string;
  // Where a source's page can be shown in the original PDF, or undefined if it cannot.
  viewerFor: (source: ChatSource) => ViewerTarget | undefined;
  className?: string;
  // A name for tests to find this chat by.
  testId?: string;
}

// A question-and-answer conversation whose answers cite their sources. Used for one document and
// for the whole library: only where the questions go and how a source maps to a PDF differ.
export default function ChatPanel({ path, idPrefix, emptyText, assistantLabel, viewerFor, className, testId }: ChatPanelProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [question, setQuestion] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState("");
  // The cited source currently expanded: which answer it belongs to, and its number.
  const [openSource, setOpenSource] = useState<{ message: number; id: number } | null>(null);
  // The original document open in the viewer, at a cited page.
  const [viewing, setViewing] = useState<ViewerTarget | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [messages, isSending]);

  // Bring a source into view when a citation is clicked.
  useEffect(() => {
    if (!openSource) return;
    document.getElementById(`${idPrefix}-${openSource.message}-${openSource.id}`)?.scrollIntoView({
      behavior: "smooth",
      block: "nearest",
    });
  }, [openSource, idPrefix]);

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    const text = question.trim();
    if (!text || isSending) return;

    // Only the role and text go back to the server; the sources of earlier answers do not.
    const history = messages.slice(-HISTORY_SENT).map(({ role, content }) => ({ role, content }));
    setMessages((prev) => [...prev, { role: "user", content: text }]);
    setQuestion("");
    setError("");
    setIsSending(true);

    try {
      const response = await axios.post(`${API_URL}${path}`, { question: text, history });
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: response.data.answer, sources: response.data.sources ?? [] },
      ]);
    } catch (err) {
      setError(apiError(err, "Something went wrong. Please try again."));
    } finally {
      setIsSending(false);
    }
  };

  return (
    <div className={className ?? "mt-4 border-2 border-ink rounded-md p-2"} data-testid={testId}>
      <div className="border border-dashed border-ink/50 rounded-sm p-3">
        <div className="max-h-72 overflow-y-auto space-y-3 text-xl">
          {messages.length === 0 && <p className="text-ink/50 tracking-wider">{emptyText}</p>}
          {messages.map((m, i) => (
            <div key={i} className={m.role === "user" ? "text-right" : "text-left border-l-2 border-ink/40 pl-3"}>
              <p className="uppercase text-sm tracking-widest text-ink/50">
                {m.role === "user" ? "You" : assistantLabel}
              </p>
              {m.role === "user" ? (
                <p className="whitespace-pre-wrap">{m.content}</p>
              ) : (
                <Markdown
                  options={{
                    overrides: {
                      a: {
                        component: CitationLink,
                        props: { onSelect: (id: number) => setOpenSource({ message: i, id }) },
                      },
                      p: { props: { className: "mb-2" } },
                      ul: { props: { className: "list-disc list-inside mb-2 ml-4" } },
                      ol: { props: { className: "list-decimal list-inside mb-2 ml-4" } },
                    },
                  }}
                >
                  {linkifyCitations(m.content, m.sources)}
                </Markdown>
              )}
              {m.role === "assistant" && m.sources && (
                <SourceList
                  sources={m.sources}
                  idPrefix={`${idPrefix}-${i}`}
                  openId={openSource?.message === i ? openSource.id : null}
                  onToggle={(id, open) => setOpenSource(open ? { message: i, id } : null)}
                  canOpenDocument={(source) => viewerFor(source) !== undefined}
                  onOpenDocument={(source) => {
                    const target = viewerFor(source);
                    if (target) setViewing(target);
                  }}
                />
              )}
            </div>
          ))}
          {isSending && <p className="text-ink/50 tracking-widest uppercase">Thinking...</p>}
          <div ref={bottomRef} />
        </div>

        {error && (
          <p className="text-red-500 text-lg mt-2" role="alert">
            {error}
          </p>
        )}

        <form onSubmit={handleSend} className="flex gap-3 mt-3">
          <input
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            maxLength={MAX_QUESTION_CHARS}
            placeholder="Type a question"
            className="flex-grow p-2 bg-canvas border-2 border-ink rounded-md focus:outline-none text-xl"
          />
          <button
            type="submit"
            disabled={isSending || !question.trim()}
            className="bg-ink text-canvas uppercase font-bold px-5 rounded-md border-2 border-ink hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Ask
          </button>
        </form>
      </div>
      {viewing && (
        <PdfViewer
          documentId={viewing.documentId}
          file={viewing.file}
          page={viewing.page}
          passage={viewing.passage}
          onClose={() => setViewing(null)}
        />
      )}
    </div>
  );
}
