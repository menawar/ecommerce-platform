-- Migration 000003 (up): email verification. A new account starts unverified;
-- a single-use, expiring token (emailed as a link) flips email_verified to true.
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE verification_tokens (
    token      UUID PRIMARY KEY,
    user_id    UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_verification_tokens_user ON verification_tokens(user_id);
