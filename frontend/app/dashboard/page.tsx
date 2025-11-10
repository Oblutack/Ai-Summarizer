"use client";

import { useEffect, useState } from 'react';
import { useAuth } from '../../contexts/AuthContext';
import { useRouter } from 'next/navigation';
import axios from 'axios';
import type { Document } from '../../types';
import EInkForm from '../../components/EInkForm'; 

export default function DashboardPage() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const [documents, setDocuments] = useState<Document[]>([]);

  const fetchDocuments = async () => {
    const token = localStorage.getItem('token');
    if (!token) return;

    try {
      const response = await axios.get('http://localhost:8080/documents', {
        headers: { Authorization: `Bearer ${token}` },
      });
      setDocuments(response.data.sort((a: Document, b: Document) => new Date(b.CreatedAt).getTime() - new Date(a.CreatedAt).getTime()));
    } catch (error) {
      console.error('Failed to fetch documents', error);
    }
  };

  useEffect(() => {
    if (!loading && !user) {
      router.push('/login');
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

      {/* Sekcija za istoriju */}
      <div>
        <h2 className="text-3xl uppercase tracking-widest text-center mb-6">
          Saved Summaries
        </h2>
        <div className="space-y-6">
          {documents.length > 0 ? (
            documents.map((doc) => (
              <div key={doc.ID} className="bg-canvas border-2 border-ink p-6 rounded-md">
                <h3 className="font-bold text-2xl tracking-wider">{doc.Filename}</h3>
                <p className="text-ink/70 text-lg">
                  Created on: {new Date(doc.CreatedAt).toLocaleDateString()}
                </p>
                <hr className="border-t border-dashed border-ink/50 my-3" />
                <p className="mt-2 text-xl whitespace-pre-wrap">{doc.Summary}</p>
              </div>
            ))
          ) : (
            <p className="text-center text-xl text-ink/60">You have no saved documents yet.</p>
          )}
        </div>
      </div>
    </div>
  ) : null;
}