ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS daily_quota_knives DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS weekly_quota_knives DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS monthly_quota_knives DECIMAL(20,10);

ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS plan_id BIGINT,
    ADD COLUMN IF NOT EXISTS plan_name_snapshot VARCHAR(100),
    ADD COLUMN IF NOT EXISTS plan_price_snapshot DECIMAL(20,2),
    ADD COLUMN IF NOT EXISTS daily_quota_knives DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS weekly_quota_knives DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS monthly_quota_knives DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS daily_used_knives DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS weekly_used_knives DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS monthly_used_knives DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS superseded_by_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_plan_id
    ON user_subscriptions(plan_id);

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_superseded_by_id
    ON user_subscriptions(superseded_by_id);

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS subscription_action VARCHAR(20),
    ADD COLUMN IF NOT EXISTS subscription_plan_name_snapshot VARCHAR(100),
    ADD COLUMN IF NOT EXISTS subscription_plan_price_snapshot DECIMAL(20,2),
    ADD COLUMN IF NOT EXISTS subscription_validity_days_snapshot INT,
    ADD COLUMN IF NOT EXISTS subscription_daily_quota_knives_snapshot DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS subscription_weekly_quota_knives_snapshot DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS subscription_monthly_quota_knives_snapshot DECIMAL(20,10);

CREATE INDEX IF NOT EXISTS idx_payment_orders_subscription_action
    ON payment_orders(subscription_action);

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS plan_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_plan_id
    ON redeem_codes(plan_id);
