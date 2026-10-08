"use client";
import Markdown from "markdown-to-jsx";
import { motion } from "framer-motion";
import { useState } from "react";
import axios from "axios";
import dynamic from "next/dynamic";
import type { Document, FileInfo, ProofPassage, ProofResult } from "../types";
import { API_URL, apiError } from "../lib/api";
import { createMarkdownOptions } from "../lib/markdown";
import { saveElementAsPdf } from "../lib/pdfExport";
import CopyButton from "./CopyButton";
import DocumentChat from "./DocumentChat";
import PodcastPlayer from "./PodcastPlayer";
import ProofView from "./ProofView";
import { DownloadIcon, TrashIcon } from "./Icon";

interface DocumentCardProps {
  doc: Document;
  onDelete: (id: number) => void;
}

// The PDF viewer is large; it is only fetched when someone opens a page.
const PdfViewer = dynamic(() => import("./PdfViewer"), { ssr: false });

// Saved cards use larger body text than the live form.
const markdownOptions = createMarkdownOptions("text-2xl");

export default function DocumentCard({ doc, onDelete }: DocumentCardProps) {
  const [chatOpen, setChatOpen] = useState(false);
  const [podcastOpen, setPodcastOpen] = useState(false);
  // The proof check: each sentence of the summary compared with the original document.
  const [proof, setProof] = useState<ProofResult | null>(null);
  const [proofOpen, setProofOpen] = useState(false);
  const [proofLoading, setProofLoading] = useState(false);
  const [proofError, setProofError] = useState("");
  const [viewing, setViewing] = useState<{ file: FileInfo; page: number; passage: string } | null>(null);

  const toggleProof = async () => {
    if (proofOpen) {
      setProofOpen(false);
      return;
    }
    setProofError("");
    if (!proof) {
      setProofLoading(true);
      try {
        const response = await axios.post<ProofResult>(`${API_URL}/documents/${doc.ID}/proof`);
        setProof(response.data);
      } catch (err) {
        setProofError(apiError(err, "Could not check this summary."));
        return;
      } finally {
        setProofLoading(false);
      }
    }
    setProofOpen(true);
  };

  const handleDownloadPDF = () => {
    const element = document.getElementById(`doc-content-${doc.ID}`);
    if (!element) return;
    saveElementAsPdf(element, doc.Filename.replace(/\.[^/.]+$/, "") + "-summary.pdf");
  };

  return (
    <motion.div
      layout // animate position changes when cards are added or removed
      initial={{ opacity: 0, y: 50, scale: 0.8 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, scale: 0.5, transition: { duration: 0.2 } }}
      className="bg-canvas border-2 border-ink p-6 rounded-md"
    >
      <div className="flex justify-between items-start">
        <h3 className="flex-grow font-bold text-2xl tracking-wider mr-4">{doc.Filename}</h3>

        <div className="flex-shrink-0 flex items-center space-x-4">
          <CopyButton variant="icon" what={`${doc.Filename} summary`} text={doc.Summary} />
          <button
            onClick={handleDownloadPDF}
            title="Save as PDF"
            aria-label={`Save ${doc.Filename} as PDF`}
            className="text-ink hover:opacity-70"
          >
            <DownloadIcon className="w-8 h-8" />
          </button>

          <button
            onClick={() => onDelete(doc.ID)}
            title="Delete Summary"
            aria-label={`Delete ${doc.Filename}`}
            className="text-red-600 hover:opacity-70"
          >
            <TrashIcon className="w-7 h-7" />
          </button>
        </div>
      </div>

      <p className="text-ink/70 text-lg">Created on: {new Date(doc.CreatedAt).toLocaleDateString()}</p>

      <div className={proofOpen && proof ? "" : "max-h-48 overflow-y-auto"}>
        {/* The id is how the PDF export finds this card's content. */}
        <div id={`doc-content-${doc.ID}`}>
          <hr className="border-t border-dashed border-ink/50 my-3" />
          {proofOpen && proof ? (
            <ProofView
              result={proof}
              files={doc.files}
              onOpenDocument={(file: FileInfo, p: ProofPassage) =>
                setViewing({ file, page: p.page ?? 1, passage: p.text })
              }
            />
          ) : (
            <Markdown options={markdownOptions}>{doc.Summary}</Markdown>
          )}
        </div>
      </div>

      {proofError && (
        <p className="mt-2 text-red-500 text-lg" role="alert">
          {proofError}
        </p>
      )}

      {doc.hasContent && (
        <div className="mt-4">
          <button
            type="button"
            onClick={toggleProof}
            disabled={proofLoading}
            className="mr-3 bg-canvas text-ink text-xl uppercase font-bold py-1 px-5 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas disabled:opacity-50"
          >
            {proofLoading ? "Checking..." : proofOpen ? "Back to summary" : "Check against the original"}
          </button>
          <button
            type="button"
            onClick={() => setPodcastOpen((open) => !open)}
            className="mr-3 bg-canvas text-ink text-xl uppercase font-bold py-1 px-5 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas"
          >
            {podcastOpen ? "Close podcast" : "Listen as a podcast"}
          </button>
          <button
            type="button"
            onClick={() => setChatOpen((open) => !open)}
            className="bg-canvas text-ink text-xl uppercase font-bold py-1 px-5 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas"
          >
            {chatOpen ? "Close Chat" : "Chat With Document"}
          </button>
          {podcastOpen && <PodcastPlayer documentId={doc.ID} />}
          {chatOpen && <DocumentChat documentId={doc.ID} files={doc.files} />}
        </div>
      )}

      {viewing && (
        <PdfViewer
          documentId={doc.ID}
          file={viewing.file}
          page={viewing.page}
          passage={viewing.passage}
          onClose={() => setViewing(null)}
        />
      )}
    </motion.div>
  );
}
