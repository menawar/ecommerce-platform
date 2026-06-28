-- User persistence queries. The server generates the id and timestamps (so the
-- in-memory and Postgres stores behave identically), so CreateUser inserts all
-- columns rather than relying on column defaults.

-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash, full_name, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- Refresh-token tracking (revocation + rotation).

-- name: SaveRefreshToken :exec
INSERT INTO refresh_tokens (jti, user_id, expires_at) VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens WHERE jti = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE jti = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;
