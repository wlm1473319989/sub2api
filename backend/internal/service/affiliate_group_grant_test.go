package service

import (
	"testing"
	"time"
)

func TestAffiliateGroupGrantDays(t *testing.T) {
	tests := []struct {
		name      string
		payAmount float64
		want      int
	}{
		{name: "zero", payAmount: 0, want: 0},
		{name: "under ten", payAmount: 0.01, want: 1},
		{name: "ten", payAmount: 10, want: 1},
		{name: "under twenty", payAmount: 19.99, want: 1},
		{name: "twenty", payAmount: 20, want: 2},
		{name: "hundred", payAmount: 100, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AffiliateGroupGrantDays(tt.payAmount); got != tt.want {
				t.Fatalf("AffiliateGroupGrantDays(%v) = %d, want %d", tt.payAmount, got, tt.want)
			}
		})
	}
}

func TestEffectiveAllowedGroupsFromSnapshotFiltersExpiredTimedGrants(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	got := effectiveAllowedGroupsFromSnapshot(
		[]int64{1, 2},
		[]APIKeyAuthTimedGroupGrantSnapshot{
			{GroupID: 2, ExpiresAt: now.Add(time.Hour)},
			{GroupID: 3, ExpiresAt: now.Add(time.Hour)},
			{GroupID: 4, ExpiresAt: now.Add(-time.Hour)},
		},
		now,
	)

	want := []int64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNextAffiliateGroupGrantExpiryRun(t *testing.T) {
	loc := time.Local
	before := time.Date(2026, 7, 7, 0, 4, 0, 0, loc)
	wantToday := time.Date(2026, 7, 7, 0, 5, 0, 0, loc)
	if got := NextAffiliateGroupGrantExpiryRun(before); !got.Equal(wantToday) {
		t.Fatalf("before 00:05 got %s, want %s", got, wantToday)
	}

	atRun := time.Date(2026, 7, 7, 0, 5, 0, 0, loc)
	wantTomorrow := time.Date(2026, 7, 8, 0, 5, 0, 0, loc)
	if got := NextAffiliateGroupGrantExpiryRun(atRun); !got.Equal(wantTomorrow) {
		t.Fatalf("at 00:05 got %s, want %s", got, wantTomorrow)
	}
}
