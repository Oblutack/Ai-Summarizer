"use client";

import { useEffect, useState } from "react";
import { useAuth } from "../../contexts/AuthContext";
import { useRouter } from "next/navigation";
import axios from "axios";
import type { Document } from "../../types";
import DocumentCard from "../../components/DocumentCard";
import dynamic from "next/dynamic";
import { motion, AnimatePresence } from "framer-motion";

const EInkForm = dynamic(() => import("../../components/EInkForm"), {
  ssr: false,
  loading: () => <p>Loading form...</p>,
});

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1, // Razmak od 0.1s između svake kartice
    },
  },
};

export default function DashboardPage() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const [documents, setDocuments] = useState<Document[]>([]);
  const DocumentCard = dynamic(() => import("../../components/DocumentCard"), {
    ssr: false,
    loading: () => <p>Loading document...</p>, // Opciona poruka
  });

  const fetchDocuments = async () => {
    const token = localStorage.getItem("token");
    if (!token) return;

    try {
      const response = await axios.get("http://localhost:8080/documents", {
        headers: { Authorization: `Bearer ${token}` },
      });
      setDocuments(
        response.data.sort(
          (a: Document, b: Document) =>
            new Date(b.CreatedAt).getTime() - new Date(a.CreatedAt).getTime()
        )
      );
    } catch (error) {
      console.error("Failed to fetch documents", error);
    }
  };
  const handleDeleteDocument = async (id: number) => {
    // Pitamo korisnika da potvrdi
    if (!window.confirm("Are you sure you want to delete this summary?")) {
      return;
    }

    const token = localStorage.getItem("token");
    if (!token) return;

    try {
      await axios.delete(`http://localhost:8080/documents/${id}`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      // Ukloni dokument iz lokalnog stanja za trenutan odziv UI-ja
      setDocuments(documents.filter((doc) => doc.ID !== id));
    } catch (error) {
      console.error("Failed to delete document", error);
      // Opciono: Prikazati grešku korisniku
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
  }, [user]);

  if (loading) {
    return <p className="text-center mt-20 text-2xl">Loading Dashboard...</p>;
  }

  return user ? (
    <div className="max-w-5xl mx-auto mt-8">
      <h1 className="text-4xl uppercase tracking-widest text-center mb-8">
        Your Dashboard
      </h1>

      {/* Sekcija za novu formu */}
      <div className="mb-12 border-2 border-ink rounded-lg p-6">
        <EInkForm
          endpoint="http://localhost:8080/summarize"
          onSummaryCreated={fetchDocuments}
        />
      </div>

      {/* Sekcija za historiju */}
      <div>
        <h2 className="text-3xl uppercase tracking-widest text-center mb-6">
          Saved Summaries
        </h2>
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
      </div>
    </div>
  ) : null;
}
