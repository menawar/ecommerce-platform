-- Migration 000001 (up): core catalog schema for productdb.
--
-- gen_random_uuid() is built into Postgres 13+, so no pgcrypto extension is
-- needed. Each migration is applied inside a transaction by golang-migrate, so a
-- failure rolls the whole file back — the schema never ends up half-applied.

CREATE TABLE categories (
    id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    slug TEXT UNIQUE NOT NULL
);

CREATE TABLE products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku         TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- Money is stored as integer minor units (cents/kobo), never float — floats
    -- can't represent 0.10 exactly and rounding errors compound over a cart.
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    currency    TEXT NOT NULL DEFAULT 'NGN',
    category_id UUID REFERENCES categories(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One inventory row per product. "available" is derived (quantity - reserved),
-- not stored, so it can't drift. version is the optimistic-lock counter, bumped
-- on every stock mutation. The CHECK constraints make the invariants enforceable
-- at the DB level: stock can't go negative and we can never reserve more than we
-- physically have, even if app code has a bug.
CREATE TABLE inventory (
    product_id UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    quantity   INT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    reserved   INT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    version    INT NOT NULL DEFAULT 0,
    CONSTRAINT inventory_reserved_le_quantity CHECK (reserved <= quantity)
);

-- Supports ListProducts filtering by category.
CREATE INDEX idx_products_category ON products(category_id);
