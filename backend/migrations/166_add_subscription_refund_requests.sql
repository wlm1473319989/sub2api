-- Add settlement-based subscription refund request tables.
-- This migration only creates the additive data model used by the staged
-- settlement refund workflow; business paths are wired in later steps.

CREATE TABLE IF NOT EXISTS subscription_refund_requests (
    id                                      BIGSERIAL PRIMARY KEY,
    user_id                                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id                         BIGINT NOT NULL REFERENCES user_subscriptions(id) ON DELETE RESTRICT,
    settlement_id                           BIGINT NOT NULL REFERENCES subscription_settlement_orders(id) ON DELETE RESTRICT,
    expected_settlement_id                  BIGINT NOT NULL REFERENCES subscription_settlement_orders(id) ON DELETE RESTRICT,

    status                                  VARCHAR(32) NOT NULL DEFAULT 'previewed',
    refund_mode                             VARCHAR(32) NOT NULL,
    currency                                VARCHAR(10),
    reason                                  TEXT,

    refund_residual_value                   NUMERIC(20, 8) NOT NULL DEFAULT 0,
    gateway_refundable_total                NUMERIC(20, 8) NOT NULL DEFAULT 0,
    manual_transfer_amount                  NUMERIC(20, 8) NOT NULL DEFAULT 0,

    preview_token_hash                      VARCHAR(128) NOT NULL,
    preview_issued_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    preview_expires_at                      TIMESTAMPTZ NOT NULL,
    submitted_at                            TIMESTAMPTZ,
    frozen_at                               TIMESTAMPTZ,
    completed_at                            TIMESTAMPTZ,
    cancelled_at                            TIMESTAMPTZ,

    original_subscription_status            VARCHAR(20),
    original_subscription_expires_at        TIMESTAMPTZ,

    manual_receiver_type                    VARCHAR(32),
    manual_receiver_name                    VARCHAR(100),
    manual_receiver_account                 VARCHAR(255),
    manual_receiver_qr_image_url            TEXT,
    manual_transfer_proof_url               TEXT,
    manual_transfer_proof_uploaded_at       TIMESTAMPTZ,
    manual_transfer_operator_user_id        BIGINT REFERENCES users(id) ON DELETE SET NULL,

    admin_note                              TEXT,
    created_at                              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_subscription_refund_request_status
        CHECK (status IN (
            'previewed',
            'expired',
            'submitted',
            'gateway_processing',
            'manual_pending',
            'completed',
            'failed',
            'cancelled'
        )),
    CONSTRAINT chk_subscription_refund_request_mode
        CHECK (refund_mode IN ('gateway_refund', 'manual_transfer', 'hybrid', 'entitlement_only')),
    CONSTRAINT chk_subscription_refund_request_nonnegative_values
        CHECK (
            refund_residual_value >= 0
            AND gateway_refundable_total >= 0
            AND manual_transfer_amount >= 0
        ),
    CONSTRAINT chk_subscription_refund_request_preview_window
        CHECK (preview_expires_at > preview_issued_at),
    CONSTRAINT chk_subscription_refund_request_completion_status
        CHECK (
            (status = 'completed' AND completed_at IS NOT NULL)
            OR status <> 'completed'
        ),
    CONSTRAINT chk_subscription_refund_request_cancel_status
        CHECK (
            (status = 'cancelled' AND cancelled_at IS NOT NULL)
            OR status <> 'cancelled'
        )
);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_requests_user_id
    ON subscription_refund_requests(user_id);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_requests_subscription_id
    ON subscription_refund_requests(subscription_id);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_requests_settlement_id
    ON subscription_refund_requests(settlement_id);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_requests_status
    ON subscription_refund_requests(status);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_requests_preview_expires_at
    ON subscription_refund_requests(preview_expires_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_refund_requests_subscription_open
    ON subscription_refund_requests(subscription_id)
    WHERE status IN ('previewed', 'submitted', 'gateway_processing', 'manual_pending', 'failed');

CREATE TABLE IF NOT EXISTS subscription_refund_allocations (
    id                                      BIGSERIAL PRIMARY KEY,
    refund_request_id                       BIGINT NOT NULL REFERENCES subscription_refund_requests(id) ON DELETE CASCADE,
    payment_order_id                        BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    payment_provider_instance_id            BIGINT REFERENCES payment_provider_instances(id) ON DELETE SET NULL,

    order_amount                            NUMERIC(20, 8) NOT NULL DEFAULT 0,
    order_pay_amount                        NUMERIC(20, 8) NOT NULL DEFAULT 0,
    already_refunded_amount                 NUMERIC(20, 8) NOT NULL DEFAULT 0,
    refundable_order_amount                 NUMERIC(20, 8) NOT NULL DEFAULT 0,
    allocated_refund_value                  NUMERIC(20, 8) NOT NULL DEFAULT 0,
    gateway_refund_amount                   NUMERIC(20, 8) NOT NULL DEFAULT 0,
    currency                                VARCHAR(10),

    status                                  VARCHAR(32) NOT NULL DEFAULT 'pending',
    gateway_refund_trade_no                 VARCHAR(128),
    failed_reason                           TEXT,
    processed_at                            TIMESTAMPTZ,
    created_at                              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_subscription_refund_allocation_status
        CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'skipped')),
    CONSTRAINT chk_subscription_refund_allocation_nonnegative_values
        CHECK (
            order_amount >= 0
            AND order_pay_amount >= 0
            AND already_refunded_amount >= 0
            AND refundable_order_amount >= 0
            AND allocated_refund_value >= 0
            AND gateway_refund_amount >= 0
        ),
    CONSTRAINT chk_subscription_refund_allocation_caps
        CHECK (
            allocated_refund_value <= refundable_order_amount
            AND gateway_refund_amount <= order_pay_amount
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_refund_allocations_request_order
    ON subscription_refund_allocations(refund_request_id, payment_order_id);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_allocations_request_id
    ON subscription_refund_allocations(refund_request_id);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_allocations_payment_order_id
    ON subscription_refund_allocations(payment_order_id);

CREATE INDEX IF NOT EXISTS idx_subscription_refund_allocations_status
    ON subscription_refund_allocations(status);
