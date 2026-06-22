-- Migration 000001 (up): the users table for userdb.
--
-- CREATE EXTENSION here (not just in initdb) makes the migration self-contained:
-- it works on any fresh userdb, not only one bootstrapped by our compose initdb.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- CITEXT = case-insensitive text. "A@x.com" and "a@x.com" collide on UNIQUE,
    -- so the database enforces "one account per email" regardless of casing — the
    -- in-memory store emulated this by lowercasing keys; now it's a real constraint.
    email         CITEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    full_name     TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'customer',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
