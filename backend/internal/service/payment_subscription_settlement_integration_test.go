package service_test

import (
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestExecuteSubscriptionFulfillmentCreatesSettlementOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)

	user := h.createUser(t, "fulfill-settlement@test.com")
	group := h.createGroup(t, "fulfill-settlement-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Settlement Starter", 100, 30, "day", &groupID, nil, nil, &monthly)
	order := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)

	err := paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID)
	require.NoError(t, err)

	reloadedOrder, err := h.client.PaymentOrder.Get(h.ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusCompleted, reloadedOrder.Status)

	active, err := h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, active.PlanID)
	require.Equal(t, plan.ID, *active.PlanID)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 1)
	settlement := settlements[0]
	require.Equal(t, domain.SettlementActionPurchase, settlement.ActionType)
	require.Equal(t, domain.SettlementActionSourceUserPurchase, settlement.ActionSource)
	require.Equal(t, domain.SettlementStatusEffective, settlement.Status)
	require.Equal(t, domain.SettlementTriggerRefPaymentOrder, settlement.TriggerRefType)
	require.NotNil(t, settlement.TriggerRefID)
	require.Equal(t, order.ID, *settlement.TriggerRefID)
	require.NotNil(t, settlement.AfterUserSubscriptionID)
	require.Equal(t, active.ID, *settlement.AfterUserSubscriptionID)
	require.NotNil(t, settlement.AfterPlanID)
	require.Equal(t, plan.ID, *settlement.AfterPlanID)
	require.InDelta(t, plan.Price, settlement.ActionDeltaValue, 1e-9)
	require.InDelta(t, plan.Price, settlement.AfterSettlementValue, 1e-9)
}

func createPaidSettlementSubscriptionOrder(t *testing.T, h *subscriptionOpsHarness, userID int64, email string, planID int64, planName string, planPrice float64) *dbent.PaymentOrder {
	t.Helper()
	now := time.Now()
	outTradeNo := fmt.Sprintf("sub2_settlement_%d", now.UnixNano())
	order, err := h.client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail(email).
		SetUserName(email).
		SetAmount(planPrice).
		SetPayAmount(planPrice).
		SetFeeRate(0).
		SetRechargeCode(outTradeNo).
		SetOutTradeNo(outTradeNo).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-settlement").
		SetOrderType(payment.OrderTypeSubscription).
		SetStatus(service.OrderStatusPaid).
		SetPaidAt(now).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetPlanID(planID).
		SetSubscriptionAction(domain.SettlementActionPurchase).
		SetSubscriptionPlanNameSnapshot(planName).
		SetSubscriptionPlanPriceSnapshot(planPrice).
		SetSubscriptionValidityDaysSnapshot(30).
		Save(h.ctx)
	require.NoError(t, err)
	return order
}
