package service_test

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRedeemSubscriptionCreatesPurchaseSettlementOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	redeemSvc := newRedeemSubscriptionTestService(h)

	user := h.createUser(t, "redeem-purchase@test.com")
	group := h.createGroup(t, "redeem-purchase-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Redeem Starter", 100, 30, "day", &groupID, nil, nil, &monthly)
	code := createRedeemSubscriptionCode(t, h, "REDEEM-PURCHASE", plan.ID)

	result, err := redeemSvc.Redeem(h.ctx, user.ID, code.Code)
	require.NoError(t, err)
	require.Equal(t, service.StatusUsed, result.Status)

	active, err := h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, active.PlanID)
	require.Equal(t, plan.ID, *active.PlanID)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 1)
	settlement := settlements[0]
	require.Equal(t, domain.SettlementActionPurchase, settlement.ActionType)
	require.Equal(t, domain.SettlementActionSourceExchangeCode, settlement.ActionSource)
	require.Equal(t, domain.SettlementTriggerRefRedeemCode, settlement.TriggerRefType)
	require.NotNil(t, settlement.TriggerRefID)
	require.Equal(t, code.ID, *settlement.TriggerRefID)
	require.Equal(t, domain.SettlementStatusEffective, settlement.Status)
	require.Equal(t, active.ID, *settlement.AfterUserSubscriptionID)
	require.InDelta(t, plan.Price, settlement.AfterSettlementValue, 1e-9)
}

func TestRedeemSubscriptionRenewEnablesZeroDeltaUpgrade(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)
	redeemSvc := newRedeemSubscriptionTestService(h)

	user := h.createUser(t, "redeem-renew@test.com")
	group := h.createGroup(t, "redeem-renew-group")
	groupID := group.ID
	monthly := 100.0
	basePlan := h.createPlan(t, "Redeem Base", 100, 30, "day", &groupID, nil, nil, &monthly)
	targetPlan := h.createPlan(t, "Redeem Pro", 160, 30, "day", &groupID, nil, nil, &monthly)

	order := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, basePlan.ID, basePlan.Name, basePlan.Price)
	err := paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID)
	require.NoError(t, err)

	code := createRedeemSubscriptionCode(t, h, "REDEEM-RENEW", basePlan.ID)
	result, err := redeemSvc.Redeem(h.ctx, user.ID, code.Code)
	require.NoError(t, err)
	require.Equal(t, service.StatusUsed, result.Status)

	preview, err := paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, targetPlan.ID)
	require.NoError(t, err)
	require.Equal(t, "upgrade", preview.Action)
	require.True(t, preview.CanCompleteDirectly)
	require.InDelta(t, 0, preview.OrderAmount, 1e-9)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 2)
	require.Equal(t, domain.SettlementStatusClosed, settlements[0].Status)
	require.Equal(t, domain.SettlementStatusEffective, settlements[1].Status)
	require.Equal(t, domain.SettlementActionRenew, settlements[1].ActionType)
	require.Equal(t, domain.SettlementActionSourceExchangeCode, settlements[1].ActionSource)
	require.Equal(t, domain.SettlementTriggerRefRedeemCode, settlements[1].TriggerRefType)
	require.NotNil(t, settlements[1].TriggerRefID)
	require.Equal(t, code.ID, *settlements[1].TriggerRefID)
	require.InDelta(t, 200, settlements[1].AfterSettlementValue, 0.01)
}

func newRedeemSubscriptionTestService(h *subscriptionOpsHarness) *service.RedeemService {
	return service.NewRedeemService(
		repository.NewRedeemCodeRepository(h.client),
		repository.NewUserRepository(h.client, h.db),
		h.svc,
		nil,
		nil,
		h.client,
		nil,
		nil,
	)
}

func createRedeemSubscriptionCode(t *testing.T, h *subscriptionOpsHarness, code string, planID int64) *dbent.RedeemCode {
	t.Helper()
	redeemCode, err := h.client.RedeemCode.Create().
		SetCode(code).
		SetType(service.RedeemTypeSubscription).
		SetStatus(service.StatusUnused).
		SetPlanID(planID).
		Save(h.ctx)
	require.NoError(t, err)
	return redeemCode
}
