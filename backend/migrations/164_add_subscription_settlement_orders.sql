-- Add subscription settlement order chain for subscription accounting.
-- This migration only creates the additive data model; business paths are wired in later steps.

CREATE TABLE IF NOT EXISTS subscription_settlement_orders (
    id                                      BIGSERIAL PRIMARY KEY,
    user_id                                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prev_settlement_id                      BIGINT REFERENCES subscription_settlement_orders(id) ON DELETE RESTRICT,

    action_type                             VARCHAR(32) NOT NULL,
    action_source                           VARCHAR(32) NOT NULL,
    status                                  VARCHAR(16) NOT NULL DEFAULT 'effective',
    trigger_ref_type                        VARCHAR(32) NOT NULL,
    trigger_ref_id                          BIGINT,
    operator_user_id                        BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    action_note                             TEXT,

    carry_in_residual_value                 NUMERIC(20, 8) NOT NULL DEFAULT 0,
    action_delta_value                      NUMERIC(20, 8) NOT NULL DEFAULT 0,
    after_settlement_value                  NUMERIC(20, 8) NOT NULL DEFAULT 0,
    refund_residual_value                   NUMERIC(20, 8),
    writeoff_value                          NUMERIC(20, 8) NOT NULL DEFAULT 0,

    after_user_subscription_id              BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL,
    after_plan_id                           BIGINT REFERENCES subscription_plans(id) ON DELETE SET NULL,
    after_plan_name_snapshot                VARCHAR(100),
    after_plan_price_snapshot               NUMERIC(20, 8),
    after_validity_days_snapshot            INT,
    after_validity_unit_snapshot            VARCHAR(16),
    after_starts_at                         TIMESTAMPTZ,
    after_expires_at                        TIMESTAMPTZ,
    after_daily_quota_knives_snapshot       NUMERIC(20, 10),
    after_weekly_quota_knives_snapshot      NUMERIC(20, 10),
    after_monthly_quota_knives_snapshot     NUMERIC(20, 10),
    after_subscription_status               VARCHAR(16) NOT NULL,

    effective_at                            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at                               TIMESTAMPTZ,
    created_at                              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_subscription_settlement_action_type
        CHECK (action_type IN ('purchase', 'renew', 'upgrade', 'refund')),
    CONSTRAINT chk_subscription_settlement_action_source
        CHECK (action_source IN ('user_purchase', 'exchange_code', 'subscription_assign')),
    CONSTRAINT chk_subscription_settlement_status
        CHECK (status IN ('effective', 'closed')),
    CONSTRAINT chk_subscription_settlement_trigger_ref_type
        CHECK (trigger_ref_type IN ('payment_order', 'redeem_code', 'admin_assignment', 'direct_action')),
    CONSTRAINT chk_subscription_settlement_after_status
        CHECK (after_subscription_status IN ('active', 'refunded')),
    CONSTRAINT chk_subscription_settlement_source_trigger
        CHECK (
            (
                action_source = 'user_purchase'
                AND (
                    (trigger_ref_type = 'payment_order' AND trigger_ref_id IS NOT NULL)
                    OR (trigger_ref_type = 'direct_action' AND trigger_ref_id IS NULL)
                )
            )
            OR (action_source = 'exchange_code' AND trigger_ref_type = 'redeem_code' AND trigger_ref_id IS NOT NULL)
            OR (action_source = 'subscription_assign' AND trigger_ref_type = 'admin_assignment')
        ),
    CONSTRAINT chk_subscription_settlement_closed_at
        CHECK (
            (status = 'effective' AND closed_at IS NULL)
            OR (status = 'closed' AND closed_at IS NOT NULL)
        ),
    CONSTRAINT chk_subscription_settlement_nonnegative_values
        CHECK (
            carry_in_residual_value >= 0
            AND after_settlement_value >= 0
            AND writeoff_value >= 0
            AND (refund_residual_value IS NULL OR refund_residual_value >= 0)
        ),
    CONSTRAINT chk_subscription_settlement_refund_value
        CHECK (
            (action_type = 'refund' AND refund_residual_value IS NOT NULL)
            OR (action_type <> 'refund' AND refund_residual_value IS NULL)
        )
);

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_user_id
    ON subscription_settlement_orders(user_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_settlement_orders_prev_unique
    ON subscription_settlement_orders(prev_settlement_id)
    WHERE prev_settlement_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_status
    ON subscription_settlement_orders(status);

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_action_type
    ON subscription_settlement_orders(action_type);

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_action_source
    ON subscription_settlement_orders(action_source);

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_trigger
    ON subscription_settlement_orders(trigger_ref_type, trigger_ref_id);

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_after_subscription
    ON subscription_settlement_orders(after_user_subscription_id);

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_after_plan
    ON subscription_settlement_orders(after_plan_id);

CREATE INDEX IF NOT EXISTS idx_subscription_settlement_orders_effective_at
    ON subscription_settlement_orders(effective_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_settlement_orders_user_effective
    ON subscription_settlement_orders(user_id)
    WHERE status = 'effective';
