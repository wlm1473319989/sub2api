//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPrepareRefund_NewSubscriptionOrderCapturesActiveSnapshot(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	groupRepo := newPaymentSubscriptionGroupRepoStub()
	userSubRepo := newSubscriptionUserSubRepoStub()
	subscriptionSvc := NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	configSvc := NewPaymentConfigService(client, nil, nil)
	svc := &PaymentService{
		entClient:       client,
		configService:   configSvc,
		subscriptionSvc: subscriptionSvc,
	}

	user, err := client.User.Create().
		SetEmail("refund-user-level@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-user-level").
		Save(ctx)
	require.NoError(t, err)

	group, err := client.Group.Create().
		SetName("refund-user-level-group").
		SetStatus(StatusActive).
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	groupRepo.byID[group.ID] = groupToService(group)

	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("User Level").
		SetDescription("plan").
		SetPrice(88).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	sub, err := subscriptionSvc.PurchaseNewPlan(ctx, &PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   plan,
	})
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-USER-LEVEL").
		SetOutTradeNo("sub2_refund_user_level").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(plan.ID).
		SetSubscriptionAction(subscriptionActionPurchase).
		SetSubscriptionPlanNameSnapshot(plan.Name).
		SetSubscriptionPlanPriceSnapshot(plan.Price).
		SetSubscriptionValidityDaysSnapshot(30).
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	p, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.NotNil(t, p)
	require.Equal(t, payment.DeductionTypeSubscription, p.DeductionType)
	require.Equal(t, sub.ID, p.SubscriptionID)
	require.NotNil(t, p.SubscriptionSnapshot)
	require.Nil(t, p.SubscriptionSnapshot.SupersededByID)
}

func TestExecuteRefund_UserLevelSubscriptionOrderMarksSubscriptionRefunded(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	groupRepo := newPaymentSubscriptionGroupRepoStub()
	userSubRepo := newSubscriptionUserSubRepoStub()
	subscriptionSvc := NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	configSvc := NewPaymentConfigService(client, nil, nil)
	svc := &PaymentService{
		entClient:       client,
		configService:   configSvc,
		subscriptionSvc: subscriptionSvc,
	}

	user, err := client.User.Create().
		SetEmail("refund-exec@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-exec").
		Save(ctx)
	require.NoError(t, err)

	group, err := client.Group.Create().
		SetName("refund-exec-group").
		SetStatus(StatusActive).
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	groupRepo.byID[group.ID] = groupToService(group)

	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("Refundable").
		SetDescription("plan").
		SetPrice(88).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	sub, err := subscriptionSvc.PurchaseNewPlan(ctx, &PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   plan,
	})
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-EXEC").
		SetOutTradeNo("sub2_refund_exec").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(plan.ID).
		SetSubscriptionAction(subscriptionActionPurchase).
		SetSubscriptionPlanNameSnapshot(plan.Name).
		SetSubscriptionPlanPriceSnapshot(plan.Price).
		SetSubscriptionValidityDaysSnapshot(30).
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	p, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.NotNil(t, p)

	got, err := svc.ExecuteRefund(ctx, p)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, got.Success)

	refunded, err := subscriptionSvc.GetByID(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusRefunded, refunded.Status)
}

func TestValidateRefundRequestRejectsLegacyGuessedProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetAllowUserRefund(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ORDER").
		SetOutTradeNo("sub2_refund_legacy_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	_, err = svc.validateRefundRequest(ctx, order.ID, user.ID)
	require.Error(t, err)
	require.Equal(t, "USER_REFUND_DISABLED", infraerrors.Reason(err))
}

func TestPrepareRefundRejectsLegacyGuessedProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy-admin@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-admin-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-admin-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetAllowUserRefund(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(188).
		SetPayAmount(188).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ADMIN-ORDER").
		SetOutTradeNo("sub2_refund_legacy_admin_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-admin-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, false)
	require.Nil(t, plan)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_DISABLED", infraerrors.Reason(err))
}

func TestGwRefundRejectsAlipayMerchantIdentitySnapshotMismatch(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-snapshot-mismatch@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-snapshot-mismatch-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-mismatch-instance").
		SetConfig(encryptWebhookProviderConfig(t, map[string]string{
			"appId":      "runtime-alipay-app",
			"privateKey": "runtime-private-key",
		})).
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-SNAPSHOT-MISMATCH-ORDER").
		SetOutTradeNo("sub2_refund_snapshot_mismatch_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-snapshot-mismatch").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		SetProviderSnapshot(map[string]any{
			"schema_version":       2,
			"provider_instance_id": instID,
			"provider_key":         payment.TypeAlipay,
			"merchant_app_id":      "expected-alipay-app",
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient:    client,
		loadBalancer: newWebhookProviderTestLoadBalancer(client),
	}

	err = svc.gwRefund(ctx, &RefundPlan{
		OrderID:       order.ID,
		Order:         order,
		RefundAmount:  order.Amount,
		GatewayAmount: order.Amount,
		Reason:        "snapshot mismatch",
	})
	require.ErrorContains(t, err, "alipay app_id mismatch")
}

func TestCalculateGatewayRefundAmountUsesCurrencyPrecision(t *testing.T) {
	require.InDelta(t, 6.173, calculateGatewayRefundAmount(100, 12.345, 50, "KWD"), 1e-12)
	require.InDelta(t, 12.345, calculateGatewayRefundAmount(100, 12.345, 100, "KWD"), 1e-12)
	require.InDelta(t, 52, calculateGatewayRefundAmount(100, 103, 50, "JPY"), 1e-12)
}

func TestFormatGatewayRefundAmountUsesOrderCurrency(t *testing.T) {
	order := &dbent.PaymentOrder{
		ProviderSnapshot: map[string]any{
			"currency": "KWD",
		},
	}

	require.Equal(t, "12.345", formatGatewayRefundAmount(12.345, order))
}

func TestValidateRefundProviderResponseAcceptsPending(t *testing.T) {
	require.NoError(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusPending}))
	require.NoError(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusSuccess}))
	require.Error(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusFailed}))
	require.Error(t, validateRefundProviderResponse(nil))
}
