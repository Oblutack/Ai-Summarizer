-- Serves the dashboard query: a user's live documents, newest first, paged by id.
CREATE INDEX IF NOT EXISTS idx_documents_user_id_id
    ON documents (user_id, id DESC)
    WHERE deleted_at IS NULL;
