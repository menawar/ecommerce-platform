-- Migration 000004 (up): password reset. A single-use, short-lived token (emailed
-- as a link) lets a user set a new password; a successful reset also revokes all
-- their refresh tokens. Same shape/convention as verification_tokens (no FK, an
-- index on user_id) — see migration 000003.
CREATE TABLE password_reset_tokens (
    token      UUID PRIMARY KEY,
    user_id    UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_reset_tokens_user ON password_reset_tokens(user_id);
