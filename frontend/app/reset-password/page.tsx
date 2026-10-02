"use client";

import { Suspense, useState } from "react";
import axios from "axios";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import FormContainer from "../../components/FormContainer";
import { ErrorText, Field, PrimaryButton } from "../../components/ui";
import { API_URL, apiError } from "../../lib/api";

function ResetPasswordForm() {
  // The token travels in the link's query string; it is sent to the API in a POST body, never logged by us.
  const token = useSearchParams().get("token") ?? "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (password !== confirm) {
      setError("The two passwords don't match.");
      return;
    }
    setSubmitting(true);
    try {
      await axios.post(`${API_URL}/auth/reset-password`, { token, password });
      setDone(true);
    } catch (err) {
      setError(apiError(err, "Something went wrong. Please try again."));
    } finally {
      setSubmitting(false);
    }
  };

  if (!token) {
    return (
      <FormContainer title="Reset Password">
        <div className="space-y-6 text-2xl text-center">
          <p>This link is incomplete. Please use the link from your email, or request a new one.</p>
          <Link href="/forgot-password" className="underline hover:opacity-70">
            Request a new link
          </Link>
        </div>
      </FormContainer>
    );
  }

  if (done) {
    return (
      <FormContainer title="Password changed">
        <div className="space-y-6 text-2xl text-center">
          <p>Your password has been changed and you were signed out everywhere.</p>
          <Link
            href="/login"
            className="inline-block bg-ink text-canvas text-3xl uppercase font-bold py-3 px-12 rounded-md border-2 border-b-8 border-ink hover:opacity-90"
          >
            Log in
          </Link>
        </div>
      </FormContainer>
    );
  }

  return (
    <FormContainer title="Choose a new password">
      <form onSubmit={handleSubmit} className="w-full flex flex-col items-center space-y-6 text-2xl">
        <Field
          id="password"
          label="New password"
          type="password"
          value={password}
          onChange={setPassword}
          autoComplete="new-password"
          minLength={8}
          maxLength={72}
          hint="At least 8 characters."
        />
        <Field
          id="confirm"
          label="Repeat password"
          type="password"
          value={confirm}
          onChange={setConfirm}
          autoComplete="new-password"
          minLength={8}
          maxLength={72}
        />
        <ErrorText>{error}</ErrorText>
        <PrimaryButton type="submit" disabled={submitting}>
          {submitting ? "Saving..." : "Save"}
        </PrimaryButton>
      </form>
    </FormContainer>
  );
}

export default function ResetPasswordPage() {
  // useSearchParams needs a Suspense boundary so the page can still be pre-rendered.
  return (
    <Suspense fallback={null}>
      <ResetPasswordForm />
    </Suspense>
  );
}
