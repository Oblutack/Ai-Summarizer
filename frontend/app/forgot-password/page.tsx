"use client";

import { useState } from "react";
import axios from "axios";
import Link from "next/link";
import FormContainer from "../../components/FormContainer";
import TurnstileWidget, { turnstileEnabled } from "../../components/Turnstile";
import { ErrorText, Field, PrimaryButton, SuccessText } from "../../components/ui";
import { API_URL, TURNSTILE_HEADER, apiError } from "../../lib/api";
import { useT } from "../../components/I18nProvider";

export default function ForgotPasswordPage() {
  const t = useT();
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [humanToken, setHumanToken] = useState("");
  const [humanReset, setHumanReset] = useState(0);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setMessage("");
    setSubmitting(true);

    try {
      // The server answers the same way whether or not the address has an account.
      const response = await axios.post(
        `${API_URL}/auth/forgot-password`,
        { email },
        { headers: humanToken ? { [TURNSTILE_HEADER]: humanToken } : {} }
      );
      setMessage(response.data.message);
    } catch (err) {
      setError(apiError(err, t("common.tryAgain")));
    } finally {
      setHumanToken("");
      setHumanReset((n) => n + 1);
      setSubmitting(false);
    }
  };

  return (
    <FormContainer title={t("forgot.title")}>
      <form
        onSubmit={handleSubmit}
        className="w-full flex flex-col items-center space-y-6 text-2xl"
      >
        <p className="text-lg text-center opacity-80">
          {t("forgot.intro")}
        </p>
        <Field id="email" label={t("common.email")} type="email" value={email} onChange={setEmail} autoComplete="email" />

        <TurnstileWidget onToken={setHumanToken} resetKey={humanReset} />
        <SuccessText>{message}</SuccessText>
        <ErrorText>{error}</ErrorText>

        <PrimaryButton type="submit" disabled={submitting || (turnstileEnabled && !humanToken)}>
          {submitting ? t("forgot.sending") : t("forgot.send")}
        </PrimaryButton>
        <Link href="/login" className="text-lg underline hover:opacity-70">
          {t("forgot.back")}
        </Link>
      </form>
    </FormContainer>
  );
}
