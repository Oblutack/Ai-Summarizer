"use client";

import { useCallback, useEffect, useState } from "react";
import axios from "axios";
import { useRouter } from "next/navigation";
import { useAuth } from "../../contexts/AuthContext";
import { DangerButton, ErrorText, Field, SecondaryButton, SuccessText } from "../../components/ui";
import { API_URL, apiError } from "../../lib/api";
import type { SessionInfo, Usage } from "../../types";

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="border-2 border-ink rounded-lg p-6">
      <h2 className="text-3xl uppercase tracking-widest mb-4">{title}</h2>
      <div className="space-y-4 text-xl">{children}</div>
    </section>
  );
}

// A friendly device name from a User-Agent string; we only keep the broad family.
function deviceName(userAgent: string): string {
  const ua = userAgent.toLowerCase();
  const browser = ua.includes("edg/")
    ? "Edge"
    : ua.includes("chrome")
    ? "Chrome"
    : ua.includes("firefox")
    ? "Firefox"
    : ua.includes("safari")
    ? "Safari"
    : "Browser";
  const os = ua.includes("windows")
    ? "Windows"
    : ua.includes("iphone") || ua.includes("ipad")
    ? "iOS"
    : ua.includes("android")
    ? "Android"
    : ua.includes("mac os")
    ? "macOS"
    : ua.includes("linux")
    ? "Linux"
    : "";
  return os ? `${browser} on ${os}` : browser;
}

function UsageMeter({ label, used, limit }: { label: string; used: number; limit: number }) {
  const unlimited = limit === 0;
  const pct = unlimited ? 0 : Math.min(100, (used / limit) * 100);
  return (
    <div>
      <div className="flex justify-between">
        <span>{label}</span>
        <span>{unlimited ? `${used} today` : `${used} / ${limit}`}</span>
      </div>
      <div className="mt-1 h-4 border-2 border-ink rounded-md p-[2px]" aria-hidden="true">
        <div className="bg-ink h-full" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

export default function AccountPage() {
  const { user, loading, logout, refresh } = useAuth();
  const router = useRouter();

  const [usage, setUsage] = useState<Usage | null>(null);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [passwordMessage, setPasswordMessage] = useState("");
  const [passwordError, setPasswordError] = useState("");

  const [confirmEmail, setConfirmEmail] = useState("");
  const [deletePassword, setDeletePassword] = useState("");
  const [deleteError, setDeleteError] = useState("");
  const [deleting, setDeleting] = useState(false);
  // After deleting, go home instead of letting the "not signed in" redirect below send us to login.
  const [deleted, setDeleted] = useState(false);

  const load = useCallback(async () => {
    try {
      const [u, s] = await Promise.all([
        axios.get<Usage>(`${API_URL}/usage`),
        axios.get<SessionInfo[]>(`${API_URL}/auth/sessions`),
      ]);
      setUsage(u.data);
      setSessions(s.data);
    } catch (err) {
      setError(apiError(err, "Could not load your account details."));
    }
  }, []);

  useEffect(() => {
    if (!loading && !user && !deleted) router.push("/login");
  }, [user, loading, deleted, router]);

  useEffect(() => {
    if (user) load();
  }, [user, load]);

  if (loading || !user) {
    return <p className="text-center mt-20 text-2xl">Loading...</p>;
  }

  const resendVerification = async () => {
    setError("");
    setNotice("");
    try {
      const response = await axios.post(`${API_URL}/auth/resend-verification`);
      setNotice(response.data.message);
    } catch (err) {
      setError(apiError(err, "Could not send the email."));
    }
  };

  const changePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError("");
    setPasswordMessage("");
    try {
      const response = await axios.post(`${API_URL}/auth/change-password`, { currentPassword, newPassword });
      setPasswordMessage(response.data.message);
      setCurrentPassword("");
      setNewPassword("");
      load();
    } catch (err) {
      setPasswordError(apiError(err, "Could not change the password."));
    }
  };

  const revokeSession = async (id: number) => {
    setError("");
    try {
      await axios.delete(`${API_URL}/auth/sessions/${id}`);
      load();
    } catch (err) {
      setError(apiError(err, "Could not sign that device out."));
    }
  };

  const signOutEverywhere = async () => {
    try {
      await axios.post(`${API_URL}/auth/logout-all`);
    } catch (err) {
      setError(apiError(err, "Could not sign out."));
      return;
    }
    await logout();
    router.push("/login");
  };

  const exportData = async () => {
    setError("");
    try {
      const response = await axios.get(`${API_URL}/account/export`, { responseType: "blob" });
      const url = URL.createObjectURL(response.data);
      const link = document.createElement("a");
      link.href = url;
      link.download = "inkling-export.json";
      link.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(apiError(err, "Could not export your data."));
    }
  };

  const deleteAccount = async (e: React.FormEvent) => {
    e.preventDefault();
    setDeleteError("");
    setDeleting(true);
    try {
      await axios.delete(`${API_URL}/account`, { data: { confirmEmail, password: deletePassword } });
      setDeleted(true);
      await logout();
      router.push("/");
    } catch (err) {
      setDeleteError(apiError(err, "Could not delete your account."));
      setDeleting(false);
    }
  };

  return (
    <div className="max-w-3xl mx-auto mt-8 mb-16 space-y-8">
      <h1 className="text-4xl uppercase tracking-widest text-center">Your Account</h1>
      <ErrorText>{error}</ErrorText>
      <SuccessText>{notice}</SuccessText>

      <Section title="Profile">
        <p>
          <span className="opacity-70">Email:</span> {user.email}
        </p>
        {user.emailVerified ? (
          <p>Your email address is confirmed.</p>
        ) : (
          <div className="space-y-3">
            <p>Your email address isn&apos;t confirmed yet.</p>
            <SecondaryButton onClick={resendVerification}>Resend confirmation</SecondaryButton>
          </div>
        )}
      </Section>

      {usage && (
        <Section title="Today's usage">
          <UsageMeter label="Summaries" used={usage.summaries.used} limit={usage.summaries.limit} />
          <UsageMeter label="Chat questions" used={usage.chats.used} limit={usage.chats.limit} />
          <p className="text-base opacity-70">Limits reset at midnight UTC.</p>
        </Section>
      )}

      <Section title="Password">
        {user.hasPassword ? (
          <form onSubmit={changePassword} className="space-y-4">
            <Field
              id="current-password"
              label="Current password"
              type="password"
              value={currentPassword}
              onChange={setCurrentPassword}
              autoComplete="current-password"
            />
            <Field
              id="new-password"
              label="New password"
              type="password"
              value={newPassword}
              onChange={setNewPassword}
              autoComplete="new-password"
              minLength={8}
              maxLength={72}
              hint="At least 8 characters. Your other devices will be signed out."
            />
            <ErrorText>{passwordError}</ErrorText>
            <SuccessText>{passwordMessage}</SuccessText>
            <SecondaryButton type="submit">Change password</SecondaryButton>
          </form>
        ) : (
          <p>
            You sign in with Google and don&apos;t have a password. To add one, use{" "}
            <a href="/forgot-password" className="underline">
              Forgot password
            </a>
            .
          </p>
        )}
      </Section>

      <Section title="Signed-in devices">
        <ul className="space-y-3">
          {sessions.map((s) => (
            <li key={s.id} className="flex items-center justify-between gap-4 border-b border-dashed border-ink/40 pb-3">
              <div>
                <p>
                  {deviceName(s.userAgent)}
                  {s.current && <span className="ml-2 text-base border border-ink rounded px-2">This device</span>}
                </p>
                <p className="text-base opacity-70">Last active {new Date(s.lastUsedAt).toLocaleString()}</p>
              </div>
              {!s.current && <SecondaryButton onClick={() => revokeSession(s.id)}>Sign out</SecondaryButton>}
            </li>
          ))}
        </ul>
        <SecondaryButton onClick={signOutEverywhere}>Sign out everywhere</SecondaryButton>
      </Section>

      <Section title="Your data">
        <p>
          Download everything we store about you: your account, your saved summaries and the text they came from. The
          original PDFs we keep for the document viewer are listed by name; the files themselves are not included, as
          you uploaded them.
        </p>
        <SecondaryButton onClick={exportData}>Download my data</SecondaryButton>
      </Section>

      <Section title="Delete account">
        <p>
          This permanently deletes your account, summaries and chat history. It can&apos;t be undone.
        </p>
        <form onSubmit={deleteAccount} className="space-y-4">
          <Field
            id="confirm-email"
            label={`Type ${user.email} to confirm`}
            value={confirmEmail}
            onChange={setConfirmEmail}
            autoComplete="off"
          />
          {user.hasPassword && (
            <Field
              id="delete-password"
              label="Your password"
              type="password"
              value={deletePassword}
              onChange={setDeletePassword}
              autoComplete="current-password"
            />
          )}
          <ErrorText>{deleteError}</ErrorText>
          <DangerButton type="submit" disabled={deleting}>
            {deleting ? "Deleting..." : "Delete my account"}
          </DangerButton>
        </form>
      </Section>
    </div>
  );
}
