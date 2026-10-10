-- A limited database account for the running API.
--
-- Why: the API connects to the database with whatever account its DSN names, and that is usually the account that owns
-- everything (it has to be, to run the migrations). If the API were ever broken into, that account could drop tables,
-- change the schema or read the migration history. The account below can only read and write rows in the tables, so the
-- worst a break-in reaches is the data the app itself can reach.
--
-- How, once per database:
--   1. Replace CHANGE_ME below with a long random password (for example: openssl rand -base64 32).
--   2. Run this file as the database owner, connected to the application's database:
--        psql "<the owner's connection string>" -f docs/least-privilege.sql
--   3. Set DSN on the API to the new account, and MIGRATE_DSN to the owner's connection string (the API uses it only
--      to apply migrations when it starts):
--        DSN=host=... user=inkling_app password=<the new password> dbname=... sslmode=require
--        MIGRATE_DSN=host=... user=<the owner> password=<the owner's password> dbname=... sslmode=require
--
-- Tables that later migrations add are covered too (see ALTER DEFAULT PRIVILEGES): the owner creates them, so they are
-- granted to the new account as they appear.

CREATE ROLE inkling_app LOGIN PASSWORD 'CHANGE_ME' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;

DO $$
BEGIN
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO inkling_app', current_database());
END
$$;

GRANT USAGE ON SCHEMA public TO inkling_app;
REVOKE CREATE ON SCHEMA public FROM inkling_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inkling_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inkling_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inkling_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO inkling_app;

-- The record of which migrations have run is for the owner alone.
DO $$
BEGIN
  IF to_regclass('schema_migrations') IS NOT NULL THEN
    REVOKE ALL ON TABLE schema_migrations FROM inkling_app;
  END IF;
END
$$;
