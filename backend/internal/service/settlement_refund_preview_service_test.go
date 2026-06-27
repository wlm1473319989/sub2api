package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type settlementRefundPreviewStoreStub struct {
	lastInput *CreateSettlementRefundPreviewInput
	createFn  func(CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error)
}

func (s *settlementRefundPreviewStoreStub) CreateSettlementRefundPreview(_ context.Context, input CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	s.lastInput = &input
	if s.createFn != nil {
		return s.createFn(input)
	}
	record := &SettlementRefundRequestRecord{
		ID:                     9001,
		UserID:                 input.UserID,
		SubscriptionID:         input.SubscriptionID,
		SettlementID:           input.SettlementID,
		ExpectedSettlementID:   input.ExpectedSettlementID,
		Status:                 input.Status,
		RefundMode:             input.RefundMode,
		Currency:               input.Currency,
		Reason:                 input.Reason,
		RefundResidualValue:    input.RefundResidualValue,
		GatewayRefundableTotal: input.GatewayRefundableTotal,
		ManualTransferAmount:   input.ManualTransferAmount,
		PreviewTokenHash:       input.PreviewTokenHash,
		PreviewFingerprint:     settlementRefundNullableReason(input.PreviewFingerprint),
		PreviewIssuedAt:        input.PreviewIssuedAt,
		PreviewExpiresAt:       input.PreviewExpiresAt,
		Allocations:            make([]SettlementRefundAllocationRecord, 0, len(input.Allocations)),
	}
	for idx, allocation := range input.Allocations {
		record.Allocations = append(record.Allocations, SettlementRefundAllocationRecord{
			ID:                    int64(9100 + idx + 1),
			RefundRequestID:       record.ID,
			PaymentOrderID:        allocation.PaymentOrderID,
			OrderAmount:           allocation.OrderAmount,
			OrderPayAmount:        allocation.OrderPayAmount,
			AlreadyRefundedAmount: allocation.AlreadyRefundedAmount,
			RefundableOrderAmount: allocation.RefundableOrderAmount,
			AllocatedRefundValue:  allocation.AllocatedRefundValue,
			GatewayRefundAmount:   allocation.GatewayRefundAmount,
			Currency:              allocation.Currency,
			Status:                allocation.Status,
			FailedReason:          allocation.FailedReason,
		})
	}
	return record, nil
}

type settlementRefundPreviewCacheStub struct {
	entry     *SettlementRefundPreviewCacheEntry
	lastSet   *SettlementRefundPreviewCacheEntry
	lastTTL   time.Duration
	getFn     func(int64, int64) (*SettlementRefundPreviewCacheEntry, error)
	setFn     func(*SettlementRefundPreviewCacheEntry, time.Duration) error
	deleteFn  func(int64, int64) error
}

func (s *settlementRefundPreviewCacheStub) GetSettlementRefundPreview(_ context.Context, userID, subscriptionID int64) (*SettlementRefundPreviewCacheEntry, error) {
	if s.getFn != nil {
		return s.getFn(userID, subscriptionID)
	}
	if s.entry == nil {
		return nil, nil
	}
	return s.entry, nil
}

func (s *settlementRefundPreviewCacheStub) SetSettlementRefundPreview(_ context.Context, entry *SettlementRefundPreviewCacheEntry, ttl time.Duration) error {
	s.lastSet = entry
	s.lastTTL = ttl
	if s.setFn != nil {
		return s.setFn(entry, ttl)
	}
	s.entry = entry
	return nil
}

func (s *settlementRefundPreviewCacheStub) DeleteSettlementRefundPreview(_ context.Context, userID, subscriptionID int64) error {
	if s.deleteFn != nil {
		return s.deleteFn(userID, subscriptionID)
	}
	s.entry = nil
	return nil
}

func TestSettlementRefundServicePreviewRequiresActiveSubscription(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	service := &SettlementRefundService{
		previewCache: &settlementRefundPreviewCacheStub{},
		now:          func() time.Time { return now },
		generatePreviewToken: func() (string, string, error) {
			return "preview-token", "preview-hash", nil
		},
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return nil, ErrSubscriptionNotFound
		},
	}

	preview, err := service.PreviewSettlementRefund(context.Background(), SettlementRefundPreviewInput{
		UserID:         11,
		SubscriptionID: 22,
	})
	require.Nil(t, preview)
	require.ErrorIs(t, err, ErrActiveSubscriptionRequired)
}

func TestSettlementRefundServicePreviewRequiresSettlementHead(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	service := &SettlementRefundService{
		previewCache: &settlementRefundPreviewCacheStub{},
		now:          func() time.Time { return now },
		generatePreviewToken: func() (string, string, error) {
			return "preview-token", "preview-hash", nil
		},
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return active, nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return nil, nil
		},
	}

	preview, err := service.PreviewSettlementRefund(context.Background(), SettlementRefundPreviewInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
	})
	require.Nil(t, preview)
	require.ErrorIs(t, err, ErrSettlementHeadRequired)
}

func TestSettlementRefundServicePreviewRejectsHeadSubscriptionMismatch(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, 999, domain.SettlementActionSourceExchangeCode)
	service := &SettlementRefundService{
		previewCache: &settlementRefundPreviewCacheStub{},
		now:          func() time.Time { return now },
		generatePreviewToken: func() (string, string, error) {
			return "preview-token", "preview-hash", nil
		},
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return active, nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return head, nil
		},
	}

	preview, err := service.PreviewSettlementRefund(context.Background(), SettlementRefundPreviewInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
	})
	require.Nil(t, preview)
	require.ErrorIs(t, err, ErrSettlementHeadSubscriptionMismatch)
}

func TestSettlementRefundServicePreviewRejectsZeroResidual(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.DailyQuotaKnives = nil
	active.WeeklyQuotaKnives = nil
	active.MonthlyQuotaKnives = nil
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	service := &SettlementRefundService{
		previewCache: &settlementRefundPreviewCacheStub{},
		now:          func() time.Time { return now },
		generatePreviewToken: func() (string, string, error) {
			return "preview-token", "preview-hash", nil
		},
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return active, nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return head, nil
		},
	}

	preview, err := service.PreviewSettlementRefund(context.Background(), SettlementRefundPreviewInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
	})
	require.Nil(t, preview)
	require.ErrorIs(t, err, ErrSettlementRefundZeroResidual)
}

func TestSettlementRefundServicePreviewBuildsHybridPaymentRefund(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceUserPurchase)
	cache := &settlementRefundPreviewCacheStub{}
	service := &SettlementRefundService{
		previewCache: cache,
		now:          func() time.Time { return now },
		generatePreviewToken: func() (string, string, error) {
			return "preview-token", "preview-hash", nil
		},
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return active, nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return head, nil
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

	preview, err := service.PreviewSettlementRefund(context.Background(), SettlementRefundPreviewInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
		Reason:         "no longer needed",
	})
	require.NoError(t, err)
	require.NotNil(t, preview)
	require.Equal(t, SettlementRefundModeHybrid, preview.RefundMode)
	require.Equal(t, "preview-token", preview.PreviewToken)
	require.Equal(t, now, preview.PreviewIssuedAt)
	require.Equal(t, now.Add(2*time.Minute), preview.PreviewExpiresAt)
	require.Greater(t, preview.RefundResidualValue, 99.0)
	require.Equal(t, 99.0, preview.GatewayRefundableTotal)
	require.Greater(t, preview.ManualTransferAmount, 0.0)
	require.True(t, preview.ManualTransferRequired)
	require.Equal(t, payment.DefaultPaymentCurrency, settlementRefundPreviewResponseCurrency(""))
	require.Len(t, preview.Allocations, 1)
	require.Equal(t, int64(1001), preview.Allocations[0].PaymentOrderID)
	require.NotNil(t, cache.lastSet)
	require.Equal(t, SettlementRefundModeHybrid, cache.lastSet.RefundMode)
	require.Equal(t, 1, len(cache.lastSet.Allocations))
	require.Equal(t, int64(1001), cache.lastSet.Allocations[0].PaymentOrderID)
	require.Equal(t, "preview-hash", cache.lastSet.PreviewTokenHash)
	require.Equal(t, settlementRefundPreviewTTL, cache.lastTTL)
}

func TestSettlementRefundServicePreviewBuildsEntitlementOnlyRefund(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	cache := &settlementRefundPreviewCacheStub{}
	service := &SettlementRefundService{
		previewCache: cache,
		now:          func() time.Time { return now },
		generatePreviewToken: func() (string, string, error) {
			return "preview-token", "preview-hash", nil
		},
		loadActiveSubscription: func(context.Context, int64) (*UserSubscription, error) {
			return active, nil
		},
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return head, nil
		},
		loadPaymentOrderCandidates: func(context.Context, *dbent.SubscriptionSettlementOrder) ([]SettlementRefundPaymentOrderCandidate, error) {
			t.Fatal("payment order candidates should not be loaded for entitlement-only refund")
			return nil, nil
		},
	}

	preview, err := service.PreviewSettlementRefund(context.Background(), SettlementRefundPreviewInput{
		UserID:         active.UserID,
		SubscriptionID: active.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, preview)
	require.Equal(t, SettlementRefundModeEntitlementOnly, preview.RefundMode)
	require.Equal(t, 0.0, preview.GatewayRefundableTotal)
	require.Equal(t, 0.0, preview.ManualTransferAmount)
	require.False(t, preview.ManualTransferRequired)
	require.Equal(t, payment.DefaultPaymentCurrency, preview.Currency)
	require.Nil(t, preview.Allocations)
	require.NotNil(t, cache.lastSet)
	require.Equal(t, SettlementRefundModeEntitlementOnly, cache.lastSet.RefundMode)
	require.Nil(t, cache.lastSet.Allocations)
}

func TestSettlementRefundManualTransferRequiredUsesCurrencyTolerance(t *testing.T) {
	require.False(t, SettlementRefundManualTransferRequired(0.0046, "CNY"))
	require.True(t, SettlementRefundManualTransferRequired(0.01, "CNY"))
	require.True(t, SettlementRefundManualTransferRequired(1, "JPY"))
	require.False(t, SettlementRefundManualTransferRequired(0.4, "JPY"))
}

func TestSettlementRefundServicePreviewReusesLiveCachedPreview(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	cache := &settlementRefundPreviewCacheStub{
		entry: &SettlementRefundPreviewCacheEntry{
			PreviewID:               9001,
			PreviewToken:            "preview-token",
			UserID:                  11,
			SubscriptionID:          22,
			SettlementID:            33,
			ExpectedSettlementID:    33,
			RefundMode:              SettlementRefundModeEntitlementOnly,
			Currency:                "CNY",
			RefundResidualValue:     88.8888,
			PreviewTokenHash:        hashSettlementRefundPreviewToken("preview-token"),
			PreviewFingerprint:      "fingerprint",
			PreviewIssuedAt:         now,
			PreviewExpiresAt:        now.Add(90 * time.Second),
		},
	}
	service := &SettlementRefundService{
		previewCache: cache,
		now:          func() time.Time { return now },
	}

	preview, err := service.PreviewSettlementRefund(context.Background(), SettlementRefundPreviewInput{
		UserID:         11,
		SubscriptionID: 22,
	})
	require.NoError(t, err)
	require.NotNil(t, preview)
	require.Equal(t, int64(9001), preview.PreviewID)
	require.Equal(t, "preview-token", preview.PreviewToken)
	require.Nil(t, cache.lastSet)
}

func TestSettlementRefundPaymentOrderCandidateUsesSucceededGatewayRefundHistory(t *testing.T) {
	client, mock := newSettlementRefundStoreSQLMock(t)
	service := &SettlementRefundService{entClient: client}

	order := &dbent.PaymentOrder{
		ID:           1001,
		Amount:       100,
		PayAmount:    80,
		RefundAmount: 20,
		PaymentType:  "stripe",
		OrderType:    "subscription",
		Status:       OrderStatusPartiallyRefunded,
	}

	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(gateway_refund_amount\), 0\)::double precision\s+FROM subscription_refund_allocations`).
		WithArgs(order.ID).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(10.0))

	candidate := service.settlementRefundPaymentOrderCandidate(context.Background(), order)
	require.InDelta(t, 20, candidate.AlreadyRefundedAmount, 1e-9)
	require.InDelta(t, 10, candidate.GatewayRefundedAmount, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func settlementRefundPreviewTestActiveSubscription() *UserSubscription {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	dailyQuota := 10.0
	weeklyQuota := 70.0
	monthlyQuota := 300.0
	dailyWindowStart := now.Add(-2 * time.Hour)
	weeklyWindowStart := now.Add(-24 * time.Hour)
	monthlyWindowStart := now.Add(-10 * 24 * time.Hour)
	planPrice := 168.5
	return &UserSubscription{
		ID:                 22,
		UserID:             11,
		StartsAt:           now.Add(-10 * 24 * time.Hour),
		ExpiresAt:          now.Add(20 * 24 * time.Hour),
		Status:             SubscriptionStatusActive,
		PlanPriceSnapshot:  &planPrice,
		DailyQuotaKnives:   &dailyQuota,
		WeeklyQuotaKnives:  &weeklyQuota,
		MonthlyQuotaKnives: &monthlyQuota,
		DailyUsedKnives:    1,
		WeeklyUsedKnives:   5,
		MonthlyUsedKnives:  12,
		DailyWindowStart:   &dailyWindowStart,
		WeeklyWindowStart:  &weeklyWindowStart,
		MonthlyWindowStart: &monthlyWindowStart,
	}
}

func settlementRefundPreviewTestSettlementHead(userID, subscriptionID int64, actionSource string) *dbent.SubscriptionSettlementOrder {
	triggerRefID := int64(1001)
	afterPlanPrice := 168.5
	expiresAt := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	return &dbent.SubscriptionSettlementOrder{
		ID:                      33,
		UserID:                  userID,
		ActionType:              domain.SettlementActionRenew,
		ActionSource:            actionSource,
		Status:                  domain.SettlementStatusEffective,
		TriggerRefType:          domain.SettlementTriggerRefPaymentOrder,
		TriggerRefID:            &triggerRefID,
		AfterSettlementValue:    168.5,
		AfterPlanPriceSnapshot:  &afterPlanPrice,
		AfterUserSubscriptionID: &subscriptionID,
		AfterSubscriptionStatus: domain.SubscriptionStatusActive,
		AfterExpiresAt:          &expiresAt,
	}
}
