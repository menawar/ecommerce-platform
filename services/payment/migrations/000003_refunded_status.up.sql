-- Migration 000003 (up): allow the 'refunded' payment status (Phase 11.3 refunds).
ALTER TABLE payments DROP CONSTRAINT payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded'));
