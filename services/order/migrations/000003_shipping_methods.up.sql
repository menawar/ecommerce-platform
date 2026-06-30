-- Migration 000003 (up): configurable shipping methods. Checkout lists the active
-- ones and snapshots the chosen name + cost onto the order (so later edits/deletes
-- here never rewrite past orders). active toggles checkout visibility; deletes are
-- hard (safe, because orders keep their own snapshot).
CREATE TABLE shipping_methods (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    sort_order  INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Checkout queries active methods ordered for display.
CREATE INDEX idx_shipping_methods_active ON shipping_methods(active, sort_order);
