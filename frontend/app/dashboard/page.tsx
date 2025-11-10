"use client";

import { useEffect, useState } from "react";
import { useAuth } from "../../contexts/AuthContext";
import { useRouter } from "next/navigation";
import axios from "axios";
import type { Document } from "../../types"; 
import SummarizeForm from "../../components/SummarizeForm";

export default function DashboardPage() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const [documents, setDocuments] = useState<Document[]>([]);

  const fetchDocuments = async () => {
    const token = localStorage.getItem("token");
    if (!token) return;

    try {
      const response = await axios.get("http://localhost:8080/documents", {
        headers: { Authorization: `Bearer ${token}` },
      });
      setDocuments(response.data);
    } catch (error) {
      console.error("Failed to fetch documents", error);
    }
  };

  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  useEffect(() => {
    if (user) {
      const loadDocuments = async () => {
        await fetchDocuments();
      };
      loadDocuments();
    }
  }, [user]);

  if (loading) {
    return <p>Loading...</p>;
  }

  return user ? (
    <div>
      <h1 className="text-3xl font-bold my-6">Welcome to your Dashboard</h1>

      <div className="mb-10">
        <h2 className="text-2xl font-semibold mb-4">New Summary</h2>
        <SummarizeForm
          endpoint="http://localhost:8080/summarize"
          onSummaryCreated={fetchDocuments}
        />
      </div>

      <div>
        <h2 className="text-2xl font-semibold mb-4">Your Saved Documents</h2>
        <div className="space-y-4">
          {documents.length > 0 ? (
            documents.map((doc) => (
              <div
                key={doc.ID}
                className="bg-white p-4 rounded-lg shadow-md border"
              >
                <h3 className="font-bold text-lg">{doc.Filename}</h3>
                <p className="text-gray-600 text-sm">
                  Created on: {new Date(doc.CreatedAt).toLocaleDateString()}
                </p>
                <p className="mt-2 text-gray-800">{doc.Summary}</p>
              </div>
            ))
          ) : (
            <p>You have no saved documents yet.</p>
          )}
        </div>
      </div>
    </div>
  ) : null; 
}
