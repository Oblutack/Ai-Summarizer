-- A vector for every passage, so a question can find passages by meaning and not only by their words.
-- Stored as the raw little-endian float32 bytes from the AI service. A person's library is searched
-- by comparing against their own passages only, so no vector index or database extension is needed.
-- NULL until embedded (embedding is optional: without it, search is by keyword only).
-- embedding_model says which model made the vector, because vectors of different models cannot be compared.
ALTER TABLE document_passages ADD COLUMN IF NOT EXISTS embedding BYTEA;
ALTER TABLE document_passages ADD COLUMN IF NOT EXISTS embedding_model TEXT NOT NULL DEFAULT '';
