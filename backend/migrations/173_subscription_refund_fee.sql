ALTER TABLE subscription_refund_requests
    ADD COLUMN IF NOT EXISTS refund_amount NUMERIC(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refund_fee_amount NUMERIC(20, 8) NOT NULL DEFAULT 0;

ALTER TABLE subscription_refund_requests
    DROP CONSTRAINT IF EXISTS chk_subscription_refund_request_fee_nonnegative_values;

ALTER TABLE subscription_refund_requests
    ADD CONSTRAINT chk_subscription_refund_request_fee_nonnegative_values
    CHECK (refund_amount >= 0 AND refund_fee_amount >= 0);
