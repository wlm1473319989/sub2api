package service_test

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBulkAdjustSubscriptionAggregatesSuccessAndFailure(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	group := h.createGroup(t, "bulk-adjust-group")
	groupID := group.ID
	plan := h.createPlan(t, "Bulk Adjust", 29.9, 30, "day", &groupID, nil, nil, nil)

	userActive := h.createUser(t, "bulk-adjust-active@example.com")
	userExpired := h.createUser(t, "bulk-adjust-expired@example.com")

	activeSub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: userActive.ID,
		Plan:   plan,
		Notes:  "seed active",
	})
	require.NoError(t, err)

	expiredSub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: userExpired.ID,
		Plan:   plan,
		Notes:  "seed expired",
	})
	require.NoError(t, err)

	_, err = h.client.UserSubscription.UpdateOneID(expiredSub.ID).
		SetExpiresAt(time.Now().Add(-24 * time.Hour)).
		SetStatus(service.SubscriptionStatusExpired).
		Save(h.ctx)
	require.NoError(t, err)

	result, err := h.svc.BulkAdjustSubscription(h.ctx, &service.BulkAdjustSubscriptionInput{
		SubscriptionIDs: []int64{activeSub.ID, expiredSub.ID},
		Days:            -3,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 1, result.FailedCount)
	require.Equal(t, "adjusted", result.Statuses[activeSub.ID])
	require.Equal(t, "failed", result.Statuses[expiredSub.ID])
	require.Len(t, result.Subscriptions, 1)
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0], "cannot shorten an expired subscription")

	currentActive, err := h.svc.GetByID(h.ctx, activeSub.ID)
	require.NoError(t, err)
	require.True(t, currentActive.ExpiresAt.Before(activeSub.ExpiresAt))

	currentExpired, err := h.svc.GetByID(h.ctx, expiredSub.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusExpired, currentExpired.Status)
}

func TestBulkAdjustSubscriptionCanReactivateExpiredSubscriptions(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	group := h.createGroup(t, "bulk-adjust-reactivate-group")
	groupID := group.ID
	plan := h.createPlan(t, "Bulk Reactivate", 29.9, 30, "day", &groupID, nil, nil, nil)

	user := h.createUser(t, "bulk-adjust-reactivate@example.com")
	expiredSub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   plan,
		Notes:  "seed expired",
	})
	require.NoError(t, err)

	_, err = h.client.UserSubscription.UpdateOneID(expiredSub.ID).
		SetExpiresAt(time.Now().Add(-48 * time.Hour)).
		SetStatus(service.SubscriptionStatusExpired).
		Save(h.ctx)
	require.NoError(t, err)

	result, err := h.svc.BulkAdjustSubscription(h.ctx, &service.BulkAdjustSubscriptionInput{
		SubscriptionIDs: []int64{expiredSub.ID},
		Days:            7,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 0, result.FailedCount)
	require.Len(t, result.Subscriptions, 1)
	require.Equal(t, "adjusted", result.Statuses[expiredSub.ID])

	reactivated, err := h.svc.GetByID(h.ctx, expiredSub.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusActive, reactivated.Status)
	require.True(t, reactivated.ExpiresAt.After(time.Now()))
}
