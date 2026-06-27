package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSettlementRefundCancelState       = infraerrors.Conflict("SETTLEMENT_REFUND_CANCEL_STATE_INVALID", "settlement refund cancel state is invalid")
	ErrSettlementRefundCancelSubscription = infraerrors.Conflict("SETTLEMENT_REFUND_CANCEL_SUBSCRIPTION_INVALID", "settlement refund cancel subscription is invalid")
	ErrSettlementRefundCancelAfterPayout = infraerrors.Conflict("SETTLEMENT_REFUND_CANNOT_CANCEL_AFTER_PAYOUT", "settlement refund cannot be cancelled after payout")
)

type settlementRefundCancelStore interface {
	GetSettlementRefundRequest(context.Context, int64) (*SettlementRefundRequestRecord, error)
	CancelSettlementRefundRequest(context.Context, CancelSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error)
}

type SettlementRefundCancelInput struct {
	RefundRequestID int64
	OperatorUserID  int64
	AdminNote       string
}

type SettlementRefundCancelResult struct {
	RefundRequestID    int64     `json:"refund_request_id"`
	Status             string    `json:"status"`
	SubscriptionID     int64     `json:"subscription_id"`
	SubscriptionStatus string    `json:"subscription_status"`
	CancelledAt        time.Time `json:"cancelled_at"`
}

func (s *SettlementRefundService) CancelSettlementRefund(ctx context.Context, input SettlementRefundCancelInput) (*SettlementRefundCancelResult, error) {
	if input.RefundRequestID <= 0 {
		return nil, ErrSettlementRefundCancelInput
	}
	if s == nil || s.subscription == nil || s.requestStore == nil {
		return nil, ErrSettlementRefundStoreRequired
	}
	store, ok := s.requestStore.(settlementRefundCancelStore)
	if !ok {
		return nil, ErrSettlementRefundStoreRequired
	}

	record, err := store.GetSettlementRefundRequest(ctx, input.RefundRequestID)
	if err != nil {
		return nil, err
	}
	if !settlementRefundCanCancel(record) {
		return nil, ErrSettlementRefundCancelState
	}
	if settlementRefundHasPayoutEvidence(record) {
		return nil, ErrSettlementRefundCancelAfterPayout
	}

	now := s.previewNow()
	var result *SettlementRefundCancelResult
	err = s.subscription.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		sub, subErr := s.loadLockedSubscriptionByID(txCtx, record.SubscriptionID)
		if subErr != nil {
			return subErr
		}
		if sub.UserID != record.UserID || sub.Status != SubscriptionStatusSuspended {
			return ErrSettlementRefundCancelSubscription
		}

		restored := *sub
		restored.Status = settlementRefundRestoreSubscriptionStatus(record, now)
		if record.OriginalSubscriptionExpiresAt != nil {
			restored.ExpiresAt = *record.OriginalSubscriptionExpiresAt
		}
		if err := s.subscription.userSubRepo.Update(txCtx, &restored); err != nil {
			return err
		}

		cancelledRecord, cancelErr := store.CancelSettlementRefundRequest(txCtx, CancelSettlementRefundRequestInput{
			RequestID:      record.ID,
			ExpectedStatus: record.Status,
			CancelledAt:    now,
			AdminNote:      settlementRefundNullableReason(input.AdminNote),
		})
		if cancelErr != nil {
			return cancelErr
		}

		result = &SettlementRefundCancelResult{
			RefundRequestID:    cancelledRecord.ID,
			Status:             cancelledRecord.Status,
			SubscriptionID:     restored.ID,
			SubscriptionStatus: restored.Status,
			CancelledAt:        now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.auditSettlementRefundEvent(ctx, "cancelled", &SettlementRefundRequestRecord{
		ID:                    result.RefundRequestID,
		UserID:                record.UserID,
		SubscriptionID:        result.SubscriptionID,
		SettlementID:          record.SettlementID,
		ExpectedSettlementID:  record.ExpectedSettlementID,
		Status:                result.Status,
		RefundMode:            record.RefundMode,
		Currency:              record.Currency,
		RefundResidualValue:   record.RefundResidualValue,
		GatewayRefundableTotal: record.GatewayRefundableTotal,
		ManualTransferAmount:  record.ManualTransferAmount,
		CancelledAt:           &result.CancelledAt,
	}, map[string]any{
		"operator_user_id":    input.OperatorUserID,
		"subscription_status": result.SubscriptionStatus,
		"admin_note":          strings.TrimSpace(input.AdminNote),
	})
	s.subscription.invalidateSubscriptionCaches(record.UserID)
	return result, nil
}

func settlementRefundCanCancel(record *SettlementRefundRequestRecord) bool {
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

func settlementRefundHasPayoutEvidence(record *SettlementRefundRequestRecord) bool {
	if record == nil {
		return false
	}
	if settlementRefundStringValue(record.ManualTransferProofURL) != "" {
		return true
	}
	for _, allocation := range record.Allocations {
		if allocation.Status == SettlementRefundAllocationStatusSucceeded {
			return true
		}
	}
	return false
}

func settlementRefundRestoreSubscriptionStatus(record *SettlementRefundRequestRecord, now time.Time) string {
	if record == nil {
		return SubscriptionStatusExpired
	}
	if record.OriginalSubscriptionExpiresAt != nil && !record.OriginalSubscriptionExpiresAt.After(now) {
		return SubscriptionStatusExpired
	}
	status := settlementRefundStringValue(record.OriginalSubscriptionStatus)
	if status == "" || status == SubscriptionStatusSuspended {
		return SubscriptionStatusActive
	}
	return status
}
