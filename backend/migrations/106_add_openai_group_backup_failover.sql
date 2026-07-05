-- OpenAI 分组备用组自动熔断切换

ALTER TABLE groups
ADD COLUMN IF NOT EXISTS backup_failover_enabled BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS backup_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS backup_failover_config JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_groups_backup_group_id
ON groups(backup_group_id)
WHERE deleted_at IS NULL AND backup_group_id IS NOT NULL;

ALTER TABLE api_keys
ADD COLUMN IF NOT EXISTS allow_paid_failover BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE usage_logs
ADD COLUMN IF NOT EXISTS origin_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS routed_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS failover_reason VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_usage_logs_origin_group_created_at
ON usage_logs(origin_group_id, created_at)
WHERE origin_group_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_routed_group_created_at
ON usage_logs(routed_group_id, created_at)
WHERE routed_group_id IS NOT NULL;

COMMENT ON COLUMN groups.backup_failover_enabled IS '是否启用 OpenAI 备用分组自动熔断切换';
COMMENT ON COLUMN groups.backup_group_id IS 'OpenAI 自动熔断切换使用的备用分组 ID';
COMMENT ON COLUMN groups.backup_failover_config IS 'OpenAI 备用分组熔断配置';
COMMENT ON COLUMN api_keys.allow_paid_failover IS '是否允许该 Key 自动切换到付费备用分组';
COMMENT ON COLUMN usage_logs.origin_group_id IS 'API Key 原始绑定分组 ID';
COMMENT ON COLUMN usage_logs.routed_group_id IS '本次实际执行分组 ID';
COMMENT ON COLUMN usage_logs.failover_reason IS '触发备用分组路由的原因';
