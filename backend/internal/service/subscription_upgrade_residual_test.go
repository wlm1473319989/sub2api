package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func floatPtr(v float64) *float64 { return &v }

func TestCalculateUpgradeResidual_MonthlyExhaustedHasNoResidual(t *testing.T) {
	now := time.Date(2026, 1, 20, 12, 0, 0, 0, time.UTC)
	startsAt := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 1, 30, 9, 0, 0, 0, time.UTC)
	monthlyStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	result, err := CalculateUpgradeResidual(UpgradeResidualInput{
		Now:                now,
		StartsAt:           startsAt,
		ExpiresAt:          expiresAt,
		PlanPrice:          100,
		TargetPlanPrice:    180,
		MonthlyQuotaKnives: floatPtr(100),
		MonthlyUsedKnives:  100,
		MonthlyWindowStart: &monthlyStart,
	})
	require.NoError(t, err)
	require.InDelta(t, 100, result.TheoreticalFullMaxKnives, 1e-9)
	require.InDelta(t, 0, result.ResidualQuotaKnives, 1e-9)
	require.InDelta(t, 1, result.UnitCost, 1e-9)
	require.InDelta(t, 0, result.ResidualValue, 1e-9)
	require.InDelta(t, 180, result.UpgradeDelta, 1e-9)
}

func TestCalculateUpgradeResidual_WeeklyExhaustedStillHasFutureWeeklyResidual(t *testing.T) {
	now := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	startsAt := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 1, 16, 9, 0, 0, 0, time.UTC)
	weeklyStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	result, err := CalculateUpgradeResidual(UpgradeResidualInput{
		Now:               now,
		StartsAt:          startsAt,
		ExpiresAt:         expiresAt,
		PlanPrice:         70,
		TargetPlanPrice:   140,
		WeeklyQuotaKnives: floatPtr(70),
		WeeklyUsedKnives:  70,
		WeeklyWindowStart: &weeklyStart,
	})
	require.NoError(t, err)
	require.InDelta(t, 210, result.TheoreticalFullMaxKnives, 1e-9)
	require.InDelta(t, 140, result.ResidualQuotaKnives, 1e-9)
	require.InDelta(t, 70.0/210.0, result.UnitCost, 1e-9)
	require.InDelta(t, 46.6666667, result.ResidualValue, 1e-6)
	require.InDelta(t, 93.3333333, result.UpgradeDelta, 1e-6)
}

func TestCalculateUpgradeResidual_DailyExhaustedButWeekAndMonthStillCapTotal(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	startsAt := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 1, 4, 9, 0, 0, 0, time.UTC)
	dailyStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	weeklyStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	monthlyStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	result, err := CalculateUpgradeResidual(UpgradeResidualInput{
		Now:                now,
		StartsAt:           startsAt,
		ExpiresAt:          expiresAt,
		PlanPrice:          18,
		TargetPlanPrice:    30,
		DailyQuotaKnives:   floatPtr(10),
		WeeklyQuotaKnives:  floatPtr(15),
		MonthlyQuotaKnives: floatPtr(18),
		DailyUsedKnives:    10,
		WeeklyUsedKnives:   10,
		MonthlyUsedKnives:  10,
		DailyWindowStart:   &dailyStart,
		WeeklyWindowStart:  &weeklyStart,
		MonthlyWindowStart: &monthlyStart,
	})
	require.NoError(t, err)
	require.InDelta(t, 15, result.TheoreticalFullMaxKnives, 1e-9)
	require.InDelta(t, 5, result.ResidualQuotaKnives, 1e-9)
	require.InDelta(t, 1.2, result.UnitCost, 1e-9)
	require.InDelta(t, 6, result.ResidualValue, 1e-9)
	require.InDelta(t, 24, result.UpgradeDelta, 1e-9)
}

func TestCalculateUpgradeResidual_OneDayPlanDailyQuotaIsSingleUse(t *testing.T) {
	now := time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)
	startsAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	expiresAt := startsAt.Add(24 * time.Hour)
	dailyStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	result, err := CalculateUpgradeResidual(UpgradeResidualInput{
		Now:              now,
		StartsAt:         startsAt,
		ExpiresAt:        expiresAt,
		PlanPrice:        10,
		TargetPlanPrice:  20,
		DailyQuotaKnives: floatPtr(10),
		DailyUsedKnives:  4,
		DailyWindowStart: &dailyStart,
	})
	require.NoError(t, err)
	require.InDelta(t, 10, result.TheoreticalFullMaxKnives, 1e-9)
	require.InDelta(t, 6, result.ResidualQuotaKnives, 1e-9)
}
