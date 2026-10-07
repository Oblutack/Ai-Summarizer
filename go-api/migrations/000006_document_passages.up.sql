-- A searchable index of every saved document, cut into page-sized passages, so one question can be
-- answered from all of a user's documents at once. The search is Postgres full-text search (by
-- keyword, with English stemming); no extra service is needed.
CREATE TABLE IF NOT EXISTS document_passages (
    id          BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ord         INT    NOT NULL,
    page        INT,
    page_end    INT,
    doc_name    TEXT   NOT NULL DEFAULT '',
    text        TEXT   NOT NULL,
    tsv         TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', text)) STORED
);
CREATE INDEX IF NOT EXISTS idx_document_passages_tsv      ON document_passages USING GIN (tsv);
CREATE INDEX IF NOT EXISTS idx_document_passages_document ON document_passages (document_id);
CREATE INDEX IF NOT EXISTS idx_document_passages_user     ON document_passages (user_id);

-- NULL until the document has been cut into passages (older documents are indexed on first use).
ALTER TABLE documents ADD COLUMN IF NOT EXISTS indexed_at TIMESTAMPTZ;
