package service_test

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAssignSubscriptionCreatesPurchaseSettlementOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)

	operator := h.createUser(t, "assign-settlement-operator@test.com")
	user := h.createUser(t, "assign-settlement-user@test.com")
	group := h.createGroup(t, "assign-settlement-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Assign Settlement Starter", 100, 30, "day", &groupID, nil, nil, &monthly)

	sub, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID:     user.ID,
		PlanID:     plan.ID,
		AssignedBy: operator.ID,
		Notes:      "admin assign settlement",
	})
	require.NoError(t, err)
	require.False(t, reused)
	require.NotNil(t, sub.PlanID)
	require.Equal(t, plan.ID, *sub.PlanID)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 1)
	settlement := settlements[0]
	require.Equal(t, domain.SettlementActionPurchase, settlement.ActionType)
	require.Equal(t, domain.SettlementActionSourceSubscriptionAssign, settlement.ActionSource)
	require.Equal(t, domain.SettlementTriggerRefAdminAssignment, settlement.TriggerRefType)
	require.Nil(t, settlement.TriggerRefID)
	require.Equal(t, operator.ID, settlement.OperatorUserID)
	require.Equal(t, domain.SettlementStatusEffective, settlement.Status)
	require.Equal(t, sub.ID, *settlement.AfterUserSubscriptionID)
	require.InDelta(t, plan.Price, settlement.AfterSettlementValue, 1e-9)
}

func TestAssignSubscriptionRenewThenUpgradeUsesSettlementHead(t *testing.T) {
	h := newSubscriptionOpsHarness(t)

	operator := h.createUser(t, "assign-chain-operator@test.com")
	user := h.createUser(t, "assign-chain-user@test.com")
	group := h.createGroup(t, "assign-chain-group")
	groupID := group.ID
	monthly := 100.0
	basePlan := h.createPlan(t, "Assign Chain Base", 100, 30, "day", &groupID, nil, nil, &monthly)
	targetPlan := h.createPlan(t, "Assign Chain Pro", 160, 30, "day", &groupID, nil, nil, &monthly)

	seed, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   basePlan,
		Notes:  "seed",
	})
	require.NoError(t, err)

	renewed, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID:     user.ID,
		PlanID:     basePlan.ID,
		AssignedBy: operator.ID,
		Notes:      "admin renew settlement",
	})
	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, seed.ID, renewed.ID)

	upgraded, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID:     user.ID,
		PlanID:     targetPlan.ID,
		AssignedBy: operator.ID,
		Notes:      "admin upgrade settlement",
	})
	require.NoError(t, err)
	require.False(t, reused)
	require.NotEqual(t, seed.ID, upgraded.ID)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 2)
	require.Equal(t, domain.SettlementStatusClosed, settlements[0].Status)
	require.Equal(t, domain.SettlementActionRenew, settlements[0].ActionType)
	require.Equal(t, domain.SettlementActionSourceSubscriptionAssign, settlements[0].ActionSource)
	require.InDelta(t, 200, settlements[0].AfterSettlementValue, 0.01)
	require.Equal(t, domain.SettlementStatusEffective, settlements[1].Status)
	require.Equal(t, domain.SettlementActionUpgrade, settlements[1].ActionType)
	require.Equal(t, domain.SettlementActionSourceSubscriptionAssign, settlements[1].ActionSource)
	require.Equal(t, domain.SettlementTriggerRefAdminAssignment, settlements[1].TriggerRefType)
	require.Nil(t, settlements[1].TriggerRefID)
	require.NotNil(t, settlements[1].PrevSettlementID)
	require.Equal(t, settlements[0].ID, *settlements[1].PrevSettlementID)
	require.InDelta(t, 0, settlements[1].ActionDeltaValue, 0.01)
	require.InDelta(t, targetPlan.Price, settlements[1].AfterSettlementValue, 1e-9)
	require.InDelta(t, 40, settlements[1].WriteoffValue, 0.01)
}
