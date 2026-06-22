ALTER TABLE user_subscriptions
    ALTER COLUMN group_id DROP NOT NULL;

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_groups_subscriptions;

ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_groups_subscriptions
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL;
