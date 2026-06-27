package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

func TestSettlementRefundServiceListSettlementRefundRequestsHydratesAllocationSummary(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM subscription_refund_requests WHERE status NOT IN \('previewed', 'expired'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT\s+id,\s+user_id,\s+subscription_id,\s+settlement_id,`).
		WithArgs(0, 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"subscription_id",
			"settlement_id",
			"expected_settlement_id",
			"status",
			"refund_mode",
			"currency",
			"reason",
			"refund_residual_value",
			"gateway_refundable_total",
			"manual_transfer_amount",
			"preview_token_hash",
			"preview_fingerprint",
			"preview_issued_at",
			"preview_expires_at",
			"submitted_at",
			"frozen_at",
			"completed_at",
			"cancelled_at",
			"original_subscription_status",
			"original_subscription_expires_at",
			"manual_receiver_type",
			"manual_receiver_name",
			"manual_receiver_account",
			"manual_receiver_qr_image_url",
			"manual_receiver_remark",
			"manual_transfer_proof_url",
			"manual_transfer_proof_uploaded_at",
			"manual_transfer_operator_user_id",
			"admin_note",
			"created_at",
			"updated_at",
		}).AddRow(
			int64(9001),
			int64(11),
			int64(22),
			int64(33),
			int64(33),
			SettlementRefundStatusManualPending,
			SettlementRefundModeHybrid,
			"CNY",
			"user requested refund",
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now,
			now.Add(2*time.Minute),
			now,
			now,
			nil,
			nil,
			SubscriptionStatusActive,
			now.Add(24*time.Hour),
			"wechat_qr",
			"zhangsan",
			"wx-account",
			"https://example.com/qr.png",
			"remark",
			nil,
			nil,
			nil,
			nil,
			now,
			now,
		))
	mock.ExpectQuery(`(?s)SELECT\s+id,\s+refund_request_id,\s+payment_order_id,\s+payment_provider_instance_id,`).
		WithArgs(int64(9001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"refund_request_id",
			"payment_order_id",
			"payment_provider_instance_id",
			"order_amount",
			"order_pay_amount",
			"already_refunded_amount",
			"refundable_order_amount",
			"allocated_refund_value",
			"gateway_refund_amount",
			"currency",
			"status",
			"gateway_refund_trade_no",
			"failed_reason",
			"processed_at",
			"created_at",
			"updated_at",
		}).
			AddRow(int64(9101), int64(9001), int64(1001), int64(88), 99.0, 99.0, 0.0, 99.0, 99.0, 99.0, "CNY", SettlementRefundAllocationStatusSucceeded, "refund-1001", nil, now, now, now).
			AddRow(int64(9102), int64(9001), int64(1002), nil, 50.0, 0.0, 0.0, 50.0, 0.0, 0.0, "CNY", SettlementRefundAllocationStatusSkipped, nil, "refund_channel_unavailable", now, now, now).
			AddRow(int64(9103), int64(9001), int64(1003), int64(89), 20.0, 20.0, 0.0, 20.0, 20.0, 20.0, "CNY", SettlementRefundAllocationStatusFailed, nil, "gateway_failed", now, now, now))
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE "users"\."id" IN \(\$1\) AND "users"\."deleted_at" IS NULL`).
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "password_hash", "role", "balance", "concurrency", "status", "username", "notes",
			"totp_secret_encrypted", "totp_enabled", "totp_enabled_at", "signup_source", "last_login_at", "last_active_at",
			"balance_notify_enabled", "balance_notify_threshold_type", "balance_notify_threshold", "balance_notify_extra_emails",
			"total_recharged", "rpm_limit", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			int64(11), "user@example.com", "hash", "user", 0.0, 1, "active", "tester", "",
			nil, false, nil, "email", nil, nil,
			true, "fixed", nil, "[]",
			0.0, 0, now, now, nil,
		))
	mock.ExpectQuery(`SELECT .* FROM "user_subscriptions" WHERE "user_subscriptions"\."id" IN \(\$1\)`).
		WithArgs(int64(22)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "plan_id", "plan_name_snapshot", "plan_price_snapshot", "starts_at", "expires_at", "status",
			"daily_window_start", "weekly_window_start", "monthly_window_start", "daily_usage_usd", "weekly_usage_usd", "monthly_usage_usd",
			"daily_quota_knives", "weekly_quota_knives", "monthly_quota_knives", "daily_used_knives", "weekly_used_knives", "monthly_used_knives",
			"superseded_by_id", "assigned_by", "assigned_at", "notes", "created_at", "updated_at",
		}).AddRow(
			int64(22), int64(11), int64(5), "Pro Plan", 199.0, now.Add(-24*time.Hour), now.Add(24*time.Hour), SubscriptionStatusSuspended,
			now.Add(-24*time.Hour), now.Add(-24*time.Hour), now.Add(-24*time.Hour), 0.0, 0.0, 0.0,
			100.0, 700.0, 3000.0, 10.0, 50.0, 100.0,
			nil, int64(1), now.Add(-24*time.Hour), nil, now, now,
		))
	mock.ExpectQuery(`SELECT .* FROM "subscription_settlement_orders" WHERE "subscription_settlement_orders"\."id" IN \(\$1\)`).
		WithArgs(int64(33)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "prev_settlement_id", "action_type", "action_source", "status", "trigger_ref_type", "trigger_ref_id",
			"operator_user_id", "action_note", "carry_in_residual_value", "action_delta_value", "after_settlement_value", "refund_residual_value",
			"writeoff_value", "after_user_subscription_id", "after_plan_id", "after_plan_name_snapshot", "after_plan_price_snapshot",
			"after_validity_days_snapshot", "after_validity_unit_snapshot", "after_starts_at", "after_expires_at",
			"after_daily_quota_knives_snapshot", "after_weekly_quota_knives_snapshot", "after_monthly_quota_knives_snapshot",
			"after_subscription_status", "effective_at", "closed_at", "created_at", "updated_at",
		}).AddRow(
			int64(33), int64(11), nil, "purchase", "user_purchase", "effective", "payment_order", int64(1001),
			int64(11), "note", 0.0, 199.0, 199.0, nil,
			0.0, int64(22), int64(5), "Pro Plan", 199.0,
			30, "day", now.Add(-24*time.Hour), now.Add(24*time.Hour),
			100.0, 700.0, 3000.0, SubscriptionStatusActive, now.Add(-24*time.Hour), nil, now, now,
		))

	service := NewSettlementRefundService(client, nil, nil)

	views, page, err := service.ListSettlementRefundRequests(context.Background(), pagination.PaginationParams{
		Page:     1,
		PageSize: 20,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Len(t, views, 1)
	require.Equal(t, 99.0, views[0].GatewayRefundedTotal)
	require.Equal(t, 1, views[0].SucceededAllocations)
	require.Equal(t, 1, views[0].FailedAllocations)
	require.Equal(t, 1, views[0].SkippedAllocations)
	require.NoError(t, mock.ExpectationsWereMet())
}
