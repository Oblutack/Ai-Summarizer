"use client";
import { useEffect, useRef, useState } from "react";
import axios from "axios";
import type { PDFDocumentProxy, RenderTask } from "pdfjs-dist";
import type { TextItem } from "pdfjs-dist/types/src/display/api";
import { API_URL, apiError } from "../lib/api";
import { findHighlight } from "../lib/pdfHighlight";
import type { FileInfo } from "../types";
import { useT } from "./I18nProvider";

interface PdfViewerProps {
  documentId: number;
  file: Pick<FileInfo, "id" | "name">;
  // The page to open on (1-based) and the passage to mark on it.
  page: number;
  passage: string;
  onClose: () => void;
}

interface Box {
  left: number;
  top: number;
  width: number;
  height: number;
}

const MAX_PAGE_WIDTH = 900;

// Shows one of the user's original PDFs in a pop-up, opened on a cited page with the cited passage
// marked. The file is fetched from the API with the session cookie and drawn by PDF.js.
export default function PdfViewer({ documentId, file, page: openOn, passage, onClose }: PdfViewerProps) {
  const t = useT();
  const [pdf, setPdf] = useState<PDFDocumentProxy | null>(null);
  const [pageNumber, setPageNumber] = useState(openOn);
  const [error, setError] = useState("");
  const [boxes, setBoxes] = useState<Box[]>([]);
  // null until the page is drawn; false when the passage could not be found on it.
  const [located, setLocated] = useState<boolean | null>(null);

  const dialogRef = useRef<HTMLDivElement | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  // Keyboard: Escape closes. Focus moves into the dialog and returns to what opened it.
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    dialogRef.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      opener?.focus?.();
    };
  }, [onClose]);

  // Load the document once.
  useEffect(() => {
    let cancelled = false;
    let loaded: PDFDocumentProxy | null = null;
    (async () => {
      try {
        const pdfjs = await import("pdfjs-dist");
        pdfjs.GlobalWorkerOptions.workerSrc = new URL("pdfjs-dist/build/pdf.worker.min.mjs", import.meta.url).toString();
        const response = await axios.get<ArrayBuffer>(`${API_URL}/documents/${documentId}/files/${file.id}`, {
          responseType: "arraybuffer",
        });
        const doc = await pdfjs.getDocument({ data: new Uint8Array(response.data), useWasm: false }).promise;
        if (cancelled) {
          void doc.loadingTask.destroy();
          return;
        }
        loaded = doc;
        setPdf(doc);
        setPageNumber((n) => Math.min(Math.max(1, n), doc.numPages));
      } catch (err) {
        if (!cancelled) setError(apiError(err, t("pdf.openFailed")));
      }
    })();
    return () => {
      cancelled = true;
      void loaded?.loadingTask.destroy();
    };
  }, [documentId, file.id]);

  // Draw the current page, and mark the passage when it is the cited page.
  useEffect(() => {
    if (!pdf) return;
    let cancelled = false;
    let task: RenderTask | null = null;
    setBoxes([]);
    setLocated(null);

    (async () => {
      try {
        const pdfjs = await import("pdfjs-dist");
        const page = await pdf.getPage(pageNumber);
        const canvas = canvasRef.current;
        if (!canvas || cancelled) return;

        const available = scrollRef.current?.clientWidth ?? MAX_PAGE_WIDTH;
        const width = Math.min(Math.max(available - 24, 280), MAX_PAGE_WIDTH);
        const scale = width / page.getViewport({ scale: 1 }).width;
        const viewport = page.getViewport({ scale });
        // Draw at the screen's pixel density so text stays sharp.
        const density = window.devicePixelRatio || 1;
        canvas.width = Math.floor(viewport.width * density);
        canvas.height = Math.floor(viewport.height * density);
        canvas.style.width = `${viewport.width}px`;
        canvas.style.height = `${viewport.height}px`;

        task = page.render({ canvas, viewport: page.getViewport({ scale: scale * density }) });
        await task.promise;
        if (cancelled) return;

        if (pageNumber !== openOn) {
          setLocated(true);
          return;
        }
        const content = await page.getTextContent();
        const items = content.items.filter((item): item is TextItem => "str" in item);
        const marked = findHighlight(items, passage);
        const found = marked.map((i) => {
          const item = items[i];
          const t = pdfjs.Util.transform(viewport.transform, item.transform);
          const fontHeight = Math.hypot(t[2], t[3]);
          return { left: t[4], top: t[5] - fontHeight, width: item.width * scale, height: fontHeight * 1.2 };
        });
        if (cancelled) return;
        setBoxes(found);
        setLocated(found.length > 0);
        if (found.length > 0) {
          scrollRef.current?.scrollTo({ top: Math.max(0, found[0].top - 120), behavior: "smooth" });
        }
      } catch (err) {
        // A render interrupted by switching pages is not an error.
        if (!cancelled && !(err instanceof Error && err.name === "RenderingCancelledException")) {
          setError(t("pdf.drawFailed"));
        }
      }
    })();

    return () => {
      cancelled = true;
      task?.cancel();
    };
  }, [pdf, pageNumber, openOn, passage]);

  const total = pdf?.numPages ?? 0;
  const go = (delta: number) => setPageNumber((n) => Math.min(Math.max(1, n + delta), Math.max(total, 1)));

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/60 p-3"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={t("pdf.aria", { name: file.name, page: pageNumber })}
        tabIndex={-1}
        data-testid="pdf-viewer"
        className="flex max-h-[94vh] w-full max-w-4xl flex-col rounded-lg border-2 border-ink bg-canvas outline-none"
      >
        <div className="flex flex-wrap items-center justify-between gap-3 border-b-2 border-ink px-4 py-2 text-xl">
          <p className="truncate font-bold" title={file.name}>
            {file.name}
          </p>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => go(-1)}
              disabled={!pdf || pageNumber <= 1}
              aria-label={t("pdf.prev")}
              className="rounded border-2 border-ink px-3 hover:bg-ink hover:text-canvas disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-ink"
            >
              &larr;
            </button>
            <span aria-live="polite" data-testid="pdf-page">
              {pdf ? t("pdf.pageOf", { page: pageNumber, total }) : t("common.loading")}
            </span>
            <button
              type="button"
              onClick={() => go(1)}
              disabled={!pdf || pageNumber >= total}
              aria-label={t("pdf.next")}
              className="rounded border-2 border-ink px-3 hover:bg-ink hover:text-canvas disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-ink"
            >
              &rarr;
            </button>
            <button
              type="button"
              onClick={onClose}
              className="ml-2 rounded border-2 border-ink px-3 uppercase hover:bg-ink hover:text-canvas"
            >
              {t("pdf.close")}
            </button>
          </div>
        </div>

        {pageNumber === openOn && located === false && (
          <p className="border-b border-dashed border-ink/40 px-4 py-1 text-base" role="status">
            {t("pdf.notLocated")}
          </p>
        )}

        <div ref={scrollRef} className="overflow-auto p-3">
          {error ? (
            <p className="text-red-500 text-lg" role="alert">
              {error}
            </p>
          ) : (
            <div className="relative mx-auto w-fit bg-white shadow">
              <canvas ref={canvasRef} data-testid="pdf-canvas" className="block" />
              {boxes.map((b, i) => (
                <div
                  key={i}
                  data-testid="pdf-highlight"
                  className="pointer-events-none absolute bg-yellow-300/50 mix-blend-multiply"
                  style={{ left: b.left, top: b.top, width: b.width, height: b.height }}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
