package service

import (
	"context"
	"time"
)

type SettlementRefundPreviewCache interface {
	GetSettlementRefundPreview(context.Context, int64, int64) (*SettlementRefundPreviewCacheEntry, error)
	SetSettlementRefundPreview(context.Context, *SettlementRefundPreviewCacheEntry, time.Duration) error
	DeleteSettlementRefundPreview(context.Context, int64, int64) error
}

type SettlementRefundPreviewCacheEntry struct {
	PreviewID             int64                               `json:"preview_id"`
	PreviewToken          string                              `json:"preview_token"`
	UserID                int64                               `json:"user_id"`
	SubscriptionID        int64                               `json:"subscription_id"`
	SettlementID          int64                               `json:"settlement_id"`
	ExpectedSettlementID  int64                               `json:"expected_settlement_id"`
	ActionSource          string                              `json:"action_source"`
	TriggerRefType        string                              `json:"trigger_ref_type"`
	TriggerRefID          *int64                              `json:"trigger_ref_id,omitempty"`
	PlanName              string                              `json:"plan_name"`
	SubscriptionExpiresAt time.Time                           `json:"subscription_expires_at"`
	AfterSettlementValue  float64                             `json:"after_settlement_value"`
	TheoreticalFullMaxKnives float64                          `json:"theoretical_full_max_knives"`
	ResidualQuotaKnives   float64                             `json:"residual_quota_knives"`
	UnitCost              float64                             `json:"unit_cost"`
	RefundMode            string                              `json:"refund_mode"`
	Reason                *string                             `json:"reason,omitempty"`
	RefundResidualValue   float64                             `json:"refund_residual_value"`
	GatewayRefundableTotal float64                            `json:"gateway_refundable_total"`
	ManualTransferAmount  float64                             `json:"manual_transfer_amount"`
	Currency              string                              `json:"currency"`
	PreviewTokenHash      string                              `json:"preview_token_hash"`
	PreviewFingerprint    string                              `json:"preview_fingerprint"`
	PreviewIssuedAt       time.Time                           `json:"preview_issued_at"`
	PreviewExpiresAt      time.Time                           `json:"preview_expires_at"`
	Allocations           []SettlementRefundPreviewAllocation `json:"allocations"`
}
