package service

import (
	"context"
	"net/url"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSettlementRefundPreviewExpired             = infraerrors.Conflict("SETTLEMENT_REFUND_PREVIEW_EXPIRED", "settlement refund preview has expired")
	ErrSettlementRefundPreviewStale               = infraerrors.Conflict("SETTLEMENT_REFUND_PREVIEW_STALE", "settlement refund preview is stale")
	ErrSettlementRefundPreviewTokenInvalid        = infraerrors.BadRequest("SETTLEMENT_REFUND_PREVIEW_TOKEN_INVALID", "settlement refund preview token is invalid")
	ErrSettlementRefundManualReceiverRequired     = infraerrors.BadRequest("SETTLEMENT_REFUND_MANUAL_RECEIVER_REQUIRED", "manual transfer receiver info is required")
	ErrSettlementRefundManualReceiverImageInvalid = infraerrors.BadRequest("SETTLEMENT_REFUND_MANUAL_RECEIVER_IMAGE_INVALID", "manual transfer receiver qr image must be a valid http(s) URL or stored path")
)

type settlementRefundSubmitStore interface {
	CreateSettlementRefundRequest(context.Context, CreateSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error)
}

type settlementRefundSubmitLegacyStore interface {
	GetSettlementRefundRequest(context.Context, int64) (*SettlementRefundRequestRecord, error)
}

type SettlementRefundSubmitInput struct {
	SubscriptionID int64
	UserID         int64
	PreviewID      int64
	PreviewToken   string
	Reason         string
	ManualTransfer *ManualTransferInput
}

type ManualTransferInput struct {
	ReceiverType           string
	ReceiverName           string
	ReceiverAccount        string
	ReceiverQRCodeImageURL string
	Remark                 string
}

type SettlementRefundSubmitResult struct {
	Success                bool    `json:"success"`
	RefundRequestID        int64   `json:"refund_request_id"`
	SubscriptionID         int64   `json:"subscription_id"`
	SubscriptionStatus     string  `json:"subscription_status"`
	RefundStatus           string  `json:"refund_status"`
	RefundResidualValue    float64 `json:"refund_residual_value"`
	GatewayRefundableTotal float64 `json:"gateway_refundable_total"`
	ManualTransferAmount   float64 `json:"manual_transfer_amount"`
	Currency               string  `json:"currency"`
}

func (s *SettlementRefundService) SubmitSettlementRefund(ctx context.Context, input SettlementRefundSubmitInput) (*SettlementRefundSubmitResult, error) {
	if input.UserID <= 0 || input.SubscriptionID <= 0 || input.PreviewID <= 0 {
		return nil, ErrSettlementRefundSubmitInput
	}
	if s == nil || s.subscription == nil || s.requestStore == nil {
		return nil, ErrSettlementRefundStoreRequired
	}
	store, ok := s.requestStore.(settlementRefundSubmitStore)
	if !ok {
		return nil, ErrSettlementRefundStoreRequired
	}

	var preview *SettlementRefundPreviewCacheEntry
	var err error
	if s.previewCache != nil {
		preview, err = s.previewCache.GetSettlementRefundPreview(ctx, input.UserID, input.SubscriptionID)
		if err != nil {
			return nil, err
		}
	}
	if preview == nil {
		legacyStore, ok := s.requestStore.(settlementRefundSubmitLegacyStore)
		if !ok {
			return nil, ErrSettlementRefundPreviewStale
		}
		record, err := legacyStore.GetSettlementRefundRequest(ctx, input.PreviewID)
		if err != nil {
			return nil, err
		}
		if record.UserID != input.UserID || record.SubscriptionID != input.SubscriptionID {
			return nil, ErrSettlementRefundPreviewStale
		}
		preview = settlementRefundPreviewCacheEntryFromRequestRecord(record, input.PreviewToken)
	}
	if preview == nil || preview.PreviewID != input.PreviewID {
		return nil, ErrSettlementRefundPreviewStale
	}
	if !verifySettlementRefundPreviewToken(input.PreviewToken, preview.PreviewTokenHash) {
		return nil, ErrSettlementRefundPreviewTokenInvalid
	}

	now := s.previewNow()
	if settlementRefundPreviewExpired(now, preview.PreviewExpiresAt) {
		return nil, ErrSettlementRefundPreviewExpired
	}

	computation, err := s.computeSettlementRefundPreview(ctx, SettlementRefundPreviewInput{
		SubscriptionID: input.SubscriptionID,
		UserID:         input.UserID,
		Reason:         input.Reason,
	})
	if err != nil {
		return nil, err
	}
	if !settlementRefundComputationMatchesPreview(computation, preview) {
		return nil, ErrSettlementRefundPreviewStale
	}
	manualTransfer, err := validateSettlementRefundSubmitManualTransfer(preview.ManualTransferAmount, preview.Currency, input.ManualTransfer)
	if err != nil {
		return nil, err
	}

	var manualReceiverType *string
	var manualReceiverName *string
	var manualReceiverAccount *string
	var manualReceiverQRCodeImageURL *string
	var manualReceiverRemark *string
	reason := preview.Reason
	if latestReason := settlementRefundNullableReason(input.Reason); latestReason != nil {
		reason = latestReason
	}
	if manualTransfer != nil {
		manualReceiverType = settlementRefundNullableReason(manualTransfer.ReceiverType)
		manualReceiverName = settlementRefundNullableReason(manualTransfer.ReceiverName)
		manualReceiverAccount = settlementRefundNullableReason(manualTransfer.ReceiverAccount)
		manualReceiverQRCodeImageURL = settlementRefundNullableReason(manualTransfer.ReceiverQRCodeImageURL)
		manualReceiverRemark = settlementRefundNullableReason(manualTransfer.Remark)
	}

	var result *SettlementRefundSubmitResult
	err = s.subscription.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		active, activeErr := s.loadLockedSubscriptionByID(txCtx, input.SubscriptionID)
		if activeErr != nil {
			return activeErr
		}
		if active.UserID != input.UserID || active.Status != SubscriptionStatusActive {
			return ErrSettlementRefundPreviewStale
		}
		if !active.ExpiresAt.After(now) {
			return ErrSettlementRefundPreviewStale
		}

		head, headErr := s.loadLockedEffectiveHead(txCtx, input.UserID, now)
		if headErr != nil {
			return headErr
		}
		if head == nil || head.ID != preview.ExpectedSettlementID {
			return ErrSettlementRefundPreviewStale
		}
		if head.AfterUserSubscriptionID == nil || *head.AfterUserSubscriptionID != active.ID {
			return ErrSettlementRefundPreviewStale
		}

		updatedRecord, submitErr := store.CreateSettlementRefundRequest(txCtx, CreateSettlementRefundRequestInput{
			UserID:                        preview.UserID,
			SubscriptionID:                preview.SubscriptionID,
			SettlementID:                  preview.SettlementID,
			ExpectedSettlementID:          preview.ExpectedSettlementID,
			Status:                        SettlementRefundStatusSubmitted,
			RefundMode:                    preview.RefundMode,
			Currency:                      preview.Currency,
			Reason:                        reason,
			RefundResidualValue:           preview.RefundResidualValue,
			GatewayRefundableTotal:        preview.GatewayRefundableTotal,
			ManualTransferAmount:          preview.ManualTransferAmount,
			PreviewTokenHash:              preview.PreviewTokenHash,
			PreviewFingerprint:            preview.PreviewFingerprint,
			PreviewIssuedAt:               preview.PreviewIssuedAt,
			PreviewExpiresAt:              preview.PreviewExpiresAt,
			SubmittedAt:                   now,
			FrozenAt:                      now,
			OriginalSubscriptionStatus:    active.Status,
			OriginalSubscriptionExpiresAt: active.ExpiresAt,
			ManualReceiverType:            manualReceiverType,
			ManualReceiverName:            manualReceiverName,
			ManualReceiverAccount:         manualReceiverAccount,
			ManualReceiverQRCodeImageURL:  manualReceiverQRCodeImageURL,
			ManualReceiverRemark:          manualReceiverRemark,
			Allocations:                   settlementRefundPreviewAllocationsToStore(preview.Allocations),
		})
		if submitErr != nil {
			return submitErr
		}

		frozen := *active
		frozen.Status = SubscriptionStatusSuspended
		if err := s.subscription.userSubRepo.Update(txCtx, &frozen); err != nil {
			return err
		}

		result = settlementRefundSubmitResultFromRecord(updatedRecord)
		result.SubscriptionStatus = SubscriptionStatusSuspended
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.previewCache != nil {
		_ = s.previewCache.DeleteSettlementRefundPreview(ctx, input.UserID, input.SubscriptionID)
	}
	s.auditSettlementRefundEvent(ctx, "submitted", &SettlementRefundRequestRecord{
		ID:                     result.RefundRequestID,
		UserID:                 input.UserID,
		SubscriptionID:         result.SubscriptionID,
		Status:                 result.RefundStatus,
		RefundResidualValue:    result.RefundResidualValue,
		GatewayRefundableTotal: result.GatewayRefundableTotal,
		ManualTransferAmount:   result.ManualTransferAmount,
		Currency:               result.Currency,
	}, map[string]any{
		"subscription_status": result.SubscriptionStatus,
		"preview_id":          input.PreviewID,
	})
	s.subscription.invalidateSubscriptionCaches(input.UserID)
	return result, nil
}

func settlementRefundComputationMatchesPreview(computation *settlementRefundPreviewComputation, preview *SettlementRefundPreviewCacheEntry) bool {
	if computation == nil || preview == nil {
		return false
	}
	if strings.TrimSpace(preview.PreviewFingerprint) != settlementRefundPreviewFingerprint(computation) {
		return false
	}
	if computation.Head == nil || computation.Head.ID != preview.ExpectedSettlementID {
		return false
	}
	if computation.RefundMode != preview.RefundMode {
		return false
	}
	if !settlementRefundFloatEquals(computation.AllocationResult.RefundResidualValue, preview.RefundResidualValue) {
		return false
	}
	if !settlementRefundFloatEquals(computation.AllocationResult.GatewayRefundableTotal, preview.GatewayRefundableTotal) {
		return false
	}
	if !settlementRefundFloatEquals(computation.AllocationResult.ManualTransferAmount, preview.ManualTransferAmount) {
		return false
	}
	if len(computation.AllocationResult.Allocations) != len(preview.Allocations) {
		return false
	}
	for idx := range computation.AllocationResult.Allocations {
		previewAllocation := computation.AllocationResult.Allocations[idx]
		cachedAllocation := preview.Allocations[idx]
		if previewAllocation.PaymentOrderID != cachedAllocation.PaymentOrderID {
			return false
		}
		if !settlementRefundFloatEquals(previewAllocation.AllocatedRefundValue, cachedAllocation.AllocatedRefundValue) {
			return false
		}
		if !settlementRefundFloatEquals(previewAllocation.GatewayRefundAmount, cachedAllocation.GatewayRefundAmount) {
			return false
		}
	}
	return true
}

func settlementRefundFloatEquals(left, right float64) bool {
	return roundSettlementAmountValue(left) == roundSettlementAmountValue(right)
}

func settlementRefundSubmitResultFromRecord(record *SettlementRefundRequestRecord) *SettlementRefundSubmitResult {
	if record == nil {
		return nil
	}
	return &SettlementRefundSubmitResult{
		Success:                true,
		RefundRequestID:        record.ID,
		SubscriptionID:         record.SubscriptionID,
		SubscriptionStatus:     SubscriptionStatusSuspended,
		RefundStatus:           record.Status,
		RefundResidualValue:    record.RefundResidualValue,
		GatewayRefundableTotal: record.GatewayRefundableTotal,
		ManualTransferAmount:   record.ManualTransferAmount,
		Currency:               settlementRefundPreviewResponseCurrency(record.Currency),
	}
}

func validateSettlementRefundSubmitManualTransfer(requiredAmount float64, currency string, input *ManualTransferInput) (*ManualTransferInput, error) {
	required := SettlementRefundManualTransferRequired(requiredAmount, currency)
	if input == nil {
		if required {
			return nil, ErrSettlementRefundManualReceiverRequired
		}
		return nil, nil
	}

	normalized := &ManualTransferInput{
		ReceiverType:           settlementRefundStringValue(settlementRefundNullableReason(input.ReceiverType)),
		ReceiverName:           settlementRefundStringValue(settlementRefundNullableReason(input.ReceiverName)),
		ReceiverAccount:        settlementRefundStringValue(settlementRefundNullableReason(input.ReceiverAccount)),
		ReceiverQRCodeImageURL: settlementRefundStringValue(settlementRefundNullableReason(input.ReceiverQRCodeImageURL)),
		Remark:                 settlementRefundStringValue(settlementRefundNullableReason(input.Remark)),
	}
	if !required {
		return normalized, nil
	}
	if normalized.ReceiverType == "" || normalized.ReceiverName == "" {
		return nil, ErrSettlementRefundManualReceiverRequired
	}
	if normalized.ReceiverAccount == "" && normalized.ReceiverQRCodeImageURL == "" {
		return nil, ErrSettlementRefundManualReceiverRequired
	}
	if normalized.ReceiverQRCodeImageURL != "" {
		normalizedQRCodeImageURL, err := normalizeSettlementRefundStoredImageRef(normalized.ReceiverQRCodeImageURL)
		if err != nil {
			return nil, ErrSettlementRefundManualReceiverImageInvalid
		}
		normalized.ReceiverQRCodeImageURL = normalizedQRCodeImageURL
	}
	return normalized, nil
}

func normalizeSettlementRefundStoredImageRef(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed != nil && parsed.Scheme != "" {
		if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
			return "", ErrSettlementRefundManualReceiverImageInvalid
		}
		if strings.TrimSpace(parsed.Host) == "" {
			return "", ErrSettlementRefundManualReceiverImageInvalid
		}
	}

	return trimmed, nil
}

func settlementRefundStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func settlementRefundPreviewAllocationsToStore(allocations []SettlementRefundPreviewAllocation) []CreateSettlementRefundAllocationInput {
	if len(allocations) == 0 {
		return nil
	}
	result := make([]CreateSettlementRefundAllocationInput, 0, len(allocations))
	for _, allocation := range allocations {
		status := SettlementRefundAllocationStatusSkipped
		if allocation.GatewayRefundAmount > 0 {
			status = SettlementRefundAllocationStatusPending
		}
		input := CreateSettlementRefundAllocationInput{
			PaymentOrderID:            allocation.PaymentOrderID,
			PaymentProviderInstanceID: allocation.ProviderInstanceID,
			OrderAmount:               allocation.OrderAmount,
			OrderPayAmount:            allocation.PayAmount,
			AlreadyRefundedAmount:     allocation.AlreadyRefundedAmount,
			RefundableOrderAmount:     allocation.RefundableOrderAmount,
			AllocatedRefundValue:      allocation.AllocatedRefundValue,
			GatewayRefundAmount:       allocation.GatewayRefundAmount,
			Currency:                  allocation.Currency,
			Status:                    status,
		}
		if reason := strings.TrimSpace(allocation.SkippedReason); reason != "" {
			input.FailedReason = &reason
		}
		result = append(result, input)
	}
	return result
}

func settlementRefundPreviewCacheEntryFromRequestRecord(record *SettlementRefundRequestRecord, previewToken string) *SettlementRefundPreviewCacheEntry {
	if record == nil {
		return nil
	}
	return &SettlementRefundPreviewCacheEntry{
		PreviewID:               record.ID,
		PreviewToken:            previewToken,
		UserID:                  record.UserID,
		SubscriptionID:          record.SubscriptionID,
		SettlementID:            record.SettlementID,
		ExpectedSettlementID:    record.ExpectedSettlementID,
		RefundMode:              record.RefundMode,
		Reason:                  record.Reason,
		RefundResidualValue:     record.RefundResidualValue,
		GatewayRefundableTotal:  record.GatewayRefundableTotal,
		ManualTransferAmount:    record.ManualTransferAmount,
		Currency:                record.Currency,
		PreviewTokenHash:        record.PreviewTokenHash,
		PreviewFingerprint:      settlementRefundStringValue(record.PreviewFingerprint),
		PreviewIssuedAt:         record.PreviewIssuedAt,
		PreviewExpiresAt:        record.PreviewExpiresAt,
		Allocations:             settlementRefundAllocationRecordsToPreviewCacheAllocations(record.Allocations),
	}
}

func settlementRefundAllocationRecordsToPreviewCacheAllocations(allocations []SettlementRefundAllocationRecord) []SettlementRefundPreviewAllocation {
	if len(allocations) == 0 {
		return nil
	}
	result := make([]SettlementRefundPreviewAllocation, 0, len(allocations))
	for _, allocation := range allocations {
		result = append(result, SettlementRefundPreviewAllocation{
			PaymentOrderID:         allocation.PaymentOrderID,
			OrderAmount:            allocation.OrderAmount,
			PayAmount:              allocation.OrderPayAmount,
			ProviderInstanceID:     allocation.PaymentProviderInstanceID,
			AlreadyRefundedAmount:  allocation.AlreadyRefundedAmount,
			RefundableOrderAmount:  allocation.RefundableOrderAmount,
			AllocatedRefundValue:   allocation.AllocatedRefundValue,
			GatewayRefundAmount:    allocation.GatewayRefundAmount,
			Currency:               allocation.Currency,
			RefundChannelAvailable: allocation.Status != SettlementRefundAllocationStatusSkipped,
			SkippedReason:          settlementRefundStringValue(allocation.FailedReason),
		})
	}
	return result
}
