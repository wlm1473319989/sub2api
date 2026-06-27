package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSettlementRefundRequestStoreUpdateSettlementRefundRequestStatus(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 21, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE subscription_refund_requests`).
		WithArgs(
			SettlementRefundStatusGatewayProcessing,
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
			SettlementRefundStatusGatewayProcessing,
			SettlementRefundModeHybrid,
			"CNY",
			"gateway retry",
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
			"active",
			now.Add(24*time.Hour),
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			now.Add(-2*time.Minute),
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
		}).AddRow(
			int64(9101),
			int64(9001),
			int64(1001),
			int64(88),
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
			now.Add(-time.Minute),
			now,
		))
	mock.ExpectCommit()

	record, err := store.UpdateSettlementRefundRequestStatus(context.Background(), UpdateSettlementRefundRequestStatusInput{
		RequestID:      9001,
		ExpectedStatus: SettlementRefundStatusSubmitted,
		Status:         SettlementRefundStatusGatewayProcessing,
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, SettlementRefundStatusGatewayProcessing, record.Status)
	require.Len(t, record.Allocations, 1)
	require.Equal(t, int64(9101), record.Allocations[0].ID)
	require.Equal(t, SettlementRefundAllocationStatusPending, record.Allocations[0].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettlementRefundRequestStoreUpdateSettlementRefundAllocationStatus(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	store := newSettlementRefundRequestStore(client)

	now := time.Date(2026, 6, 25, 21, 30, 0, 0, time.UTC)
	tradeNo := "refund-trade-no"

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)UPDATE subscription_refund_allocations`).
		WithArgs(
			SettlementRefundAllocationStatusSucceeded,
			tradeNo,
			nil,
			now,
			int64(9101),
			SettlementRefundAllocationStatusProcessing,
		).
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
		}).AddRow(
			int64(9101),
			int64(9001),
			int64(1001),
			int64(88),
			99.0,
			99.0,
			0.0,
			99.0,
			99.0,
			99.0,
			"CNY",
			SettlementRefundAllocationStatusSucceeded,
			tradeNo,
			nil,
			now,
			now.Add(-time.Minute),
			now,
		))
	mock.ExpectCommit()

	record, err := store.UpdateSettlementRefundAllocationStatus(context.Background(), UpdateSettlementRefundAllocationStatusInput{
		AllocationID:         9101,
		ExpectedStatus:       SettlementRefundAllocationStatusProcessing,
		Status:               SettlementRefundAllocationStatusSucceeded,
		GatewayRefundTradeNo: &tradeNo,
		ProcessedAt:          &now,
	})
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, SettlementRefundAllocationStatusSucceeded, record.Status)
	require.Equal(t, tradeNo, derefStringPtr(record.GatewayRefundTradeNo))
	require.NoError(t, mock.ExpectationsWereMet())
}
