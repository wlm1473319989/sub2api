package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type subscriptionWindowMaintenanceRepoStub struct {
	userSubRepoNoop

	subs            []UserSubscription
	dailyResetIDs   []int64
	weeklyResetIDs  []int64
	monthlyResetIDs []int64
}

func (r *subscriptionWindowMaintenanceRepoStub) List(_ context.Context, params pagination.PaginationParams, _ *int64, status, _, _ string) ([]UserSubscription, *pagination.PaginationResult, error) {
	requireStatus := status == SubscriptionStatusActive
	now := time.Now()
	active := make([]UserSubscription, 0, len(r.subs))
	for i := range r.subs {
		sub := r.subs[i]
		if requireStatus && (sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(now)) {
			continue
		}
		active = append(active, sub)
	}

	limit := params.Limit()
	page := params.Page
	if page < 1 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= len(active) {
		return nil, &pagination.PaginationResult{
			Total:    int64(len(active)),
			Page:     page,
			PageSize: limit,
			Pages:    pagesForTotal(len(active), limit),
		}, nil
	}
	end := start + limit
	if end > len(active) {
		end = len(active)
	}
	out := append([]UserSubscription(nil), active[start:end]...)
	return out, &pagination.PaginationResult{
		Total:    int64(len(active)),
		Page:     page,
		PageSize: limit,
		Pages:    pagesForTotal(len(active), limit),
	}, nil
}

func (r *subscriptionWindowMaintenanceRepoStub) ResetDailyUsage(_ context.Context, id int64, newWindowStart time.Time) error {
	sub := r.find(id)
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	r.dailyResetIDs = append(r.dailyResetIDs, id)
	sub.DailyUsageUSD = 0
	sub.DailyUsedKnives = 0
	sub.DailyWindowStart = &newWindowStart
	return nil
}

func (r *subscriptionWindowMaintenanceRepoStub) ResetWeeklyUsage(_ context.Context, id int64, newWindowStart time.Time) error {
	sub := r.find(id)
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	r.weeklyResetIDs = append(r.weeklyResetIDs, id)
	sub.WeeklyUsageUSD = 0
	sub.WeeklyUsedKnives = 0
	sub.WeeklyWindowStart = &newWindowStart
	return nil
}

func (r *subscriptionWindowMaintenanceRepoStub) ResetMonthlyUsage(_ context.Context, id int64, newWindowStart time.Time) error {
	sub := r.find(id)
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	r.monthlyResetIDs = append(r.monthlyResetIDs, id)
	sub.MonthlyUsageUSD = 0
	sub.MonthlyUsedKnives = 0
	sub.MonthlyWindowStart = &newWindowStart
	return nil
}

func (r *subscriptionWindowMaintenanceRepoStub) find(id int64) *UserSubscription {
	for i := range r.subs {
		if r.subs[i].ID == id {
			return &r.subs[i]
		}
	}
	return nil
}

func pagesForTotal(total, limit int) int {
	if total == 0 {
		return 0
	}
	return (total + limit - 1) / limit
}

func TestNextSubscriptionWindowMaintenanceRun(t *testing.T) {
	loc := time.FixedZone("test", 8*60*60)

	before2AM := time.Date(2026, 6, 29, 1, 30, 0, 0, loc)
	require.Equal(t, time.Date(2026, 6, 29, 2, 0, 0, 0, loc), nextSubscriptionWindowMaintenanceRun(before2AM))

	at2AM := time.Date(2026, 6, 29, 2, 0, 0, 0, loc)
	require.Equal(t, at2AM, nextSubscriptionWindowMaintenanceRun(at2AM))

	after2AM := time.Date(2026, 6, 29, 2, 1, 0, 0, loc)
	require.Equal(t, time.Date(2026, 6, 30, 2, 0, 0, 0, loc), nextSubscriptionWindowMaintenanceRun(after2AM))
}

func TestSubscriptionWindowMaintenance_RunOnce_ResetsOnlyExpiredWindows(t *testing.T) {
	now := time.Now()
	expectedWindowStart := startOfDay(now)
	startsAt := now.Add(-10 * 24 * time.Hour)
	expiresAt := now.Add(10 * 24 * time.Hour)

	dailyExpired := now.Add(-25 * time.Hour)
	weeklyFresh := now.Add(-6 * 24 * time.Hour)
	weeklyExpired := now.Add(-8 * 24 * time.Hour)
	monthlyFresh := now.Add(-29 * 24 * time.Hour)
	monthlyExpired := now.Add(-31 * 24 * time.Hour)

	repo := &subscriptionWindowMaintenanceRepoStub{subs: []UserSubscription{
		{
			ID:                 1,
			UserID:             10,
			Status:             SubscriptionStatusActive,
			StartsAt:           startsAt,
			ExpiresAt:          expiresAt,
			DailyWindowStart:   &dailyExpired,
			WeeklyWindowStart:  &weeklyFresh,
			MonthlyWindowStart: &monthlyFresh,
			DailyUsageUSD:      10,
			WeeklyUsageUSD:     20,
			MonthlyUsageUSD:    30,
			DailyUsedKnives:    10,
			WeeklyUsedKnives:   20,
			MonthlyUsedKnives:  30,
		},
		{
			ID:                2,
			UserID:            20,
			Status:            SubscriptionStatusActive,
			StartsAt:          startsAt,
			ExpiresAt:         expiresAt,
			WeeklyWindowStart: &weeklyExpired,
			WeeklyUsageUSD:    40,
			WeeklyUsedKnives:  40,
		},
		{
			ID:                 3,
			UserID:             30,
			Status:             SubscriptionStatusActive,
			StartsAt:           startsAt,
			ExpiresAt:          expiresAt,
			MonthlyWindowStart: &monthlyExpired,
			MonthlyUsageUSD:    50,
			MonthlyUsedKnives:  50,
		},
	}}
	subscriptionSvc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	maintenanceSvc := NewSubscriptionWindowMaintenanceService(repo, subscriptionSvc)

	maintenanceSvc.runOnce()

	sub1 := repo.find(1)
	require.NotNil(t, sub1)
	require.Equal(t, []int64{1}, repo.dailyResetIDs)
	require.Zero(t, sub1.DailyUsageUSD)
	require.Zero(t, sub1.DailyUsedKnives)
	require.WithinDuration(t, expectedWindowStart, *sub1.DailyWindowStart, time.Second)
	require.Equal(t, 20.0, sub1.WeeklyUsageUSD, "fresh weekly window must not be reset")
	require.Equal(t, 20.0, sub1.WeeklyUsedKnives)
	require.Equal(t, weeklyFresh, *sub1.WeeklyWindowStart)
	require.Equal(t, 30.0, sub1.MonthlyUsageUSD, "fresh monthly window must not be reset")
	require.Equal(t, 30.0, sub1.MonthlyUsedKnives)
	require.Equal(t, monthlyFresh, *sub1.MonthlyWindowStart)

	sub2 := repo.find(2)
	require.NotNil(t, sub2)
	require.Equal(t, []int64{2}, repo.weeklyResetIDs)
	require.Zero(t, sub2.WeeklyUsageUSD)
	require.Zero(t, sub2.WeeklyUsedKnives)
	require.WithinDuration(t, expectedWindowStart, *sub2.WeeklyWindowStart, time.Second)

	sub3 := repo.find(3)
	require.NotNil(t, sub3)
	require.Equal(t, []int64{3}, repo.monthlyResetIDs)
	require.Zero(t, sub3.MonthlyUsageUSD)
	require.Zero(t, sub3.MonthlyUsedKnives)
	require.WithinDuration(t, expectedWindowStart, *sub3.MonthlyWindowStart, time.Second)
}

func TestSubscriptionWindowMaintenance_RunOnce_DailyCardKeepsOneTimeQuota(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-23 * time.Hour)
	expiresAt := startsAt.Add(24 * time.Hour)
	dailyExpired := now.Add(-25 * time.Hour)

	repo := &subscriptionWindowMaintenanceRepoStub{subs: []UserSubscription{
		{
			ID:               1,
			UserID:           10,
			Status:           SubscriptionStatusActive,
			StartsAt:         startsAt,
			ExpiresAt:        expiresAt,
			DailyWindowStart: &dailyExpired,
			DailyUsageUSD:    10,
			DailyUsedKnives:  10,
		},
	}}
	subscriptionSvc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	maintenanceSvc := NewSubscriptionWindowMaintenanceService(repo, subscriptionSvc)

	maintenanceSvc.runOnce()

	sub := repo.find(1)
	require.NotNil(t, sub)
	require.Empty(t, repo.dailyResetIDs)
	require.Equal(t, 10.0, sub.DailyUsageUSD)
	require.Equal(t, 10.0, sub.DailyUsedKnives)
	require.Equal(t, dailyExpired, *sub.DailyWindowStart)
}

func TestValidateAndCheckLimits_LazyResetFallbackBeforeMaintenance(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-48 * time.Hour)
	dailyExpired := now.Add(-25 * time.Hour)
	quota := 10.0
	sub := &UserSubscription{
		Status:           SubscriptionStatusActive,
		StartsAt:         startsAt,
		ExpiresAt:        now.Add(24 * time.Hour),
		DailyWindowStart: &dailyExpired,
		DailyQuotaKnives: &quota,
		DailyUsageUSD:    quota,
		DailyUsedKnives:  quota,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, &Group{})

	require.NoError(t, err)
	require.True(t, needsMaintenance)
	require.Zero(t, sub.DailyUsageUSD)
	require.Zero(t, sub.DailyUsedKnives)
}

func TestUserSubscriptionRuntimeLimitsUseUsageUSD(t *testing.T) {
	limit := 10.0
	sub := &UserSubscription{
		DailyQuotaKnives:   &limit,
		WeeklyQuotaKnives:  &limit,
		MonthlyQuotaKnives: &limit,
		DailyUsageUSD:      0,
		WeeklyUsageUSD:     0,
		MonthlyUsageUSD:    0,
		DailyUsedKnives:    limit,
		WeeklyUsedKnives:   limit,
		MonthlyUsedKnives:  limit,
	}

	require.True(t, sub.CheckDailyLimitForNextRequest())
	require.True(t, sub.CheckWeeklyLimitForNextRequest())
	require.True(t, sub.CheckMonthlyLimitForNextRequest())
	require.True(t, sub.CheckDailyLimit(0))
	require.True(t, sub.CheckWeeklyLimit(0))
	require.True(t, sub.CheckMonthlyLimit(0))

	sub.DailyUsageUSD = limit
	sub.WeeklyUsageUSD = limit
	sub.MonthlyUsageUSD = limit
	sub.DailyUsedKnives = 0
	sub.WeeklyUsedKnives = 0
	sub.MonthlyUsedKnives = 0

	require.False(t, sub.CheckDailyLimitForNextRequest())
	require.False(t, sub.CheckWeeklyLimitForNextRequest())
	require.False(t, sub.CheckMonthlyLimitForNextRequest())
	require.False(t, sub.CheckDailyLimit(0.01))
	require.False(t, sub.CheckWeeklyLimit(0.01))
	require.False(t, sub.CheckMonthlyLimit(0.01))
}
