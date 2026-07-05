-- Normalize current subscription entitlements to natural-day boundaries.
--
-- The old purchase flow used the payment timestamp as starts_at and
-- starts_at + validity_days as expires_at. Quota windows are reset at day
-- boundaries, so an active subscription could receive an extra quota window
-- on its last calendar day. Keep historical closed records unchanged, and
-- only normalize records that can still affect entitlement or refund recovery.

WITH normalized AS (
    SELECT
        id,
        date_trunc('day', starts_at) AS normalized_starts_at,
        date_trunc('day', starts_at)
            + ((expires_at::date - starts_at::date) * INTERVAL '1 day') AS normalized_expires_at
    FROM user_subscriptions
    WHERE deleted_at IS NULL
      AND status IN ('active', 'suspended')
      AND expires_at > starts_at
      AND expires_at::date > starts_at::date
      AND expires_at < TIMESTAMPTZ '2099-12-31 23:59:59+00'
      AND (
          starts_at IS DISTINCT FROM date_trunc('day', starts_at)
          OR expires_at IS DISTINCT FROM (
              date_trunc('day', starts_at)
                  + ((expires_at::date - starts_at::date) * INTERVAL '1 day')
          )
      )
)
UPDATE user_subscriptions AS us
SET
    starts_at = normalized.normalized_starts_at,
    expires_at = normalized.normalized_expires_at,
    updated_at = NOW()
FROM normalized
WHERE us.id = normalized.id;

-- Keep the effective settlement head aligned with the normalized current
-- entitlement. Closed settlement orders are immutable audit history.
UPDATE subscription_settlement_orders AS sso
SET
    after_starts_at = us.starts_at,
    after_expires_at = us.expires_at,
    updated_at = NOW()
FROM user_subscriptions AS us
WHERE sso.after_user_subscription_id = us.id
  AND sso.status = 'effective'
  AND us.deleted_at IS NULL
  AND us.status IN ('active', 'suspended')
  AND (
      sso.after_starts_at IS DISTINCT FROM us.starts_at
      OR sso.after_expires_at IS DISTINCT FROM us.expires_at
  );

-- Submitted refund requests restore this field when a refund is cancelled.
-- Align cancellable requests so cancellation cannot bring back the old
-- timestamp-based expiry.
UPDATE subscription_refund_requests AS srr
SET
    original_subscription_expires_at = us.expires_at,
    updated_at = NOW()
FROM user_subscriptions AS us
WHERE srr.subscription_id = us.id
  AND us.deleted_at IS NULL
  AND us.status = 'suspended'
  AND srr.status IN ('submitted', 'gateway_processing', 'manual_pending', 'failed')
  AND srr.original_subscription_expires_at IS NOT NULL
  AND srr.original_subscription_expires_at IS DISTINCT FROM us.expires_at;
