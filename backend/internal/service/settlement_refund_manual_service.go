package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSettlementRefundManualProofRequired = infraerrors.BadRequest("SETTLEMENT_REFUND_MANUAL_PROOF_REQUIRED", "settlement refund manual proof is required")
	ErrSettlementRefundManualProofState    = infraerrors.Conflict("SETTLEMENT_REFUND_MANUAL_PROOF_STATE_INVALID", "settlement refund manual proof state is invalid")
	ErrSettlementRefundManualProofInvalid  = infraerrors.BadRequest("SETTLEMENT_REFUND_MANUAL_PROOF_INVALID", "settlement refund manual proof must be a valid http(s) URL or stored path")
)

type settlementRefundManualStore interface {
	GetSettlementRefundRequest(context.Context, int64) (*SettlementRefundRequestRecord, error)
	UpdateSettlementRefundManualProof(context.Context, UpdateSettlementRefundManualProofInput) (*SettlementRefundRequestRecord, error)
}

type SettlementRefundManualProofInput struct {
	RefundRequestID int64
	OperatorUserID  int64
	ProofURL        string
	AdminNote       string
}

type SettlementRefundManualProofResult struct {
	RefundRequestID               int64      `json:"refund_request_id"`
	Status                        string     `json:"status"`
	ManualTransferProofURL        string     `json:"manual_transfer_proof_url"`
	ManualTransferProofUploadedAt time.Time  `json:"manual_transfer_proof_uploaded_at"`
	ManualTransferOperatorUserID  int64      `json:"manual_transfer_operator_user_id"`
	AdminNote                     string     `json:"admin_note,omitempty"`
}

func (s *SettlementRefundService) UploadSettlementRefundManualProof(ctx context.Context, input SettlementRefundManualProofInput) (*SettlementRefundManualProofResult, error) {
	if input.RefundRequestID <= 0 || input.OperatorUserID <= 0 {
		return nil, ErrSettlementRefundManualProofInput
	}
	proofURL := strings.TrimSpace(input.ProofURL)
	if proofURL == "" {
		return nil, ErrSettlementRefundManualProofRequired
	}
	normalizedProofURL, err := normalizeSettlementRefundStoredImageRef(proofURL)
	if err != nil {
		return nil, ErrSettlementRefundManualProofInvalid
	}
	if s == nil || s.requestStore == nil {
		return nil, ErrSettlementRefundStoreRequired
	}
	store, ok := s.requestStore.(settlementRefundManualStore)
	if !ok {
		return nil, ErrSettlementRefundStoreRequired
	}

	record, err := store.GetSettlementRefundRequest(ctx, input.RefundRequestID)
	if err != nil {
		return nil, err
	}
	if record.ManualTransferAmount <= 0 {
		return nil, ErrSettlementRefundManualProofState
	}
	if !settlementRefundCanUploadManualProof(record.Status) {
		return nil, ErrSettlementRefundManualProofState
	}

	now := s.previewNow()
	adminNote := settlementRefundNullableReason(input.AdminNote)
	updated, err := store.UpdateSettlementRefundManualProof(ctx, UpdateSettlementRefundManualProofInput{
		RequestID:      record.ID,
		ExpectedStatus: record.Status,
		Status:         SettlementRefundStatusManualPending,
		ProofURL:       normalizedProofURL,
		UploadedAt:     now,
		OperatorUserID: input.OperatorUserID,
		AdminNote:      adminNote,
	})
	if err != nil {
		return nil, err
	}

	result := &SettlementRefundManualProofResult{
		RefundRequestID:               updated.ID,
		Status:                        updated.Status,
		ManualTransferProofURL:        settlementRefundStringValue(updated.ManualTransferProofURL),
		ManualTransferProofUploadedAt: now,
		ManualTransferOperatorUserID:  input.OperatorUserID,
	}
	if updated.ManualTransferProofUploadedAt != nil {
		result.ManualTransferProofUploadedAt = *updated.ManualTransferProofUploadedAt
	}
	result.AdminNote = settlementRefundStringValue(updated.AdminNote)
	if updated.ManualTransferOperatorUserID != nil {
		result.ManualTransferOperatorUserID = *updated.ManualTransferOperatorUserID
	}
	s.auditSettlementRefundEvent(ctx, "manual_proof_uploaded", updated, map[string]any{
		"operator_user_id":                  input.OperatorUserID,
		"manual_transfer_proof_url":         result.ManualTransferProofURL,
		"manual_transfer_proof_uploaded_at": result.ManualTransferProofUploadedAt.UTC().Format(time.RFC3339Nano),
		"admin_note":                        strings.TrimSpace(result.AdminNote),
	})
	return result, nil
}

func settlementRefundCanUploadManualProof(status string) bool {
	switch status {
	case SettlementRefundStatusSubmitted, SettlementRefundStatusGatewayProcessing, SettlementRefundStatusManualPending, SettlementRefundStatusFailed:
		return true
	default:
		return false
	}
}
