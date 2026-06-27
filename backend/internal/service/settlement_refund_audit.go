package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

func (s *SettlementRefundService) auditSettlementRefundEvent(
	_ context.Context,
	action string,
	request *SettlementRefundRequestRecord,
	fields map[string]any,
) {
	action = strings.TrimSpace(action)
	if action == "" || request == nil {
		return
	}

	payload := map[string]any{
		"action":          action,
		"refund_request_id": request.ID,
		"user_id":         request.UserID,
		"subscription_id": request.SubscriptionID,
		"settlement_id":   request.SettlementID,
		"status":          request.Status,
		"refund_mode":     request.RefundMode,
		"currency":        request.Currency,
	}
	if request.ExpectedSettlementID > 0 {
		payload["expected_settlement_id"] = request.ExpectedSettlementID
	}
	if request.RefundResidualValue > 0 {
		payload["refund_residual_value"] = request.RefundResidualValue
	}
	if request.GatewayRefundableTotal > 0 {
		payload["gateway_refundable_total"] = request.GatewayRefundableTotal
	}
	if request.ManualTransferAmount > 0 {
		payload["manual_transfer_amount"] = request.ManualTransferAmount
	}
	if request.ManualTransferOperatorUserID != nil {
		payload["manual_transfer_operator_user_id"] = *request.ManualTransferOperatorUserID
	}
	if request.CompletedAt != nil {
		payload["completed_at"] = request.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if request.CancelledAt != nil {
		payload["cancelled_at"] = request.CancelledAt.UTC().Format(time.RFC3339Nano)
	}
	if request.FrozenAt != nil {
		payload["frozen_at"] = request.FrozenAt.UTC().Format(time.RFC3339Nano)
	}

	for key, value := range fields {
		payload[key] = value
	}

	logger.WriteSinkEvent(
		"info",
		"audit.subscription_refund",
		"subscription refund "+action,
		payload,
	)
}

func settlementRefundAuditFieldsFromGatewayResult(result *SettlementRefundGatewayResult) map[string]any {
	if result == nil {
		return nil
	}
	fields := map[string]any{
		"processed_at":            result.ProcessedAt.UTC().Format(time.RFC3339Nano),
		"result_status":           result.Status,
		"succeeded_allocations":   result.SucceededAllocations,
		"failed_allocations":      result.FailedAllocations,
		"skipped_allocations":     result.SkippedAllocations,
		"gateway_refunded_total":  result.GatewayRefundedTotal,
		"manual_transfer_amount":  result.ManualTransferAmount,
	}
	if len(result.Allocations) > 0 {
		allocationSummaries := make([]string, 0, len(result.Allocations))
		for _, allocation := range result.Allocations {
			summary := strings.Join([]string{
				strconv.FormatInt(allocation.AllocationID, 10),
				strconv.FormatInt(allocation.PaymentOrderID, 10),
				allocation.Status,
				strings.TrimSpace(allocation.FailedReason),
			}, ":")
			allocationSummaries = append(allocationSummaries, summary)
		}
		fields["allocation_summaries"] = allocationSummaries
	}
	return fields
}
