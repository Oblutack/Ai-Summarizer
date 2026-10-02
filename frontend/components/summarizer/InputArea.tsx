"use client";
import { MAX_FILES } from "../../lib/summaryOptions";

interface InputAreaProps {
  files: File[];
  text: string;
  onTextChange: (value: string) => void;
  onFilesPicked: (files: File[]) => void;
  onRemoveFile: (index: number) => void;
}

const attachButtonClass =
  "cursor-pointer flex items-center space-x-3 border-2 border-ink px-4 py-2 rounded-md bg-canvas hover:bg-ink hover:text-canvas";

function AttachLabel({ className = "" }: { className?: string }) {
  return (
    <label htmlFor="pdf-upload" className={`${attachButtonClass} ${className}`}>
      <span className="text-2xl">📎</span>
      <span className="text-xl tracking-wider">ATTACH PDFS</span>
    </label>
  );
}

export default function InputArea({
  files,
  text,
  onTextChange,
  onFilesPicked,
  onRemoveFile,
}: InputAreaProps) {
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const picked = Array.from(e.target.files ?? []);
    e.target.value = ""; // allow picking the same file again after removing it
    onFilesPicked(picked);
  };

  return (
    <div className="w-full h-56 p-2 border-2 border-ink rounded-md">
      <div className="relative w-full h-full border border-dashed border-ink/50 rounded-sm p-4">
        {files.length > 0 ? (
          // Auto margins on the first/last child center the list when it fits, but unlike
          // justify-center they don't clip the top rows when it overflows and scrolls.
          <div className="flex flex-col items-center h-full w-full gap-3 overflow-y-auto [&>:first-child]:mt-auto [&>:last-child]:mb-auto">
            {files.map((f, i) => (
              <div
                key={`${f.name}-${f.size}`}
                className="flex items-center justify-between space-x-4 border-2 border-dashed border-ink/50 px-3 py-1 rounded-md w-auto max-w-full"
              >
                <p className="text-lg md:text-xl tracking-wider text-center truncate">{f.name}</p>
                <button
                  type="button"
                  onClick={() => onRemoveFile(i)}
                  className="text-ink/50 hover:text-red-600 text-3xl leading-none flex-shrink-0"
                  title="Remove file"
                  aria-label={`Remove ${f.name}`}
                >
                  &times;
                </button>
              </div>
            ))}
            {files.length < MAX_FILES && (
              <label
                htmlFor="pdf-upload"
                className="cursor-pointer border-2 border-ink px-4 py-1 rounded-md bg-canvas hover:bg-ink hover:text-canvas text-xl tracking-wider"
              >
                + ADD MORE PDFS
              </label>
            )}
          </div>
        ) : (
          <>
            {/* One textarea stays mounted for the whole typing session, so focus and the first
                keystroke are never lost to a swap between elements. */}
            <textarea
              id="main-textarea"
              value={text}
              onChange={(e) => onTextChange(e.target.value)}
              aria-label="Text to summarize"
              className="w-full h-full pb-12 bg-transparent focus:outline-none resize-none text-xl tracking-wider text-left scrollbar-hide ms-overflow-style-none"
            />
            {text ? (
              <AttachLabel className="absolute bottom-4 left-4" />
            ) : (
              <div className="absolute inset-0 flex flex-col justify-center items-center space-y-4 pointer-events-none">
                <p className="text-3xl text-center tracking-wider text-ink/50 md:text-2xl">
                  PASTE TEXT OR ATTACH PDF DOCUMENTS...
                </p>
                <AttachLabel className="pointer-events-auto" />
              </div>
            )}
          </>
        )}

        {/* Always mounted so both the empty-state and the file-list buttons can open it. */}
        <input
          id="pdf-upload"
          type="file"
          className="hidden"
          onChange={handleFileChange}
          accept=".pdf"
          multiple
        />
      </div>
    </div>
  );
}
