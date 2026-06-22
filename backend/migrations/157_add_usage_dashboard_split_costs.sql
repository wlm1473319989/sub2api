ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS subscription_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS balance_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS subscription_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS balance_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;
