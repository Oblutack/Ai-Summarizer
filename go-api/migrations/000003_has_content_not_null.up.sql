-- Rows saved before chat existed have a NULL flag; treat them as "no stored text".
UPDATE documents SET has_content = FALSE WHERE has_content IS NULL;
ALTER TABLE documents ALTER COLUMN has_content SET DEFAULT FALSE;
ALTER TABLE documents ALTER COLUMN has_content SET NOT NULL;
