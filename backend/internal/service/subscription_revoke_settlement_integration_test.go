package service_test

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRevokeSubscriptionMarksRevokedAndCreatesSettlementOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)

	operator := h.createUser(t, "revoke-settlement-operator@test.com")
	user := h.createUser(t, "revoke-settlement-user@test.com")
	group := h.createGroup(t, "revoke-settlement-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Revoke Settlement Starter", 120, 30, "day", &groupID, nil, nil, &monthly)

	sub, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID:     user.ID,
		PlanID:     plan.ID,
		AssignedBy: operator.ID,
		Notes:      "admin assign before revoke",
	})
	require.NoError(t, err)
	require.False(t, reused)

	revoked, err := h.svc.RevokeSubscription(h.ctx, &service.RevokeSubscriptionInput{
		SubscriptionID: sub.ID,
		OperatorUserID: operator.ID,
		Notes:          "admin revoke settlement",
	})
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusRevoked, revoked.Status)
	require.Contains(t, revoked.Notes, "admin revoke settlement")

	reloaded, err := h.svc.GetByID(h.ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusRevoked, reloaded.Status)

	_, err = h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.Error(t, err)
	require.Equal(t, infraerrors.Reason(service.ErrSubscriptionNotFound), infraerrors.Reason(err))

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 2)

	require.Equal(t, domain.SettlementStatusClosed, settlements[0].Status)
	require.Equal(t, domain.SettlementActionPurchase, settlements[0].ActionType)
	require.Equal(t, domain.SettlementStatusEffective, settlements[1].Status)
	require.Equal(t, domain.SettlementActionRevoke, settlements[1].ActionType)
	require.Equal(t, domain.SettlementActionSourceAdminRevoke, settlements[1].ActionSource)
	require.Equal(t, domain.SettlementTriggerRefDirectAction, settlements[1].TriggerRefType)
	require.Equal(t, operator.ID, settlements[1].OperatorUserID)
	require.NotNil(t, settlements[1].PrevSettlementID)
	require.Equal(t, settlements[0].ID, *settlements[1].PrevSettlementID)
	require.NotNil(t, settlements[1].AfterUserSubscriptionID)
	require.Equal(t, sub.ID, *settlements[1].AfterUserSubscriptionID)
	require.Equal(t, domain.SubscriptionStatusRevoked, settlements[1].AfterSubscriptionStatus)
	require.InDelta(t, 0, settlements[1].AfterSettlementValue, 1e-9)
	require.Greater(t, settlements[1].WriteoffValue, 0.0)
	require.Nil(t, settlements[1].RefundResidualValue)

	history, err := h.svc.ListUserSettlementHistory(h.ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, domain.SettlementActionRevoke, history[1].ActionType)
	require.Equal(t, domain.SubscriptionStatusRevoked, history[1].AfterSubscriptionStatus)
}
