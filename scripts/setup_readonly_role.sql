-- One-time infrastructure setup. Run as a superuser / master user.
-- Requires CREATEROLE privilege and ownership of the public schema.
--
-- Usage:
--   psql $MASTER_DATABASE_URL -f scripts/setup_readonly_role.sql
--
-- Replace 'finagent_ro_secret' with a strong password before running.
-- Set database.readonly_url in config to activate the SQL agent tools.

DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'finagent_ro') THEN
    CREATE ROLE finagent_ro WITH LOGIN PASSWORD 'finagent_ro_secret' NOINHERIT;
  END IF;
END
$$;

GRANT CONNECT ON DATABASE finagent TO finagent_ro;
GRANT USAGE ON SCHEMA public TO finagent_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO finagent_ro;
-- Cover tables created by the superuser running this script.
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO finagent_ro;
-- Cover tables created by the finagent app role (e.g. future migrations).
ALTER DEFAULT PRIVILEGES FOR ROLE finagent IN SCHEMA public GRANT SELECT ON TABLES TO finagent_ro;
