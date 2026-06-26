-- Migration 000004 (down): drop product soft-delete.
ALTER TABLE products DROP COLUMN archived_at;
