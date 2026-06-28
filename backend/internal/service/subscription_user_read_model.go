package service

import "context"

// GetActiveSubscriptionByUser returns the user's unique active subscription.
func (s *SubscriptionService) GetActiveSubscriptionByUser(ctx context.Context, userID int64) (*UserSubscription, error) {
	sub, err := s.userSubRepo.GetActiveByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	normalizeSubscriptionReadModel(sub)
	return sub, nil
}

// HasActiveSubscription reports whether the user currently has a unique active subscription.
func (s *SubscriptionService) HasActiveSubscription(ctx context.Context, userID int64) (bool, error) {
	return s.userSubRepo.HasActiveByUserID(ctx, userID)
}

func normalizeSubscriptionReadModel(sub *UserSubscription) {
	if sub == nil {
		return
	}
	if sub.NeedsDailyReset() {
		sub.DailyWindowStart = nil
		sub.DailyUsageUSD = 0
		sub.DailyUsedKnives = 0
	}
	if sub.NeedsWeeklyReset() {
		sub.WeeklyWindowStart = nil
		sub.WeeklyUsageUSD = 0
		sub.WeeklyUsedKnives = 0
	}
	if sub.NeedsMonthlyReset() {
		sub.MonthlyWindowStart = nil
		sub.MonthlyUsageUSD = 0
		sub.MonthlyUsedKnives = 0
	}
	if sub.Status == SubscriptionStatusActive && sub.IsExpired() {
		sub.Status = SubscriptionStatusExpired
	}
}
