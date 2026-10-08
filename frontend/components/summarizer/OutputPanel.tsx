"use client";
import { useEffect, useRef } from "react";
import Markdown from "markdown-to-jsx";
import { createMarkdownOptions } from "../../lib/markdown";
import { useT } from "../I18nProvider";

const markdownOptions = createMarkdownOptions();

interface OutputPanelProps {
  summary: string;
  isLoading: boolean;
  progress: number;
  stageLabel: string;
}

export default function OutputPanel({ summary, isLoading, progress, stageLabel }: OutputPanelProps) {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const t = useT();

  // Follow the text as it streams in.
  useEffect(() => {
    if (isLoading && summary && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [isLoading, summary]);

  // Before the first words arrive, show progress; after that, show the text as it is written.
  const showProgress = isLoading && !summary;

  return (
    <div className="w-full h-72 p-2 border-2 border-ink rounded-md">
      <div
        ref={scrollRef}
        className="w-full h-full p-4 border border-dashed border-ink/50 rounded-sm overflow-y-auto"
      >
        {showProgress ? (
          <div
            className="flex flex-col items-center justify-center h-full text-center"
            role="status"
            aria-live="polite"
          >
            <p className="text-2xl md:text-3xl text-ink/70 tracking-widest uppercase">{stageLabel}</p>
            <p className="font-bebas text-5xl text-ink font-bold my-4 tracking-wider">
              {Math.round(progress)}%
            </p>
            <div className="w-full max-w-md p-1 border-2 border-ink rounded-md">
              <div className="w-full h-8 border border-dashed border-ink/50 p-1">
                <div
                  className="bg-ink h-full transition-[width] duration-500"
                  style={{ width: `${progress}%` }}
                />
              </div>
            </div>
          </div>
        ) : (
          // The id is how the PDF export finds the rendered summary.
          <div
            id="summary-output-content"
            aria-busy={isLoading}
            className="text-xl md:text-2xl text-ink/70 tracking-wider whitespace-pre-wrap text-left"
          >
            <Markdown options={markdownOptions}>
              {summary || t("form.outputPlaceholder")}
            </Markdown>
            {isLoading && (
              <p className="mt-2 text-base uppercase tracking-widest text-ink/50" role="status">
                {t("form.writing")}
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
