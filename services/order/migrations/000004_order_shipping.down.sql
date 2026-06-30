-- Migration 000004 (down): drop the shipping snapshot columns.
ALTER TABLE orders
    DROP COLUMN IF EXISTS shipping_method_id,
    DROP COLUMN IF EXISTS shipping_method_name,
    DROP COLUMN IF EXISTS shipping_cents,
    DROP COLUMN IF EXISTS ship_recipient,
    DROP COLUMN IF EXISTS ship_phone,
    DROP COLUMN IF EXISTS ship_line1,
    DROP COLUMN IF EXISTS ship_line2,
    DROP COLUMN IF EXISTS ship_city,
    DROP COLUMN IF EXISTS ship_state,
    DROP COLUMN IF EXISTS ship_postal_code,
    DROP COLUMN IF EXISTS ship_country;
