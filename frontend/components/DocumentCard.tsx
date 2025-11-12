"use client";
import type { Document } from "../types";
import { DownloadIcon, TrashIcon } from "./Icon";
import html2pdf from "html2pdf.js";
import Markdown from "markdown-to-jsx";

interface DocumentCardProps {
  doc: Document;
  onDelete: (id: number) => void;
}

export default function DocumentCard({ doc, onDelete }: DocumentCardProps) {
  const handleDownloadPDF = () => {
    const element = document.getElementById(`doc-content-${doc.ID}`);
    if (!element) return;

    const pdfFileName = doc.Filename.replace(/\.[^/.]+$/, "") + "-summary.pdf";

    const opt = {
      margin: [0.5, 0.5, 0.5, 0.5] as [number, number, number, number],
      filename: pdfFileName,
      image: { type: "jpeg" as const, quality: 0.98 },
      html2canvas: { scale: 2, useCORS: true, backgroundColor: "#F5F0E6" },
      jsPDF: {
        unit: "in" as const,
        format: "letter" as const,
        orientation: "portrait" as const,
      },
    };

    html2pdf().from(element).set(opt).save();
  };

  return (
    <div className="bg-canvas border-2 border-ink p-6 rounded-md">
      <div className="flex justify-between items-start">
        <h3 className="flex-grow font-bold text-2xl tracking-wider mr-4">
          {doc.Filename}
        </h3>

        {/* --- NOVI KONTEJNER ZA DUGMAD --- */}
        <div className="flex-shrink-0 flex items-center space-x-4">
          <button
            onClick={handleDownloadPDF}
            title="Save as PDF"
            className="text-ink hover:opacity-70"
          >
            <DownloadIcon className="w-8 h-8" />
          </button>

          <button
            onClick={() => onDelete(doc.ID)}
            title="Delete Summary"
            className="text-red-600 hover:opacity-70"
          >
            <TrashIcon className="w-7 h-7" />
          </button>
        </div>
        {/* --- KRAJ NOVOG KONTEJNERA --- */}
      </div>

      <p className="text-ink/70 text-lg">
        Created on: {new Date(doc.CreatedAt).toLocaleDateString()}
      </p>

      <div className="max-h-48 overflow-y-auto">
        {/* Kontejner sa jedinstvenim ID-jem */}
        <div id={`doc-content-${doc.ID}`}>
          <hr className="border-t border-dashed border-ink/50 my-3" />
          <Markdown
            options={{
              overrides: {
                h1: { props: { className: "text-3xl font-bold my-4" } },
                h2: { props: { className: "text-2xl font-bold my-3" } },

                p: { props: { className: "mb-4 text-2xl" } },
                ul: {
                  props: {
                    className: "list-disc list-inside mb-4 ml-4 text-2xl",
                  },
                },
                ol: {
                  props: {
                    className: "list-decimal list-inside mb-4 ml-4 text-2xl",
                  },
                },

                strong: { props: { className: "text-ink" } },
              },
            }}
          >
            {doc.Summary}
          </Markdown>
        </div>
      </div>
    </div>
  );
}
