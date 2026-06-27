-- Runs ONCE, the first time the postgres data volume is initialized
-- (docker-entrypoint-initdb.d). This is how we honor "database-per-service" on a
-- single Postgres instance: one physical server, five logically isolated dbs.
-- A service connects only to its own db and can't even name another's tables.
--
-- NOTE: psql meta-commands (\connect) work here because the entrypoint pipes this
-- file through psql. CREATE DATABASE cannot run inside a transaction block, so
-- each is its own statement.

CREATE DATABASE userdb;
CREATE DATABASE productdb;
CREATE DATABASE orderdb;
CREATE DATABASE paymentdb;
CREATE DATABASE notificationdb;

-- userdb stores emails as CITEXT (case-insensitive) per the spec, so "A@x.com"
-- and "a@x.com" collide on the UNIQUE constraint. Extensions are per-database,
-- so we must connect into userdb to install it there.
\connect userdb
CREATE EXTENSION IF NOT EXISTS citext;
