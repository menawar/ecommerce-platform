-- Migration 000006 (down): drop the deletion tombstone marker.
ALTER TABLE users DROP COLUMN deleted_at;
