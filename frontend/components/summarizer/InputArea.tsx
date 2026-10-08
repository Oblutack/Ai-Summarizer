"use client";
import { SAMPLE_TEXT } from "../../lib/sampleText";
import { DOCUMENT_EXTENSIONS } from "../../lib/links";
import { MAX_FILES } from "../../lib/summaryOptions";
import { useT } from "../I18nProvider";

interface InputAreaProps {
  files: File[];
  text: string;
  // Set when the text box holds just a web address.
  link?: string | null;
  onTextChange: (value: string) => void;
  onFilesPicked: (files: File[]) => void;
  onRemoveFile: (index: number) => void;
}

function PaperclipIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21.4 11.1l-9.2 9.2a6 6 0 0 1-8.5-8.5l9.2-9.2a4 4 0 0 1 5.7 5.7l-9.2 9.2a2 2 0 0 1-2.8-2.8l8.5-8.5" />
    </svg>
  );
}

// The place to put the words: a text box that also takes PDFs. Once PDFs are attached they replace the
// text box with a list, since a summary comes from one or the other.
export default function InputArea({ files, text, link, onTextChange, onFilesPicked, onRemoveFile }: InputAreaProps) {
  const t = useT();

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const picked = Array.from(e.target.files ?? []);
    e.target.value = ""; // allow picking the same file again after removing it
    onFilesPicked(picked);
  };

  return (
    <div className="rounded-xl border border-ink/40 bg-surface focus-within:border-accent focus-within:ring-2 focus-within:ring-accent/30">
      {files.length > 0 ? (
        <ul className="space-y-2 p-3">
          {files.map((f, i) => (
            <li
              key={`${f.name}-${f.size}`}
              className="flex items-center justify-between gap-3 rounded-lg border border-ink/20 bg-canvas/60 px-3 py-2"
            >
              <span className="truncate text-base font-medium" title={f.name}>
                {f.name}
              </span>
              <button
                type="button"
                onClick={() => onRemoveFile(i)}
                className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md text-2xl leading-none text-ink/70 hover:bg-ink/10 hover:text-danger"
                title={t("form.removeFile")}
                aria-label={t("form.removeFileNamed", { name: f.name })}
              >
                &times;
              </button>
            </li>
          ))}
        </ul>
      ) : (
        // One textarea stays mounted for the whole typing session, so focus and the first keystroke are
        // never lost to a swap between elements. The frame around it shows the focus.
        <textarea
          id="main-textarea"
          value={text}
          onChange={(e) => onTextChange(e.target.value)}
          aria-label={t("form.textAria")}
          placeholder={t("form.dropHint")}
          className="block min-h-[11rem] w-full resize-y rounded-t-xl bg-transparent px-4 py-3 text-base leading-relaxed focus:outline-none"
        />
      )}

      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-ink/15 px-2 py-1.5">
        {(files.length === 0 || files.length < MAX_FILES) && (
          <label htmlFor="pdf-upload" className="btn btn-quiet cursor-pointer gap-2">
            <PaperclipIcon />
            {files.length === 0 ? t("form.attach") : t("form.addMore")}
          </label>
        )}
        {files.length === 0 && !text && (
          <button type="button" onClick={() => onTextChange(SAMPLE_TEXT)} className="btn btn-quiet underline">
            {t("form.trySample")}
          </button>
        )}
        {link && (
          <p className="px-2 text-sm font-medium text-accent" role="status" data-testid="link-detected">
            {t("form.linkDetected")}
          </p>
        )}
      </div>

      {/* Always mounted so both the empty-state and the file-list buttons can open it. */}
      <input id="pdf-upload" type="file" className="sr-only" onChange={handleFileChange} accept={DOCUMENT_EXTENSIONS.join(",")} multiple tabIndex={-1} />
    </div>
  );
}
