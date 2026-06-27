-- Allow admin subscription revocation to write a settlement order.
-- Revoke settlement orders use:
--   action_type = 'revoke'
--   action_source = 'admin_revoke'
--   trigger_ref_type = 'direct_action'
--   after_subscription_status = 'revoked'

ALTER TABLE subscription_settlement_orders
    DROP CONSTRAINT IF EXISTS chk_subscription_settlement_action_type,
    ADD CONSTRAINT chk_subscription_settlement_action_type
        CHECK (action_type IN ('purchase', 'renew', 'upgrade', 'refund', 'revoke'));

ALTER TABLE subscription_settlement_orders
    DROP CONSTRAINT IF EXISTS chk_subscription_settlement_action_source,
    ADD CONSTRAINT chk_subscription_settlement_action_source
        CHECK (action_source IN ('user_purchase', 'exchange_code', 'subscription_assign', 'admin_revoke'));

ALTER TABLE subscription_settlement_orders
    DROP CONSTRAINT IF EXISTS chk_subscription_settlement_after_status,
    ADD CONSTRAINT chk_subscription_settlement_after_status
        CHECK (after_subscription_status IN ('active', 'refunded', 'revoked'));

ALTER TABLE subscription_settlement_orders
    DROP CONSTRAINT IF EXISTS chk_subscription_settlement_source_trigger,
    ADD CONSTRAINT chk_subscription_settlement_source_trigger
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
            OR (action_source = 'admin_revoke' AND trigger_ref_type = 'direct_action' AND trigger_ref_id IS NULL)
        );
