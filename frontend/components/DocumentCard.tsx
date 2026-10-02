"use client";
import Markdown from "markdown-to-jsx";
import { motion } from "framer-motion";
import { useState } from "react";
import type { Document } from "../types";
import { createMarkdownOptions } from "../lib/markdown";
import { saveElementAsPdf } from "../lib/pdfExport";
import DocumentChat from "./DocumentChat";
import { DownloadIcon, TrashIcon } from "./Icon";

interface DocumentCardProps {
  doc: Document;
  onDelete: (id: number) => void;
}

// Saved cards use larger body text than the live form.
const markdownOptions = createMarkdownOptions("text-2xl");

export default function DocumentCard({ doc, onDelete }: DocumentCardProps) {
  const [chatOpen, setChatOpen] = useState(false);

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

      <div className="max-h-48 overflow-y-auto">
        {/* The id is how the PDF export finds this card's content. */}
        <div id={`doc-content-${doc.ID}`}>
          <hr className="border-t border-dashed border-ink/50 my-3" />
          <Markdown options={markdownOptions}>{doc.Summary}</Markdown>
        </div>
      </div>

      {doc.hasContent && (
        <div className="mt-4">
          <button
            type="button"
            onClick={() => setChatOpen((open) => !open)}
            className="bg-canvas text-ink text-xl uppercase font-bold py-1 px-5 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas"
          >
            {chatOpen ? "Close Chat" : "Chat With Document"}
          </button>
          {chatOpen && <DocumentChat documentId={doc.ID} />}
        </div>
      )}
    </motion.div>
  );
}
