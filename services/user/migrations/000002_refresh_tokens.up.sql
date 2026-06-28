-- Migration 000002 (up): server-side refresh-token tracking so logout/rotation
-- can actually REVOKE a session. We store only the token's jti (never the token),
-- plus who it belongs to, when it expires, and whether it's been revoked.
CREATE TABLE refresh_tokens (
    jti        UUID PRIMARY KEY,
    user_id    UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Logout-all / reuse-detection revokes every token for a user.
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
