-- The original PDFs behind a saved summary, so a cited passage can be shown in the document itself.
-- Stored in the database (bytea) to keep the stack free of extra services; the size per user is
-- capped by the API (STORED_FILES_MB_PER_USER). Removing a document or an account removes them.
CREATE TABLE IF NOT EXISTS document_files (
    id          BIGSERIAL PRIMARY KEY,
    document_id BIGINT      NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    filename    TEXT        NOT NULL,
    size_bytes  BIGINT      NOT NULL,
    content     BYTEA       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_document_files_document_id ON document_files (document_id);
CREATE INDEX IF NOT EXISTS idx_document_files_user_id ON document_files (user_id);
