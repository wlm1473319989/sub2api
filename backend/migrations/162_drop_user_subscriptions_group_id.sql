ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_groups_subscriptions,
    DROP CONSTRAINT IF EXISTS user_subscriptions_group_id_fkey,
    DROP CONSTRAINT IF EXISTS user_subscriptions_user_id_group_id_key;

DROP INDEX IF EXISTS idx_user_subscriptions_group_id;
DROP INDEX IF EXISTS usersubscription_group_id;
DROP INDEX IF EXISTS usersubscription_user_id_group_id;
DROP INDEX IF EXISTS user_subscriptions_user_id_group_id_key;
DROP INDEX IF EXISTS user_subscriptions_user_group_unique_active;

ALTER TABLE user_subscriptions
    DROP COLUMN IF EXISTS group_id;
