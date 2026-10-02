"use client";

import { useState } from "react";
import axios from "axios";
import Link from "next/link";
import FormContainer from "../../components/FormContainer";
import TurnstileWidget, { turnstileEnabled } from "../../components/Turnstile";
import { ErrorText, Field, PrimaryButton } from "../../components/ui";
import { API_URL, TURNSTILE_HEADER, apiError } from "../../lib/api";

export default function SignupPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [created, setCreated] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [humanToken, setHumanToken] = useState("");
  const [humanReset, setHumanReset] = useState(0);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSubmitting(true);

    try {
      await axios.post(
        `${API_URL}/signup`,
        { email, password },
        { headers: humanToken ? { [TURNSTILE_HEADER]: humanToken } : {} }
      );
      setCreated(true);
    } catch (err) {
      setError(apiError(err, "An unexpected error occurred."));
      // The human-check token is single use, so ask for a new one before the next attempt.
      setHumanToken("");
      setHumanReset((n) => n + 1);
    } finally {
      setSubmitting(false);
    }
  };

  if (created) {
    return (
      <FormContainer title="Check your email">
        <div className="space-y-6 text-2xl text-center">
          <p>
            We sent a confirmation link to <strong>{email}</strong>. Open it to confirm your address.
          </p>
          <p className="text-lg opacity-70">You can already log in while you wait.</p>
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
    <FormContainer title="Sign Up">
      <form
        onSubmit={handleSubmit}
        className="w-full flex flex-col items-center space-y-6 text-2xl"
      >
        <Field
          id="email"
          label="Email"
          type="email"
          value={email}
          onChange={setEmail}
          autoComplete="email"
        />
        <Field
          id="password"
          label="Password"
          type="password"
          value={password}
          onChange={setPassword}
          autoComplete="new-password"
          minLength={8}
          maxLength={72}
          hint="At least 8 characters."
        />

        <TurnstileWidget onToken={setHumanToken} resetKey={humanReset} />
        <ErrorText>{error}</ErrorText>

        <PrimaryButton type="submit" disabled={submitting || (turnstileEnabled && !humanToken)}>
          {submitting ? "Creating..." : "Sign Up"}
        </PrimaryButton>
      </form>
    </FormContainer>
  );
}
