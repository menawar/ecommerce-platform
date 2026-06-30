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

-- Email verification.

-- name: SetEmailVerified :exec
UPDATE users SET email_verified = true, updated_at = now() WHERE id = $1;

-- name: SaveVerificationToken :exec
INSERT INTO verification_tokens (token, user_id, expires_at) VALUES ($1, $2, $3);

-- name: GetVerificationToken :one
SELECT * FROM verification_tokens WHERE token = $1;

-- name: UseVerificationToken :exec
UPDATE verification_tokens SET used_at = now() WHERE token = $1 AND used_at IS NULL;

-- Password reset.

-- name: UpdatePassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: SavePasswordResetToken :exec
INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES ($1, $2, $3);

-- name: GetPasswordResetToken :one
SELECT * FROM password_reset_tokens WHERE token = $1;

-- ConsumePasswordResetToken flips the token to used ONLY if it was still unused,
-- and reports how many rows changed (1 = this caller won the single-use race).
-- name: ConsumePasswordResetToken :execrows
UPDATE password_reset_tokens SET used_at = now() WHERE token = $1 AND used_at IS NULL;

-- InvalidateUserPasswordResetTokens spends all of a user's outstanding reset
-- tokens, so issuing a fresh link makes any prior link stop working.
-- name: InvalidateUserPasswordResetTokens :exec
UPDATE password_reset_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL;

-- Address book.

-- name: CreateAddress :one
INSERT INTO addresses (user_id, label, recipient, phone, line1, line2, city, state, postal_code, country, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListAddressesByUser :many
SELECT * FROM addresses WHERE user_id = $1 ORDER BY is_default DESC, created_at DESC;

-- name: GetAddress :one
SELECT * FROM addresses WHERE id = $1 AND user_id = $2;

-- UpdateAddress full-replaces the mutable fields; scoped by user_id so one user
-- can't edit another's. Reports rows affected (0 = not found / not owned).
-- name: UpdateAddress :execrows
UPDATE addresses
SET label = $3, recipient = $4, phone = $5, line1 = $6, line2 = $7,
    city = $8, state = $9, postal_code = $10, country = $11, updated_at = now()
WHERE id = $1 AND user_id = $2;

-- name: DeleteAddress :execrows
DELETE FROM addresses WHERE id = $1 AND user_id = $2;

-- ClearDefaultAddresses + SetAddressDefault run in one tx so a user has at most
-- one default at a time.
-- name: ClearDefaultAddresses :exec
UPDATE addresses SET is_default = false, updated_at = now() WHERE user_id = $1 AND is_default = true;

-- name: SetAddressDefault :execrows
UPDATE addresses SET is_default = true, updated_at = now() WHERE id = $1 AND user_id = $2;
