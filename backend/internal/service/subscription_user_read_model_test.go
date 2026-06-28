package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type subscriptionUserReadRepoStub struct {
	userSubRepoNoop

	getActiveByUserID func(context.Context, int64) (*UserSubscription, error)
	hasActiveByUserID func(context.Context, int64) (bool, error)
}

func (s *subscriptionUserReadRepoStub) GetActiveByUserID(_ context.Context, userID int64) (*UserSubscription, error) {
	if s.getActiveByUserID == nil {
		return nil, ErrSubscriptionNotFound
	}
	return s.getActiveByUserID(context.Background(), userID)
}

func (s *subscriptionUserReadRepoStub) HasActiveByUserID(_ context.Context, userID int64) (bool, error) {
	if s.hasActiveByUserID == nil {
		return false, nil
	}
	return s.hasActiveByUserID(context.Background(), userID)
}

func TestGetActiveSubscriptionByUser_NormalizesReadModel(t *testing.T) {
	now := time.Now()
	dailyStart := now.Add(-48 * time.Hour)
	weeklyStart := now.Add(-8 * 24 * time.Hour)
	monthlyStart := now.Add(-31 * 24 * time.Hour)

	repo := &subscriptionUserReadRepoStub{
		getActiveByUserID: func(context.Context, int64) (*UserSubscription, error) {
			return &UserSubscription{
				ID:                 1,
				UserID:             42,
				Status:             SubscriptionStatusActive,
				StartsAt:           now.Add(-10 * time.Hour),
				ExpiresAt:          now.Add(24 * time.Hour),
				DailyWindowStart:   &dailyStart,
				WeeklyWindowStart:  &weeklyStart,
				MonthlyWindowStart: &monthlyStart,
				DailyUsageUSD:      3,
				WeeklyUsageUSD:     7,
				MonthlyUsageUSD:    9,
				DailyUsedKnives:    3,
				WeeklyUsedKnives:   7,
				MonthlyUsedKnives:  9,
			}, nil
		},
	}

	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub, err := svc.GetActiveSubscriptionByUser(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Nil(t, sub.DailyWindowStart)
	require.Nil(t, sub.WeeklyWindowStart)
	require.Nil(t, sub.MonthlyWindowStart)
	require.Zero(t, sub.DailyUsageUSD)
	require.Zero(t, sub.WeeklyUsageUSD)
	require.Zero(t, sub.MonthlyUsageUSD)
	require.Zero(t, sub.DailyUsedKnives)
	require.Zero(t, sub.WeeklyUsedKnives)
	require.Zero(t, sub.MonthlyUsedKnives)
}

func TestGetActiveSubscriptionByUser_Conflict(t *testing.T) {
	repo := &subscriptionUserReadRepoStub{
		getActiveByUserID: func(context.Context, int64) (*UserSubscription, error) {
			return nil, ErrMultipleActiveSubscriptions
		},
	}

	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	_, err := svc.GetActiveSubscriptionByUser(context.Background(), 7)
	require.ErrorIs(t, err, ErrMultipleActiveSubscriptions)
}

func TestHasActiveSubscription(t *testing.T) {
	repo := &subscriptionUserReadRepoStub{
		hasActiveByUserID: func(context.Context, int64) (bool, error) {
			return true, nil
		},
	}

	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	hasActive, err := svc.HasActiveSubscription(context.Background(), 9)
	require.NoError(t, err)
	require.True(t, hasActive)
}
