-- Migration 000003 (down): revert to the pre-refund status set.
ALTER TABLE payments DROP CONSTRAINT payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed'));
