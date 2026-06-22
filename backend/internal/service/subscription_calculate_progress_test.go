package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSubscriptionService() *SubscriptionService {
	return &SubscriptionService{}
}

func ptrFloat64(v float64) *float64  { return &v }
func ptrTime(t time.Time) *time.Time { return &t }

type progressByIDRepoStub struct {
	userSubRepoNoop
	sub *UserSubscription
}

func (s progressByIDRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	if s.sub == nil {
		return nil, ErrSubscriptionNotFound
	}
	cp := *s.sub
	return &cp, nil
}

type progressActiveRepoStub struct {
	userSubRepoNoop
	subs []UserSubscription
}

func (s progressActiveRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	out := make([]UserSubscription, len(s.subs))
	copy(out, s.subs)
	return out, nil
}

func TestCalculateProgress_BasicFields(t *testing.T) {
	svc := newTestSubscriptionService()
	now := time.Now()
	planName := "Premium"

	sub := &UserSubscription{
		ID:               100,
		PlanNameSnapshot: &planName,
		ExpiresAt:        now.Add(30 * 24 * time.Hour),
	}

	progress := svc.calculateProgress(sub)

	assert.Equal(t, int64(100), progress.ID)
	assert.Equal(t, "Premium", progress.DisplayName)
	assert.Equal(t, sub.ExpiresAt, progress.ExpiresAt)
	assert.True(t, progress.ExpiresInDays == 29 || progress.ExpiresInDays == 30)
	assert.Nil(t, progress.Daily)
	assert.Nil(t, progress.Weekly)
	assert.Nil(t, progress.Monthly)
}

func TestCalculateProgress_PrefersPlanSnapshotNameWithoutGroup(t *testing.T) {
	svc := newTestSubscriptionService()
	planName := "Starter Plan"
	sub := &UserSubscription{
		ID:               101,
		PlanNameSnapshot: &planName,
		ExpiresAt:        time.Now().Add(7 * 24 * time.Hour),
	}

	progress := svc.calculateProgress(sub)

	assert.Equal(t, "Starter Plan", progress.DisplayName)
}

func TestCalculateProgress_FallsBackToGenericName(t *testing.T) {
	svc := newTestSubscriptionService()
	sub := &UserSubscription{
		ID:        102,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	progress := svc.calculateProgress(sub)

	assert.Equal(t, "Subscription #102", progress.DisplayName)
}

func TestGetSubscriptionProgress_PlanOnlySubscriptionDoesNotRequireGroupLookup(t *testing.T) {
	planName := "Starter Plan"
	sub := &UserSubscription{
		ID:               103,
		PlanNameSnapshot: &planName,
		ExpiresAt:        time.Now().Add(7 * 24 * time.Hour),
	}
	svc := &SubscriptionService{
		groupRepo:   groupRepoNoop{},
		userSubRepo: progressByIDRepoStub{sub: sub},
	}

	progress, err := svc.GetSubscriptionProgress(context.Background(), sub.ID)

	require.NoError(t, err)
	require.NotNil(t, progress)
	assert.Equal(t, "Starter Plan", progress.DisplayName)
}

func TestGetUserSubscriptionsWithProgress_IncludesPlanOnlySubscriptionWithoutGroup(t *testing.T) {
	planName := "Starter Plan"
	svc := &SubscriptionService{
		groupRepo: groupRepoNoop{},
		userSubRepo: progressActiveRepoStub{
			subs: []UserSubscription{
				{
					ID:               104,
					UserID:           42,
					PlanNameSnapshot: &planName,
					Status:           SubscriptionStatusActive,
					ExpiresAt:        time.Now().Add(7 * 24 * time.Hour),
				},
			},
		},
	}

	progresses, err := svc.GetUserSubscriptionsWithProgress(context.Background(), 42)

	require.NoError(t, err)
	require.Len(t, progresses, 1)
	assert.Equal(t, "Starter Plan", progresses[0].DisplayName)
}

func TestCalculateProgress_DailyUsage(t *testing.T) {
	svc := newTestSubscriptionService()
	now := time.Now()
	dailyStart := now.Add(-12 * time.Hour)

	sub := &UserSubscription{
		ID:               1,
		ExpiresAt:        now.Add(10 * 24 * time.Hour),
		DailyUsedKnives:  3.0,
		DailyQuotaKnives: ptrFloat64(10.0),
		DailyWindowStart: ptrTime(dailyStart),
	}

	progress := svc.calculateProgress(sub)

	require.NotNil(t, progress.Daily)
	assert.Equal(t, 10.0, progress.Daily.LimitUSD)
	assert.Equal(t, 3.0, progress.Daily.UsedUSD)
	assert.Equal(t, 7.0, progress.Daily.RemainingUSD)
	assert.Equal(t, 30.0, progress.Daily.Percentage)
	assert.Equal(t, dailyStart, progress.Daily.WindowStart)
}

func TestCalculateProgress_DailyCardUsesExpiryAsDailyResetTime(t *testing.T) {
	svc := newTestSubscriptionService()
	startsAt := time.Now().Add(-12 * time.Hour)
	dailyStart := time.Date(startsAt.Year(), startsAt.Month(), startsAt.Day(), 0, 0, 0, 0, startsAt.Location())
	expiresAt := startsAt.Add(24 * time.Hour)

	sub := &UserSubscription{
		ID:               1,
		StartsAt:         startsAt,
		ExpiresAt:        expiresAt,
		DailyUsedKnives:  3.0,
		DailyQuotaKnives: ptrFloat64(10.0),
		DailyWindowStart: ptrTime(dailyStart),
	}

	progress := svc.calculateProgress(sub)

	require.NotNil(t, progress.Daily)
	assert.Equal(t, expiresAt, progress.Daily.ResetsAt)
}

func TestCalculateProgress_WeeklyUsage(t *testing.T) {
	svc := newTestSubscriptionService()
	now := time.Now()
	weeklyStart := now.Add(-3 * 24 * time.Hour)

	sub := &UserSubscription{
		ID:                1,
		ExpiresAt:         now.Add(10 * 24 * time.Hour),
		WeeklyUsedKnives:  25.0,
		WeeklyQuotaKnives: ptrFloat64(50.0),
		WeeklyWindowStart: ptrTime(weeklyStart),
	}

	progress := svc.calculateProgress(sub)

	require.NotNil(t, progress.Weekly)
	assert.Equal(t, 50.0, progress.Weekly.LimitUSD)
	assert.Equal(t, 25.0, progress.Weekly.UsedUSD)
	assert.Equal(t, 25.0, progress.Weekly.RemainingUSD)
	assert.Equal(t, 50.0, progress.Weekly.Percentage)
}

func TestCalculateProgress_MonthlyUsage(t *testing.T) {
	svc := newTestSubscriptionService()
	now := time.Now()
	monthlyStart := now.Add(-15 * 24 * time.Hour)

	sub := &UserSubscription{
		ID:                 1,
		ExpiresAt:          now.Add(10 * 24 * time.Hour),
		MonthlyUsedKnives:  80.0,
		MonthlyQuotaKnives: ptrFloat64(100.0),
		MonthlyWindowStart: ptrTime(monthlyStart),
	}

	progress := svc.calculateProgress(sub)

	require.NotNil(t, progress.Monthly)
	assert.Equal(t, 100.0, progress.Monthly.LimitUSD)
	assert.Equal(t, 80.0, progress.Monthly.UsedUSD)
	assert.Equal(t, 20.0, progress.Monthly.RemainingUSD)
	assert.Equal(t, 80.0, progress.Monthly.Percentage)
}

func TestCalculateProgress_OverLimit_ClampedTo100Percent(t *testing.T) {
	svc := newTestSubscriptionService()
	now := time.Now()

	sub := &UserSubscription{
		ID:               1,
		ExpiresAt:        now.Add(10 * 24 * time.Hour),
		DailyUsedKnives:  15.0,
		DailyQuotaKnives: ptrFloat64(10.0),
		DailyWindowStart: ptrTime(now.Add(-1 * time.Hour)),
	}

	progress := svc.calculateProgress(sub)

	require.NotNil(t, progress.Daily)
	assert.Equal(t, 100.0, progress.Daily.Percentage)
	assert.Equal(t, 0.0, progress.Daily.RemainingUSD)
}

func TestCalculateProgress_NoWindowStart_NoProgress(t *testing.T) {
	svc := newTestSubscriptionService()
	now := time.Now()

	sub := &UserSubscription{
		ID:                1,
		ExpiresAt:         now.Add(10 * 24 * time.Hour),
		DailyUsedKnives:   0,
		WeeklyUsedKnives:  0,
		DailyQuotaKnives:  ptrFloat64(10.0),
		WeeklyQuotaKnives: ptrFloat64(50.0),
	}

	progress := svc.calculateProgress(sub)

	assert.Nil(t, progress.Daily)
	assert.Nil(t, progress.Weekly)
}

func TestCalculateProgress_AllLimits(t *testing.T) {
	svc := newTestSubscriptionService()
	now := time.Now()

	sub := &UserSubscription{
		ID:                 1,
		ExpiresAt:          now.Add(10 * 24 * time.Hour),
		DailyUsedKnives:    5.0,
		WeeklyUsedKnives:   20.0,
		MonthlyUsedKnives:  60.0,
		DailyQuotaKnives:   ptrFloat64(10.0),
		WeeklyQuotaKnives:  ptrFloat64(50.0),
		MonthlyQuotaKnives: ptrFloat64(100.0),
		DailyWindowStart:   ptrTime(now.Add(-6 * time.Hour)),
		WeeklyWindowStart:  ptrTime(now.Add(-3 * 24 * time.Hour)),
		MonthlyWindowStart: ptrTime(now.Add(-15 * 24 * time.Hour)),
	}

	progress := svc.calculateProgress(sub)

	require.NotNil(t, progress.Daily)
	require.NotNil(t, progress.Weekly)
	require.NotNil(t, progress.Monthly)
	assert.Equal(t, 50.0, progress.Daily.Percentage)
	assert.Equal(t, 40.0, progress.Weekly.Percentage)
	assert.Equal(t, 60.0, progress.Monthly.Percentage)
}

func TestCalculateProgress_ExpiredSubscription(t *testing.T) {
	svc := newTestSubscriptionService()

	sub := &UserSubscription{
		ID:        1,
		ExpiresAt: time.Now().Add(-24 * time.Hour),
	}

	progress := svc.calculateProgress(sub)

	assert.Equal(t, 0, progress.ExpiresInDays)
}

func TestCalculateProgress_ResetsInSeconds_NotNegative(t *testing.T) {
	svc := newTestSubscriptionService()
	pastStart := time.Now().Add(-48 * time.Hour)

	sub := &UserSubscription{
		ID:               1,
		ExpiresAt:        time.Now().Add(10 * 24 * time.Hour),
		DailyUsedKnives:  1.0,
		DailyQuotaKnives: ptrFloat64(10.0),
		DailyWindowStart: ptrTime(pastStart),
	}

	progress := svc.calculateProgress(sub)

	require.NotNil(t, progress.Daily)
	assert.GreaterOrEqual(t, progress.Daily.ResetsInSeconds, int64(0))
}
