package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSettlementRefundCompleteState          = infraerrors.Conflict("SETTLEMENT_REFUND_COMPLETE_STATE_INVALID", "settlement refund complete state is invalid")
	ErrSettlementRefundCompleteSubscription   = infraerrors.Conflict("SETTLEMENT_REFUND_COMPLETE_SUBSCRIPTION_INVALID", "settlement refund complete subscription is invalid")
	ErrSettlementRefundCompleteManualRequired = infraerrors.BadRequest("SETTLEMENT_REFUND_COMPLETE_MANUAL_PROOF_REQUIRED", "settlement refund manual proof is required before completion")
)

type settlementRefundCompleteStore interface {
	GetSettlementRefundRequest(context.Context, int64) (*SettlementRefundRequestRecord, error)
	CompleteSettlementRefundRequest(context.Context, CompleteSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error)
}

type SettlementRefundCompleteInput struct {
	RefundRequestID int64
	OperatorUserID  int64
}

type SettlementRefundCompleteResult struct {
	RefundRequestID      int64                     `json:"refund_request_id"`
	Status               string                    `json:"status"`
	SubscriptionID       int64                     `json:"subscription_id"`
	SubscriptionStatus   string                    `json:"subscription_status"`
	CompletedAt          time.Time                 `json:"completed_at"`
	SettlementOrderID    int64                     `json:"settlement_order_id"`
	RefundResidualValue  float64                   `json:"refund_residual_value"`
	SettlementOrder      *SettlementRefundSettlementOrderView `json:"settlement_order,omitempty"`
}

type SettlementRefundSettlementOrderView struct {
	ID                   int64   `json:"id"`
	ActionType           string  `json:"action_type"`
	ActionSource         string  `json:"action_source"`
	TriggerRefType       string  `json:"trigger_ref_type"`
	RefundResidualValue  float64 `json:"refund_residual_value"`
	AfterSettlementValue float64 `json:"after_settlement_value"`
}

func (s *SettlementRefundService) CompleteSettlementRefund(ctx context.Context, input SettlementRefundCompleteInput) (*SettlementRefundCompleteResult, error) {
	if input.RefundRequestID <= 0 {
		return nil, ErrSettlementRefundCompleteInput
	}
	if s == nil || s.subscription == nil || s.settlement == nil || s.requestStore == nil {
		return nil, ErrSettlementRefundStoreRequired
	}
	store, ok := s.requestStore.(settlementRefundCompleteStore)
	if !ok {
		return nil, ErrSettlementRefundStoreRequired
	}

	record, err := store.GetSettlementRefundRequest(ctx, input.RefundRequestID)
	if err != nil {
		return nil, err
	}
	if !settlementRefundCanComplete(record) {
		return nil, ErrSettlementRefundCompleteState
	}
	if SettlementRefundManualTransferRequired(record.ManualTransferAmount, record.Currency) && settlementRefundStringValue(record.ManualTransferProofURL) == "" {
		return nil, ErrSettlementRefundCompleteManualRequired
	}
	if !settlementRefundGatewayAllocationsReady(record.Allocations) {
		return nil, ErrSettlementRefundCompleteState
	}

	now := s.previewNow()
	operatorUserID := input.OperatorUserID
	if operatorUserID <= 0 {
		operatorUserID = record.UserID
	}

	var result *SettlementRefundCompleteResult
	err = s.subscription.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		sub, subErr := s.loadLockedSubscriptionByID(txCtx, record.SubscriptionID)
		if subErr != nil {
			return subErr
		}
		if sub.UserID != record.UserID || sub.Status != SubscriptionStatusSuspended {
			return ErrSettlementRefundCompleteSubscription
		}

		head, headErr := s.loadLockedEffectiveHead(txCtx, record.UserID, now)
		if headErr != nil {
			return headErr
		}
		if head == nil || head.ID != record.ExpectedSettlementID {
			return ErrSettlementRefundPreviewStale
		}
		if head.AfterUserSubscriptionID == nil || *head.AfterUserSubscriptionID != sub.ID {
			return ErrSettlementHeadSubscriptionMismatch
		}

		refunded := *sub
		refunded.Status = SubscriptionStatusRefunded
		refunded.ExpiresAt = now
		if err := s.subscription.userSubRepo.Update(txCtx, &refunded); err != nil {
			return err
		}

		triggerRefID := copyInt64Pointer(head.TriggerRefID)
		refundResidual := record.RefundResidualValue
		createSettlementOrder := s.createSettlementOrder
		if createSettlementOrder == nil && s.settlement != nil {
			createSettlementOrder = s.settlement.CreateSettlementOrder
		}
		if createSettlementOrder == nil {
			return ErrSettlementEntClientRequired
		}
		settlementOrder, settlementErr := createSettlementOrder(txCtx, SettlementOrderInput{
			UserID:                  record.UserID,
			OperatorUserID:          operatorUserID,
			ActionType:              domain.SettlementActionRefund,
			ActionSource:            head.ActionSource,
			TriggerRefType:          head.TriggerRefType,
			TriggerRefID:            triggerRefID,
			ActionNote:              settlementRefundStringValue(record.Reason),
			CarryInResidualValue:    refundResidual,
			ActionDeltaValue:        -refundResidual,
			AfterSettlementValue:    0,
			RefundResidualValue:     &refundResidual,
			WriteoffValue:           0,
			AfterUserSubscription:   &refunded,
			AfterSubscriptionStatus: domain.SubscriptionStatusRefunded,
			EffectiveAt:             now,
		})
		if settlementErr != nil {
			return settlementErr
		}

		completedRecord, completeErr := store.CompleteSettlementRefundRequest(txCtx, CompleteSettlementRefundRequestInput{
			RequestID:      record.ID,
			ExpectedStatus: record.Status,
			CompletedAt:    now,
		})
		if completeErr != nil {
			return completeErr
		}

		result = &SettlementRefundCompleteResult{
			RefundRequestID:     completedRecord.ID,
			Status:              completedRecord.Status,
			SubscriptionID:      refunded.ID,
			SubscriptionStatus:  refunded.Status,
			CompletedAt:         now,
			SettlementOrderID:   settlementOrder.ID,
			RefundResidualValue: refundResidual,
			SettlementOrder: &SettlementRefundSettlementOrderView{
				ID:                   settlementOrder.ID,
				ActionType:           settlementOrder.ActionType,
				ActionSource:         settlementOrder.ActionSource,
				TriggerRefType:       settlementOrder.TriggerRefType,
				RefundResidualValue:  refundResidual,
				AfterSettlementValue: settlementOrder.AfterSettlementValue,
			},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.auditSettlementRefundEvent(ctx, "completed", &SettlementRefundRequestRecord{
		ID:                    result.RefundRequestID,
		UserID:                record.UserID,
		SubscriptionID:        result.SubscriptionID,
		SettlementID:          record.SettlementID,
		ExpectedSettlementID:  record.ExpectedSettlementID,
		Status:                result.Status,
		RefundMode:            record.RefundMode,
		Currency:              record.Currency,
		RefundResidualValue:   result.RefundResidualValue,
		GatewayRefundableTotal: record.GatewayRefundableTotal,
		ManualTransferAmount:  record.ManualTransferAmount,
		CompletedAt:           &result.CompletedAt,
	}, map[string]any{
		"operator_user_id":    operatorUserID,
		"subscription_status": result.SubscriptionStatus,
		"settlement_order_id": result.SettlementOrderID,
	})
	s.subscription.invalidateSubscriptionCaches(record.UserID)
	return result, nil
}

func settlementRefundCanComplete(record *SettlementRefundRequestRecord) bool {
	if record == nil {
		return false
	}
	switch record.Status {
	case SettlementRefundStatusSubmitted, SettlementRefundStatusGatewayProcessing, SettlementRefundStatusManualPending:
		return true
	default:
		return false
	}
}

func settlementRefundGatewayAllocationsReady(allocations []SettlementRefundAllocationRecord) bool {
	for _, allocation := range allocations {
		switch allocation.Status {
		case SettlementRefundAllocationStatusPending, SettlementRefundAllocationStatusProcessing, SettlementRefundAllocationStatusFailed:
			return false
		}
	}
	return true
}
