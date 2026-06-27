package service_test

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBulkResetQuotaAggregatesSuccessAndFailure(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	group := h.createGroup(t, "bulk-reset-group")
	groupID := group.ID
	daily := 10.0
	weekly := 70.0
	monthly := 300.0
	plan := h.createPlan(t, "Bulk Reset", 29.9, 30, "day", &groupID, &daily, &weekly, &monthly)

	user := h.createUser(t, "bulk-reset@example.com")
	sub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   plan,
		Notes:  "seed subscription",
	})
	require.NoError(t, err)

	oldDailyStart := time.Now().Add(-36 * time.Hour)
	oldWeeklyStart := time.Now().Add(-2 * 24 * time.Hour)
	oldMonthlyStart := time.Now().Add(-10 * 24 * time.Hour)
	_, err = h.client.UserSubscription.UpdateOneID(sub.ID).
		SetDailyWindowStart(oldDailyStart).
		SetWeeklyWindowStart(oldWeeklyStart).
		SetMonthlyWindowStart(oldMonthlyStart).
		SetDailyUsageUsd(3.5).
		SetWeeklyUsageUsd(14.2).
		SetMonthlyUsageUsd(88.8).
		SetDailyUsedKnives(2.25).
		SetWeeklyUsedKnives(9.75).
		SetMonthlyUsedKnives(55.5).
		Save(h.ctx)
	require.NoError(t, err)

	result, err := h.svc.BulkResetQuota(h.ctx, &service.BulkResetSubscriptionQuotaInput{
		SubscriptionIDs: []int64{sub.ID, 999999},
		Daily:           true,
		Weekly:          false,
		Monthly:         true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 1, result.FailedCount)
	require.Equal(t, "reset", result.Statuses[sub.ID])
	require.Equal(t, "failed", result.Statuses[int64(999999)])
	require.Len(t, result.Subscriptions, 1)
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0], "subscription not found")

	refreshed, err := h.svc.GetByID(h.ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, float64(0), refreshed.DailyUsageUSD)
	require.Equal(t, float64(0), refreshed.MonthlyUsageUSD)
	require.InDelta(t, 14.2, refreshed.WeeklyUsageUSD, 0.000001)
	require.Equal(t, float64(0), refreshed.DailyUsedKnives)
	require.Equal(t, float64(0), refreshed.MonthlyUsedKnives)
	require.InDelta(t, 9.75, refreshed.WeeklyUsedKnives, 0.000001)
	require.NotNil(t, refreshed.DailyWindowStart)
	require.NotNil(t, refreshed.MonthlyWindowStart)
	require.NotNil(t, refreshed.WeeklyWindowStart)
	require.True(t, refreshed.DailyWindowStart.After(oldDailyStart))
	require.True(t, refreshed.MonthlyWindowStart.After(oldMonthlyStart))
	require.Equal(t, oldWeeklyStart.Unix(), refreshed.WeeklyWindowStart.Unix())
}

func TestBulkResetQuotaRequiresAtLeastOneWindow(t *testing.T) {
	h := newSubscriptionOpsHarness(t)

	_, err := h.svc.BulkResetQuota(h.ctx, &service.BulkResetSubscriptionQuotaInput{
		SubscriptionIDs: []int64{1},
		Daily:           false,
		Weekly:          false,
		Monthly:         false,
	})

	require.ErrorIs(t, err, service.ErrInvalidInput)
}
