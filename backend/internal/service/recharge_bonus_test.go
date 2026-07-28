//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCalculateRechargeRuleBonus(t *testing.T) {
	t.Parallel()

	require.Equal(t, 12.34, calculateRechargeRuleBonus(100, RechargeBonusTypeFixed, 12.34))
	require.Equal(t, 12.35, calculateRechargeRuleBonus(123.45, RechargeBonusTypePercentage, 10))
}

func TestMatchRechargeBonusRuleUsesClosedRangesAndHighestPriority(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	now := time.Now().UTC()

	low, err := client.RechargeBonusRule.Create().
		SetName("low priority").
		SetEnabled(true).
		SetPriority(1).
		SetMinAmount(100).
		SetMaxAmount(200).
		SetBonusType(RechargeBonusTypeFixed).
		SetBonusValue(5).
		Save(ctx)
	require.NoError(t, err)

	high, err := client.RechargeBonusRule.Create().
		SetName("high priority").
		SetEnabled(true).
		SetPriority(10).
		SetMinAmount(100).
		SetMaxAmount(200).
		SetBonusType(RechargeBonusTypePercentage).
		SetBonusValue(20).
		SetStartsAt(now).
		SetEndsAt(now.Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client}
	for _, amount := range []float64{100, 200} {
		matched, matchErr := svc.matchRechargeBonusRule(ctx, amount, now)
		require.NoError(t, matchErr)
		require.Equal(t, high.ID, matched.ID)
	}

	matched, err := svc.matchRechargeBonusRule(ctx, 150, now.Add(2*time.Hour))
	require.NoError(t, err)
	require.Equal(t, low.ID, matched.ID)

	matched, err = svc.matchRechargeBonusRule(ctx, 99.99, now)
	require.NoError(t, err)
	require.Nil(t, matched)
}

func TestCalculateRechargeQuoteUsesRuleThenLegacyMultiplier(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	configSvc := &PaymentConfigService{entClient: client}
	svc := &PaymentService{entClient: client, configService: configSvc}
	cfg := &PaymentConfig{BalanceRechargeMultiplier: 1.5, RechargeFeeRate: 0}

	rule, err := client.RechargeBonusRule.Create().
		SetName("fixed bonus").
		SetEnabled(true).
		SetPriority(1).
		SetMinAmount(100).
		SetMaxAmount(200).
		SetBonusType(RechargeBonusTypeFixed).
		SetBonusValue(20).
		Save(ctx)
	require.NoError(t, err)

	quote, err := svc.calculateRechargeQuote(ctx, 100, payment.TypeAlipay, cfg)
	require.NoError(t, err)
	require.Equal(t, 20.0, quote.Bonus)
	require.Equal(t, 120.0, quote.CreditedAmount)
	require.NotNil(t, quote.Rule)
	require.Equal(t, rule.ID, quote.Rule.ID)

	quote, err = svc.calculateRechargeQuote(ctx, 50, payment.TypeAlipay, cfg)
	require.NoError(t, err)
	require.Equal(t, 25.0, quote.Bonus)
	require.Equal(t, 75.0, quote.CreditedAmount)
	require.NotNil(t, quote.Rule)
	require.Equal(t, rechargeBonusSourceMultiplier, quote.Rule.Source)
}

func TestRechargeQuoteTokenRejectsExpiryAndQuoteChanges(t *testing.T) {
	resumeSvc := NewPaymentResumeService([]byte("recharge-quote-test-key"))
	svc := &PaymentService{resumeService: resumeSvc}
	quote := &RechargeQuote{
		Principal: 100, Bonus: 20, CreditedAmount: 120, PayAmount: 100,
		Currency: payment.DefaultPaymentCurrency,
		snapshot: map[string]any{"source": rechargeBonusSourceNone},
	}
	claims := rechargeQuoteClaimsFromQuote(quote, payment.TypeAlipay)
	token, err := resumeSvc.CreateRechargeQuoteToken(claims)
	require.NoError(t, err)
	require.NoError(t, svc.validateRechargeQuoteToken(token, payment.TypeAlipay, quote))

	changed := *quote
	changed.Bonus = 30
	changed.CreditedAmount = 130
	err = svc.validateRechargeQuoteToken(token, payment.TypeAlipay, &changed)
	require.Equal(t, "RECHARGE_QUOTE_CHANGED", infraerrors.Reason(err))

	claims.TokenType = rechargeQuoteTokenType
	claims.IssuedAt = time.Now().Add(-10 * time.Minute).Unix()
	claims.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	expiredToken, err := resumeSvc.createSignedToken(claims)
	require.NoError(t, err)
	err = svc.validateRechargeQuoteToken(expiredToken, payment.TypeAlipay, quote)
	require.Equal(t, "RECHARGE_QUOTE_CHANGED", infraerrors.Reason(err))
}

func TestPrepareRechargeBonusRefundAmountsRecoversBonusFirstAndAccumulates(t *testing.T) {
	order := &dbent.PaymentOrder{
		Amount: 120, PayAmount: 100, OrderType: payment.OrderTypeBalance,
		RechargePrincipal: 100, RechargeBonus: 20,
		RechargeBonusSnapshot: map[string]any{"source": rechargeBonusSourceRule},
	}

	first := &RefundPlan{Order: order}
	require.NoError(t, prepareRechargeBonusRefundAmounts(first, 20, "CNY"))
	require.Equal(t, 20.0, first.BonusRecoveryAmount)
	require.Zero(t, first.PrincipalRefundAmount)
	require.Zero(t, first.GatewayAmount)

	order.RefundAmount = first.CumulativeRecoveryAmount
	order.RefundedBonusAmount = first.CumulativeBonusAmount
	order.RefundedPrincipalAmount = first.CumulativePrincipalAmount
	order.RefundedGatewayAmount = first.CumulativeGatewayAmount
	second := &RefundPlan{Order: order}
	require.NoError(t, prepareRechargeBonusRefundAmounts(second, 30, "CNY"))
	require.Zero(t, second.BonusRecoveryAmount)
	require.Equal(t, 30.0, second.PrincipalRefundAmount)
	require.Equal(t, 30.0, second.GatewayAmount)

	order.RefundAmount = second.CumulativeRecoveryAmount
	order.RefundedBonusAmount = second.CumulativeBonusAmount
	order.RefundedPrincipalAmount = second.CumulativePrincipalAmount
	order.RefundedGatewayAmount = second.CumulativeGatewayAmount
	final := &RefundPlan{Order: order}
	require.NoError(t, prepareRechargeBonusRefundAmounts(final, 0, "CNY"))
	require.Equal(t, 70.0, final.RecoveryAmount)
	require.Equal(t, 70.0, final.PrincipalRefundAmount)
	require.Equal(t, 70.0, final.GatewayAmount)
	require.Equal(t, 120.0, final.CumulativeRecoveryAmount)
	require.Equal(t, 100.0, final.CumulativePrincipalAmount)
	require.Equal(t, 100.0, final.CumulativeGatewayAmount)
}

func TestPrepareRechargeBonusRefundAmountsUsesCurrencyRounding(t *testing.T) {
	order := &dbent.PaymentOrder{
		Amount: 120, PayAmount: 12.345, OrderType: payment.OrderTypeBalance,
		RechargePrincipal: 100, RechargeBonus: 20, RefundedBonusAmount: 20, RefundAmount: 20,
		RechargeBonusSnapshot: map[string]any{"source": rechargeBonusSourceRule},
	}
	plan := &RefundPlan{Order: order}
	require.NoError(t, prepareRechargeBonusRefundAmounts(plan, 50, "KWD"))
	require.Equal(t, 6.173, plan.GatewayAmount)
}

func TestExecuteRechargeBonusRefundAccumulatesAcrossPartialRefunds(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}

	user, err := client.User.Create().
		SetEmail("recharge-refund@example.com").
		SetPasswordHash("hash").
		SetUsername("recharge-refund").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(120).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargePrincipal(100).
		SetRechargeBonus(20).
		SetRechargeBonusSnapshot(map[string]any{"source": rechargeBonusSourceRule}).
		SetRechargeCode("RECHARGE-REFUND").
		SetOutTradeNo("sub2_recharge_refund").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	execute := func(current *dbent.PaymentOrder, requested float64) *dbent.PaymentOrder {
		plan := &RefundPlan{OrderID: current.ID, Order: current, Reason: "test refund"}
		require.NoError(t, prepareRechargeBonusRefundAmounts(plan, requested, payment.DefaultPaymentCurrency))
		result, executeErr := svc.ExecuteRefund(ctx, plan)
		require.NoError(t, executeErr)
		require.True(t, result.Success)
		updated, queryErr := client.PaymentOrder.Get(ctx, current.ID)
		require.NoError(t, queryErr)
		return updated
	}

	order = execute(order, 20)
	require.Equal(t, OrderStatusPartiallyRefunded, order.Status)
	require.Equal(t, 20.0, order.RefundAmount)
	require.Equal(t, 20.0, order.RefundedBonusAmount)
	require.Zero(t, order.RefundedPrincipalAmount)
	require.Zero(t, order.RefundedGatewayAmount)

	order = execute(order, 30)
	require.Equal(t, OrderStatusPartiallyRefunded, order.Status)
	require.Equal(t, 50.0, order.RefundAmount)
	require.Equal(t, 20.0, order.RefundedBonusAmount)
	require.Equal(t, 30.0, order.RefundedPrincipalAmount)
	require.Equal(t, 30.0, order.RefundedGatewayAmount)

	order = execute(order, 0)
	require.Equal(t, OrderStatusRefunded, order.Status)
	require.Equal(t, 120.0, order.RefundAmount)
	require.Equal(t, 20.0, order.RefundedBonusAmount)
	require.Equal(t, 100.0, order.RefundedPrincipalAmount)
	require.Equal(t, 100.0, order.RefundedGatewayAmount)
}
