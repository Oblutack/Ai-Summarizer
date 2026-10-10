"use client";
import { useCallback, useEffect, useState } from "react";
import axios from "axios";
import { API_URL, apiError } from "../lib/api";
import CopyButton from "./CopyButton";
import { useT } from "./I18nProvider";
import { ErrorText, Field, SecondaryButton } from "./ui";

// What the API tells about a key: never the key itself, only what tells keys apart.
interface ApiKeyInfo {
  id: number;
  name: string;
  prefix: string;
  createdAt: string;
  lastUsedAt: string | null;
  revokedAt: string | null;
}

// The most live keys one person may hold: keep in sync with go-api (auth.MaxAPIKeysPerUser).
const MAX_KEYS = 5;
const MAX_NAME = 40;

// Keys for programs: made here, shown once, ended here. They only ever open the summarizing routes of the API (/v1).
export default function ApiKeys() {
  const t = useT();
  const [keys, setKeys] = useState<ApiKeyInfo[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  // The key just made, in the clear: it is in this reply and nowhere else.
  const [fresh, setFresh] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const response = await axios.get<ApiKeyInfo[]>(`${API_URL}/account/api-keys`);
      setKeys(response.data);
      setLoaded(true);
    } catch (err) {
      setError(apiError(err, t("apiKeys.loadFailed")));
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const live = keys.filter((k) => !k.revokedAt);
  const full = live.length >= MAX_KEYS;

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setCreating(true);
    try {
      const response = await axios.post<ApiKeyInfo & { key: string }>(`${API_URL}/account/api-keys`, { name });
      setFresh(response.data.key);
      setName("");
      await load();
    } catch (err) {
      setError(apiError(err, t("apiKeys.createFailed")));
    } finally {
      setCreating(false);
    }
  };

  const revoke = async (id: number) => {
    setError("");
    try {
      await axios.delete(`${API_URL}/account/api-keys/${id}`);
      setConfirming(null);
      await load();
    } catch (err) {
      setError(apiError(err, t("apiKeys.revokeFailed")));
    }
  };

  return (
    <div className="space-y-4" data-testid="api-keys">
      <p>{t("apiKeys.intro")}</p>
      <ErrorText>{error}</ErrorText>

      {fresh && (
        <div className="space-y-3 rounded-lg border border-accent/60 bg-surface p-4" role="group" aria-label={t("apiKeys.newKeyTitle")} data-testid="new-api-key">
          <h3 className="text-lg font-semibold">{t("apiKeys.newKeyTitle")}</h3>
          <code className="block break-all rounded-md bg-canvas px-3 py-2 text-sm" data-testid="new-api-key-value">
            {fresh}
          </code>
          <p className="text-sm">{t("apiKeys.copyNow")}</p>
          <div className="flex flex-wrap items-center gap-3">
            <CopyButton text={fresh} what={t("apiKeys.what")} />
            <SecondaryButton type="button" onClick={() => setFresh(null)}>
              {t("apiKeys.saved")}
            </SecondaryButton>
          </div>
        </div>
      )}

      {full ? (
        <p className="muted" role="status">
          {t("apiKeys.limitReached", { max: MAX_KEYS })}
        </p>
      ) : (
        <form onSubmit={create} className="space-y-3">
          <Field id="api-key-name" label={t("apiKeys.nameLabel")} value={name} onChange={setName} required={false} maxLength={MAX_NAME} hint={t("apiKeys.nameHint")} autoComplete="off" />
          <SecondaryButton type="submit" disabled={creating}>
            {creating ? t("apiKeys.creating") : t("apiKeys.create")}
          </SecondaryButton>
        </form>
      )}

      {loaded && keys.length === 0 && <p className="muted">{t("apiKeys.none")}</p>}
      {keys.length > 0 && (
        <ul className="space-y-3">
          {keys.map((k) => (
            <li key={k.id} className="flex flex-wrap items-center justify-between gap-3 border-b border-ink/15 pb-3 last:border-0" data-testid="api-key-row">
              <div className="min-w-0">
                <p className="break-words">
                  <span className="font-medium">{k.name}</span>{" "}
                  <code className="text-sm text-ink/70">{k.prefix}…</code>
                  {k.revokedAt && <span className="chip pointer-events-none ml-2 text-xs">{t("apiKeys.revokedChip")}</span>}
                </p>
                <p className="muted text-sm">
                  {t("apiKeys.createdOn", { when: new Date(k.createdAt).toLocaleDateString() })}
                  {" · "}
                  {k.lastUsedAt ? t("apiKeys.lastUsed", { when: new Date(k.lastUsedAt).toLocaleString() }) : t("apiKeys.neverUsed")}
                </p>
              </div>
              {!k.revokedAt &&
                (confirming === k.id ? (
                  <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("apiKeys.confirmRevoke", { name: k.name })}>
                    <span className="text-sm">{t("apiKeys.confirmRevoke", { name: k.name })}</span>
                    <SecondaryButton type="button" onClick={() => revoke(k.id)}>
                      {t("apiKeys.confirmYes")}
                    </SecondaryButton>
                    <SecondaryButton type="button" onClick={() => setConfirming(null)}>
                      {t("apiKeys.cancel")}
                    </SecondaryButton>
                  </div>
                ) : (
                  <SecondaryButton type="button" onClick={() => setConfirming(k.id)} aria-label={t("apiKeys.revokeNamed", { name: k.name })}>
                    {t("apiKeys.revoke")}
                  </SecondaryButton>
                ))}
            </li>
          ))}
        </ul>
      )}

      <div className="space-y-2">
        <p className="muted text-sm">{t("apiKeys.example")}</p>
        <pre className="overflow-x-auto rounded-md bg-canvas p-3 text-sm" data-testid="api-example">
          <code>{`curl -X POST ${API_URL}/v1/summarize-text \\
  -H "Authorization: Bearer ink_YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"text": "Paste the text to summarize here."}'`}</code>
        </pre>
      </div>
    </div>
  );
}
