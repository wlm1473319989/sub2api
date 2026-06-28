//go:build unit

package service

import (
	"context"
	"testing"
	"time"
)

func TestResolveUsageBillingSplitFromRawCost_SubscriptionFirstAcrossDifferentMultipliers(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	oneDayStart := now.Add(-time.Hour)
	oneDayExpiry := oneDayStart.Add(24 * time.Hour)
	dailyQuota := 10.0
	newSubscription := func(remaining float64) *UserSubscription {
		used := maxFloat64(dailyQuota-remaining, 0)
		return &UserSubscription{
			ID:               42,
			StartsAt:         oneDayStart,
			ExpiresAt:        oneDayExpiry,
			DailyQuotaKnives: &dailyQuota,
			DailyUsedKnives:  used,
			DailyWindowStart: &oneDayStart,
		}
	}

	tests := []struct {
		name                 string
		totalCost            float64
		subscription         *UserSubscription
		subscriptionRate     float64
		balanceRate          float64
		wantSubscriptionCost float64
		wantBalanceCost      float64
		wantType             int8
	}{
		{
			name:                 "subscription fully covers raw cost at subscription rate",
			totalCost:            1.0,
			subscription:         newSubscription(dailyQuota),
			subscriptionRate:     0.5,
			balanceRate:          2.0,
			wantSubscriptionCost: 0.5,
			wantBalanceCost:      0,
			wantType:             BillingTypeSubscription,
		},
		{
			name:                 "mixed billing spends subscription quota first then falls back to balance",
			totalCost:            1.0,
			subscription:         newSubscription(0.25),
			subscriptionRate:     0.5,
			balanceRate:          2.0,
			wantSubscriptionCost: 0.25,
			wantBalanceCost:      1.0,
			wantType:             BillingTypeMixed,
		},
		{
			name:                 "exhausted subscription falls back to balance multiplier",
			totalCost:            1.0,
			subscription:         newSubscription(0),
			subscriptionRate:     0.5,
			balanceRate:          2.0,
			wantSubscriptionCost: 0,
			wantBalanceCost:      2.0,
			wantType:             BillingTypeBalance,
		},
		{
			name:                 "zero subscription multiplier makes covered usage free",
			totalCost:            1.0,
			subscription:         newSubscription(dailyQuota),
			subscriptionRate:     0,
			balanceRate:          2.0,
			wantSubscriptionCost: 0,
			wantBalanceCost:      0,
			wantType:             BillingTypeBalance,
		},
		{
			name:                 "missing subscription uses balance billing only",
			totalCost:            1.0,
			subscription:         nil,
			subscriptionRate:     0.5,
			balanceRate:          2.0,
			wantSubscriptionCost: 0,
			wantBalanceCost:      2.0,
			wantType:             BillingTypeBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			split := resolveUsageBillingSplitFromRawCost(
				tt.totalCost,
				tt.subscription,
				tt.subscriptionRate,
				tt.balanceRate,
			)

			if diff := split.SubscriptionCost - tt.wantSubscriptionCost; diff > 1e-12 || diff < -1e-12 {
				t.Errorf("SubscriptionCost = %v, want %v", split.SubscriptionCost, tt.wantSubscriptionCost)
			}
			if diff := split.BalanceCost - tt.wantBalanceCost; diff > 1e-12 || diff < -1e-12 {
				t.Errorf("BalanceCost = %v, want %v", split.BalanceCost, tt.wantBalanceCost)
			}
			if split.billingType() != tt.wantType {
				t.Errorf("BillingType = %v, want %v", split.billingType(), tt.wantType)
			}
			if tt.totalCost > 0 {
				wantEffective := (tt.wantSubscriptionCost + tt.wantBalanceCost) / tt.totalCost
				if diff := split.EffectiveRateMultiplier - wantEffective; diff > 1e-12 || diff < -1e-12 {
					t.Errorf("EffectiveRateMultiplier = %v, want %v", split.EffectiveRateMultiplier, wantEffective)
				}
			}
		})
	}
}

func TestApplyUsageBilling_SyncsSplitCostsBackToUsageLog(t *testing.T) {
	t.Parallel()

	subID := int64(42)
	log := &UsageLog{
		RequestID: "req-mixed-sync",
		Model:     "gpt-5",
	}
	p := &postUsageBillingParams{
		Cost:                       &CostBreakdown{TotalCost: 1.0, ActualCost: 1.0},
		User:                       &User{ID: 1},
		APIKey:                     &APIKey{ID: 2},
		Account:                    &Account{ID: 3},
		BalanceRateMultiplier:      1,
		SubscriptionRateMultiplier: 1,
		Subscription: &UserSubscription{
			ID:               subID,
			StartsAt:         time.Now().Add(-time.Hour),
			ExpiresAt:        time.Now().Add(24 * time.Hour),
			DailyQuotaKnives: func() *float64 { v := 10.0; return &v }(),
		},
	}

	_, err := applyUsageBilling(context.Background(), "req-mixed-sync", log, p, &billingDeps{}, nil)
	if err != nil {
		t.Fatalf("applyUsageBilling returned error: %v", err)
	}
	if log.SubscriptionCost != 1.0 {
		t.Fatalf("SubscriptionCost = %v, want 1.0", log.SubscriptionCost)
	}
	if log.BalanceCost != 0 {
		t.Fatalf("BalanceCost = %v, want 0", log.BalanceCost)
	}
	if log.BillingType != BillingTypeSubscription {
		t.Fatalf("BillingType = %v, want %v", log.BillingType, BillingTypeSubscription)
	}
}

func TestApplyUsageBilling_LegacyFallbackAllowsNegativeBalancePortion(t *testing.T) {
	t.Parallel()

	subID := int64(43)
	groupID := int64(8)
	dailyQuota := 1.0
	startsAt := time.Now().Add(-time.Hour)
	expiresAt := startsAt.Add(24 * time.Hour)
	log := &UsageLog{
		RequestID: "req-negative-balance-fallback",
		Model:     "gpt-5",
	}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	p := &postUsageBillingParams{
		Cost:                       &CostBreakdown{TotalCost: 2.0, ActualCost: 2.0},
		User:                       &User{ID: 1, Balance: 0.1},
		APIKey:                     &APIKey{ID: 2, GroupID: &groupID},
		Account:                    &Account{ID: 3},
		BalanceRateMultiplier:      1,
		SubscriptionRateMultiplier: 1,
		Subscription: &UserSubscription{
			ID:               subID,
			StartsAt:         startsAt,
			ExpiresAt:        expiresAt,
			DailyQuotaKnives: &dailyQuota,
			DailyWindowStart: &startsAt,
		},
	}
	deps := &billingDeps{
		userRepo:            userRepo,
		userSubRepo:         subRepo,
		billingCacheService: &BillingCacheService{},
		deferredService:     &DeferredService{},
	}

	applied, err := applyUsageBilling(context.Background(), log.RequestID, log, p, deps, nil)
	if err != nil {
		t.Fatalf("applyUsageBilling returned error: %v", err)
	}
	if !applied {
		t.Fatal("applyUsageBilling should report applied for legacy fallback path")
	}
	if log.BillingType != BillingTypeMixed {
		t.Fatalf("BillingType = %v, want %v", log.BillingType, BillingTypeMixed)
	}
	if log.SubscriptionCost != 1.0 {
		t.Fatalf("SubscriptionCost = %v, want 1.0", log.SubscriptionCost)
	}
	if log.BalanceCost != 1.0 {
		t.Fatalf("BalanceCost = %v, want 1.0", log.BalanceCost)
	}
	if subRepo.incrementCalls != 1 {
		t.Fatalf("IncrementUsage calls = %d, want 1", subRepo.incrementCalls)
	}
	if userRepo.deductCalls != 1 {
		t.Fatalf("DeductBalance calls = %d, want 1", userRepo.deductCalls)
	}
	if userRepo.lastAmount != 1.0 {
		t.Fatalf("DeductBalance amount = %v, want 1.0", userRepo.lastAmount)
	}
}
