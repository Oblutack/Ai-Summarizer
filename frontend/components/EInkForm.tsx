"use client";
import { useState, useEffect, useRef } from "react";
import axios, { CancelTokenSource } from "axios";
import html2pdf from "html2pdf.js";
import Markdown from "markdown-to-jsx";

interface EInkFormProps {
  endpoint: string;
  onSummaryCreated?: () => void;
}

export default function EInkForm({
  onSummaryCreated,
  endpoint,
}: EInkFormProps) {
  const [wordCount, setWordCount] = useState(150);
  const [file, setFile] = useState<File | null>(null);
  const [inputText, setInputText] = useState("");
  const [summary, setSummary] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [showPageLimit, setShowPageLimit] = useState(false);
  const [pageLimit, setPageLimit] = useState("");
  const cancelTokenSourceRef = useRef<CancelTokenSource | null>(null);

  useEffect(() => {
    const textWordCount = inputText.trim().split(/\s+/).length;

    if (textWordCount > 1000 || file) {
      setShowPageLimit(true);
    } else {
      setShowPageLimit(false);
    }
  }, [inputText, file]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      setFile(e.target.files[0]);
      setInputText("");
      setError("");
    }
  };

  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInputText(e.target.value);
    setFile(null);
    setError("");
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file && !inputText) {
      setError("Please attach a PDF or paste some text.");
      return;
    }

    console.log("--- handleSubmit START ---");

    setError("");
    setSummary("");
    setIsLoading(true);

    // Kreiramo novi kontroler i čuvamo ga u ref
    cancelTokenSourceRef.current = axios.CancelToken.source();
    console.log(
      "1. CREATED new AbortController:",
      cancelTokenSourceRef.current
    );

    try {
      const token = localStorage.getItem("token");
      const headers: { [key: string]: string } = {};
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }

      let response;
      const config = {
        headers: headers,
        cancelToken: cancelTokenSourceRef.current.token, // Koristimo CancelToken
      };

      if (file) {
        headers["Content-Type"] = "multipart/form-data";
        const formData = new FormData();
        formData.append("file", file);
        formData.append("wordCount", String(wordCount));
        formData.append("pageLimit", pageLimit);
        response = await axios.post(endpoint, formData, config);
      } else if (inputText) {
        headers["Content-Type"] = "application/json";
        const textEndpoint = `${endpoint.replace(
          "summarize",
          "summarize-text"
        )}?wordCount=${wordCount}&pageLimit=${pageLimit}`;
        response = await axios.post(textEndpoint, { text: inputText }, config);
      }

      if (response) {
        setSummary(response.data.summary);
        if (onSummaryCreated) onSummaryCreated();
      }
    } catch (err) {
      if (axios.isCancel(err)) {
        setError("Summarization was cancelled.");
      } else if (axios.isAxiosError(err) && err.response) {
        setError(
          err.response.data?.details ||
            err.response.data?.error ||
            "An error occurred."
        );
      } else {
        setError("An unexpected error occurred.");
      }
    } finally {
      setIsLoading(false);
      cancelTokenSourceRef.current = null;
    }
  };

  const isSubmitDisabled = isLoading || (!file && !inputText);

  const handleDownloadPDF = () => {
    const element = document.getElementById("summary-output-content");
    if (!element) {
      console.error("Could not find element to export.");
      return;
    }

    const pdfFileName =
      (file ? file.name.replace(/\.[^/.]+$/, "") : "pasted-text") +
      "-summary.pdf";

    const opt = {
      margin: [0.5, 0.5, 0.5, 0.5] as [number, number, number, number],
      filename: pdfFileName,
      image: { type: "jpeg" as const, quality: 0.98 },
      html2canvas: {
        scale: 2,
        useCORS: true,
        backgroundColor: "#F5F0E6",
        onclone: (document: Document) => {
          const textureDiv = document.querySelector(
            ".texture-div-for-pdf-export"
          );
          if (textureDiv && textureDiv instanceof HTMLElement) {
            textureDiv.style.display = "none";
          }
        },
      },
      jsPDF: {
        unit: "in" as const,
        format: "letter" as const,
        orientation: "portrait" as const,
      },
    };

    html2pdf().from(element).set(opt).save();
  };
  const incrementPageLimit = () => {
    // Pretvara string u broj, dodaje 1, i vraća nazad u string
    setPageLimit(String(Number(pageLimit || 0) + 1));
  };

  const decrementPageLimit = () => {
    // Smanjuje samo ako je broj veći od 1
    const currentValue = Number(pageLimit || 0);
    if (currentValue > 1) {
      setPageLimit(String(currentValue - 1));
    }
  };

  const handleCancel = () => {
    if (cancelTokenSourceRef.current) {
      cancelTokenSourceRef.current.cancel("Operation canceled by the user.");
    }
  };

  return (
    <div className="flex w-full space-x-8">
      <div className="flex-grow">
        <form
          onSubmit={handleSubmit}
          className="w-full flex flex-col items-center space-y-6 text-2xl font-bebas"
        >
          {/* --- Slider za broj riječi --- */}
          <div className="w-full flex justify-center items-center space-x-4">
            <label
              htmlFor="word-count"
              className={`uppercase tracking-widest ${
                showPageLimit && "opacity-50"
              }`}
            >
              Summary Word Count:
            </label>
            <div className="flex flex-col">
              <input
                id="word-count"
                type="range"
                disabled={showPageLimit}
                min="50"
                max="500"
                step="10"
                value={wordCount}
                onChange={(e) => setWordCount(Number(e.target.value))}
                className="w-60 disabled:opacity-50 disabled:cursor-not-allowed"
              />
              <div className="w-60 flex justify-between px-1 -mt-1 text-ink opacity-40 text-xs">
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
                <span>|</span>
              </div>
            </div>
            <span
              className={`w-28 text-center tracking-widest ${
                showPageLimit && "opacity-50"
              }`}
            >
              {wordCount} Words
            </span>
          </div>

          {/* --- Separator --- */}
          <hr className="w-full border-t-2 border-ink" />

          {/* --- Input Polje --- */}
          <div className="w-full h-56 p-2 border-2 border-ink rounded-md">
            <div className="relative w-full h-full border border-dashed border-ink/50 rounded-sm">
              <textarea
                value={inputText}
                onChange={handleTextChange}
                placeholder="PASTE TEXT OR ATTACH PDF DOCUMENT..."
                className="w-full h-full p-4 bg-transparent focus:outline-none resize-none text-2xl tracking-wider text-center scrollbar-hide ms-overflow-style-none"
              />
              <label
                htmlFor="pdf-upload"
                className="absolute bottom-4 left-4 cursor-pointer flex items-center space-x-3 border-2 border-ink px-4 py-2 rounded-md bg-canvas hover:bg-ink hover:text-canvas"
              >
                <span className="text-2xl">📎</span>
                <span className="text-xl tracking-wider">
                  {file ? file.name : "ATTACH PDF"}
                </span>
              </label>
              <input
                id="pdf-upload"
                type="file"
                className="hidden"
                onChange={handleFileChange}
                accept=".pdf"
              />
            </div>
          </div>

          {/* --- Ispis greške --- */}
          {error && <p className="text-red-500 text-lg">{error}</p>}

          {/* --- Output Polje --- */}
          <div className="w-full h-72 p-2 border-2 border-ink rounded-md">
            {/* Unutrašnji div sada ima ID i skrolovanje */}
            <div className="w-full h-full p-4 border border-dashed border-ink/50 rounded-sm overflow-y-auto">
              {isLoading ? (
                <p className="text-3xl text-ink/70 tracking-wider text-center pt-16">
                  Summarizing...
                </p>
              ) : (
                <div
                  id="summary-output-content"
                  className="text-2xl text-ink/70 tracking-wider whitespace-pre-wrap text-left"
                >
                  <Markdown
                    options={{
                      overrides: {
                        h1: {
                          props: {
                            className:
                              "text-3xl font-bold my-4 break-after-avoid'",
                          },
                        },
                        h2: {
                          props: {
                            className:
                              "text-2xl font-bold my-3 break-after-avoid'",
                          },
                        },
                        p: { props: { className: "mb-4" } },
                        ul: {
                          props: {
                            className: "list-disc list-inside mb-4 ml-4",
                          },
                        },
                        ol: {
                          props: {
                            className: "list-decimal list-inside mb-4 ml-4",
                          },
                        },
                        strong: { props: { className: "text-ink" } },
                      },
                    }}
                  >
                    {summary || "SUMMARIZED TEXT OUTPUT APPEARS HERE."}
                  </Markdown>
                </div>
              )}
            </div>
          </div>
          {/*DUGME ZA EXPORT*/}
          {!isLoading && (
            <button
              type="submit"
              disabled={isSubmitDisabled}
              className="bg-ink text-canvas text-3xl uppercase font-bold py-3 px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Summarize
            </button>
          )}
        </form>
        {isLoading && (
          <div className="w-full flex justify-center mt-6">
            <button
              type="button"
              onClick={handleCancel}
              className="bg-red-600 text-white text-3xl uppercase font-bold py-3 px-12 rounded-md hover:bg-red-700"
            >
              Cancel
            </button>
          </div>
        )}

        {summary && !isLoading && (
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

      {/* --- DESNA STRANA: NOVI BLOK ZA PAGE LIMITER --- */}
      {/* Ovaj div će se prikazati samo ako je showPageLimit === true */}
      {showPageLimit && (
        <div className="w-48 flex-shrink-0 pt-24 text-2xl font-bebas">
          <h3 className="uppercase tracking-widest text-center mb-2">
            Page Limit
          </h3>
          <div className="relative p-1 border-2 border-ink rounded-md">
            <div className="border border-dashed border-ink/50 rounded-sm">
              <input
                type="number"
                value={pageLimit}
                onChange={(e) => setPageLimit(e.target.value)}
                placeholder="e.g. 5"
                className="w-full p-2 bg-transparent focus:outline-none text-center"
              />
            </div>
            {/* --- DUGMIĆI ZA KONTROLU --- */}
            {/* Kontejner za dugmiće, pozicioniran apsolutno sa desne strane */}
            <div className="absolute right-2 top-1/2 -translate-y-1/2 flex flex-col space-y-1">
              {/* Dugme GORE */}
              <button
                type="button"
                onClick={incrementPageLimit}
                className="text-ink h-4 w-4 flex items-center justify-center hover:opacity-70"
              >
                {/* SVG trougao koji gleda gore */}
                <svg viewBox="0 0 10 10" className="w-full h-full fill-current">
                  <polygon points="5 2, 8 8, 2 8" />
                </svg>
              </button>
              {/* Dugme DOLE */}
              <button
                type="button"
                onClick={decrementPageLimit}
                className="text-ink h-4 w-4 flex items-center justify-center hover:opacity-70"
              >
                {/* SVG trougao koji gleda dole */}
                <svg viewBox="0 0 10 10" className="w-full h-full fill-current">
                  <polygon points="5 8, 2 2, 8 2" />
                </svg>
              </button>
            </div>
          </div>
          <p className="text-center text-base mt-2 text-ink/70">
            (Approx. 250 words per page)
          </p>
        </div>
      )}
    </div>
  );
}
