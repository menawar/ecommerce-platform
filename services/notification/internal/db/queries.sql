-- name: InsertNotification :exec
-- Fails with a unique violation if event_id was already processed (the dedup gate).
INSERT INTO notifications (event_id, user_id, channel, template, payload)
VALUES ($1, $2, $3, $4, $5);

-- name: CountByEventID :one
SELECT count(*) FROM notifications WHERE event_id = $1;

-- name: ListByUser :many
SELECT * FROM notifications WHERE user_id = $1 ORDER BY sent_at DESC LIMIT $2;
