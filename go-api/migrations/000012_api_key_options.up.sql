-- More control over an API key: it can expire, it can have a daily limit of its own (on top of its owner's allowance),
-- and what it does is counted per day, so a person can see which key is used and how much.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS daily_limit INT;

CREATE TABLE IF NOT EXISTS api_key_usage (
    key_id   BIGINT NOT NULL REFERENCES api_keys (id) ON DELETE CASCADE,
    day      DATE   NOT NULL,
    requests INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (key_id, day)
);
