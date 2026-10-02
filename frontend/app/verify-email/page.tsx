"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import axios from "axios";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import FormContainer from "../../components/FormContainer";
import { useAuth } from "../../contexts/AuthContext";
import { API_URL, apiError } from "../../lib/api";

type Status = "checking" | "ok" | "failed";

function VerifyEmail() {
  const token = useSearchParams().get("token") ?? "";
  const { refresh } = useAuth();
  const [status, setStatus] = useState<Status>("checking");
  const [message, setMessage] = useState("");
  // React strict mode runs effects twice in development; the link is single-use, so send it once.
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;

    if (!token) {
      setStatus("failed");
      setMessage("This link is incomplete. Please use the link from your email.");
      return;
    }
    // A POST, not a GET: mail scanners that open every link in an email must not use the token up.
    axios
      .post(`${API_URL}/auth/verify-email`, { token })
      .then(async (response) => {
        setMessage(response.data.message);
        setStatus("ok");
        await refresh(); // update the "please confirm your email" banner if they're signed in
      })
      .catch((err) => {
        setMessage(apiError(err, "This confirmation link is invalid or has expired."));
        setStatus("failed");
      });
  }, [token, refresh]);

  return (
    <FormContainer title={status === "ok" ? "Email confirmed" : "Confirm your email"}>
      <div className="space-y-6 text-2xl text-center">
        {status === "checking" && <p>Confirming...</p>}
        {status !== "checking" && <p role={status === "failed" ? "alert" : "status"}>{message}</p>}
        {status === "failed" && (
          <p className="text-lg opacity-80">
            Log in and use &quot;Resend confirmation&quot; on your account page to get a new link.
          </p>
        )}
        <Link
          href={status === "ok" ? "/dashboard" : "/login"}
          className="inline-block bg-ink text-canvas text-3xl uppercase font-bold py-3 px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90"
        >
          {status === "ok" ? "Continue" : "Log in"}
        </Link>
      </div>
    </FormContainer>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={null}>
      <VerifyEmail />
    </Suspense>
  );
}
