package service_test

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRefundExchangeCodeSettlementHeadCreatesRefundSettlementOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)
	redeemSvc := newRedeemSubscriptionTestService(h)

	user := h.createUser(t, "refund-exchange-head@test.com")
	group := h.createGroup(t, "refund-exchange-head-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Refund Exchange Head", 100, 30, "day", &groupID, nil, nil, &monthly)
	code := createRedeemSubscriptionCode(t, h, "REFUND-EXCHANGE-HEAD", plan.ID)

	redeemResult, err := redeemSvc.Redeem(h.ctx, user.ID, code.Code)
	require.NoError(t, err)
	require.Equal(t, service.StatusUsed, redeemResult.Status)

	active, err := h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)

	refundResult, err := h.svc.RefundActiveSettlementHead(h.ctx, &service.RefundActiveSettlementHeadInput{
		UserID: user.ID,
		Notes:  "refund exchange settlement",
	})
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusRefunded, refundResult.Subscription.Status)
	require.Equal(t, active.ID, refundResult.Subscription.ID)
	require.Greater(t, refundResult.RefundResidualValue, 0.0)
	require.LessOrEqual(t, refundResult.RefundResidualValue, plan.Price)

	paymentOrderCount, err := h.client.PaymentOrder.Query().
		Where(paymentorder.UserIDEQ(user.ID)).
		Count(h.ctx)
	require.NoError(t, err)
	require.Zero(t, paymentOrderCount)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 2)
	require.Equal(t, domain.SettlementStatusClosed, settlements[0].Status)
	require.Equal(t, domain.SettlementActionPurchase, settlements[0].ActionType)
	require.Equal(t, domain.SettlementStatusEffective, settlements[1].Status)
	require.Equal(t, domain.SettlementActionRefund, settlements[1].ActionType)
	require.Equal(t, domain.SettlementActionSourceExchangeCode, settlements[1].ActionSource)
	require.Equal(t, domain.SettlementTriggerRefRedeemCode, settlements[1].TriggerRefType)
	require.NotNil(t, settlements[1].TriggerRefID)
	require.Equal(t, code.ID, *settlements[1].TriggerRefID)
	require.NotNil(t, settlements[1].PrevSettlementID)
	require.Equal(t, settlements[0].ID, *settlements[1].PrevSettlementID)
	require.NotNil(t, settlements[1].RefundResidualValue)
	require.InDelta(t, refundResult.RefundResidualValue, *settlements[1].RefundResidualValue, 1e-9)
	require.InDelta(t, -refundResult.RefundResidualValue, settlements[1].ActionDeltaValue, 1e-9)
	require.InDelta(t, 0, settlements[1].AfterSettlementValue, 1e-9)
	require.Equal(t, domain.SubscriptionStatusRefunded, settlements[1].AfterSubscriptionStatus)

	head, err := service.NewSettlementService(h.client).GetEffectiveHead(h.ctx, user.ID, settlements[1].EffectiveAt)
	require.NoError(t, err)
	require.Nil(t, head)

	preview, err := paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, plan.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SettlementActionPurchase, preview.Action)
	require.InDelta(t, plan.Price, preview.OrderAmount, 1e-9)
}

func TestRefundAdminAssignmentSettlementHeadCreatesRefundSettlementOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)

	operator := h.createUser(t, "refund-admin-head-operator@test.com")
	user := h.createUser(t, "refund-admin-head-user@test.com")
	group := h.createGroup(t, "refund-admin-head-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Refund Admin Head", 100, 30, "day", &groupID, nil, nil, &monthly)

	active, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID:     user.ID,
		PlanID:     plan.ID,
		AssignedBy: operator.ID,
		Notes:      "admin assign settlement",
	})
	require.NoError(t, err)
	require.False(t, reused)

	refundResult, err := h.svc.RefundActiveSettlementHead(h.ctx, &service.RefundActiveSettlementHeadInput{
		UserID:         user.ID,
		OperatorUserID: operator.ID,
		Notes:          "refund admin settlement",
	})
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusRefunded, refundResult.Subscription.Status)
	require.Equal(t, active.ID, refundResult.Subscription.ID)
	require.Greater(t, refundResult.RefundResidualValue, 0.0)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 2)
	require.Equal(t, domain.SettlementStatusClosed, settlements[0].Status)
	require.Equal(t, domain.SettlementStatusEffective, settlements[1].Status)
	require.Equal(t, domain.SettlementActionRefund, settlements[1].ActionType)
	require.Equal(t, domain.SettlementActionSourceSubscriptionAssign, settlements[1].ActionSource)
	require.Equal(t, domain.SettlementTriggerRefAdminAssignment, settlements[1].TriggerRefType)
	require.Nil(t, settlements[1].TriggerRefID)
	require.Equal(t, operator.ID, settlements[1].OperatorUserID)
	require.NotNil(t, settlements[1].PrevSettlementID)
	require.Equal(t, settlements[0].ID, *settlements[1].PrevSettlementID)
	require.NotNil(t, settlements[1].RefundResidualValue)
	require.InDelta(t, refundResult.RefundResidualValue, *settlements[1].RefundResidualValue, 1e-9)
	require.Equal(t, domain.SubscriptionStatusRefunded, settlements[1].AfterSubscriptionStatus)
}

func TestRefundPaymentSettlementHeadRequiresPaymentRefund(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)

	user := h.createUser(t, "refund-payment-head@test.com")
	group := h.createGroup(t, "refund-payment-head-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Refund Payment Head", 100, 30, "day", &groupID, nil, nil, &monthly)

	order := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)
	err := paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID)
	require.NoError(t, err)

	_, err = h.svc.RefundActiveSettlementHead(h.ctx, &service.RefundActiveSettlementHeadInput{
		UserID: user.ID,
		Notes:  "direct refund should reject payment source",
	})
	require.ErrorIs(t, err, service.ErrSettlementRefundRequiresPayment)

	active, err := h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusActive, active.Status)
}
