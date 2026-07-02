ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS purchase_limit_per_user INTEGER;
