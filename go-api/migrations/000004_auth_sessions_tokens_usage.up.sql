-- Accounts: email verification and whether the user has a password they know.
-- Google-created accounts have a random unguessable password, so has_password is false for them
-- from now on. Existing rows default to TRUE because we cannot tell which were Google-created.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS has_password BOOLEAN NOT NULL DEFAULT TRUE;

-- Server-side sessions replace stateless JWTs, so a session can be revoked immediately.
-- Only a SHA-256 hash of the session token is stored: a database leak does not leak live sessions.
CREATE TABLE IF NOT EXISTS sessions (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL UNIQUE,
    user_agent   TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);

-- Single-use tokens for emailed links (email verification, password reset); hashed like sessions.
CREATE TABLE IF NOT EXISTS email_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    TEXT        NOT NULL,
    token_hash TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_email_tokens_user_purpose ON email_tokens (user_id, purpose);

-- Per-user daily usage for quotas. One row per user per UTC day.
CREATE TABLE IF NOT EXISTS daily_usage (
    user_id   BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day       DATE   NOT NULL,
    summaries INT    NOT NULL DEFAULT 0,
    chats     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);
