-- Migration 000001 (down). We drop the table but leave the citext extension
-- installed — other migrations or tables may rely on it, and dropping an
-- extension is rarely what a rollback intends.
DROP TABLE IF EXISTS users;
