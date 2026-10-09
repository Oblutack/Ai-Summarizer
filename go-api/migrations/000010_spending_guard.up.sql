-- Limits on how much AI work can be started in a day, so that nobody (and no bug) can run up the model bill.
--
-- anonymous_usage: summaries made without an account, per visitor per UTC day. A visitor is a keyed hash of their
--   network address (the first 64 bits for IPv6), never the address itself, and the rows are cleared after a few days.
-- global_usage: every piece of AI work started anywhere on the site, per UTC day; when it reaches the daily budget
--   the work is paused until midnight UTC.
CREATE TABLE IF NOT EXISTS anonymous_usage (
    ip_hash   TEXT NOT NULL,
    day       DATE NOT NULL,
    summaries INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (ip_hash, day)
);

CREATE TABLE IF NOT EXISTS global_usage (
    day      DATE PRIMARY KEY,
    requests INT NOT NULL DEFAULT 0
);
