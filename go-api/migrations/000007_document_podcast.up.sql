-- The podcast script written for a document (title, language and the two hosts' turns), kept so a
-- document's podcast is generated once and replayed for free afterwards. NULL until generated.
ALTER TABLE documents ADD COLUMN IF NOT EXISTS podcast JSONB;
