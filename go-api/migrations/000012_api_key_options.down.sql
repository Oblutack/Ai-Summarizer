DROP TABLE IF EXISTS api_key_usage;
ALTER TABLE api_keys DROP COLUMN IF EXISTS daily_limit;
ALTER TABLE api_keys DROP COLUMN IF EXISTS expires_at;
