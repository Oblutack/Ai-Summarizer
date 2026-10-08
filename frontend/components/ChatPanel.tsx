"use client";
import { useEffect, useRef, useState } from "react";
import axios from "axios";
import Markdown from "markdown-to-jsx";
import dynamic from "next/dynamic";
import type { ChatMessage, ChatSource } from "../types";
import { API_URL, apiError } from "../lib/api";
import { linkifyCitations } from "../lib/citations";
import CitationLink from "./CitationLink";
import CopyButton from "./CopyButton";
import { useT } from "./I18nProvider";
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
  // Where questions worth asking can be fetched (POST), shown before the first question.
  suggestionsPath?: string;
  // Text to put in the question box (and focus it). A new object each time, so the same text can be sent again.
  prefill?: { text: string };
}

// A question-and-answer conversation whose answers cite their sources. Used for one document and
// for the whole library: only where the questions go and how a source maps to a PDF differ.
export default function ChatPanel({
  path,
  idPrefix,
  emptyText,
  assistantLabel,
  viewerFor,
  className,
  testId,
  suggestionsPath,
  prefill,
}: ChatPanelProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [question, setQuestion] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState("");
  // The cited source currently expanded: which answer it belongs to, and its number.
  const [openSource, setOpenSource] = useState<{ message: number; id: number } | null>(null);
  // The original document open in the viewer, at a cited page.
  const [viewing, setViewing] = useState<ViewerTarget | null>(null);
  const bottomRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const t = useT();

  // Questions worth asking, if this chat can offer them. They are a convenience, so a failure shows nothing.
  useEffect(() => {
    if (!suggestionsPath) return;
    let cancelled = false;
    axios
      .post<{ questions: string[] }>(`${API_URL}${suggestionsPath}`)
      .then((response) => !cancelled && setSuggestions(response.data.questions ?? []))
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [suggestionsPath]);

  useEffect(() => {
    if (!prefill) return;
    setQuestion(prefill.text);
    inputRef.current?.focus();
  }, [prefill]);

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

  const send = async (text: string) => {
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
      setError(apiError(err, t("common.tryAgain")));
    } finally {
      setIsSending(false);
    }
  };

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault();
    return send(question.trim());
  };

  return (
    <div className={className ?? "mt-4 rounded-xl border border-ink/20 bg-canvas/50 p-4"} data-testid={testId}>
      <div>
        <div className="max-h-96 space-y-4 overflow-y-auto pr-1 text-base">
          {messages.length === 0 && <p className="text-ink/70">{emptyText}</p>}
          {messages.length === 0 && suggestions.length > 0 && (
            <ul className="flex flex-wrap gap-2" aria-label={t("chat.suggested")} data-testid="suggestions">
              {suggestions.map((s) => (
                <li key={s}>
                  <button
                    type="button"
                    onClick={() => send(s)}
                    disabled={isSending}
                    className="chip py-1.5 text-sm disabled:opacity-50"
                  >
                    {s}
                  </button>
                </li>
              ))}
            </ul>
          )}
          {messages.map((m, i) => (
            <div key={i} className={m.role === "user" ? "ml-auto w-fit max-w-[90%] rounded-2xl rounded-br-sm bg-ink/10 px-4 py-2" : "reading max-w-[95%] border-l-2 border-accent/60 pl-4 text-base"}>
              <p className="mb-0.5 font-sans text-xs font-semibold uppercase tracking-widest text-ink/70">
                {m.role === "user" ? t("chat.you") : assistantLabel}
              </p>
              {m.role === "user" ? (
                <p className="whitespace-pre-wrap">{m.content}</p>
              ) : (
                <Markdown
                  options={{
                    disableParsingRawHTML: true,
                    overrides: {
                      a: {
                        component: CitationLink,
                        props: { onSelect: (id: number) => setOpenSource({ message: i, id }) },
                      },
                      p: { props: { className: "mb-2 last:mb-0" } },
                      ul: { props: { className: "mb-2 list-disc pl-5" } },
                      ol: { props: { className: "mb-2 list-decimal pl-5" } },
                    },
                  }}
                >
                  {linkifyCitations(m.content, m.sources)}
                </Markdown>
              )}
              {m.role === "assistant" && (
                <div className="mt-1">
                  <CopyButton variant="link" what={t("copy.whatAnswer")} text={m.content} />
                </div>
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
          {isSending && <p className="text-sm font-medium text-ink/70">{t("chat.thinking")}</p>}
          <div ref={bottomRef} />
        </div>

        {error && (
          <p className="mt-2 text-sm font-medium text-danger" role="alert">
            {error}
          </p>
        )}

        <form onSubmit={handleSend} className="mt-4 flex gap-2">
          <input
            ref={inputRef}
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            maxLength={MAX_QUESTION_CHARS}
            placeholder={t("chat.placeholder")}
            className="field flex-1"
          />
          <button
            type="submit"
            disabled={isSending || !question.trim()}
            className="btn btn-primary"
          >
            {t("chat.ask")}
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
