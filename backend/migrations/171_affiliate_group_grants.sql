-- Invite-payment grants for temporary exclusive group access.
--
-- These rows are source-order scoped and do not mutate users.allowed_groups.
-- The request path must always enforce expires_at > NOW(); expired_processed_at
-- is only for cache invalidation/maintenance bookkeeping.

CREATE TABLE IF NOT EXISTS affiliate_group_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    source_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    source_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE CASCADE,
    pay_amount DECIMAL(20,2) NOT NULL,
    grant_days INTEGER NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    expired_processed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT affiliate_group_grants_grant_days_positive CHECK (grant_days > 0),
    CONSTRAINT affiliate_group_grants_pay_amount_positive CHECK (pay_amount > 0),
    CONSTRAINT affiliate_group_grants_period_valid CHECK (expires_at > starts_at),
    CONSTRAINT affiliate_group_grants_source_order_unique UNIQUE (user_id, group_id, source_order_id)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_group_grants_user_group_expires
    ON affiliate_group_grants(user_id, group_id, expires_at);

CREATE INDEX IF NOT EXISTS idx_affiliate_group_grants_active_user
    ON affiliate_group_grants(user_id, expires_at)
    WHERE expired_processed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_affiliate_group_grants_expiry_unprocessed
    ON affiliate_group_grants(expires_at)
    WHERE expired_processed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_affiliate_group_grants_source_user_id
    ON affiliate_group_grants(source_user_id)
    WHERE source_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_affiliate_group_grants_source_order_id
    ON affiliate_group_grants(source_order_id);

COMMENT ON TABLE affiliate_group_grants IS '邀请支付赠送的限时专属分组权限，不写入 users.allowed_groups';
COMMENT ON COLUMN affiliate_group_grants.user_id IS '获得专属分组限时权限的邀请人用户ID';
COMMENT ON COLUMN affiliate_group_grants.group_id IS '被赠送访问权的专属分组ID';
COMMENT ON COLUMN affiliate_group_grants.source_user_id IS '触发赠送的被邀请人用户ID';
COMMENT ON COLUMN affiliate_group_grants.source_order_id IS '触发赠送的支付订单ID，用于幂等';
COMMENT ON COLUMN affiliate_group_grants.pay_amount IS '订单实际支付金额，grant_days 基于该金额计算';
COMMENT ON COLUMN affiliate_group_grants.grant_days IS '赠送自然日数：max(1, floor(pay_amount / 10))';
COMMENT ON COLUMN affiliate_group_grants.expires_at IS '权限失效时间；请求路径必须按 expires_at > NOW() 判定';
COMMENT ON COLUMN affiliate_group_grants.expired_processed_at IS '过期扫描已处理时间，仅用于清理认证缓存兜底';
