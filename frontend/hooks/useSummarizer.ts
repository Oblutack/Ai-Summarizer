"use client";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { TURNSTILE_HEADER } from "../lib/api";
import { readSummaryEvents, type SummaryEvent } from "../lib/sse";
import { MAX_FILES } from "../lib/summaryOptions";

// Past this many words in pasted text, a page limit makes more sense than a word count.
const LONG_TEXT_WORDS = 1000;
// Until the server reports real progress, ease toward this value so the bar keeps moving.
const WAITING_PROGRESS_CAP = 35;

export type Stage = "idle" | "preparing" | "summarizing" | "writing";

interface UseSummarizerOptions {
  endpoint: string;
  onSummaryCreated?: () => void;
  // The human-check token for anonymous requests, and a function that asks for a fresh one (a
  // token works once).
  humanCheck?: { token: string; reset: () => void };
}

export function useSummarizer({ endpoint, onSummaryCreated, humanCheck }: UseSummarizerOptions) {
  const { refresh } = useAuth();

  const [wordCount, setWordCount] = useState(150);
  const [pageLimit, setPageLimit] = useState("");
  const [style, setStyle] = useState("default");
  const [language, setLanguage] = useState("English");
  const [files, setFiles] = useState<File[]>([]);
  const [inputText, setInputText] = useState("");
  const [summary, setSummary] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [stage, setStage] = useState<Stage>("idle");
  const [chunks, setChunks] = useState({ done: 0, total: 0 });
  const [waiting, setWaiting] = useState(0);
  const abortRef = useRef<AbortController | null>(null);

  const showPageLimit = useMemo(
    () => files.length > 0 || inputText.trim().split(/\s+/).length > LONG_TEXT_WORDS,
    [files, inputText]
  );

  // Keeps the bar moving while the server hasn't reported anything yet.
  useEffect(() => {
    if (!isLoading) return;
    setWaiting(0);
    const id = setInterval(() => {
      setWaiting((prev) => Math.min(WAITING_PROGRESS_CAP, prev + (WAITING_PROGRESS_CAP - prev) / 8));
    }, 800);
    return () => clearInterval(id);
  }, [isLoading]);

  const progress = useMemo(() => {
    if (!isLoading) return 100;
    if (stage === "writing") return 95;
    if (stage === "summarizing" && chunks.total > 0) {
      return Math.max(waiting, 10 + (80 * chunks.done) / chunks.total);
    }
    return waiting;
  }, [isLoading, stage, chunks, waiting]);

  const stageLabel =
    stage === "summarizing" && chunks.total > 1
      ? `Summarized ${chunks.done} of ${chunks.total} sections...`
      : stage === "writing"
      ? "Writing the summary..."
      : "Reading your document...";

  const changeText = useCallback((value: string) => {
    setInputText(value);
    // Keep the same array when already empty so typing doesn't re-render for nothing.
    setFiles((prev) => (prev.length === 0 ? prev : []));
    setError("");
  }, []);

  const addFiles = useCallback(
    (picked: File[]) => {
      if (picked.length === 0) return;

      const pdfs = picked.filter((f) => f.name.toLowerCase().endsWith(".pdf"));
      const merged = [...files];
      for (const f of pdfs) {
        if (!merged.some((m) => m.name === f.name && m.size === f.size)) {
          merged.push(f);
        }
      }

      if (pdfs.length < picked.length) {
        setError("Only PDF files are supported.");
      } else if (merged.length > MAX_FILES) {
        setError(`You can attach up to ${MAX_FILES} PDFs at once.`);
      } else {
        setError("");
      }
      setFiles(merged.slice(0, MAX_FILES));
      setInputText("");
    },
    [files]
  );

  const removeFile = useCallback((index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const incrementPageLimit = () => setPageLimit(String(Number(pageLimit || 0) + 1));
  const decrementPageLimit = () => {
    const current = Number(pageLimit || 0);
    if (current > 1) setPageLimit(String(current - 1));
  };

  const cancel = () => abortRef.current?.abort();

  // Builds the request for the current input. Streaming is requested with ?stream=true.
  const buildRequest = (headers: Record<string, string>): { url: string; init: RequestInit } => {
    if (files.length > 0) {
      const multiple = files.length > 1;
      const formData = new FormData();
      files.forEach((f) => formData.append(multiple ? "files" : "file", f));
      formData.append("wordCount", String(wordCount));
      formData.append("pageLimit", pageLimit);
      formData.append("style", style);
      formData.append("language", language);
      const base = multiple ? endpoint.replace("summarize", "summarize-multiple") : endpoint;
      // The browser sets the multipart Content-Type (with its boundary) itself.
      return { url: `${base}?stream=true`, init: { method: "POST", headers, body: formData } };
    }

    const query = new URLSearchParams({ wordCount: String(wordCount), pageLimit, style, language, stream: "true" });
    return {
      url: `${endpoint.replace("summarize", "summarize-text")}?${query.toString()}`,
      init: {
        method: "POST",
        headers: { ...headers, "Content-Type": "application/json" },
        body: JSON.stringify({ text: inputText }),
      },
    };
  };

  const handleEvent = (event: SummaryEvent): "done" | "error" | undefined => {
    switch (event.type) {
      case "status":
        setStage(event.stage);
        if (event.stage === "summarizing") setChunks({ done: event.done ?? 0, total: event.total ?? 0 });
        return;
      case "delta":
        setStage("writing");
        setSummary((prev) => prev + event.text);
        return;
      case "error":
        setError(event.message || "An error occurred.");
        return "error";
      case "done":
        return "done";
    }
  };

  const submit = async () => {
    if (files.length === 0 && !inputText) {
      setError("Please attach a PDF or paste some text.");
      return;
    }

    setError("");
    setSummary("");
    setStage("preparing");
    setChunks({ done: 0, total: 0 });
    setIsLoading(true);
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const headers: Record<string, string> = {};
      if (humanCheck?.token) headers[TURNSTILE_HEADER] = humanCheck.token;
      const { url, init } = buildRequest(headers);
      // credentials: "include" sends the session cookie to the API.
      const response = await fetch(url, { ...init, credentials: "include", signal: controller.signal });

      if (!response.ok) {
        const body = await response.json().catch(() => null);
        // A rejected session means we were signed out elsewhere: re-check, which clears the user.
        if (response.status === 401 && body?.code === "unauthenticated") refresh();
        setError(body?.error || "An error occurred.");
        return;
      }

      let outcome: "done" | "error" | undefined;
      await readSummaryEvents(response, (event) => {
        outcome = handleEvent(event) ?? outcome;
      });

      if (outcome === "done") {
        onSummaryCreated?.();
      } else if (!outcome) {
        setError("The summary was interrupted. Please try again.");
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") {
        setError("Summarization was cancelled.");
      } else if (err instanceof TypeError) {
        setError("Could not reach the server. Check your connection and try again.");
      } else {
        setError("An unexpected error occurred.");
      }
    } finally {
      humanCheck?.reset();
      setStage("idle");
      setIsLoading(false);
      abortRef.current = null;
    }
  };

  // Name for the exported PDF.
  const exportBaseName =
    files.length === 1
      ? files[0].name.replace(/\.[^/.]+$/, "")
      : files.length > 1
      ? "combined-documents"
      : "pasted-text";

  return {
    // options
    wordCount, setWordCount, pageLimit, setPageLimit, style, setStyle, language, setLanguage,
    // input
    files, inputText, changeText, addFiles, removeFile,
    // result
    summary, error, isLoading, progress, stageLabel, showPageLimit, exportBaseName,
    // actions
    submit, cancel, incrementPageLimit, decrementPageLimit,
    canSubmit: !isLoading && (files.length > 0 || inputText.length > 0),
  };
}
