"use client";

import { useState } from "react";
import axios from "axios";

interface SummarizeFormProps {
  onSummaryCreated?: () => void;
  endpoint: string;
}

export default function SummarizeForm({
  onSummaryCreated,
  endpoint,
}: SummarizeFormProps) {
  const [file, setFile] = useState<File | null>(null);
  const [summary, setSummary] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      setFile(e.target.files[0]);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) {
      setError("Please select a file first.");
      return;
    }

    setError("");
    setSummary("");
    setIsLoading(true);

    const formData = new FormData();
    formData.append("file", file);

    try {
      const token = localStorage.getItem("token");

      const response = await axios.post(endpoint, formData, {
        headers: {
          "Content-Type": "multipart/form-data",
          ...(token && { Authorization: `Bearer ${token}` }),
        },
      });

      setSummary(response.data.summary);
      if (onSummaryCreated) {
        onSummaryCreated();
      }
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        if (err.response.status === 401) {
          setError(
            "You must be logged in to save summaries. The summary was not saved."
          );
        } else {
          setError(
            err.response.data?.details ||
              err.response.data?.error ||
              "An error occurred."
          );
        }
      } else {
        setError("An unexpected error occurred.");
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="w-full max-w-2xl mx-auto bg-white p-8 border border-gray-200 rounded-lg shadow-md">
      <form onSubmit={handleSubmit}>
        <div className="mb-4">
          <label
            htmlFor="file"
            className="block text-lg font-medium text-gray-700 mb-2"
          >
            Upload your PDF
          </label>
          <input
            type="file"
            id="file"
            accept=".pdf"
            onChange={handleFileChange}
            className="block w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
          />
        </div>
        <button
          type="submit"
          disabled={isLoading || !file}
          className="w-full bg-blue-500 text-white font-bold py-2 px-4 rounded-md hover:bg-blue-600 disabled:bg-gray-400"
        >
          {isLoading ? "Summarizing..." : "Generate Summary"}
        </button>
      </form>

      {error && <p className="mt-4 text-red-500">{error}</p>}

      {summary && (
        <div className="mt-6 p-4 border border-gray-200 rounded-md bg-gray-50">
          <h3 className="text-xl font-bold mb-2">Summary:</h3>
          <p className="text-gray-700 whitespace-pre-wrap">{summary}</p>
        </div>
      )}
    </div>
  );
}
