-- Things a person can do with a saved document beyond reading it.
-- tags: labels the owner puts on it (a JSON list of short strings).
-- suggestions / study: questions to ask and flashcards or a quiz made by the model, kept so they are
--   generated once and shown again for free (NULL until generated).
-- share_token: when set, anyone with the link can read the summary (not the source text); clearing it
--   revokes the link.
ALTER TABLE documents ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS suggestions JSONB;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS study JSONB;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS share_token TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_share_token ON documents (share_token) WHERE share_token IS NOT NULL;

-- Standing preferences added to every summary prompt ("focus on costs and deadlines").
ALTER TABLE users ADD COLUMN IF NOT EXISTS custom_instructions TEXT NOT NULL DEFAULT '';
