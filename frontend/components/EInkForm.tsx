"use client";
import { motion } from "framer-motion";
import { useSummarizer } from "../hooks/useSummarizer";
import { saveElementAsPdf } from "../lib/pdfExport";
import InputArea from "./summarizer/InputArea";
import OutputPanel from "./summarizer/OutputPanel";
import PageLimitPanel from "./summarizer/PageLimitPanel";
import SummaryOptions from "./summarizer/SummaryOptions";

interface EInkFormProps {
  endpoint: string;
  onSummaryCreated?: () => void;
}

export default function EInkForm({ endpoint, onSummaryCreated }: EInkFormProps) {
  const s = useSummarizer({ endpoint, onSummaryCreated });

  const handleDownloadPDF = () => {
    const element = document.getElementById("summary-output-content");
    if (element) saveElementAsPdf(element, `${s.exportBaseName}-summary.pdf`);
  };

  return (
    <div className="flex w-full flex-col lg:flex-row lg:space-x-8">
      <div className="flex-grow">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            s.submit();
          }}
          className="w-full flex flex-col items-center space-y-6 text-xl md:text-2xl font-bebas"
        >
          <SummaryOptions
            wordCount={s.wordCount}
            onWordCountChange={s.setWordCount}
            wordCountDisabled={s.showPageLimit}
            style={s.style}
            onStyleChange={s.setStyle}
            language={s.language}
            onLanguageChange={s.setLanguage}
          />

          <hr className="w-full border-t-2 border-ink" />

          <InputArea
            files={s.files}
            text={s.inputText}
            onTextChange={s.changeText}
            onFilesPicked={s.addFiles}
            onRemoveFile={s.removeFile}
          />

          {s.error && <p className="text-red-500 text-lg">{s.error}</p>}

          <OutputPanel
            summary={s.summary}
            isLoading={s.isLoading}
            progress={s.progress}
            stageLabel={s.stageLabel}
          />

          {!s.isLoading && (
            <motion.button
              type="submit"
              disabled={!s.canSubmit}
              className="bg-ink text-canvas text-2xl md:text-3xl uppercase font-bold py-2 px-8 md:py-3 md:px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
              whileTap={{ scale: 0.97 }}
              whileHover={{ scale: 1.03 }}
            >
              Summarize
            </motion.button>
          )}
        </form>

        {s.isLoading && (
          <div className="w-full flex justify-center mt-6">
            <button
              type="button"
              onClick={s.cancel}
              className="bg-red-600 text-white text-2xl md:text-3xl uppercase font-bold py-2 px-8 md:py-3 md:px-12 rounded-md hover:bg-red-700"
            >
              Cancel
            </button>
          </div>
        )}

        {s.summary && !s.isLoading && (
          <div className="mt-4 w-full flex justify-center">
            <button
              type="button"
              onClick={handleDownloadPDF}
              className="bg-canvas text-ink text-xl uppercase font-bold py-2 px-6 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas"
            >
              Save as PDF
            </button>
          </div>
        )}
      </div>

      {s.showPageLimit && (
        <PageLimitPanel
          value={s.pageLimit}
          onChange={s.setPageLimit}
          onIncrement={s.incrementPageLimit}
          onDecrement={s.decrementPageLimit}
        />
      )}
    </div>
  );
}
