package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionNaturalDayPeriod(t *testing.T) {
	loc := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 7, 5, 15, 30, 45, 123, loc)

	startsAt, expiresAt := subscriptionNaturalDayPeriod(now, 30)

	require.Equal(t, time.Date(2026, 7, 5, 0, 0, 0, 0, loc), startsAt)
	require.Equal(t, time.Date(2026, 8, 4, 0, 0, 0, 0, loc), expiresAt)
}

func TestSubscriptionRollingWindowStartAnchorsToSubscriptionStart(t *testing.T) {
	loc := time.FixedZone("test", 8*60*60)
	startsAt := time.Date(2026, 7, 5, 15, 30, 0, 0, loc)
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, loc)

	weeklyStart := subscriptionRollingWindowStart(now, startsAt, 7*24*time.Hour)
	monthlyStart := subscriptionRollingWindowStart(now, startsAt, 30*24*time.Hour)

	require.Equal(t, time.Date(2026, 7, 19, 0, 0, 0, 0, loc), weeklyStart)
	require.Equal(t, time.Date(2026, 7, 5, 0, 0, 0, 0, loc), monthlyStart)
}
