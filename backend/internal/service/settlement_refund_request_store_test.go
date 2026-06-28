package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func newSettlementRefundStoreSQLMock(t *testing.T) (*dbent.Client, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func TestSettlementRefundRequestStoreCreateSettlementRefundPreview(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(2 * time.Minute)
	providerID := int64(88)
	reason := "user no longer needs subscription"

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusExpired,
			now,
			int64(22),
			SettlementRefundStatusPreviewed,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)INSERT INTO subscription_refund_requests`).
		WithArgs(
			int64(11),
			int64(22),
			int64(33),
			int64(33),
			SettlementRefundStatusPreviewed,
			SettlementRefundModeHybrid,
			"CNY",
			reason,
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now,
			expiresAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(9001), now, now))
	mock.ExpectQuery(`(?s)INSERT INTO subscription_refund_allocations`).
		WithArgs(
			int64(9001),
			int64(1001),
			providerID,
			99.0,
			99.0,
			0.0,
			99.0,
			99.0,
			99.0,
			"CNY",
			SettlementRefundAllocationStatusPending,
			nil,
			nil,
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(9101), now, now))
	mock.ExpectQuery(`(?s)INSERT INTO subscription_refund_allocations`).
		WithArgs(
			int64(9001),
			int64(1002),
			nil,
			50.0,
			0.0,
			0.0,
			50.0,
			0.0,
			0.0,
			"CNY",
			SettlementRefundAllocationStatusSkipped,
			nil,
			"refund_channel_unavailable",
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(9102), now, now))
	mock.ExpectCommit()

	record, err := store.CreateSettlementRefundPreview(context.Background(), CreateSettlementRefundPreviewInput{
		UserID:                 11,
		SubscriptionID:         22,
		SettlementID:           33,
		ExpectedSettlementID:   33,
		Status:                 SettlementRefundStatusPreviewed,
		RefundMode:             SettlementRefundModeHybrid,
		Currency:               "CNY",
		Reason:                 &reason,
		RefundResidualValue:    168.5,
		GatewayRefundableTotal: 99.0,
		ManualTransferAmount:   69.5,
		PreviewTokenHash:       "preview-hash",
		PreviewFingerprint:     "preview-fingerprint",
		PreviewIssuedAt:        now,
		PreviewExpiresAt:       expiresAt,
		Allocations: []CreateSettlementRefundAllocationInput{
			{
				PaymentOrderID:            1001,
				PaymentProviderInstanceID: &providerID,
				OrderAmount:               99.0,
				OrderPayAmount:            99.0,
				AlreadyRefundedAmount:     0.0,
				RefundableOrderAmount:     99.0,
				AllocatedRefundValue:      99.0,
				GatewayRefundAmount:       99.0,
				Currency:                  "CNY",
				Status:                    SettlementRefundAllocationStatusPending,
			},
			{
				PaymentOrderID:        1002,
				OrderAmount:           50.0,
				OrderPayAmount:        0.0,
				AlreadyRefundedAmount: 0.0,
				RefundableOrderAmount: 50.0,
				AllocatedRefundValue:  0.0,
				GatewayRefundAmount:   0.0,
				Currency:              "CNY",
				FailedReason:          refundRequestPtrString("refund_channel_unavailable"),
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, int64(9001), record.ID)
	require.Equal(t, SettlementRefundModeHybrid, record.RefundMode)
	require.Equal(t, 2, len(record.Allocations))
	require.Equal(t, int64(9101), record.Allocations[0].ID)
	require.Equal(t, SettlementRefundAllocationStatusPending, record.Allocations[0].Status)
	require.Equal(t, SettlementRefundAllocationStatusSkipped, record.Allocations[1].Status)
	require.Equal(t, "refund_channel_unavailable", derefStringPtr(record.Allocations[1].FailedReason))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreCreateSettlementRefundPreviewExpiresPreviousPreviewed(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC)
	expiresAt := now.Add(2 * time.Minute)

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusExpired,
			now,
			int64(22),
			SettlementRefundStatusPreviewed,
		).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`(?s)INSERT INTO subscription_refund_requests`).
		WithArgs(
			int64(11),
			int64(22),
			int64(33),
			int64(33),
			SettlementRefundStatusPreviewed,
			SettlementRefundModeEntitlementOnly,
			"CNY",
			nil,
			120.0,
			0.0,
			0.0,
			"preview-hash",
			"preview-fingerprint",
			now,
			expiresAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(9002), now, now))
	mock.ExpectCommit()

	record, err := store.CreateSettlementRefundPreview(context.Background(), CreateSettlementRefundPreviewInput{
		UserID:               11,
		SubscriptionID:       22,
		SettlementID:         33,
		ExpectedSettlementID: 33,
		Status:               SettlementRefundStatusPreviewed,
		RefundMode:           SettlementRefundModeEntitlementOnly,
		Currency:             "CNY",
		RefundResidualValue:  120.0,
		PreviewTokenHash:     "preview-hash",
		PreviewFingerprint:   "preview-fingerprint",
		PreviewIssuedAt:      now,
		PreviewExpiresAt:     expiresAt,
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, int64(9002), record.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreCreateSettlementRefundPreviewReturnsAlreadyPendingOnProcessingUniqueConflict(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 11, 30, 0, 0, time.UTC)
	expiresAt := now.Add(2 * time.Minute)

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusExpired,
			now,
			int64(22),
			SettlementRefundStatusPreviewed,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)INSERT INTO subscription_refund_requests`).
		WithArgs(
			int64(11),
			int64(22),
			int64(33),
			int64(33),
			SettlementRefundStatusPreviewed,
			SettlementRefundModeHybrid,
			"CNY",
			nil,
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now,
			expiresAt,
		).
		WillReturnError(&pq.Error{
			Code:       "23505",
			Constraint: "idx_subscription_refund_requests_subscription_processing",
		})
	mock.ExpectRollback()

	record, err := store.CreateSettlementRefundPreview(context.Background(), CreateSettlementRefundPreviewInput{
		UserID:                 11,
		SubscriptionID:         22,
		SettlementID:           33,
		ExpectedSettlementID:   33,
		Status:                 SettlementRefundStatusPreviewed,
		RefundMode:             SettlementRefundModeHybrid,
		Currency:               "CNY",
		RefundResidualValue:    168.5,
		GatewayRefundableTotal: 99.0,
		ManualTransferAmount:   69.5,
		PreviewTokenHash:       "preview-hash",
		PreviewFingerprint:     "preview-fingerprint",
		PreviewIssuedAt:        now,
		PreviewExpiresAt:       expiresAt,
	})
	require.Nil(t, record)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSettlementRefundAlreadyPending))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreGetSettlementRefundRequest(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	submittedAt := now.Add(30 * time.Second)
	frozenAt := submittedAt
	operatorID := int64(77)

	mock.ExpectQuery(`(?s)SELECT\s+id,\s+user_id,\s+subscription_id,\s+settlement_id`).
		WithArgs(int64(9001)).
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
			SettlementRefundStatusSubmitted,
			SettlementRefundModeHybrid,
			"CNY",
			"reason text",
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now,
			now.Add(2*time.Minute),
			submittedAt,
			frozenAt,
			nil,
			nil,
			"active",
			now.Add(24*time.Hour),
			"wechat_qr",
			"Zhang San",
			"",
			"uploads/refund/qr/9001.png",
			nil,
			nil,
			nil,
			operatorID,
			"admin note",
			now,
			now,
		))
	mock.ExpectQuery(`(?s)SELECT\s+id,\s+refund_request_id,\s+payment_order_id,\s+payment_provider_instance_id`).
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
			AddRow(int64(9101), int64(9001), int64(1001), int64(88), 99.0, 99.0, 0.0, 99.0, 99.0, 99.0, "CNY", SettlementRefundAllocationStatusSucceeded, "refund-trade-no", nil, submittedAt, now, now).
			AddRow(int64(9102), int64(9001), int64(1002), nil, 50.0, 0.0, 0.0, 50.0, 0.0, 0.0, "CNY", SettlementRefundAllocationStatusSkipped, nil, "refund_channel_unavailable", nil, now, now))

	record, err := store.GetSettlementRefundRequest(context.Background(), 9001)
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, int64(9001), record.ID)
	require.Equal(t, SettlementRefundStatusSubmitted, record.Status)
	require.Equal(t, "reason text", derefStringPtr(record.Reason))
	require.Equal(t, "active", derefStringPtr(record.OriginalSubscriptionStatus))
	require.Equal(t, int64(77), derefInt64Ptr(record.ManualTransferOperatorUserID))
	require.Equal(t, 2, len(record.Allocations))
	require.Equal(t, "refund-trade-no", derefStringPtr(record.Allocations[0].GatewayRefundTradeNo))
	require.Equal(t, "refund_channel_unavailable", derefStringPtr(record.Allocations[1].FailedReason))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreCreateSettlementRefundPreviewRejectsInvalidInput(t *testing.T) {
	store := newSettlementRefundRequestStore(nil)
	_, err := store.CreateSettlementRefundPreview(context.Background(), CreateSettlementRefundPreviewInput{})
	require.ErrorIs(t, err, ErrSettlementRefundPreviewInput)
}

func TestSettlementRefundRequestStoreCreateSettlementRefundPreviewRequiresStore(t *testing.T) {
	store := newSettlementRefundRequestStore(nil)
	now := time.Date(2026, 6, 25, 14, 0, 0, 0, time.UTC)

	_, err := store.CreateSettlementRefundPreview(context.Background(), CreateSettlementRefundPreviewInput{
		UserID:               1,
		SubscriptionID:       2,
		SettlementID:         3,
		ExpectedSettlementID: 3,
		RefundMode:           SettlementRefundModeGatewayRefund,
		PreviewTokenHash:     "preview-hash",
		PreviewFingerprint:   "preview-fingerprint",
		PreviewIssuedAt:      now,
		PreviewExpiresAt:     now.Add(2 * time.Minute),
	})
	require.ErrorIs(t, err, ErrSettlementRefundStoreRequired)
}

func TestSettlementRefundRequestStoreSubmitSettlementRefundPreview(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC)
	expiresAt := now.Add(90 * time.Second)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusSubmitted,
			now,
			now,
			SubscriptionStatusActive,
			now.Add(24*time.Hour),
			"wechat_qr",
			"Zhang San",
			"",
			"uploads/refund/qr/9001.png",
			nil,
			int64(9001),
			SettlementRefundStatusPreviewed,
			now,
		).
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
			SettlementRefundStatusSubmitted,
			SettlementRefundModeHybrid,
			"CNY",
			"reason text",
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now.Add(-30*time.Second),
			expiresAt,
			now,
			now,
			nil,
			nil,
			SubscriptionStatusActive,
			now.Add(24*time.Hour),
			"wechat_qr",
			"Zhang San",
			"",
			"uploads/refund/qr/9001.png",
			"bank note",
			nil,
			nil,
			nil,
			nil,
			now.Add(-30*time.Second),
			now,
		))
	mock.ExpectCommit()

	record, err := store.SubmitSettlementRefundPreview(context.Background(), SubmitSettlementRefundPreviewInput{
		RequestID:                     9001,
		ExpectedStatus:                SettlementRefundStatusPreviewed,
		SubmittedAt:                   now,
		PreviewNotExpiredAfter:        now,
		FrozenAt:                      now,
		OriginalSubscriptionStatus:    SubscriptionStatusActive,
		OriginalSubscriptionExpiresAt: now.Add(24 * time.Hour),
		ManualReceiverType:            refundRequestPtrString("wechat_qr"),
		ManualReceiverName:            refundRequestPtrString("Zhang San"),
		ManualReceiverAccount:         refundRequestPtrString(""),
		ManualReceiverQRCodeImageURL:  refundRequestPtrString("uploads/refund/qr/9001.png"),
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, SettlementRefundStatusSubmitted, record.Status)
	require.Equal(t, SubscriptionStatusActive, derefStringPtr(record.OriginalSubscriptionStatus))
	require.NotNil(t, record.FrozenAt)
	require.Equal(t, "wechat_qr", derefStringPtr(record.ManualReceiverType))
	require.Equal(t, "bank note", derefStringPtr(record.ManualReceiverRemark))
	require.Equal(t, "Zhang San", derefStringPtr(record.ManualReceiverName))
	require.Equal(t, "uploads/refund/qr/9001.png", derefStringPtr(record.ManualReceiverQRCodeImageURL))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreUpdateSettlementRefundManualProof(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 18, 0, 0, 0, time.UTC)
	proofURL := "uploads/refund/proof/9001.png"
	operatorID := int64(88)
	adminNote := "manual transfer completed"

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusManualPending,
			proofURL,
			now,
			operatorID,
			adminNote,
			int64(9001),
			SettlementRefundStatusSubmitted,
		).
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
			"reason text",
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now.Add(-2*time.Minute),
			now.Add(2*time.Minute),
			now.Add(-90*time.Second),
			now.Add(-90*time.Second),
			nil,
			nil,
			SubscriptionStatusActive,
			now.Add(24*time.Hour),
			"wechat_qr",
			"Zhang San",
			"",
			"uploads/refund/qr/9001.png",
			"bank note",
			proofURL,
			now,
			operatorID,
			adminNote,
			now.Add(-2*time.Minute),
			now,
		))
	mock.ExpectCommit()

	record, err := store.UpdateSettlementRefundManualProof(context.Background(), UpdateSettlementRefundManualProofInput{
		RequestID:      9001,
		ExpectedStatus: SettlementRefundStatusSubmitted,
		Status:         SettlementRefundStatusManualPending,
		ProofURL:       proofURL,
		UploadedAt:     now,
		OperatorUserID: operatorID,
		AdminNote:      &adminNote,
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, SettlementRefundStatusManualPending, record.Status)
	require.Equal(t, "bank note", derefStringPtr(record.ManualReceiverRemark))
	require.Equal(t, proofURL, derefStringPtr(record.ManualTransferProofURL))
	require.Equal(t, operatorID, derefInt64Ptr(record.ManualTransferOperatorUserID))
	require.Equal(t, adminNote, derefStringPtr(record.AdminNote))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreCompleteSettlementRefundRequest(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 19, 30, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusCompleted,
			now,
			int64(9001),
			SettlementRefundStatusManualPending,
		).
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
			SettlementRefundStatusCompleted,
			SettlementRefundModeHybrid,
			"CNY",
			"reason text",
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now.Add(-5*time.Minute),
			now.Add(-3*time.Minute),
			now.Add(-4*time.Minute),
			now.Add(-4*time.Minute),
			now,
			nil,
			SubscriptionStatusActive,
			now.Add(24*time.Hour),
			"wechat_qr",
			"Zhang San",
			"",
			"uploads/refund/qr/9001.png",
			"bank note",
			"uploads/refund/proof/9001.png",
			now.Add(-30*time.Second),
			int64(88),
			"manual transfer completed",
			now.Add(-5*time.Minute),
			now,
		))
	mock.ExpectCommit()

	record, err := store.CompleteSettlementRefundRequest(context.Background(), CompleteSettlementRefundRequestInput{
		RequestID:      9001,
		ExpectedStatus: SettlementRefundStatusManualPending,
		CompletedAt:    now,
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, SettlementRefundStatusCompleted, record.Status)
	require.NotNil(t, record.CompletedAt)
	require.Equal(t, "bank note", derefStringPtr(record.ManualReceiverRemark))
	require.Equal(t, "uploads/refund/proof/9001.png", derefStringPtr(record.ManualTransferProofURL))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreCancelSettlementRefundRequest(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 20, 30, 0, 0, time.UTC)
	adminNote := "cancel before payout"

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusCancelled,
			now,
			adminNote,
			int64(9001),
			SettlementRefundStatusSubmitted,
		).
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
			SettlementRefundStatusCancelled,
			SettlementRefundModeHybrid,
			"CNY",
			"reason text",
			168.5,
			99.0,
			69.5,
			"preview-hash",
			"preview-fingerprint",
			now.Add(-6*time.Minute),
			now.Add(-4*time.Minute),
			now.Add(-5*time.Minute),
			now.Add(-5*time.Minute),
			nil,
			now,
			SubscriptionStatusActive,
			now.Add(24*time.Hour),
			"wechat_qr",
			"Zhang San",
			"",
			"uploads/refund/qr/9001.png",
			"bank note",
			nil,
			nil,
			nil,
			adminNote,
			now.Add(-6*time.Minute),
			now,
		))
	mock.ExpectCommit()

	record, err := store.CancelSettlementRefundRequest(context.Background(), CancelSettlementRefundRequestInput{
		RequestID:      9001,
		ExpectedStatus: SettlementRefundStatusSubmitted,
		CancelledAt:    now,
		AdminNote:      &adminNote,
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, SettlementRefundStatusCancelled, record.Status)
	require.NotNil(t, record.CancelledAt)
	require.Equal(t, "bank note", derefStringPtr(record.ManualReceiverRemark))
	require.Equal(t, adminNote, derefStringPtr(record.AdminNote))
	require.NoError(t, mock.ExpectationsWereMet())
}

func refundRequestPtrString(v string) *string {
	return &v
}

func derefStringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefInt64Ptr(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
