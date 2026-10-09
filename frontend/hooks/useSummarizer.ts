"use client";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { TURNSTILE_HEADER } from "../lib/api";
import { readSummaryEvents, type SummaryEvent } from "../lib/sse";
import { isAudioFile, isDocumentFile, isImageFile, MAX_AUDIO_MB, webLinkIn } from "../lib/links";
import { shrinkPhoto } from "../lib/photos";
import { loadPreferences, savePreferences } from "../lib/preferences";
import { MAX_FILES } from "../lib/summaryOptions";
import { useT } from "../components/I18nProvider";

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
  const { user, refresh } = useAuth();
  const t = useT();

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

  // The last style, language and length come back on the next visit. They are read after the first
  // render (not in the initial state) so the server-rendered page and the first client render agree.
  const preferencesLoaded = useRef(false);
  useEffect(() => {
    const saved = loadPreferences();
    setWordCount(saved.wordCount);
    setStyle(saved.style);
    setLanguage(saved.language);
    preferencesLoaded.current = true;
  }, []);
  useEffect(() => {
    if (preferencesLoaded.current) savePreferences({ wordCount, style, language });
  }, [wordCount, style, language]);

  // The web address in the box, when that is all the box holds.
  const link = useMemo(() => (files.length === 0 ? webLinkIn(inputText) : null), [files, inputText]);

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
      ? t("stage.sections", { done: chunks.done, total: chunks.total })
      : stage === "writing"
      ? t("stage.writing")
      : files.some((f) => isAudioFile(f.name))
      ? t("stage.transcribing")
      : files.some((f) => isImageFile(f.name))
      ? t("stage.scanning")
      : t("stage.reading");

  const changeText = useCallback((value: string) => {
    setInputText(value);
    // Keep the same array when already empty so typing doesn't re-render for nothing.
    setFiles((prev) => (prev.length === 0 ? prev : []));
    setError("");
  }, []);

  const addFiles = useCallback(
    (picked: File[]) => {
      if (picked.length === 0) return;

      let documents = picked.filter((f) => isDocumentFile(f.name));
      // Recordings cost money to transcribe: signed-in people only, and not bigger than the server takes.
      let refused = "";
      if (!user && documents.some((f) => isAudioFile(f.name))) {
        documents = documents.filter((f) => !isAudioFile(f.name));
        refused = t("form.errAudioSignIn");
      }
      // Reading the text in a photo is real work on the server, so photos are for signed-in people too.
      if (!user && documents.some((f) => isImageFile(f.name))) {
        documents = documents.filter((f) => !isImageFile(f.name));
        refused = refused || t("form.errPhotoSignIn");
      }
      const tooBig = documents.filter((f) => isAudioFile(f.name) && f.size > MAX_AUDIO_MB * 1024 * 1024);
      if (tooBig.length > 0) {
        documents = documents.filter((f) => !tooBig.includes(f));
        refused = refused || t("form.errAudioSize", { name: tooBig[0].name, max: MAX_AUDIO_MB });
      }
      const merged = [...files];
      for (const f of documents) {
        if (!merged.some((m) => m.name === f.name && m.size === f.size)) {
          merged.push(f);
        }
      }

      if (refused) {
        setError(refused);
      } else if (documents.length < picked.length) {
        setError(t("form.errFileType"));
      } else if (merged.length > MAX_FILES) {
        setError(t("form.errMaxFiles", { max: MAX_FILES }));
      } else {
        setError("");
      }
      setFiles(merged.slice(0, MAX_FILES));
      setInputText("");
    },
    [files, t, user]
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
  const buildRequest = (headers: Record<string, string>, uploads: File[]): { url: string; init: RequestInit } => {
    if (uploads.length > 0) {
      const multiple = uploads.length > 1;
      const formData = new FormData();
      uploads.forEach((f) => formData.append(multiple ? "files" : "file", f));
      formData.append("wordCount", String(wordCount));
      formData.append("pageLimit", pageLimit);
      formData.append("style", style);
      formData.append("language", language);
      const base = multiple ? endpoint.replace("summarize", "summarize-multiple") : endpoint;
      // The browser sets the multipart Content-Type (with its boundary) itself.
      return { url: `${base}?stream=true`, init: { method: "POST", headers, body: formData } };
    }

    const query = new URLSearchParams({ wordCount: String(wordCount), pageLimit, style, language, stream: "true" });
    // A box that holds just a web address means "read that page".
    const target = link ? "summarize-url" : "summarize-text";
    return {
      url: `${endpoint.replace("summarize", target)}?${query.toString()}`,
      init: {
        method: "POST",
        headers: { ...headers, "Content-Type": "application/json" },
        body: JSON.stringify(link ? { url: link } : { text: inputText }),
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
        setError(event.message || t("form.errGeneric"));
        return "error";
      case "done":
        return "done";
    }
  };

  const submit = async () => {
    if (files.length === 0 && !inputText) {
      setError(t("form.errNeedInput"));
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
      // Big photos are made smaller first (the list on screen keeps showing the originals).
      const uploads = await Promise.all(files.map((f) => (isImageFile(f.name) ? shrinkPhoto(f) : f)));
      const { url, init } = buildRequest(headers, uploads);
      // credentials: "include" sends the session cookie to the API.
      const response = await fetch(url, { ...init, credentials: "include", signal: controller.signal });

      if (!response.ok) {
        const body = await response.json().catch(() => null);
        // A rejected session means we were signed out elsewhere: re-check, which clears the user.
        if (response.status === 401 && body?.code === "unauthenticated") refresh();
        setError(body?.error || t("form.errGeneric"));
        return;
      }

      let outcome: "done" | "error" | undefined;
      await readSummaryEvents(response, (event) => {
        outcome = handleEvent(event) ?? outcome;
      });

      if (outcome === "done") {
        onSummaryCreated?.();
      } else if (!outcome) {
        setError(t("form.errInterrupted"));
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") {
        setError(t("form.errCancelled"));
      } else if (err instanceof TypeError) {
        setError(t("form.errNetwork"));
      } else {
        setError(t("form.errUnexpected"));
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
      : link
      ? "web-page"
      : "pasted-text";

  return {
    // options
    wordCount, setWordCount, pageLimit, setPageLimit, style, setStyle, language, setLanguage,
    // input
    files, inputText, link, changeText, addFiles, removeFile, hasAudio: files.some((f) => isAudioFile(f.name)),
    hasPhotos: files.some((f) => isImageFile(f.name)),
    // result
    summary, error, isLoading, progress, stageLabel, showPageLimit, exportBaseName,
    // actions
    submit, cancel, incrementPageLimit, decrementPageLimit,
    canSubmit: !isLoading && (files.length > 0 || inputText.length > 0),
  };
}
