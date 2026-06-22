DROP INDEX IF EXISTS idx_groups_subscription_type;

ALTER TABLE groups
    DROP COLUMN IF EXISTS subscription_type,
    DROP COLUMN IF EXISTS daily_limit_usd,
    DROP COLUMN IF EXISTS weekly_limit_usd,
    DROP COLUMN IF EXISTS monthly_limit_usd,
    DROP COLUMN IF EXISTS default_validity_days;
