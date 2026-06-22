ALTER TABLE payment_orders
    DROP COLUMN IF EXISTS subscription_group_id,
    DROP COLUMN IF EXISTS subscription_days;
