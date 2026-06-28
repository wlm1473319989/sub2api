ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS subscription_rate_multiplier DECIMAL(10, 4) NOT NULL DEFAULT 1.0;

UPDATE groups
SET subscription_rate_multiplier = rate_multiplier
WHERE subscription_rate_multiplier IS NULL
   OR subscription_rate_multiplier = 1.0;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS subscription_rate_multiplier DECIMAL(10, 4) NOT NULL DEFAULT 1.0;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS balance_rate_multiplier DECIMAL(10, 4) NOT NULL DEFAULT 1.0;

UPDATE usage_logs
SET subscription_rate_multiplier = rate_multiplier,
    balance_rate_multiplier = rate_multiplier
WHERE (subscription_rate_multiplier IS NULL OR subscription_rate_multiplier = 1.0)
  AND (balance_rate_multiplier IS NULL OR balance_rate_multiplier = 1.0)
  AND rate_multiplier IS NOT NULL;
