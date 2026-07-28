CREATE TABLE IF NOT EXISTS recharge_bonus_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INTEGER NOT NULL DEFAULT 0,
    min_amount DECIMAL(20,2) NOT NULL,
    max_amount DECIMAL(20,2) NULL,
    bonus_type VARCHAR(20) NOT NULL,
    bonus_value DECIMAL(20,4) NOT NULL,
    starts_at TIMESTAMPTZ NULL,
    ends_at TIMESTAMPTZ NULL,
    notes TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT recharge_bonus_rules_amount_valid CHECK (min_amount > 0 AND (max_amount IS NULL OR max_amount >= min_amount)),
    CONSTRAINT recharge_bonus_rules_type_valid CHECK (bonus_type IN ('fixed', 'percentage')),
    CONSTRAINT recharge_bonus_rules_value_valid CHECK (bonus_value > 0 AND (bonus_type <> 'percentage' OR bonus_value <= 100)),
    CONSTRAINT recharge_bonus_rules_period_valid CHECK (starts_at IS NULL OR ends_at IS NULL OR ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS idx_recharge_bonus_rules_enabled_priority
    ON recharge_bonus_rules(enabled, priority DESC);
CREATE INDEX IF NOT EXISTS idx_recharge_bonus_rules_schedule
    ON recharge_bonus_rules(starts_at, ends_at);

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS recharge_principal DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS recharge_bonus DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS recharge_bonus_rule_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS recharge_bonus_snapshot JSONB NULL,
    ADD COLUMN IF NOT EXISTS refunded_bonus_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refunded_principal_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refunded_gateway_amount DECIMAL(20,8) NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'payment_orders_recharge_bonus_rule_id_fkey'
    ) THEN
        ALTER TABLE payment_orders
            ADD CONSTRAINT payment_orders_recharge_bonus_rule_id_fkey
            FOREIGN KEY (recharge_bonus_rule_id) REFERENCES recharge_bonus_rules(id) ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_payment_orders_recharge_bonus_rule_id
    ON payment_orders(recharge_bonus_rule_id);
