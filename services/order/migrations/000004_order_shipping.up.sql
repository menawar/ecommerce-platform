-- Migration 000004 (up): shipping on orders. The chosen shipping method's cost +
-- name and the chosen address are SNAPSHOTTED onto the order at checkout, so later
-- edits to the catalog of methods or the user's address book never rewrite past
-- orders. total_cents = subtotal + shipping_cents. Columns are defaulted so orders
-- created before this migration remain valid.
ALTER TABLE orders
    ADD COLUMN shipping_method_id   UUID,
    ADD COLUMN shipping_method_name TEXT   NOT NULL DEFAULT '',
    ADD COLUMN shipping_cents       BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN ship_recipient       TEXT   NOT NULL DEFAULT '',
    ADD COLUMN ship_phone           TEXT   NOT NULL DEFAULT '',
    ADD COLUMN ship_line1           TEXT   NOT NULL DEFAULT '',
    ADD COLUMN ship_line2           TEXT   NOT NULL DEFAULT '',
    ADD COLUMN ship_city            TEXT   NOT NULL DEFAULT '',
    ADD COLUMN ship_state           TEXT   NOT NULL DEFAULT '',
    ADD COLUMN ship_postal_code     TEXT   NOT NULL DEFAULT '',
    ADD COLUMN ship_country         TEXT   NOT NULL DEFAULT '';
