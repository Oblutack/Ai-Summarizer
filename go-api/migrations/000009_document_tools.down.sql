ALTER TABLE users DROP COLUMN IF EXISTS custom_instructions;
DROP INDEX IF EXISTS idx_documents_share_token;
ALTER TABLE documents DROP COLUMN IF EXISTS share_token;
ALTER TABLE documents DROP COLUMN IF EXISTS study;
ALTER TABLE documents DROP COLUMN IF EXISTS suggestions;
ALTER TABLE documents DROP COLUMN IF EXISTS tags;
