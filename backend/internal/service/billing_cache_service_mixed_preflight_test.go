//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type mixedPreflightCache struct {
	BillingCache
	balance     float64
	quotaCalled bool
}

func (m *mixedPreflightCache) GetUserBalance(_ context.Context, _ int64) (float64, error) {
	return m.balance, nil
}

func (m *mixedPreflightCache) GetSubscriptionCache(_ context.Context, _ int64) (*SubscriptionCacheData, error) {
	return nil, errors.New("cache miss")
}

func (m *mixedPreflightCache) GetUserPlatformQuotaCache(_ context.Context, _ int64, _ string) (*UserPlatformQuotaCacheEntry, bool, error) {
	m.quotaCalled = true
	daily := 0.0
	now := time.Now().UTC()
	return &UserPlatformQuotaCacheEntry{
		DailyUsageUSD:    0,
		DailyLimitUSD:    &daily,
		DailyWindowStart: &now,
		SchemaVersion:    UserPlatformQuotaCacheSchemaV1,
	}, true, nil
}

func (m *mixedPreflightCache) DeleteUserPlatformQuotaCache(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mixedPreflightCache) SetUserPlatformQuotaCache(_ context.Context, _ int64, _ string, _ *UserPlatformQuotaCacheEntry, _ time.Duration) error {
	return nil
}

func newMixedPreflightService(t *testing.T, cache BillingCache) *BillingCacheService {
	t.Helper()
	cfg := &config.Config{}
	cfg.Billing.UserPlatformQuotaCacheTTLSeconds = 60
	return &BillingCacheService{
		cache:                 cache,
		cfg:                   cfg,
		userPlatformQuotaRepo: &fakeQuotaRepo{},
	}
}

func TestCheckBillingEligibility_MixedPreflightCombinations(t *testing.T) {
	t.Parallel()

	group := &Group{
		ID:     10,
		Status: StatusActive,
	}
	user := &User{ID: 42}

	cases := []struct {
		name            string
		cache           *mixedPreflightCache
		subscription    *UserSubscription
		wantErr         error
		wantSubResolved bool
		wantQuotaCalled bool
	}{
		{
			name:  "subscription_only_allows_subscription_path",
			cache: &mixedPreflightCache{balance: 0},
			subscription: &UserSubscription{
				ID:        88,
				UserID:    42,
				GroupID:   group.ID,
				Status:    SubscriptionStatusActive,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			wantSubResolved: true,
		},
		{
			name:  "balance_only_allows_balance_fallback",
			cache: &mixedPreflightCache{balance: 12},
			subscription: &UserSubscription{
				ID:        88,
				UserID:    42,
				GroupID:   group.ID,
				Status:    SubscriptionStatusExpired,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			wantSubResolved: false,
		},
		{
			name:  "both_available_prefers_subscription_path",
			cache: &mixedPreflightCache{balance: 12},
			subscription: &UserSubscription{
				ID:        88,
				UserID:    42,
				GroupID:   group.ID,
				Status:    SubscriptionStatusActive,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			wantSubResolved: true,
		},
		{
			name:  "neither_available_rejects",
			cache: &mixedPreflightCache{balance: 0},
			subscription: &UserSubscription{
				ID:        88,
				UserID:    42,
				GroupID:   group.ID,
				Status:    SubscriptionStatusExpired,
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			wantErr: ErrSubscriptionInvalid,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMixedPreflightService(t, tc.cache)
			resolvedSub, err := svc.CheckBillingEligibility(context.Background(), user, nil, group, tc.subscription, "")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CheckBillingEligibility error = %v, want %v", err, tc.wantErr)
			}
			if (resolvedSub != nil) != tc.wantSubResolved {
				t.Fatalf("resolved subscription present = %v, want %v", resolvedSub != nil, tc.wantSubResolved)
			}
			if tc.wantQuotaCalled != tc.cache.quotaCalled {
				t.Fatalf("quotaCalled = %v, want %v", tc.cache.quotaCalled, tc.wantQuotaCalled)
			}
		})
	}
}

func TestCheckBillingEligibility_BalanceFallbackStillAppliesPlatformQuota(t *testing.T) {
	t.Parallel()

	group := &Group{
		ID:     10,
		Status: StatusActive,
	}
	subscription := &UserSubscription{
		ID:        88,
		UserID:    42,
		GroupID:   group.ID,
		Status:    SubscriptionStatusExpired,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	user := &User{ID: 42}
	cache := &mixedPreflightCache{balance: 5}
	svc := newMixedPreflightService(t, cache)

	resolvedSub, err := svc.CheckBillingEligibility(context.Background(), user, nil, group, subscription, "anthropic")
	if !errors.Is(err, ErrUserPlatformDailyQuotaExhausted) {
		t.Fatalf("CheckBillingEligibility error = %v, want %v", err, ErrUserPlatformDailyQuotaExhausted)
	}
	if resolvedSub != nil {
		t.Fatal("expected balance fallback to clear subscription context")
	}
	if !cache.quotaCalled {
		t.Fatal("expected platform quota check to run after falling back to balance")
	}
}
