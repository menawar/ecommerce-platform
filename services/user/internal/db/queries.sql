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
