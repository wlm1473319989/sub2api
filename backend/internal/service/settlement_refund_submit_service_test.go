package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

type settlementRefundSubmitStoreStub struct {
	request      *SettlementRefundRequestRecord
	submitFn     func(CreateSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error)
	lastSubmit   *CreateSettlementRefundRequestInput
}

func (s *settlementRefundSubmitStoreStub) CreateSettlementRefundPreview(context.Context, CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	panic("unexpected CreateSettlementRefundPreview call")
}

func (s *settlementRefundSubmitStoreStub) CreateSettlementRefundRequest(_ context.Context, input CreateSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	s.lastSubmit = &input
	if s.submitFn != nil {
		return s.submitFn(input)
	}
	record := cloneSettlementRefundRequestRecord(s.request)
	record.Status = SettlementRefundStatusSubmitted
	record.SubmittedAt = &input.SubmittedAt
	record.FrozenAt = &input.FrozenAt
	record.OriginalSubscriptionStatus = settlementRefundNullableReason(input.OriginalSubscriptionStatus)
	record.OriginalSubscriptionExpiresAt = &input.OriginalSubscriptionExpiresAt
	record.ManualReceiverType = input.ManualReceiverType
	record.ManualReceiverName = input.ManualReceiverName
	record.ManualReceiverAccount = input.ManualReceiverAccount
	record.ManualReceiverQRCodeImageURL = input.ManualReceiverQRCodeImageURL
	record.ManualReceiverRemark = input.ManualReceiverRemark
	return record, nil
}

type settlementRefundSubmitPreviewCacheStub struct {
	entry *SettlementRefundPreviewCacheEntry
}

func (s *settlementRefundSubmitPreviewCacheStub) GetSettlementRefundPreview(_ context.Context, userID, subscriptionID int64) (*SettlementRefundPreviewCacheEntry, error) {
	if s.entry == nil {
		return nil, nil
	}
	return s.entry, nil
}

func (s *settlementRefundSubmitPreviewCacheStub) SetSettlementRefundPreview(_ context.Context, entry *SettlementRefundPreviewCacheEntry, ttl time.Duration) error {
	s.entry = entry
	return nil
}

func (s *settlementRefundSubmitPreviewCacheStub) DeleteSettlementRefundPreview(_ context.Context, userID, subscriptionID int64) error {
	s.entry = nil
	return nil
}

func TestSettlementRefundServiceSubmitRejectsExpiredPreview(t *testing.T) {
	now := time.Date(2026, 6, 25, 16, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	record := settlementRefundPreviewedRequestRecord(now.Add(-5*time.Minute), now.Add(-3*time.Minute), active, head, SettlementRefundModeEntitlementOnly, nil)
	service := newSettlementRefundSubmitServiceForTest(t, now, active, head, record, nil, nil)

	result, err := service.SubmitSettlementRefund(context.Background(), SettlementRefundSubmitInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		PreviewID:      record.ID,
		PreviewToken:   "preview-token",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundPreviewExpired)
}

func TestSettlementRefundServiceSubmitRejectsInvalidToken(t *testing.T) {
	now := time.Date(2026, 6, 25, 16, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	record := settlementRefundPreviewedRequestRecord(now.Add(-30*time.Second), now.Add(90*time.Second), active, head, SettlementRefundModeEntitlementOnly, nil)
	service := newSettlementRefundSubmitServiceForTest(t, now, active, head, record, nil, nil)

	result, err := service.SubmitSettlementRefund(context.Background(), SettlementRefundSubmitInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		PreviewID:      record.ID,
		PreviewToken:   "wrong-token",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundPreviewTokenInvalid)
}

func TestSettlementRefundServiceSubmitRejectsStalePreview(t *testing.T) {
	now := time.Date(2026, 6, 25, 16, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	record := settlementRefundPreviewedRequestRecord(now.Add(-30*time.Second), now.Add(90*time.Second), active, head, SettlementRefundModeEntitlementOnly, nil)
	record.RefundResidualValue = record.RefundResidualValue + 1
	service := newSettlementRefundSubmitServiceForTest(t, now, active, head, record, nil, nil)

	result, err := service.SubmitSettlementRefund(context.Background(), SettlementRefundSubmitInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		PreviewID:      record.ID,
		PreviewToken:   "preview-token",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundPreviewStale)
}

func TestSettlementRefundServiceSubmitRejectsStalePreviewWhenRefundableOrderStateChanges(t *testing.T) {
	now := time.Date(2026, 6, 25, 16, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceUserPurchase)
	record := settlementRefundPreviewedRequestRecord(now.Add(-30*time.Second), now.Add(90*time.Second), active, head, SettlementRefundModeHybrid, []SettlementRefundAllocationRecord{
		{
			ID:                    9101,
			RefundRequestID:       9001,
			PaymentOrderID:        1001,
			OrderAmount:           99,
			OrderPayAmount:        99,
			AlreadyRefundedAmount: 0,
			RefundableOrderAmount: 99,
			AllocatedRefundValue:  99,
			GatewayRefundAmount:   99,
			Currency:              "CNY",
			Status:                SettlementRefundAllocationStatusPending,
		},
	})
	record.GatewayRefundableTotal = 99
	record.ManualTransferAmount = record.RefundResidualValue - 99
	store := &settlementRefundSubmitStoreStub{request: record}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	subscriptionSvc := &SubscriptionService{
		userSubRepo: repo,
	}
	service := &SettlementRefundService{
		subscription: subscriptionSvc,
		requestStore: store,
		now:          func() time.Time { return now },
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return cloneUserSubscription(active), nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return cloneSettlementHead(head), nil
		},
		loadPaymentOrderCandidates: func(context.Context, *dbent.SubscriptionSettlementOrder) ([]SettlementRefundPaymentOrderCandidate, error) {
			return []SettlementRefundPaymentOrderCandidate{
				{
					PaymentOrderID:         1001,
					OrderAmount:            99,
					PayAmount:              99,
					AlreadyRefundedAmount:  20,
					GatewayRefundedAmount:  20,
					Currency:               "CNY",
					RefundChannelAvailable: true,
				},
			}, nil
		},
	}
	record.PreviewFingerprint = settlementRefundNullableReason(settlementRefundPreviewFingerprint(&settlementRefundPreviewComputation{
		Active:     cloneUserSubscription(active),
		Head:       cloneSettlementHead(head),
		RefundMode: SettlementRefundModeHybrid,
		AllocationResult: SettlementRefundAllocationResult{
			RefundResidualValue:    record.RefundResidualValue,
			GatewayRefundableTotal: record.GatewayRefundableTotal,
			ManualTransferAmount:   record.ManualTransferAmount,
			Currency:               "CNY",
			Allocations: []SettlementRefundOrderAllocation{
				{
					PaymentOrderID:         1001,
					OrderAmount:            99,
					PayAmount:              99,
					AlreadyRefundedAmount:  0,
					RefundableOrderAmount:  99,
					AllocatedRefundValue:   99,
					GatewayRefundAmount:    99,
					Currency:               "CNY",
					RefundChannelAvailable: true,
				},
			},
		},
	}))

	result, err := service.SubmitSettlementRefund(context.Background(), SettlementRefundSubmitInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		PreviewID:      record.ID,
		PreviewToken:   "preview-token",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundPreviewStale)
}

func TestSettlementRefundServiceSubmitRejectsMissingManualReceiver(t *testing.T) {
	now := time.Date(2026, 6, 25, 16, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceUserPurchase)
	record := settlementRefundPreviewedRequestRecord(now.Add(-30*time.Second), now.Add(90*time.Second), active, head, SettlementRefundModeHybrid, []SettlementRefundAllocationRecord{
		{
			ID:                   9101,
			RefundRequestID:      9001,
			PaymentOrderID:       1001,
			AllocatedRefundValue: 99,
			GatewayRefundAmount:  99,
			Currency:             "CNY",
			Status:               SettlementRefundAllocationStatusPending,
		},
	})
	record.GatewayRefundableTotal = 99
	record.ManualTransferAmount = record.RefundResidualValue - 99
	service := newSettlementRefundSubmitServiceForTest(t, now, active, head, record, nil, []SettlementRefundPaymentOrderCandidate{
		{
			PaymentOrderID:         1001,
			OrderAmount:            99,
			PayAmount:              99,
			AlreadyRefundedAmount:  0,
			GatewayRefundedAmount:  0,
			Currency:               "CNY",
			RefundChannelAvailable: true,
		},
	})

	result, err := service.SubmitSettlementRefund(context.Background(), SettlementRefundSubmitInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		PreviewID:      record.ID,
		PreviewToken:   "preview-token",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundManualReceiverRequired)
}

func TestSettlementRefundServiceSubmitFreezesSubscription(t *testing.T) {
	now := time.Date(2026, 6, 25, 16, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	record := settlementRefundPreviewedRequestRecord(now.Add(-30*time.Second), now.Add(90*time.Second), active, head, SettlementRefundModeEntitlementOnly, nil)
	store := &settlementRefundSubmitStoreStub{request: record}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	subscriptionSvc := &SubscriptionService{
		userSubRepo: repo,
	}
	service := &SettlementRefundService{
		subscription: subscriptionSvc,
		requestStore: store,
		now:          func() time.Time { return now },
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return cloneUserSubscription(active), nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return cloneSettlementHead(head), nil
		},
	}

	result, err := service.SubmitSettlementRefund(context.Background(), SettlementRefundSubmitInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		PreviewID:      record.ID,
		PreviewToken:   "preview-token",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	require.Equal(t, SettlementRefundStatusSubmitted, result.RefundStatus)
	require.Equal(t, SubscriptionStatusSuspended, result.SubscriptionStatus)
	require.NotNil(t, store.lastSubmit)
	require.Equal(t, SettlementRefundStatusSubmitted, store.lastSubmit.Status)
	require.Equal(t, now, store.lastSubmit.SubmittedAt)
	require.Equal(t, SubscriptionStatusActive, store.lastSubmit.OriginalSubscriptionStatus)

	updated, err := repo.GetByID(context.Background(), active.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusSuspended, updated.Status)
}

func TestSettlementRefundServiceSubmitPersistsManualReceiver(t *testing.T) {
	now := time.Date(2026, 6, 25, 16, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceUserPurchase)
	record := settlementRefundPreviewedRequestRecord(now.Add(-30*time.Second), now.Add(90*time.Second), active, head, SettlementRefundModeHybrid, []SettlementRefundAllocationRecord{
		{
			ID:                   9101,
			RefundRequestID:      9001,
			PaymentOrderID:       1001,
			AllocatedRefundValue: 99,
			GatewayRefundAmount:  99,
			Currency:             "CNY",
			Status:               SettlementRefundAllocationStatusPending,
		},
	})
	record.GatewayRefundableTotal = 99
	record.ManualTransferAmount = record.RefundResidualValue - 99
	store := &settlementRefundSubmitStoreStub{request: record}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	subscriptionSvc := &SubscriptionService{
		userSubRepo: repo,
	}
	service := &SettlementRefundService{
		subscription: subscriptionSvc,
		requestStore: store,
		now:          func() time.Time { return now },
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return cloneUserSubscription(active), nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return cloneSettlementHead(head), nil
		},
		loadPaymentOrderCandidates: func(context.Context, *dbent.SubscriptionSettlementOrder) ([]SettlementRefundPaymentOrderCandidate, error) {
			return []SettlementRefundPaymentOrderCandidate{
				{
					PaymentOrderID:         1001,
					OrderAmount:            99,
					PayAmount:              99,
					AlreadyRefundedAmount:  0,
					GatewayRefundedAmount:  0,
					Currency:               "CNY",
					RefundChannelAvailable: true,
				},
			}, nil
		},
	}

	result, err := service.SubmitSettlementRefund(context.Background(), SettlementRefundSubmitInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		PreviewID:      record.ID,
		PreviewToken:   "preview-token",
		ManualTransfer: &ManualTransferInput{
			ReceiverType:           "wechat_qr",
			ReceiverName:           " Zhang San ",
			ReceiverQRCodeImageURL: " uploads/refund/qr/9001.png ",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, store.lastSubmit)
	require.Equal(t, "wechat_qr", derefStringPtr(store.lastSubmit.ManualReceiverType))
	require.Equal(t, "Zhang San", derefStringPtr(store.lastSubmit.ManualReceiverName))
	require.Equal(t, "uploads/refund/qr/9001.png", derefStringPtr(store.lastSubmit.ManualReceiverQRCodeImageURL))
	require.Equal(t, "", derefStringPtr(store.lastSubmit.ManualReceiverAccount))
	require.Equal(t, now, store.lastSubmit.SubmittedAt)
}

func TestValidateSettlementRefundSubmitManualTransferRequiresAccountOrQRCode(t *testing.T) {
	normalized, err := validateSettlementRefundSubmitManualTransfer(10, "CNY", &ManualTransferInput{
		ReceiverType: "bank_account",
		ReceiverName: "Zhang San",
	})
	require.Nil(t, normalized)
	require.ErrorIs(t, err, ErrSettlementRefundManualReceiverRequired)

	normalized, err = validateSettlementRefundSubmitManualTransfer(10, "CNY", &ManualTransferInput{
		ReceiverType:    "bank_account",
		ReceiverName:    "Zhang San",
		ReceiverAccount: " 622202020202 ",
	})
	require.NoError(t, err)
	require.NotNil(t, normalized)
	require.Equal(t, "622202020202", normalized.ReceiverAccount)
}

func TestValidateSettlementRefundSubmitManualTransferRejectsInvalidQRCodeImageURL(t *testing.T) {
	normalized, err := validateSettlementRefundSubmitManualTransfer(10, "CNY", &ManualTransferInput{
		ReceiverType:           "wechat_qr",
		ReceiverName:           "Zhang San",
		ReceiverQRCodeImageURL: "javascript:alert(1)",
	})
	require.Nil(t, normalized)
	require.ErrorIs(t, err, ErrSettlementRefundManualReceiverImageInvalid)
}

func TestValidateSettlementRefundSubmitManualTransferAllowsStoredImagePath(t *testing.T) {
	normalized, err := validateSettlementRefundSubmitManualTransfer(10, "CNY", &ManualTransferInput{
		ReceiverType:           "wechat_qr",
		ReceiverName:           "Zhang San",
		ReceiverQRCodeImageURL: "uploads/refund/qr/9001.png",
	})
	require.NoError(t, err)
	require.NotNil(t, normalized)
	require.Equal(t, "uploads/refund/qr/9001.png", normalized.ReceiverQRCodeImageURL)
}

func TestValidateSettlementRefundSubmitManualTransferAllowsSmallRemainderWithoutReceiver(t *testing.T) {
	normalized, err := validateSettlementRefundSubmitManualTransfer(0.0046, "CNY", nil)
	require.NoError(t, err)
	require.Nil(t, normalized)

	normalized, err = validateSettlementRefundSubmitManualTransfer(0.0046, "CNY", &ManualTransferInput{
		ReceiverType: "wechat_qr",
	})
	require.NoError(t, err)
	require.NotNil(t, normalized)
	require.Equal(t, "wechat_qr", normalized.ReceiverType)
}

func newSettlementRefundSubmitServiceForTest(t *testing.T, now time.Time, active *UserSubscription, head *dbent.SubscriptionSettlementOrder, record *SettlementRefundRequestRecord, mutateAfterGet func(), paymentCandidates []SettlementRefundPaymentOrderCandidate) *SettlementRefundService {
	t.Helper()
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	subscriptionSvc := &SubscriptionService{
		userSubRepo: repo,
	}
	store := &settlementRefundSubmitStoreStub{request: record}
	previewCache := &settlementRefundSubmitPreviewCacheStub{
		entry: settlementRefundPreviewCacheEntryFromRecord(record, "preview-token"),
	}
	service := &SettlementRefundService{
		subscription: subscriptionSvc,
		requestStore: store,
		previewCache: previewCache,
		now:          func() time.Time { return now },
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			cloned := cloneUserSubscription(active)
			if mutateAfterGet != nil {
				mutateAfterGet()
				mutateAfterGet = nil
			}
			return cloned, nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return cloneSettlementHead(head), nil
		},
		loadPaymentOrderCandidates: func(context.Context, *dbent.SubscriptionSettlementOrder) ([]SettlementRefundPaymentOrderCandidate, error) {
			return append([]SettlementRefundPaymentOrderCandidate(nil), paymentCandidates...), nil
		},
	}
	return service
}

func settlementRefundPreviewCacheEntryFromRecord(record *SettlementRefundRequestRecord, previewToken string) *SettlementRefundPreviewCacheEntry {
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
		PreviewFingerprint:      derefStringPtr(record.PreviewFingerprint),
		PreviewIssuedAt:         record.PreviewIssuedAt,
		PreviewExpiresAt:        record.PreviewExpiresAt,
		Allocations:             settlementRefundAllocationRecordsToPreviewAllocations(record.Allocations),
	}
}

func settlementRefundAllocationRecordsToPreviewAllocations(allocations []SettlementRefundAllocationRecord) []SettlementRefundPreviewAllocation {
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
			SkippedReason:          derefStringPtr(allocation.FailedReason),
		})
	}
	return result
}

func settlementRefundPreviewedRequestRecord(issuedAt, expiresAt time.Time, active *UserSubscription, head *dbent.SubscriptionSettlementOrder, mode string, allocations []SettlementRefundAllocationRecord) *SettlementRefundRequestRecord {
	hash := hashSettlementRefundPreviewToken("preview-token")
	residual := roundSettlementRefundValue(settlementResidualValue(active, settlementResidualBasisValue(head, active, head.AfterSettlementValue)))
	gatewayTotal := 0.0
	manualAmount := 0.0
	currency := settlementRefundPreviewResponseCurrency("")
	allocationInputs := make([]SettlementRefundOrderAllocation, 0, len(allocations))
	if mode != SettlementRefundModeEntitlementOnly {
		for _, allocation := range allocations {
			orderAmount := allocation.OrderAmount
			if orderAmount <= 0 {
				orderAmount = allocation.RefundableOrderAmount
			}
			if orderAmount <= 0 {
				orderAmount = allocation.AllocatedRefundValue
			}
			if orderAmount <= 0 {
				orderAmount = allocation.GatewayRefundAmount
			}

			payAmount := allocation.OrderPayAmount
			if payAmount <= 0 {
				payAmount = allocation.GatewayRefundAmount
			}
			if payAmount <= 0 {
				payAmount = allocation.AllocatedRefundValue
			}
			if payAmount <= 0 {
				payAmount = orderAmount
			}

			alreadyRefundedAmount := allocation.AlreadyRefundedAmount
			refundableOrderAmount := allocation.RefundableOrderAmount
			if refundableOrderAmount <= 0 {
				refundableOrderAmount = remainingRefundableAmount(orderAmount, alreadyRefundedAmount)
			}

			allocationCurrency := settlementRefundPreviewResponseCurrency(allocation.Currency)
			if allocationCurrency != "" {
				currency = allocationCurrency
			}
			refundChannelAvailable := allocation.GatewayRefundAmount > 0 || allocation.AllocatedRefundValue > 0
			if allocation.Status == SettlementRefundAllocationStatusSkipped {
				refundChannelAvailable = false
			}
			gatewayTotal = roundSettlementRefundValue(gatewayTotal + allocation.GatewayRefundAmount)
			allocationInputs = append(allocationInputs, SettlementRefundOrderAllocation{
				PaymentOrderID:         allocation.PaymentOrderID,
				OrderAmount:            orderAmount,
				PayAmount:              payAmount,
				AlreadyRefundedAmount:  alreadyRefundedAmount,
				RefundableOrderAmount:  refundableOrderAmount,
				AllocatedRefundValue:   allocation.AllocatedRefundValue,
				GatewayRefundAmount:    allocation.GatewayRefundAmount,
				Currency:               allocationCurrency,
				RefundChannelAvailable: refundChannelAvailable,
				SkippedReason:          derefStringPtr(allocation.FailedReason),
			})
		}
		switch mode {
		case SettlementRefundModeGatewayRefund:
			manualAmount = 0
		case SettlementRefundModeManualTransfer:
			gatewayTotal = 0
			manualAmount = residual
		default:
			manualAmount = roundSettlementRefundValue(residual - gatewayTotal)
			if manualAmount < 0 {
				manualAmount = 0
			}
		}
	}
	computation := &settlementRefundPreviewComputation{
		Active:     cloneUserSubscription(active),
		Head:       cloneSettlementHead(head),
		RefundMode: mode,
		AllocationResult: SettlementRefundAllocationResult{
			RefundResidualValue:    residual,
			GatewayRefundableTotal: gatewayTotal,
			ManualTransferAmount:   manualAmount,
			Currency:               currency,
			Allocations:            allocationInputs,
		},
	}
	return &SettlementRefundRequestRecord{
		ID:                     9001,
		UserID:                 active.UserID,
		SubscriptionID:         active.ID,
		SettlementID:           head.ID,
		ExpectedSettlementID:   head.ID,
		Status:                 SettlementRefundStatusPreviewed,
		RefundMode:             mode,
		Currency:               currency,
		RefundResidualValue:    residual,
		GatewayRefundableTotal: gatewayTotal,
		ManualTransferAmount:   manualAmount,
		PreviewTokenHash:       hash,
		PreviewFingerprint:     settlementRefundNullableReason(settlementRefundPreviewFingerprint(computation)),
		PreviewIssuedAt:        issuedAt,
		PreviewExpiresAt:       expiresAt,
		Allocations:            allocations,
	}
}

func cloneSettlementRefundRequestRecord(record *SettlementRefundRequestRecord) *SettlementRefundRequestRecord {
	if record == nil {
		return nil
	}
	cloned := *record
	if record.Reason != nil {
		value := *record.Reason
		cloned.Reason = &value
	}
	if record.SubmittedAt != nil {
		value := *record.SubmittedAt
		cloned.SubmittedAt = &value
	}
	if record.FrozenAt != nil {
		value := *record.FrozenAt
		cloned.FrozenAt = &value
	}
	if record.CompletedAt != nil {
		value := *record.CompletedAt
		cloned.CompletedAt = &value
	}
	if record.CancelledAt != nil {
		value := *record.CancelledAt
		cloned.CancelledAt = &value
	}
	if record.OriginalSubscriptionStatus != nil {
		value := *record.OriginalSubscriptionStatus
		cloned.OriginalSubscriptionStatus = &value
	}
	if record.OriginalSubscriptionExpiresAt != nil {
		value := *record.OriginalSubscriptionExpiresAt
		cloned.OriginalSubscriptionExpiresAt = &value
	}
	if record.ManualReceiverType != nil {
		value := *record.ManualReceiverType
		cloned.ManualReceiverType = &value
	}
	if record.ManualReceiverName != nil {
		value := *record.ManualReceiverName
		cloned.ManualReceiverName = &value
	}
	if record.ManualReceiverAccount != nil {
		value := *record.ManualReceiverAccount
		cloned.ManualReceiverAccount = &value
	}
	if record.ManualReceiverQRCodeImageURL != nil {
		value := *record.ManualReceiverQRCodeImageURL
		cloned.ManualReceiverQRCodeImageURL = &value
	}
	if record.ManualTransferProofURL != nil {
		value := *record.ManualTransferProofURL
		cloned.ManualTransferProofURL = &value
	}
	if record.PreviewFingerprint != nil {
		value := *record.PreviewFingerprint
		cloned.PreviewFingerprint = &value
	}
	if record.ManualTransferProofUploadedAt != nil {
		value := *record.ManualTransferProofUploadedAt
		cloned.ManualTransferProofUploadedAt = &value
	}
	if record.ManualTransferOperatorUserID != nil {
		value := *record.ManualTransferOperatorUserID
		cloned.ManualTransferOperatorUserID = &value
	}
	if record.AdminNote != nil {
		value := *record.AdminNote
		cloned.AdminNote = &value
	}
	if len(record.Allocations) > 0 {
		cloned.Allocations = append([]SettlementRefundAllocationRecord(nil), record.Allocations...)
	}
	return &cloned
}

func cloneUserSubscription(sub *UserSubscription) *UserSubscription {
	if sub == nil {
		return nil
	}
	cloned := *sub
	return &cloned
}

func cloneSettlementHead(head *dbent.SubscriptionSettlementOrder) *dbent.SubscriptionSettlementOrder {
	if head == nil {
		return nil
	}
	cloned := *head
	return &cloned
}
