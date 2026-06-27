package service

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrUpgradeResidualPriceInvalid = infraerrors.BadRequest("UPGRADE_RESIDUAL_PRICE_INVALID", "plan prices must be positive for upgrade residual calculation")
	ErrUpgradeResidualNoQuota      = infraerrors.BadRequest("UPGRADE_RESIDUAL_NO_QUOTA", "at least one quota window is required for upgrade residual calculation")
)

type UpgradeResidualInput struct {
	Now                time.Time
	StartsAt           time.Time
	ExpiresAt          time.Time
	PlanPrice          float64
	TargetPlanPrice    float64
	DailyQuotaKnives   *float64
	WeeklyQuotaKnives  *float64
	MonthlyQuotaKnives *float64
	DailyUsedKnives    float64
	WeeklyUsedKnives   float64
	MonthlyUsedKnives  float64
	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	MonthlyWindowStart *time.Time
}

type UpgradeResidualBreakdown struct {
	TheoreticalFullMaxKnives float64  `json:"theoretical_full_max_knives"`
	ResidualQuotaKnives      float64  `json:"residual_quota_knives"`
	UnitCost                 float64  `json:"unit_cost"`
	ResidualValue            float64  `json:"residual_value"`
	UpgradeDelta             float64  `json:"upgrade_delta"`
	DailyFamilyMax           *float64 `json:"daily_family_max,omitempty"`
	WeeklyFamilyMax          *float64 `json:"weekly_family_max,omitempty"`
	MonthlyFamilyMax         *float64 `json:"monthly_family_max,omitempty"`
}

func roundUpgradeAmountForCurrency(value float64, currency string) float64 {
	if value <= 0 {
		return 0
	}
	return decimal.NewFromFloat(value).
		Round(int32(payment.CurrencyMaxFractionDigits(currency))).
		InexactFloat64()
}

func roundUpgradeBreakdownForCurrency(breakdown *UpgradeResidualBreakdown, currency string) *UpgradeResidualBreakdown {
	if breakdown == nil {
		return nil
	}
	targetPlanPrice := breakdown.ResidualValue + breakdown.UpgradeDelta
	rounded := *breakdown
	rounded.UpgradeDelta = roundUpgradeAmountForCurrency(rounded.UpgradeDelta, currency)
	rounded.ResidualValue = roundUpgradeAmountForCurrency(targetPlanPrice-rounded.UpgradeDelta, currency)
	return &rounded
}

func CalculateUpgradeResidual(input UpgradeResidualInput) (*UpgradeResidualBreakdown, error) {
	if input.PlanPrice <= 0 || input.TargetPlanPrice <= 0 {
		return nil, ErrUpgradeResidualPriceInvalid
	}

	fullDaily := calculateQuotaFamilyCapacity(input.DailyQuotaKnives, 0, nil, input.StartsAt, input.StartsAt, input.ExpiresAt, 24*time.Hour, true)
	fullWeekly := calculateQuotaFamilyCapacity(input.WeeklyQuotaKnives, 0, nil, input.StartsAt, input.StartsAt, input.ExpiresAt, 7*24*time.Hour, false)
	fullMonthly := calculateQuotaFamilyCapacity(input.MonthlyQuotaKnives, 0, nil, input.StartsAt, input.StartsAt, input.ExpiresAt, 30*24*time.Hour, false)
	theoreticalFullMax, err := minQuotaFamilies(fullDaily, fullWeekly, fullMonthly)
	if err != nil {
		return nil, err
	}

	residualDaily := calculateQuotaFamilyCapacity(input.DailyQuotaKnives, input.DailyUsedKnives, input.DailyWindowStart, input.Now, input.StartsAt, input.ExpiresAt, 24*time.Hour, true)
	residualWeekly := calculateQuotaFamilyCapacity(input.WeeklyQuotaKnives, input.WeeklyUsedKnives, input.WeeklyWindowStart, input.Now, input.StartsAt, input.ExpiresAt, 7*24*time.Hour, false)
	residualMonthly := calculateQuotaFamilyCapacity(input.MonthlyQuotaKnives, input.MonthlyUsedKnives, input.MonthlyWindowStart, input.Now, input.StartsAt, input.ExpiresAt, 30*24*time.Hour, false)
	residualQuota, err := minQuotaFamilies(residualDaily, residualWeekly, residualMonthly)
	if err != nil {
		return nil, err
	}

	unitCost := input.PlanPrice / theoreticalFullMax
	residualValue := unitCost * residualQuota
	upgradeDelta := input.TargetPlanPrice - residualValue
	if upgradeDelta < 0 {
		upgradeDelta = 0
	}

	return &UpgradeResidualBreakdown{
		TheoreticalFullMaxKnives: theoreticalFullMax,
		ResidualQuotaKnives:      residualQuota,
		UnitCost:                 unitCost,
		ResidualValue:            residualValue,
		UpgradeDelta:             upgradeDelta,
		DailyFamilyMax:           fullDaily,
		WeeklyFamilyMax:          fullWeekly,
		MonthlyFamilyMax:         fullMonthly,
	}, nil
}

func calculateQuotaFamilyCapacity(limit *float64, used float64, windowStart *time.Time, now, startsAt, expiresAt time.Time, duration time.Duration, allowOneTimeDaily bool) *float64 {
	if limit == nil {
		return nil
	}
	if !expiresAt.After(now) {
		zero := 0.0
		return &zero
	}
	if allowOneTimeDaily && expiresAt.Before(startsAt.Add(24*time.Hour)) || allowOneTimeDaily && expiresAt.Equal(startsAt.Add(24*time.Hour)) {
		remaining := *limit - used
		if remaining < 0 {
			remaining = 0
		}
		return &remaining
	}

	start, currentUsed := normalizeQuotaWindow(now, windowStart, used, duration)

	remaining := *limit - currentUsed
	if remaining < 0 {
		remaining = 0
	}
	total := remaining
	for resetAt := start.Add(duration); resetAt.Before(expiresAt); resetAt = resetAt.Add(duration) {
		total += *limit
	}
	return &total
}

func normalizeQuotaWindow(now time.Time, windowStart *time.Time, used float64, duration time.Duration) (time.Time, float64) {
	start := startOfDay(now)
	currentUsed := 0.0
	if windowStart != nil {
		start = *windowStart
		currentUsed = used
	}

	for !now.Before(start.Add(duration)) {
		start = start.Add(duration)
		currentUsed = 0
	}
	return start, currentUsed
}

func minQuotaFamilies(values ...*float64) (float64, error) {
	var (
		min   float64
		found bool
	)
	for _, value := range values {
		if value == nil {
			continue
		}
		if !found || *value < min {
			min = *value
			found = true
		}
	}
	if !found {
		return 0, ErrUpgradeResidualNoQuota
	}
	return min, nil
}
