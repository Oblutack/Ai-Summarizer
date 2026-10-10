"use client";
import { useCallback, useEffect, useId, useState } from "react";
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
  expiresAt: string | null;
  // The most requests the key may make in a day (null: only its owner's allowance limits it).
  dailyLimit: number | null;
  usedToday: number;
  usedTotal: number;
}

// How long a new key lasts, in days (0: forever).
const EXPIRIES = [
  { days: 0, label: "apiKeys.expiresNever" },
  { days: 30, label: "apiKeys.expires30" },
  { days: 90, label: "apiKeys.expires90" },
  { days: 365, label: "apiKeys.expires365" },
] as const;

// Keep in step with go-api (maxAPIKeyDailyLimit).
const MAX_DAILY_LIMIT = 1_000_000;

// The most live keys one person may hold: keep in sync with go-api (auth.MaxAPIKeysPerUser).
const MAX_KEYS = 5;
const MAX_NAME = 40;

function isExpired(key: ApiKeyInfo): boolean {
  return key.expiresAt !== null && new Date(key.expiresAt).getTime() <= Date.now();
}

// Keys for programs: made here, shown once, ended here. They only ever open the summarizing routes of the API (/v1).
export default function ApiKeys() {
  const t = useT();
  const [keys, setKeys] = useState<ApiKeyInfo[]>([]);
  const [loaded, setLoaded] = useState(false);
  const ids = { expires: useId(), limit: useId() };
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState(0);
  const [dailyLimit, setDailyLimit] = useState("");
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
      const response = await axios.post<ApiKeyInfo & { key: string }>(`${API_URL}/account/api-keys`, {
        name,
        expiresInDays,
        dailyLimit: dailyLimit.trim() ? Number(dailyLimit) : undefined,
      });
      setFresh(response.data.key);
      setName("");
      setExpiresInDays(0);
      setDailyLimit("");
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
          <div className="flex flex-wrap items-start gap-4">
            <div className="flex flex-col gap-1">
              <label htmlFor={ids.expires} className="label mb-0">
                {t("apiKeys.expiresLabel")}
              </label>
              <select id={ids.expires} className="field" value={expiresInDays} onChange={(e) => setExpiresInDays(Number(e.target.value))}>
                {EXPIRIES.map((e) => (
                  <option key={e.days} value={e.days}>
                    {t(e.label)}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex min-w-[12rem] flex-1 flex-col gap-1">
              <label htmlFor={ids.limit} className="label mb-0">
                {t("apiKeys.limitLabel")}
              </label>
              <input
                id={ids.limit}
                type="number"
                inputMode="numeric"
                className="field max-w-[10rem]"
                min={1}
                max={MAX_DAILY_LIMIT}
                step={1}
                value={dailyLimit}
                onChange={(e) => setDailyLimit(e.target.value)}
                aria-describedby={`${ids.limit}-hint`}
              />
              <p id={`${ids.limit}-hint`} className="muted text-sm">
                {t("apiKeys.limitHint")}
              </p>
            </div>
          </div>
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
                  {!k.revokedAt && isExpired(k) && <span className="chip pointer-events-none ml-2 text-xs">{t("apiKeys.expiredChip")}</span>}
                </p>
                <p className="muted text-sm">
                  {t("apiKeys.createdOn", { when: new Date(k.createdAt).toLocaleDateString() })}
                  {" · "}
                  {k.lastUsedAt ? t("apiKeys.lastUsed", { when: new Date(k.lastUsedAt).toLocaleString() }) : t("apiKeys.neverUsed")}
                  {k.expiresAt && !isExpired(k) && !k.revokedAt && (
                    <>
                      {" · "}
                      {t("apiKeys.expiresOn", { when: new Date(k.expiresAt).toLocaleDateString() })}
                    </>
                  )}
                </p>
                <p className="muted text-sm" data-testid="api-key-usage">
                  {k.dailyLimit ? t("apiKeys.usageTodayOf", { n: k.usedToday, limit: k.dailyLimit }) : t("apiKeys.usageToday", { n: k.usedToday })}
                  {" · "}
                  {t("apiKeys.usageTotal", { n: k.usedTotal })}
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
        <p className="muted text-sm">{t("apiKeys.saveTip")}</p>
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
