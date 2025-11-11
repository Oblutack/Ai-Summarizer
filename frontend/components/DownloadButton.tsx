"use client";

import { jsPDF } from "jspdf";
import { DownloadIcon } from "./Icon";

interface DownloadButtonProps {
  summaryText: string;
  fileName: string;
}

export default function DownloadButton({
  summaryText,
  fileName,
}: DownloadButtonProps) {
  const handleDownloadPDF = () => {
    if (!summaryText) return;

    const doc = new jsPDF();
    doc.setFont("Helvetica", "normal");
    doc.setFontSize(12);

    const splitText = doc.splitTextToSize(summaryText, 180);
    doc.text(splitText, 15, 20);

    const pdfFileName = fileName.replace(/\.[^/.]+$/, "") + "-summary.pdf";
    doc.save(pdfFileName);
  };

  return (
    <button
      onClick={handleDownloadPDF}
      title="Save as PDF"
      className="text-ink hover:opacity-70"
    >
      <DownloadIcon className="w-8 h-8" />
    </button>
  );
}
