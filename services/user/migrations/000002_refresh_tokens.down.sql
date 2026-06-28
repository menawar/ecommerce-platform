-- Migration 000002 (down): drop refresh-token tracking.
DROP TABLE IF EXISTS refresh_tokens;
