"use client";
import { useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import { useT } from "./I18nProvider";

const MAX_TAGS = 8;
const MAX_TAG_CHARS = 30;

interface TagEditorProps {
  documentId: number;
  tags: string[];
  onChange: (tags: string[]) => void;
}

// A document's tags as removable chips, with a box to add one. Each change is saved at once.
export default function TagEditor({ documentId, tags, onChange }: TagEditorProps) {
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const t = useT();

  const save = async (next: string[]) => {
    setSaving(true);
    setError("");
    try {
      const response = await axios.put<{ tags: string[] }>(`${API_URL}/documents/${documentId}`, { tags: next });
      onChange(response.data.tags);
      return true;
    } catch (err) {
      setError(apiError(err, t("tags.failed")));
      return false;
    } finally {
      setSaving(false);
    }
  };

  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    const tag = draft.trim().toLowerCase();
    if (!tag || tags.includes(tag)) {
      setDraft("");
      return;
    }
    if (await save([...tags, tag])) setDraft("");
  };

  return (
    <div className="mt-2 flex flex-wrap items-center gap-2" data-testid="tags">
      {tags.map((tag) => (
        <span key={tag} className="inline-flex items-center gap-1 rounded-full border border-ink/60 px-3 text-base tracking-wider">
          {tag}
          <button
            type="button"
            onClick={() => save(tags.filter((other) => other !== tag))}
            disabled={saving}
            className="text-ink/60 hover:text-red-600 leading-none text-xl"
            aria-label={t("tags.remove", { tag })}
          >
            &times;
          </button>
        </span>
      ))}
      {tags.length < MAX_TAGS && (
        <form onSubmit={add}>
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            maxLength={MAX_TAG_CHARS}
            placeholder={t("tags.placeholder")}
            aria-label={t("tags.add")}
            className="w-24 bg-transparent border-b border-dashed border-ink/60 text-base tracking-wider focus:outline-none focus:border-ink"
          />
        </form>
      )}
      {error && (
        <span className="text-red-500 text-base" role="alert">
          {error}
        </span>
      )}
    </div>
  );
}
