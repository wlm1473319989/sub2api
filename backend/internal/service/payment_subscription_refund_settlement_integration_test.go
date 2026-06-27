package service_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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
	require.NotNil(t, result.SettlementHead)
	require.Equal(t, refundPlan.SettlementHead.ID, result.SettlementHead.HeadID)
	require.Equal(t, domain.SettlementActionSourceUserPurchase, result.SettlementHead.ActionSource)
	require.Equal(t, domain.SettlementTriggerRefPaymentOrder, result.SettlementHead.TriggerRefType)
	require.NotNil(t, result.SettlementHead.TriggerRefID)
	require.Equal(t, order.ID, *result.SettlementHead.TriggerRefID)
	require.InDelta(t, refundPlan.SettlementResidual, result.SettlementHead.CurrentResidualValue, 0.01)
	require.InDelta(t, refundPlan.SettlementResidual, result.SettlementHead.RefundResidualValue, 0.01)

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

	preview, err := paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, plan.ID, payment.DefaultPaymentCurrency)
	require.NoError(t, err)
	require.Equal(t, domain.SettlementActionPurchase, preview.Action)
	require.InDelta(t, plan.Price, preview.OrderAmount, 1e-9)
}

func TestPrepareSubscriptionRefundMismatchReturnsSettlementHeadInfo(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)

	user := h.createUser(t, "refund-preview-settlement@test.com")
	group := h.createGroup(t, "refund-preview-settlement-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Refund Preview Settlement", 100, 30, "day", &groupID, nil, nil, &monthly)

	inst, err := h.client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("refund-preview-settlement-provider").
		SetConfig("{}").
		SetSupportedTypes(payment.TypeAlipay).
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(h.ctx)
	require.NoError(t, err)
	instID := strconv.FormatInt(inst.ID, 10)

	headOrder := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)
	err = paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, headOrder.ID)
	require.NoError(t, err)

	mismatchedOrder := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)
	mismatchedOrder, err = h.client.PaymentOrder.UpdateOneID(mismatchedOrder.ID).
		SetStatus(service.OrderStatusCompleted).
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(h.ctx)
	require.NoError(t, err)

	refundPlan, earlyResult, err := paymentSvc.PrepareRefund(h.ctx, mismatchedOrder.ID, 0, "preview settlement", false, true)
	require.NoError(t, err)
	require.Nil(t, refundPlan)
	require.NotNil(t, earlyResult)
	require.True(t, earlyResult.RequireForce)
	require.NotNil(t, earlyResult.SettlementHead)
	require.Equal(t, domain.SettlementActionSourceUserPurchase, earlyResult.SettlementHead.ActionSource)
	require.Equal(t, domain.SettlementTriggerRefPaymentOrder, earlyResult.SettlementHead.TriggerRefType)
	require.NotNil(t, earlyResult.SettlementHead.TriggerRefID)
	require.Equal(t, headOrder.ID, *earlyResult.SettlementHead.TriggerRefID)
	require.Greater(t, earlyResult.SettlementHead.CurrentResidualValue, 0.0)
	require.Greater(t, earlyResult.SettlementHead.RefundResidualValue, 0.0)
}

func TestResolveSubscriptionRefundTargetAllowsSubscriptionOrderWhenUserRefundDisabled(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)

	user := h.createUser(t, "resolve-sub-refund@test.com")
	group := h.createGroup(t, "resolve-sub-refund-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Resolve Subscription Refund", 100, 30, "day", &groupID, nil, nil, &monthly)

	inst, err := h.client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("subscription-user-refund-disabled").
		SetConfig("{}").
		SetSupportedTypes(payment.TypeAlipay).
		SetEnabled(true).
		SetRefundEnabled(true).
		SetAllowUserRefund(false).
		Save(h.ctx)
	require.NoError(t, err)

	order := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)
	instID := strconv.FormatInt(inst.ID, 10)
	order, err = h.client.PaymentOrder.UpdateOneID(order.ID).
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(h.ctx)
	require.NoError(t, err)

	require.NoError(t, paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID))

	resolvedOrder, subscription, err := paymentSvc.ResolveSubscriptionRefundTarget(h.ctx, order.ID, user.ID)
	require.NoError(t, err)
	require.NotNil(t, resolvedOrder)
	require.Equal(t, order.ID, resolvedOrder.ID)
	require.NotNil(t, subscription)
}

func TestResolveSubscriptionRefundTargetRejectsNonCurrentSettlementSourceOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, nil, nil, nil)

	user := h.createUser(t, "resolve-sub-refund-mismatch@test.com")
	group := h.createGroup(t, "resolve-sub-refund-mismatch-group")
	groupID := group.ID
	monthly := 100.0
	plan := h.createPlan(t, "Resolve Subscription Refund Mismatch", 100, 30, "day", &groupID, nil, nil, &monthly)

	inst, err := h.client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("subscription-user-refund-disabled-mismatch").
		SetConfig("{}").
		SetSupportedTypes(payment.TypeAlipay).
		SetEnabled(true).
		SetRefundEnabled(true).
		SetAllowUserRefund(false).
		Save(h.ctx)
	require.NoError(t, err)
	instID := strconv.FormatInt(inst.ID, 10)

	headOrder := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)
	headOrder, err = h.client.PaymentOrder.UpdateOneID(headOrder.ID).
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(h.ctx)
	require.NoError(t, err)
	require.NoError(t, paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, headOrder.ID))

	mismatchedOrder := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, plan.ID, plan.Name, plan.Price)
	mismatchedOrder, err = h.client.PaymentOrder.UpdateOneID(mismatchedOrder.ID).
		SetStatus(service.OrderStatusCompleted).
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(h.ctx)
	require.NoError(t, err)

	_, _, err = paymentSvc.ResolveSubscriptionRefundTarget(h.ctx, mismatchedOrder.ID, user.ID)
	require.Error(t, err)
	require.Equal(t, infraerrors.Reason(service.ErrSubscriptionRefundOrderRequiresCurrentSettlement), infraerrors.Reason(err))
}

func TestResolveSubscriptionRefundTargetKeepsBalanceOrderUserRefundGuard(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, nil, nil)

	user := h.createUser(t, "resolve-balance-refund@test.com")

	inst, err := h.client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("balance-user-refund-disabled").
		SetConfig("{}").
		SetSupportedTypes(payment.TypeAlipay).
		SetEnabled(true).
		SetRefundEnabled(true).
		SetAllowUserRefund(false).
		Save(h.ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	orderNo := fmt.Sprintf("sub2_balance_refund_%d", time.Now().UnixNano())
	order, err := h.client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Email).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode(orderNo).
		SetOutTradeNo(orderNo).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-balance-refund-disabled").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(h.ctx)
	require.NoError(t, err)

	svc := service.NewPaymentService(h.client, nil, nil, nil, nil, configSvc, nil, nil, nil)

	resolvedOrder, subscription, err := svc.ResolveSubscriptionRefundTarget(h.ctx, order.ID, user.ID)
	require.Error(t, err)
	require.Nil(t, resolvedOrder)
	require.Nil(t, subscription)
	require.Equal(t, "USER_REFUND_DISABLED", infraerrors.Reason(err))
}
