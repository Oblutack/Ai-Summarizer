"use client";
import { useState, useEffect } from "react";
import axios from "axios";
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

  useEffect(() => {
    const textWordCount = inputText.trim().split(/\s+/).length;

    // Prikaži opciju ako je uneto više od npr. 1000 reči ILI ako je fajl uploadovan
    // (Pretpostavljamo da su fajlovi uglavnom duži)
    if (textWordCount > 1000 || file) {
      setShowPageLimit(true);
    } else {
      setShowPageLimit(false);
    }
  }, [inputText, file]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      setFile(e.target.files[0]);
      setInputText(""); // Resetujemo tekst ako je fajl izabran
      setError("");
    }
  };

  // Hendler za promenu teksta u textarea
  const handleTextChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInputText(e.target.value);
    setFile(null); // Resetujemo fajl ako korisnik unosi tekst
    setError("");
  };

  // Hendler za slanje forme
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!file && !inputText) {
      setError("Please attach a PDF or paste some text.");
      return;
    }

    setError("");
    setSummary("");
    setIsLoading(true);

    // Logika za slanje fajla
    if (file) {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("wordCount", String(wordCount));
      formData.append("pageLimit", pageLimit);

      try {
        const token = localStorage.getItem("token");
        const response = await axios.post(endpoint, formData, {
          headers: {
            "Content-Type": "multipart/form-data",
            ...(token && { Authorization: `Bearer ${token}` }),
          },
        });
        setSummary(response.data.summary);
        if (onSummaryCreated) onSummaryCreated();
      } catch (err) {
        if (axios.isAxiosError(err) && err.response) {
          setError(
            err.response.data?.details ||
              err.response.data?.error ||
              "An error occurred while summarizing."
          );
        } else {
          setError("An unexpected error occurred.");
        }
      } finally {
        setIsLoading(false);
      }
    }
    // Logika za slanje teksta (privremeno)
    else if (inputText) {
      // Konstruišemo ispravan endpoint za tekst
      const textEndpoint = `${endpoint.replace(
        "summarize",
        "summarize-text"
      )}?wordCount=${wordCount}&pageLimit=${pageLimit}`;

      try {
        const token = localStorage.getItem("token");
        const response = await axios.post(
          textEndpoint,
          { text: inputText }, // Šaljemo JSON
          {
            headers: {
              "Content-Type": "application/json",
              ...(token && { Authorization: `Bearer ${token}` }),
            },
          }
        );

        setSummary(response.data.summary);
        if (onSummaryCreated) onSummaryCreated();
      } catch (err) {
        if (axios.isAxiosError(err) && err.response) {
          setError(
            err.response.data?.details ||
              err.response.data?.error ||
              "An error occurred while summarizing text."
          );
        } else {
          setError("An unexpected error occurred.");
        }
      } finally {
        setIsLoading(false);
      }
    }
  };

  // Provjera da li je dugme za slanje onemogućeno
  const isSubmitDisabled = isLoading || (!file && !inputText);

  const handleDownloadPDF = () => {
    // 1. Pronalazimo element sa JEDINSTVENIM ID-jem.
    const element = document.getElementById("summary-output-content");
    if (!element) {
      console.error("Could not find element to export.");
      return;
    }

    // 2. Kreiramo ime fajla
    const pdfFileName =
      (file ? file.name.replace(/\.[^/.]+$/, "") : "pasted-text") +
      "-summary.pdf";

    // 3. Podešavanja za html2pdf biblioteku
    const opt = {
      margin: [0.5, 0.5, 0.5, 0.5] as [number, number, number, number],
      filename: pdfFileName,
      image: { type: "jpeg" as const, quality: 0.98 },
      html2canvas: {
        scale: 2,
        useCORS: true,
        backgroundColor: "#F5F0E6",
        onclone: (document: Document) => {
          // Skrivamo teksturu iz PDF-a
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

    // 4. Pozivamo biblioteku da generiše PDF
    html2pdf().from(element).set(opt).save();
  };

  return (
    // --- POČETAK PROMENA: Glavni Flex Kontejner ---
    // Ovaj novi div će držati formu levo i page limiter desno
    <div className="flex w-full space-x-8">
      {/* --- LEVA STRANA: VAŠA POSTOJEĆA FORMA --- */}
      {/* Samo smo kopirali vašu postojeću <form> unutar ovog div-a */}
      <div className="flex-grow">
        <form
          onSubmit={handleSubmit}
          className="w-full flex flex-col items-center space-y-6 text-2xl font-bebas"
        >
          {/* --- Slider za broj reči --- */}
          <div className="w-full flex justify-center items-center space-x-4">
            <label htmlFor="word-count" className="uppercase tracking-widest">
              Summary Word Count:
            </label>
            <div className="flex flex-col">
              <input
                id="word-count"
                type="range"
                min="50"
                max="500"
                step="10"
                value={wordCount}
                onChange={(e) => setWordCount(Number(e.target.value))}
                className="w-60"
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
            <span className="w-28 text-center tracking-widest">
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
                        p: { props: { className: "mb-4" } }, // Povećan razmak
                        ul: {
                          props: {
                            className: "list-disc list-inside mb-4 ml-4",
                          },
                        }, // Dodat ml
                        ol: {
                          props: {
                            className: "list-decimal list-inside mb-4 ml-4",
                          },
                        },
                        strong: { props: { className: "text-ink" } }, // Podebljan tekst je tamniji
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
          {summary && !isLoading && (
            <div className="mt-4">
              <button
                type="button"
                onClick={handleDownloadPDF}
                className="bg-canvas text-ink text-xl uppercase font-bold py-2 px-6 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas"
              >
                Save as PDF
              </button>
            </div>
          )}
          {/* --- Dugme --- */}
          <button
            type="submit"
            disabled={isSubmitDisabled}
            className="bg-ink text-canvas text-3xl uppercase font-bold py-3 px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isLoading ? "WORKING..." : "Summarize"}
          </button>
        </form>
      </div>

      {/* --- DESNA STRANA: NOVI BLOK ZA PAGE LIMITER --- */}
      {/* Ovaj div će se prikazati samo ako je showPageLimit === true */}
      {showPageLimit && (
        <div className="w-48 flex-shrink-0 pt-24 text-2xl font-bebas">
          <h3 className="uppercase tracking-widest text-center mb-2">
            Page Limit
          </h3>
          <div className="p-1 border-2 border-ink rounded-md">
            <input
              type="number"
              value={pageLimit}
              onChange={(e) => setPageLimit(e.target.value)}
              placeholder="e.g. 5"
              className="w-full p-2 bg-canvas border-2 border-ink rounded-md focus:outline-none text-center"
            />
          </div>
          <p className="text-center text-base mt-2 text-ink/70">
            (Approx. 250 words per page)
          </p>
        </div>
      )}
    </div> // --- KRAJ GLAVNOG FLEX KONTEJNERA ---
  );
}
