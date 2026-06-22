//go:build unit

package service

import (
	"context"
	"testing"
	"time"
)

// TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier locks in the fix
// that subscription-mode billing honours the group (and any user-specific) rate
// multiplier — i.e. cmd.SubscriptionCost tracks ActualCost (= TotalCost *
// RateMultiplier), not raw TotalCost.
func TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	subID := int64(42)
	now := time.Now().UTC()
	oneDayStart := now.Add(-time.Hour)
	oneDayExpiry := oneDayStart.Add(24 * time.Hour)
	dailyQuota := 10.0

	tests := []struct {
		name           string
		totalCost      float64
		actualCost     float64
		isSubscription bool
		subscription   *UserSubscription
		wantSub        float64
		wantBalance    float64
		wantType       int8
	}{
		{
			name:           "subscription with 2x multiplier consumes 2x quota",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			subscription: &UserSubscription{
				ID:               subID,
				StartsAt:         now.Add(-time.Hour),
				ExpiresAt:        now.Add(24 * time.Hour),
				DailyQuotaKnives: &dailyQuota,
			},
			wantSub:     2.0,
			wantBalance: 0,
			wantType:    BillingTypeSubscription,
		},
		{
			name:           "subscription with 0.5x multiplier consumes 0.5x quota",
			totalCost:      1.0,
			actualCost:     0.5,
			isSubscription: true,
			subscription: &UserSubscription{
				ID:               subID,
				StartsAt:         now.Add(-time.Hour),
				ExpiresAt:        now.Add(24 * time.Hour),
				DailyQuotaKnives: &dailyQuota,
			},
			wantSub:     0.5,
			wantBalance: 0,
			wantType:    BillingTypeSubscription,
		},
		{
			name:           "free subscription (multiplier 0) consumes no quota",
			totalCost:      1.0,
			actualCost:     0,
			isSubscription: true,
			subscription: &UserSubscription{
				ID:               subID,
				StartsAt:         now.Add(-time.Hour),
				ExpiresAt:        now.Add(24 * time.Hour),
				DailyQuotaKnives: &dailyQuota,
			},
			wantSub:     0,
			wantBalance: 0,
			wantType:    BillingTypeBalance,
		},
		{
			name:           "balance billing keeps using ActualCost (regression)",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: false,
			subscription:   &UserSubscription{ID: subID},
			wantSub:        0,
			wantBalance:    2.0,
			wantType:       BillingTypeBalance,
		},
		{
			name:           "mixed split uses subscription remainder then balance",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			subscription: &UserSubscription{
				ID:               subID,
				StartsAt:         oneDayStart,
				ExpiresAt:        oneDayExpiry,
				DailyQuotaKnives: &dailyQuota,
				DailyUsedKnives:  8.5,
				DailyWindowStart: &oneDayStart,
			},
			wantSub:     1.5,
			wantBalance: 0.5,
			wantType:    BillingTypeMixed,
		},
		{
			name:           "legacy subscription without snapshot quota falls back to balance billing",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			subscription: &UserSubscription{
				ID:        subID,
				StartsAt:  now.Add(-time.Hour),
				ExpiresAt: now.Add(24 * time.Hour),
			},
			wantSub:     0,
			wantBalance: 2.0,
			wantType:    BillingTypeBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &postUsageBillingParams{
				Cost:         &CostBreakdown{TotalCost: tt.totalCost, ActualCost: tt.actualCost},
				User:         &User{ID: 1},
				APIKey:       &APIKey{ID: 2, GroupID: &groupID},
				Account:      &Account{ID: 3},
				Subscription: tt.subscription,
			}

			cmd := buildUsageBillingCommand("req-1", nil, p)
			if cmd == nil {
				t.Fatal("buildUsageBillingCommand returned nil")
			}
			if cmd.SubscriptionCost != tt.wantSub {
				t.Errorf("SubscriptionCost = %v, want %v", cmd.SubscriptionCost, tt.wantSub)
			}
			if cmd.BalanceCost != tt.wantBalance {
				t.Errorf("BalanceCost = %v, want %v", cmd.BalanceCost, tt.wantBalance)
			}
			if cmd.BillingType != tt.wantType {
				t.Errorf("BillingType = %v, want %v", cmd.BillingType, tt.wantType)
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
		Cost:    &CostBreakdown{TotalCost: 1.0, ActualCost: 1.0},
		User:    &User{ID: 1},
		APIKey:  &APIKey{ID: 2},
		Account: &Account{ID: 3},
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
		Cost:    &CostBreakdown{TotalCost: 2.0, ActualCost: 2.0},
		User:    &User{ID: 1, Balance: 0.1},
		APIKey:  &APIKey{ID: 2, GroupID: &groupID},
		Account: &Account{ID: 3},
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
