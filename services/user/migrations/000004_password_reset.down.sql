-- Migration 000004 (down): drop password reset.
DROP TABLE IF EXISTS password_reset_tokens;
