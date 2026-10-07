"use client";
import { useEffect, useRef, useState } from "react";
import axios from "axios";
import Markdown from "markdown-to-jsx";
import type { ChatMessage } from "../types";
import { API_URL, apiError } from "../lib/api";
import { linkifyCitations } from "../lib/citations";
import CitationLink from "./CitationLink";
import SourceList from "./SourceList";

const MAX_QUESTION_CHARS = 1000;
const HISTORY_SENT = 10;

interface DocumentChatProps {
  documentId: number;
}

export default function DocumentChat({ documentId }: DocumentChatProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [question, setQuestion] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState("");
  // The cited source currently expanded: which answer it belongs to, and its number.
  const [openSource, setOpenSource] = useState<{ message: number; id: number } | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [messages, isSending]);

  // Bring a source into view when a citation is clicked.
  useEffect(() => {
    if (!openSource) return;
    document
      .getElementById(`source-${documentId}-${openSource.message}-${openSource.id}`)
      ?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [openSource, documentId]);

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
      const response = await axios.post(`${API_URL}/documents/${documentId}/chat`, {
        question: text,
        history,
      });
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
    <div className="mt-4 border-2 border-ink rounded-md p-2">
      <div className="border border-dashed border-ink/50 rounded-sm p-3">
        <div className="max-h-72 overflow-y-auto space-y-3 text-xl">
          {messages.length === 0 && (
            <p className="text-ink/50 tracking-wider">
              ASK ANYTHING ABOUT THIS DOCUMENT...
            </p>
          )}
          {messages.map((m, i) => (
            <div
              key={i}
              className={
                m.role === "user"
                  ? "text-right"
                  : "text-left border-l-2 border-ink/40 pl-3"
              }
            >
              <p className="uppercase text-sm tracking-widest text-ink/50">
                {m.role === "user" ? "You" : "Document"}
              </p>
              {m.role === "user" ? (
                <p className="whitespace-pre-wrap">{m.content}</p>
              ) : (
                <Markdown
                  options={{
                    overrides: {
                      a: { component: CitationLink, props: { onSelect: (id: number) => setOpenSource({ message: i, id }) } },
                      p: { props: { className: "mb-2" } },
                      ul: {
                        props: { className: "list-disc list-inside mb-2 ml-4" },
                      },
                      ol: {
                        props: {
                          className: "list-decimal list-inside mb-2 ml-4",
                        },
                      },
                    },
                  }}
                >
                  {linkifyCitations(m.content, m.sources)}
                </Markdown>
              )}
              {m.role === "assistant" && m.sources && (
                <SourceList
                  sources={m.sources}
                  idPrefix={`source-${documentId}-${i}`}
                  openId={openSource?.message === i ? openSource.id : null}
                  onToggle={(id, open) => setOpenSource(open ? { message: i, id } : null)}
                />
              )}
            </div>
          ))}
          {isSending && (
            <p className="text-ink/50 tracking-widest uppercase">Thinking...</p>
          )}
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
    </div>
  );
}
