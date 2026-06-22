-- Migration 000001 (up): orderdb — orders, items, and the transactional outbox.

CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    status          TEXT NOT NULL,
    total_cents     BIGINT NOT NULL,
    currency        TEXT NOT NULL DEFAULT 'NGN',
    -- reservation_id = the order id (we reserve stock under the order's own id),
    -- so Product's idempotent ReserveStock is keyed to this order.
    reservation_id  UUID NOT NULL,
    payment_id      UUID,
    -- dedupes "place order": a retried PlaceOrder with the same key returns the
    -- existing order instead of creating a second one.
    idempotency_key TEXT UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES orders(id),
    product_id  UUID NOT NULL,
    -- name + price are SNAPSHOTTED at order time: an order is a historical record,
    -- so later catalog price/name changes must not alter what the customer bought.
    name        TEXT NOT NULL,
    price_cents BIGINT NOT NULL,
    quantity    INT NOT NULL
);

-- Transactional outbox: events are written in the SAME tx as the state change that
-- produced them, then a background poller publishes unpublished rows to NATS and
-- stamps published_at. This is how we get "DB write + event publish" atomically
-- without a distributed transaction.
CREATE TABLE outbox (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic        TEXT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_orders_user ON orders(user_id, created_at DESC);
-- Partial index: the poller only ever scans UNPUBLISHED rows, so index just those.
CREATE INDEX idx_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;
