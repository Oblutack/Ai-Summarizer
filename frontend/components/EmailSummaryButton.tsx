"use client";
import { useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import { useT } from "./I18nProvider";
import { textButton } from "./styles";

// Sends a saved summary to the signed-in person's own email address.
export default function EmailSummaryButton({ documentId }: { documentId: number }) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [failed, setFailed] = useState(false);
  const t = useT();

  const send = async () => {
    setBusy(true);
    setMessage("");
    try {
      const response = await axios.post<{ message: string }>(`${API_URL}/documents/${documentId}/email`);
      setFailed(false);
      setMessage(response.data.message);
    } catch (err) {
      setFailed(true);
      setMessage(apiError(err, t("verify.sendFailed")));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <button type="button" onClick={send} disabled={busy} className={textButton}>
        {busy ? t("verify.sending") : t("email.send")}
      </button>
      {message && (
        <span className={failed ? "text-red-500 text-base" : "text-base text-ink/70"} role={failed ? "alert" : "status"}>
          {message}
        </span>
      )}
    </>
  );
}
