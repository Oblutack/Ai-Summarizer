"use client";
import { useState } from "react";
import axios from "axios";

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
      )}?wordCount=${wordCount}`;

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

  return (
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
      <div className="w-full min-h-48 p-2 border-2 border-ink rounded-md">
        <div className="w-full h-full border border-dashed border-ink/50 rounded-sm flex justify-center items-center p-4">
          {isLoading ? (
            <p className="text-3xl text-ink/70 tracking-wider">
              Summarizing...
            </p>
          ) : (
            <p className="text-2xl text-ink/70 tracking-wider whitespace-pre-wrap">
              {summary || "SUMMARIZED TEXT OUTPUT APPEARS HERE."}
            </p>
          )}
        </div>
      </div>

      {/* --- Dugme --- */}
      <button
        type="submit"
        disabled={isSubmitDisabled}
        className="bg-ink text-canvas text-3xl uppercase font-bold py-3 px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isLoading ? "WORKING..." : "Summarize"}
      </button>
    </form>
  );
}
