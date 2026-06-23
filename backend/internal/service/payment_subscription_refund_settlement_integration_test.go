package service_test

import (
	"strconv"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestExecuteSubscriptionRefundCreatesSettlementOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)

	user := h.createUser(t, "refund-settlement@test.com")
	group := h.createGroup(t, "refund-settlement-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Refund Settlement", 100, 30, "day", &groupID, nil, nil, &monthly)

	inst, err := h.client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("refund-settlement-provider").
		SetConfig("{}").
		SetSupportedTypes(payment.TypeAlipay).
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(h.ctx)
	require.NoError(t, err)

	order := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)
	instID := strconv.FormatInt(inst.ID, 10)
	order, err = h.client.PaymentOrder.UpdateOneID(order.ID).
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		SetPaymentTradeNo("").
		Save(h.ctx)
	require.NoError(t, err)

	err = paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID)
	require.NoError(t, err)

	refundPlan, earlyResult, err := paymentSvc.PrepareRefund(h.ctx, order.ID, 0, "refund settlement", false, true)
	require.NoError(t, err)
	require.Nil(t, earlyResult)
	require.NotNil(t, refundPlan)
	require.NotNil(t, refundPlan.SettlementHead)
	require.InDelta(t, plan.Price, refundPlan.SettlementResidual, 0.01)

	result, err := paymentSvc.ExecuteRefund(h.ctx, refundPlan)
	require.NoError(t, err)
	require.True(t, result.Success)

	reloadedOrder, err := h.client.PaymentOrder.Get(h.ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusRefunded, reloadedOrder.Status)

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
	require.Equal(t, domain.SettlementActionSourceUserPurchase, settlements[1].ActionSource)
	require.Equal(t, domain.SettlementTriggerRefPaymentOrder, settlements[1].TriggerRefType)
	require.NotNil(t, settlements[1].TriggerRefID)
	require.Equal(t, order.ID, *settlements[1].TriggerRefID)
	require.NotNil(t, settlements[1].PrevSettlementID)
	require.Equal(t, settlements[0].ID, *settlements[1].PrevSettlementID)
	require.NotNil(t, settlements[1].RefundResidualValue)
	require.InDelta(t, refundPlan.SettlementResidual, *settlements[1].RefundResidualValue, 0.01)
	require.InDelta(t, -refundPlan.SettlementResidual, settlements[1].ActionDeltaValue, 0.01)
	require.InDelta(t, 0, settlements[1].AfterSettlementValue, 1e-9)
	require.Equal(t, domain.SubscriptionStatusRefunded, settlements[1].AfterSubscriptionStatus)
}
