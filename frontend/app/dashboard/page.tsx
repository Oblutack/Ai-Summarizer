"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "../../contexts/AuthContext";
import { useRouter } from "next/navigation";
import axios from "axios";
import type { Document } from "../../types";
import dynamic from "next/dynamic";
import { AnimatePresence } from "framer-motion";

const EInkForm = dynamic(() => import("../../components/EInkForm"), {
  ssr: false,
  loading: () => <p>Loading form...</p>,
});

// Declared at module scope: creating this inside the component would give React a new component
// type on every render, remounting every card (and discarding an open chat) each time.
const DocumentCard = dynamic(() => import("../../components/DocumentCard"), {
  ssr: false,
  loading: () => <p>Loading document...</p>,
});

const PAGE_SIZE = 20;

export default function DashboardPage() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const [documents, setDocuments] = useState<Document[]>([]);
  // Cursor for the next (older) page; empty when everything is loaded.
  const [nextCursor, setNextCursor] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);
  const [loadError, setLoadError] = useState("");

  const requestPage = useCallback(async (before?: string) => {
    const token = localStorage.getItem("token");
    if (!token) return null;

    const query = new URLSearchParams({ limit: String(PAGE_SIZE) });
    if (before) query.set("before", before);
    const response = await axios.get<Document[]>(
      `${process.env.NEXT_PUBLIC_API_URL}/documents?${query.toString()}`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    return { docs: response.data, next: (response.headers["x-next-cursor"] as string) || "" };
  }, []);

  // Loads (or refreshes) the newest page. Older pages the user already loaded are kept.
  const fetchDocuments = useCallback(async () => {
    try {
      const page = await requestPage();
      if (!page) return;
      setLoadError("");
      setDocuments((prev) => {
        const oldest = page.docs.length > 0 ? page.docs[page.docs.length - 1].ID : 0;
        const older = prev.filter((d) => d.ID < oldest);
        if (older.length === 0) setNextCursor(page.next);
        return [...page.docs, ...older];
      });
    } catch (error) {
      console.error("Failed to fetch documents", error);
      setLoadError("Could not load your documents.");
    }
  }, [requestPage]);

  const loadMore = async () => {
    if (!nextCursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const page = await requestPage(nextCursor);
      if (!page) return;
      setLoadError("");
      setDocuments((prev) => [...prev, ...page.docs.filter((d) => !prev.some((p) => p.ID === d.ID))]);
      setNextCursor(page.next);
    } catch (error) {
      console.error("Failed to load more documents", error);
      setLoadError("Could not load more documents.");
    } finally {
      setLoadingMore(false);
    }
  };

  const handleDeleteDocument = async (id: number) => {
    // Ask the user to confirm
    if (!window.confirm("Are you sure you want to delete this summary?")) {
      return;
    }

    const token = localStorage.getItem("token");
    if (!token) return;

    try {
      await axios.delete(`${process.env.NEXT_PUBLIC_API_URL}/documents/${id}`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      // Remove the document locally so the UI responds immediately
      setDocuments((prev) => prev.filter((doc) => doc.ID !== id));
    } catch (error) {
      console.error("Failed to delete document", error);
      setLoadError("Could not delete that document. Please try again.");
    }
  };

  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  useEffect(() => {
    if (user) {
      fetchDocuments();
    }
  }, [user, fetchDocuments]);

  if (loading) {
    return <p className="text-center mt-20 text-2xl">Loading Dashboard...</p>;
  }

  return user ? (
    <div className="max-w-5xl mx-auto mt-8">
      <h1 className="text-4xl uppercase tracking-widest text-center mb-8">
        Your Dashboard
      </h1>

      {/* New summary form */}
      <div className="mb-12 border-2 border-ink rounded-lg p-6">
        <EInkForm
          endpoint={`${process.env.NEXT_PUBLIC_API_URL}/summarize`}
          onSummaryCreated={fetchDocuments}
        />
      </div>

      {/* Saved summaries */}
      <div>
        <h2 className="text-3xl uppercase tracking-widest text-center mb-6">
          Saved Summaries
        </h2>
        {loadError && (
          <p className="text-center text-red-500 text-lg mb-4" role="alert">
            {loadError}
          </p>
        )}
        <div className="space-y-6">
          <AnimatePresence>
            {documents.length > 0 ? (
              documents.map((doc) => (
                <DocumentCard
                  key={doc.ID}
                  doc={doc}
                  onDelete={handleDeleteDocument}
                />
              ))
            ) : (
              <p className="text-center text-xl text-ink/60">
                You have no saved documents yet.
              </p>
            )}
          </AnimatePresence>
        </div>

        {nextCursor && (
          <div className="mt-8 flex justify-center">
            <button
              type="button"
              onClick={loadMore}
              disabled={loadingMore}
              className="bg-canvas text-ink text-2xl uppercase font-bold py-2 px-8 rounded-md border-2 border-ink hover:bg-ink hover:text-canvas disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loadingMore ? "Loading..." : "Load more"}
            </button>
          </div>
        )}
      </div>
    </div>
  ) : null;
}
