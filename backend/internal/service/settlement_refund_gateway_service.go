package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSettlementRefundGatewayInput               = infraerrors.BadRequest("SETTLEMENT_REFUND_GATEWAY_INPUT_INVALID", "settlement refund gateway input is invalid")
	ErrSettlementRefundGatewayState               = infraerrors.Conflict("SETTLEMENT_REFUND_GATEWAY_STATE_INVALID", "settlement refund gateway state is invalid")
	ErrSettlementRefundGatewayNotRequired         = infraerrors.BadRequest("SETTLEMENT_REFUND_GATEWAY_NOT_REQUIRED", "settlement refund does not require gateway processing")
	ErrSettlementRefundGatewayPaymentOrderMissing = infraerrors.InternalServer("SETTLEMENT_REFUND_GATEWAY_PAYMENT_ORDER_MISSING", "settlement refund gateway payment order is missing")
	ErrSettlementRefundGatewayProviderMissing     = infraerrors.InternalServer("SETTLEMENT_REFUND_GATEWAY_PROVIDER_MISSING", "settlement refund gateway provider is missing")
)

type settlementRefundGatewayStore interface {
	GetSettlementRefundRequest(context.Context, int64) (*SettlementRefundRequestRecord, error)
	UpdateSettlementRefundRequestStatus(context.Context, UpdateSettlementRefundRequestStatusInput) (*SettlementRefundRequestRecord, error)
	UpdateSettlementRefundAllocationStatus(context.Context, UpdateSettlementRefundAllocationStatusInput) (*SettlementRefundAllocationRecord, error)
}

type SettlementRefundGatewayInput struct {
	RefundRequestID int64
	OperatorUserID  int64
}

type SettlementRefundGatewayAllocationResult struct {
	AllocationID         int64   `json:"allocation_id"`
	PaymentOrderID       int64   `json:"payment_order_id"`
	Status               string  `json:"status"`
	GatewayRefundAmount  float64 `json:"gateway_refund_amount"`
	GatewayRefundTradeNo string  `json:"gateway_refund_trade_no,omitempty"`
	FailedReason         string  `json:"failed_reason,omitempty"`
}

type SettlementRefundGatewayResult struct {
	RefundRequestID      int64                                     `json:"refund_request_id"`
	Status               string                                    `json:"status"`
	ProcessedAt          time.Time                                 `json:"processed_at"`
	SucceededAllocations int                                       `json:"succeeded_allocations"`
	FailedAllocations    int                                       `json:"failed_allocations"`
	SkippedAllocations   int                                       `json:"skipped_allocations"`
	GatewayRefundedTotal float64                                   `json:"gateway_refunded_total"`
	ManualTransferAmount float64                                   `json:"manual_transfer_amount"`
	Allocations          []SettlementRefundGatewayAllocationResult `json:"allocations"`
}

func (s *SettlementRefundService) ProcessSettlementRefundGateway(ctx context.Context, input SettlementRefundGatewayInput) (*SettlementRefundGatewayResult, error) {
	if input.RefundRequestID <= 0 || input.OperatorUserID <= 0 {
		return nil, ErrSettlementRefundGatewayInput
	}
	if s == nil || s.requestStore == nil {
		return nil, ErrSettlementRefundStoreRequired
	}
	store, ok := s.requestStore.(settlementRefundGatewayStore)
	if !ok {
		return nil, ErrSettlementRefundStoreRequired
	}

	record, err := store.GetSettlementRefundRequest(ctx, input.RefundRequestID)
	if err != nil {
		return nil, err
	}
	if !settlementRefundCanProcessGateway(record) {
		return nil, ErrSettlementRefundGatewayState
	}
	if !settlementRefundRequiresGateway(record) {
		return nil, ErrSettlementRefundGatewayNotRequired
	}

	now := s.previewNow()
	if record.Status != SettlementRefundStatusGatewayProcessing {
		updated, updateErr := store.UpdateSettlementRefundRequestStatus(ctx, UpdateSettlementRefundRequestStatusInput{
			RequestID:      record.ID,
			ExpectedStatus: record.Status,
			Status:         SettlementRefundStatusGatewayProcessing,
		})
		if updateErr != nil {
			return nil, updateErr
		}
		record = updated
	}

	result := &SettlementRefundGatewayResult{
		RefundRequestID:      record.ID,
		Status:               record.Status,
		ProcessedAt:          now,
		ManualTransferAmount: record.ManualTransferAmount,
		Allocations:          make([]SettlementRefundGatewayAllocationResult, 0, len(record.Allocations)),
	}

	anyFailed := false
	anyProcessing := false
	for _, allocation := range record.Allocations {
		switch allocation.Status {
		case SettlementRefundAllocationStatusSkipped:
			result.SkippedAllocations++
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(allocation))
			continue
		case SettlementRefundAllocationStatusSucceeded:
			order, orderErr := s.gatewayLoadPaymentOrder(ctx, allocation.PaymentOrderID)
			if orderErr != nil {
				return nil, orderErr
			}
			if err := s.gatewaySyncPaymentOrderRefund(ctx, order, record, allocation, now); err != nil {
				return nil, err
			}
			result.SucceededAllocations++
			result.GatewayRefundedTotal = roundSettlementRefundValue(result.GatewayRefundedTotal + allocation.GatewayRefundAmount)
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(allocation))
			continue
		case SettlementRefundAllocationStatusProcessing:
			anyProcessing = true
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(allocation))
			continue
		}

		if allocation.GatewayRefundAmount <= 0 {
			skipped, skipErr := store.UpdateSettlementRefundAllocationStatus(ctx, UpdateSettlementRefundAllocationStatusInput{
				AllocationID:   allocation.ID,
				ExpectedStatus: allocation.Status,
				Status:         SettlementRefundAllocationStatusSkipped,
				FailedReason:   settlementRefundNullableReason("no_gateway_refund_amount"),
				ProcessedAt:    &now,
			})
			if skipErr != nil {
				return nil, skipErr
			}
			result.SkippedAllocations++
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(*skipped))
			continue
		}

		processing, procErr := store.UpdateSettlementRefundAllocationStatus(ctx, UpdateSettlementRefundAllocationStatusInput{
			AllocationID:   allocation.ID,
			ExpectedStatus: allocation.Status,
			Status:         SettlementRefundAllocationStatusProcessing,
		})
		if procErr != nil {
			return nil, procErr
		}

		order, orderErr := s.gatewayLoadPaymentOrder(ctx, processing.PaymentOrderID)
		if orderErr != nil {
			anyFailed = true
			failedReasonText := settlementRefundFailureReasonText(orderErr)
			failed, failErr := store.UpdateSettlementRefundAllocationStatus(ctx, UpdateSettlementRefundAllocationStatusInput{
				AllocationID:   processing.ID,
				ExpectedStatus: SettlementRefundAllocationStatusProcessing,
				Status:         SettlementRefundAllocationStatusFailed,
				FailedReason:   settlementRefundNullableReason(failedReasonText),
				ProcessedAt:    &now,
			})
			if failErr != nil {
				return nil, failErr
			}
			result.FailedAllocations++
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(*failed))
			continue
		}

		provider, provErr := s.gatewayResolveProvider(ctx, order)
		if provErr != nil {
			anyFailed = true
			failedReasonText := settlementRefundFailureReasonText(provErr)
			failed, failErr := store.UpdateSettlementRefundAllocationStatus(ctx, UpdateSettlementRefundAllocationStatusInput{
				AllocationID:   processing.ID,
				ExpectedStatus: SettlementRefundAllocationStatusProcessing,
				Status:         SettlementRefundAllocationStatusFailed,
				FailedReason:   settlementRefundNullableReason(failedReasonText),
				ProcessedAt:    &now,
			})
			if failErr != nil {
				return nil, failErr
			}
			if err := s.gatewayMarkPaymentOrderRefundFailed(ctx, order, failedReasonText, now); err != nil {
				return nil, err
			}
			result.FailedAllocations++
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(*failed))
			continue
		}

		resp, refundErr := provider.Refund(ctx, payment.RefundRequest{
			TradeNo: order.PaymentTradeNo,
			OrderID: order.OutTradeNo,
			Amount:  formatGatewayRefundAmount(processing.GatewayRefundAmount, order),
			Reason:  settlementRefundGatewayReason(record),
		})
		if refundErr != nil {
			anyFailed = true
			failedReasonText := settlementRefundFailureReasonText(refundErr)
			failed, failErr := store.UpdateSettlementRefundAllocationStatus(ctx, UpdateSettlementRefundAllocationStatusInput{
				AllocationID:   processing.ID,
				ExpectedStatus: SettlementRefundAllocationStatusProcessing,
				Status:         SettlementRefundAllocationStatusFailed,
				FailedReason:   settlementRefundNullableReason(failedReasonText),
				ProcessedAt:    &now,
			})
			if failErr != nil {
				return nil, failErr
			}
			if err := s.gatewayMarkPaymentOrderRefundFailed(ctx, order, failedReasonText, now); err != nil {
				return nil, err
			}
			result.FailedAllocations++
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(*failed))
			continue
		}
		if err := validateRefundProviderResponse(resp); err != nil {
			anyFailed = true
			failedReasonText := settlementRefundFailureReasonText(err)
			failed, failErr := store.UpdateSettlementRefundAllocationStatus(ctx, UpdateSettlementRefundAllocationStatusInput{
				AllocationID:   processing.ID,
				ExpectedStatus: SettlementRefundAllocationStatusProcessing,
				Status:         SettlementRefundAllocationStatusFailed,
				FailedReason:   settlementRefundNullableReason(failedReasonText),
				ProcessedAt:    &now,
			})
			if failErr != nil {
				return nil, failErr
			}
			if err := s.gatewayMarkPaymentOrderRefundFailed(ctx, order, failedReasonText, now); err != nil {
				return nil, err
			}
			result.FailedAllocations++
			result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(*failed))
			continue
		}

		tradeNo := strings.TrimSpace(resp.RefundID)
		succeeded, succErr := store.UpdateSettlementRefundAllocationStatus(ctx, UpdateSettlementRefundAllocationStatusInput{
			AllocationID:         processing.ID,
			ExpectedStatus:       SettlementRefundAllocationStatusProcessing,
			Status:               SettlementRefundAllocationStatusSucceeded,
			GatewayRefundTradeNo: settlementRefundNullableReason(tradeNo),
			ProcessedAt:          &now,
		})
		if succErr != nil {
			return nil, succErr
		}
		if err := s.gatewaySyncPaymentOrderRefund(ctx, order, record, *succeeded, now); err != nil {
			return nil, err
		}
		result.SucceededAllocations++
		result.GatewayRefundedTotal = roundSettlementRefundValue(result.GatewayRefundedTotal + succeeded.GatewayRefundAmount)
		result.Allocations = append(result.Allocations, settlementRefundGatewayAllocationResultFromRecord(*succeeded))
	}

	finalStatus := SettlementRefundStatusGatewayProcessing
	if anyFailed {
		finalStatus = SettlementRefundStatusFailed
	} else if anyProcessing {
		finalStatus = SettlementRefundStatusGatewayProcessing
	} else if SettlementRefundManualTransferRequired(record.ManualTransferAmount, record.Currency) {
		finalStatus = SettlementRefundStatusManualPending
	}

	if record.Status != finalStatus {
		updated, updateErr := store.UpdateSettlementRefundRequestStatus(ctx, UpdateSettlementRefundRequestStatusInput{
			RequestID:      record.ID,
			ExpectedStatus: SettlementRefundStatusGatewayProcessing,
			Status:         finalStatus,
		})
		if updateErr != nil {
			return nil, updateErr
		}
		record = updated
	}
	result.Status = record.Status
	auditFields := settlementRefundAuditFieldsFromGatewayResult(result)
	if auditFields == nil {
		auditFields = make(map[string]any)
	}
	auditFields["operator_user_id"] = input.OperatorUserID
	s.auditSettlementRefundEvent(ctx, "gateway_processed", record, auditFields)
	return result, nil
}

func (s *SettlementRefundService) gatewayLoadPaymentOrder(ctx context.Context, paymentOrderID int64) (*dbent.PaymentOrder, error) {
	if s != nil && s.loadRefundPaymentOrder != nil {
		return s.loadRefundPaymentOrder(ctx, paymentOrderID)
	}
	if s == nil || s.entClient == nil {
		return nil, ErrSettlementRefundGatewayPaymentOrderMissing
	}
	order, err := s.entClient.PaymentOrder.Get(ctx, paymentOrderID)
	if err != nil {
		return nil, fmt.Errorf("load settlement refund payment order %d: %w", paymentOrderID, err)
	}
	return order, nil
}

func (s *SettlementRefundService) gatewayResolveProvider(ctx context.Context, order *dbent.PaymentOrder) (payment.Provider, error) {
	if s != nil && s.resolveRefundProvider != nil {
		return s.resolveRefundProvider(ctx, order)
	}
	if s == nil || s.paymentSvc == nil {
		return nil, ErrSettlementRefundGatewayProviderMissing
	}
	return s.paymentSvc.getRefundProvider(ctx, order)
}

func (s *SettlementRefundService) gatewaySyncPaymentOrderRefund(ctx context.Context, order *dbent.PaymentOrder, record *SettlementRefundRequestRecord, allocation SettlementRefundAllocationRecord, now time.Time) error {
	if s != nil && s.syncGatewayPaymentOrderRefund != nil {
		return s.syncGatewayPaymentOrderRefund(ctx, order, record, allocation, now)
	}
	return s.defaultSyncGatewayPaymentOrderRefund(ctx, order, record, allocation, now)
}

func (s *SettlementRefundService) defaultSyncGatewayPaymentOrderRefund(ctx context.Context, order *dbent.PaymentOrder, record *SettlementRefundRequestRecord, allocation SettlementRefundAllocationRecord, now time.Time) error {
	if s == nil || s.entClient == nil {
		return ErrSettlementRefundGatewayPaymentOrderMissing
	}
	if order == nil {
		return ErrSettlementRefundGatewayPaymentOrderMissing
	}
	refundAmount := roundSettlementRefundValue(allocation.AlreadyRefundedAmount + allocation.AllocatedRefundValue)
	if refundAmount <= 0 {
		return nil
	}
	status := OrderStatusRefunded
	if refundAmount+paymentAmountToleranceForCurrency(PaymentOrderCurrency(order)) < order.Amount {
		status = OrderStatusPartiallyRefunded
	}
	reason := settlementRefundGatewayReason(record)
	if _, err := s.entClient.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(status).
		SetRefundAmount(refundAmount).
		SetRefundReason(reason).
		SetRefundAt(now).
		Save(ctx); err != nil {
		return fmt.Errorf("sync settlement refund payment order %d: %w", order.ID, err)
	}
	return nil
}

func (s *SettlementRefundService) gatewayMarkPaymentOrderRefundFailed(ctx context.Context, order *dbent.PaymentOrder, reason string, now time.Time) error {
	if s != nil && s.markGatewayPaymentOrderRefundFailed != nil {
		return s.markGatewayPaymentOrderRefundFailed(ctx, order, reason, now)
	}
	return s.defaultMarkGatewayPaymentOrderRefundFailed(ctx, order, reason, now)
}

func (s *SettlementRefundService) defaultMarkGatewayPaymentOrderRefundFailed(ctx context.Context, order *dbent.PaymentOrder, reason string, now time.Time) error {
	if s == nil || s.entClient == nil || order == nil {
		return ErrSettlementRefundGatewayPaymentOrderMissing
	}
	if _, err := s.entClient.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusRefundFailed).
		SetFailedAt(now).
		SetFailedReason(reason).
		Save(ctx); err != nil {
		return fmt.Errorf("mark settlement refund payment order failed %d: %w", order.ID, err)
	}
	return nil
}

func settlementRefundCanProcessGateway(record *SettlementRefundRequestRecord) bool {
	if record == nil {
		return false
	}
	switch record.Status {
	case SettlementRefundStatusSubmitted, SettlementRefundStatusGatewayProcessing, SettlementRefundStatusManualPending, SettlementRefundStatusFailed:
		return true
	default:
		return false
	}
}

func settlementRefundRequiresGateway(record *SettlementRefundRequestRecord) bool {
	if record == nil {
		return false
	}
	if record.GatewayRefundableTotal > 0 {
		return true
	}
	for _, allocation := range record.Allocations {
		if allocation.GatewayRefundAmount > 0 {
			return true
		}
	}
	return false
}

func settlementRefundGatewayReason(record *SettlementRefundRequestRecord) string {
	if record == nil {
		return "settlement refund request"
	}
	reason := settlementRefundStringValue(record.Reason)
	if reason != "" {
		return reason
	}
	return fmt.Sprintf("settlement refund request:%d", record.ID)
}

func settlementRefundFailureReasonText(err error) string {
	reason := strings.TrimSpace(psErrMsg(err))
	if reason == "" {
		return "gateway refund failed"
	}
	return reason
}

func settlementRefundGatewayAllocationResultFromRecord(record SettlementRefundAllocationRecord) SettlementRefundGatewayAllocationResult {
	return SettlementRefundGatewayAllocationResult{
		AllocationID:         record.ID,
		PaymentOrderID:       record.PaymentOrderID,
		Status:               record.Status,
		GatewayRefundAmount:  record.GatewayRefundAmount,
		GatewayRefundTradeNo: settlementRefundStringValue(record.GatewayRefundTradeNo),
		FailedReason:         settlementRefundStringValue(record.FailedReason),
	}
}
