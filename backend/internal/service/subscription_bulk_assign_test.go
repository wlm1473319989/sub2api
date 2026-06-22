package service_test

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBulkAssignSubscriptionAggregatesCreatedReusedAndFailed(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	group := h.createGroup(t, "bulk-assign-group")
	groupID := group.ID
	operator := h.createUser(t, "bulk-assign-operator@example.com")

	basePlan := h.createPlan(t, "Bulk Base", 29.9, 30, "day", &groupID, nil, nil, nil)
	upgradePlan := h.createPlan(t, "Bulk Pro", 59.9, 30, "day", &groupID, nil, nil, nil)

	userRenew := h.createUser(t, "bulk-renew@example.com")
	userCreate := h.createUser(t, "bulk-create@example.com")
	userFail := h.createUser(t, "bulk-fail@example.com")

	seedRenew, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: userRenew.ID,
		Plan:   basePlan,
		Notes:  "seed renew",
	})
	require.NoError(t, err)

	_, err = h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: userFail.ID,
		Plan:   upgradePlan,
		Notes:  "seed fail",
	})
	require.NoError(t, err)

	result, err := h.svc.BulkAssignSubscription(h.ctx, &service.BulkAssignSubscriptionInput{
		UserIDs:      []int64{userRenew.ID, userCreate.ID, userFail.ID},
		PlanID:       basePlan.ID,
		AssignedBy:   operator.ID,
		ValidityDays: 30,
		Notes:        "bulk assign",
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.SuccessCount)
	require.Equal(t, 1, result.CreatedCount)
	require.Equal(t, 1, result.ReusedCount)
	require.Equal(t, 1, result.FailedCount)
	require.Equal(t, "reused", result.Statuses[userRenew.ID])
	require.Equal(t, "created", result.Statuses[userCreate.ID])
	require.Equal(t, "failed", result.Statuses[userFail.ID])
	require.Len(t, result.Subscriptions, 2)
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0], "user ")

	var renewedFound, createdFound bool
	for i := range result.Subscriptions {
		sub := result.Subscriptions[i]
		switch sub.UserID {
		case userRenew.ID:
			renewedFound = true
			require.Equal(t, seedRenew.ID, sub.ID)
		case userCreate.ID:
			createdFound = true
			require.NotZero(t, sub.ID)
			require.NotNil(t, sub.PlanID)
			require.Equal(t, basePlan.ID, *sub.PlanID)
		}
	}
	require.True(t, renewedFound)
	require.True(t, createdFound)

	createdActive, err := h.svc.GetActiveSubscriptionByUser(h.ctx, userCreate.ID)
	require.NoError(t, err)
	require.NotNil(t, createdActive.PlanID)
	require.Equal(t, basePlan.ID, *createdActive.PlanID)

	failedActive, err := h.svc.GetActiveSubscriptionByUser(h.ctx, userFail.ID)
	require.NoError(t, err)
	require.NotNil(t, failedActive.PlanID)
	require.Equal(t, upgradePlan.ID, *failedActive.PlanID)
}

func TestBulkAssignSubscriptionRequiresPlanID(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "bulk-missing-plan@example.com")

	result, err := h.svc.BulkAssignSubscription(h.ctx, &service.BulkAssignSubscriptionInput{
		UserIDs: []int64{user.ID},
		Notes:   "missing plan",
	})
	require.NoError(t, err)
	require.Equal(t, 0, result.SuccessCount)
	require.Equal(t, 1, result.FailedCount)
	require.Equal(t, "failed", result.Statuses[user.ID])
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0], "PLAN_ID_REQUIRED")
}

func TestBulkAssignSubscriptionCanUpgradeThroughAssignFlow(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	group := h.createGroup(t, "bulk-upgrade-group")
	groupID := group.ID
	operator := h.createUser(t, "bulk-upgrade-operator@example.com")

	basePlan := h.createPlan(t, "Bulk Upgrade Base", 19.9, 30, "day", &groupID, nil, nil, nil)
	targetPlan := h.createPlan(t, "Bulk Upgrade Pro", 49.9, 30, "day", &groupID, nil, nil, nil)
	user := h.createUser(t, "bulk-upgrade@example.com")

	previous, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   basePlan,
		Notes:  "seed",
	})
	require.NoError(t, err)

	result, err := h.svc.BulkAssignSubscription(h.ctx, &service.BulkAssignSubscriptionInput{
		UserIDs:    []int64{user.ID},
		PlanID:     targetPlan.ID,
		AssignedBy: operator.ID,
		Notes:      "upgrade assign",
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.SuccessCount)
	require.Equal(t, 1, result.CreatedCount)
	require.Equal(t, 0, result.ReusedCount)
	require.Equal(t, "created", result.Statuses[user.ID])

	current, err := h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, current.PlanID)
	require.Equal(t, targetPlan.ID, *current.PlanID)
	require.NotEqual(t, previous.ID, current.ID)

	superseded, err := h.svc.GetByID(h.ctx, previous.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusSuperseded, superseded.Status)
}
