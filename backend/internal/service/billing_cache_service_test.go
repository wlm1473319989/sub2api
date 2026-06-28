package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type billingCacheWorkerStub struct {
	balanceUpdates      int64
	subscriptionUpdates int64
}

func (b *billingCacheWorkerStub) GetUserBalance(ctx context.Context, userID int64) (float64, error) {
	return 0, errors.New("not implemented")
}

func (b *billingCacheWorkerStub) SetUserBalance(ctx context.Context, userID int64, balance float64) error {
	atomic.AddInt64(&b.balanceUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) SetUserBalanceIfLower(ctx context.Context, userID int64, balance float64) error {
	atomic.AddInt64(&b.balanceUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) DeductUserBalance(ctx context.Context, userID int64, amount float64) error {
	atomic.AddInt64(&b.balanceUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) InvalidateUserBalance(ctx context.Context, userID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetSubscriptionCache(ctx context.Context, userID int64) (*SubscriptionCacheData, error) {
	return nil, errors.New("not implemented")
}

func (b *billingCacheWorkerStub) SetSubscriptionCache(ctx context.Context, userID int64, data *SubscriptionCacheData) error {
	atomic.AddInt64(&b.subscriptionUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) UpdateSubscriptionUsage(ctx context.Context, userID int64, cost float64) error {
	atomic.AddInt64(&b.subscriptionUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) InvalidateSubscriptionCache(ctx context.Context, userID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetAPIKeyRateLimit(ctx context.Context, keyID int64) (*APIKeyRateLimitCacheData, error) {
	return nil, errors.New("not implemented")
}

func (b *billingCacheWorkerStub) SetAPIKeyRateLimit(ctx context.Context, keyID int64, data *APIKeyRateLimitCacheData) error {
	return nil
}

func (b *billingCacheWorkerStub) UpdateAPIKeyRateLimitUsage(ctx context.Context, keyID int64, cost float64) error {
	return nil
}

func (b *billingCacheWorkerStub) InvalidateAPIKeyRateLimit(ctx context.Context, keyID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetUserPlatformQuotaCache(ctx context.Context, userID int64, platform string) (*UserPlatformQuotaCacheEntry, bool, error) {
	return nil, false, nil
}

func (b *billingCacheWorkerStub) SetUserPlatformQuotaCache(ctx context.Context, userID int64, platform string, entry *UserPlatformQuotaCacheEntry, ttl time.Duration) error {
	return nil
}

func (b *billingCacheWorkerStub) DeleteUserPlatformQuotaCache(ctx context.Context, userID int64, platform string) error {
	return nil
}

func (b *billingCacheWorkerStub) IncrUserPlatformQuotaUsageCache(ctx context.Context, userID int64, platform string, cost float64, ttl time.Duration, markDirty bool) error {
	return nil
}

func (b *billingCacheWorkerStub) PopDirtyUserPlatformQuotaKeys(ctx context.Context, n int) ([]UserPlatformQuotaKey, error) {
	return nil, nil
}

func (b *billingCacheWorkerStub) ReaddDirtyUserPlatformQuotaKeys(ctx context.Context, keys []UserPlatformQuotaKey) error {
	return nil
}

func (b *billingCacheWorkerStub) BatchGetUserPlatformQuotaCache(ctx context.Context, keys []UserPlatformQuotaKey) ([]*UserPlatformQuotaCacheEntry, error) {
	return nil, nil
}

func TestBillingCacheServiceQueueHighLoad(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	start := time.Now()
	for i := 0; i < cacheWriteBufferSize*2; i++ {
		svc.QueueDeductBalance(1, 1)
	}
	require.Less(t, time.Since(start), 2*time.Second)

	svc.QueueUpdateSubscriptionUsage(1, 1.5)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&cache.balanceUpdates) > 0
	}, 2*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&cache.subscriptionUpdates) > 0
	}, 2*time.Second, 10*time.Millisecond)
}

func TestBillingCacheServiceEnqueueAfterStopReturnsFalse(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	svc.Stop()

	enqueued := svc.enqueueCacheWrite(cacheWriteTask{
		kind:   cacheWriteDeductBalance,
		userID: 1,
		amount: 1,
	})
	require.False(t, enqueued)
}

type billingBalanceFloorStub struct {
	BillingCache

	mu                  sync.Mutex
	hasBalance          bool
	balance             float64
	conditionalSetCalls atomic.Int64
}

func (b *billingBalanceFloorStub) GetUserBalance(ctx context.Context, userID int64) (float64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.hasBalance {
		return 0, errors.New("cache miss")
	}
	return b.balance, nil
}

func (b *billingBalanceFloorStub) SetUserBalance(ctx context.Context, userID int64, balance float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hasBalance = true
	b.balance = balance
	return nil
}

func (b *billingBalanceFloorStub) SetUserBalanceIfLower(ctx context.Context, userID int64, balance float64) error {
	b.conditionalSetCalls.Add(1)
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.hasBalance || balance <= b.balance {
		b.hasBalance = true
		b.balance = balance
	}
	return nil
}

func (b *billingBalanceFloorStub) DeductUserBalance(ctx context.Context, userID int64, amount float64) error {
	return nil
}

func (b *billingBalanceFloorStub) InvalidateUserBalance(ctx context.Context, userID int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hasBalance = false
	b.balance = 0
	return nil
}

func (b *billingBalanceFloorStub) snapshot() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.balance
}

func TestBillingCacheService_AsyncBalanceWritesDoNotRaiseCommittedNonPositiveBalance(t *testing.T) {
	cache := &billingBalanceFloorStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	require.NoError(t, svc.SyncCommittedUserBalance(context.Background(), 1, -0.5))

	svc.QueueSyncCommittedUserBalance(1, 3)
	require.True(t, svc.enqueueCacheWrite(cacheWriteTask{
		kind:    cacheWriteSetBalance,
		userID:  1,
		balance: 8,
	}))

	require.Eventually(t, func() bool {
		return cache.conditionalSetCalls.Load() >= 2
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, -0.5, cache.snapshot())
}

func TestBillingCacheServiceGetUserBalance_ClampsStalePositiveCacheHitWithFloor(t *testing.T) {
	cache := &billingBalanceFloorStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	require.NoError(t, svc.SyncCommittedUserBalance(context.Background(), 2, -1))
	require.NoError(t, cache.SetUserBalance(context.Background(), 2, 9))

	balance, err := svc.GetUserBalance(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, -1.0, balance)
	require.Equal(t, 9.0, cache.snapshot())
}

func TestBillingCacheServiceInvalidateUserBalance_ClearsCommittedBalanceFloor(t *testing.T) {
	cache := &billingBalanceFloorStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	require.NoError(t, svc.SyncCommittedUserBalance(context.Background(), 3, -1))
	require.NoError(t, svc.InvalidateUserBalance(context.Background(), 3))
	require.NoError(t, cache.SetUserBalance(context.Background(), 3, 12))

	balance, err := svc.GetUserBalance(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, 12.0, balance)
}
