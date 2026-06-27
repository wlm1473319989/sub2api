ALTER TABLE subscription_refund_requests
    ADD COLUMN IF NOT EXISTS preview_fingerprint VARCHAR(128);
