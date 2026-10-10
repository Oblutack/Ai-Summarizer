-- API keys let a program call the summarizing routes (/v1) on a person's behalf, without a browser session.
-- Like sessions, only the SHA-256 hash of a key is stored, so a database leak does not leak working keys; the prefix
-- (the first few characters) is kept so a person can tell their keys apart. A revoked key stays listed.
CREATE TABLE IF NOT EXISTS api_keys (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    prefix       TEXT        NOT NULL,
    key_hash     TEXT        NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);
