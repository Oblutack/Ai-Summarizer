"use client";
import { useState } from "react";
import axios from "axios";
import { useAuth } from "../contexts/AuthContext";
import { API_URL, apiError } from "../lib/api";

// Shown to signed-in users whose email address isn't confirmed yet.
export default function VerifyBanner() {
  const { user } = useAuth();
  const [message, setMessage] = useState("");
  const [sending, setSending] = useState(false);

  if (!user || user.emailVerified) return null;

  const resend = async () => {
    setSending(true);
    try {
      const response = await axios.post(`${API_URL}/auth/resend-verification`);
      setMessage(response.data.message);
    } catch (err) {
      setMessage(apiError(err, "Could not send the email."));
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="bg-ink text-canvas text-center py-2 px-4 text-lg" role="status">
      {message || "Please confirm your email address. We sent you a link when you signed up."}{" "}
      {!message && (
        <button onClick={resend} disabled={sending} className="underline hover:opacity-80 disabled:opacity-50">
          {sending ? "Sending..." : "Resend email"}
        </button>
      )}
    </div>
  );
}
