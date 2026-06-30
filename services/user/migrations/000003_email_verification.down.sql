-- Migration 000003 (down): drop email verification.
DROP TABLE IF EXISTS verification_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
