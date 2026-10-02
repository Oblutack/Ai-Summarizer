-- Baseline schema. Everything is IF NOT EXISTS so it is a no-op on databases that were created
-- by the old GORM AutoMigrate, and builds the full schema on a fresh one.

CREATE TABLE IF NOT EXISTS users (
    id          BIGSERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ,
    deleted_at  TIMESTAMPTZ,
    email       TEXT,
    password    TEXT,
    CONSTRAINT uni_users_email UNIQUE (email)
);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

CREATE TABLE IF NOT EXISTS documents (
    id          BIGSERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ,
    deleted_at  TIMESTAMPTZ,
    filename    TEXT,
    summary     TEXT,
    user_id     BIGINT
);
CREATE INDEX IF NOT EXISTS idx_documents_deleted_at ON documents (deleted_at);

-- Added to the model after the first release, so older databases may lack them.
ALTER TABLE documents ADD COLUMN IF NOT EXISTS content     TEXT;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS has_content BOOLEAN;
