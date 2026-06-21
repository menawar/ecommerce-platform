-- Migration 000002 (up): reservation bookkeeping for the order saga.
--
-- A reservation is the saga's claim on stock: "hold N of product P for order O".
-- It is keyed by a caller-supplied id (the saga's reservation_id) so a retry with
-- the same id is a no-op — this is what makes ReserveStock idempotent.

CREATE TABLE stock_reservations (
    -- Supplied by the caller (NOT generated here): it IS the idempotency key.
    id         UUID PRIMARY KEY,
    status     TEXT NOT NULL DEFAULT 'reserved'
               CHECK (status IN ('reserved', 'released', 'committed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The per-product lines of a reservation. A reservation can hold several products
-- at once; we record how much each line reserved so Release/Commit can undo or
-- finalize the exact amounts.
CREATE TABLE reservation_items (
    reservation_id UUID NOT NULL REFERENCES stock_reservations(id) ON DELETE CASCADE,
    product_id     UUID NOT NULL REFERENCES products(id),
    quantity       INT  NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (reservation_id, product_id)
);
