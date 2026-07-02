package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/repository"
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

func TestCreateOrderZeroDeltaUpgradeCompletesDirectly(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, &paymentDirectSettingRepoStub{
		values: map[string]string{service.SettingPaymentEnabled: "true"},
	}, nil)
	userRepo := repository.NewUserRepository(h.client, h.db)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, userRepo, nil, nil)

	user := h.createUser(t, "direct-upgrade@test.com")
	group := h.createGroup(t, "direct-upgrade-group")
	groupID := group.ID
	monthly := 100.0
	basePlan := h.createPlan(t, "Direct Starter", 100, 30, "day", &groupID, nil, nil, &monthly)
	targetPlan := h.createPlan(t, "Direct Pro", 160, 30, "day", &groupID, nil, nil, &monthly)

	active := seedRenewedSettlementHead(t, h, user.ID, basePlan.ID, basePlan.Name, basePlan.Price)

	preview, err := paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, targetPlan.ID, payment.DefaultPaymentCurrency)
	require.NoError(t, err)
	require.Equal(t, "upgrade", preview.Action)
	require.True(t, preview.CanCompleteDirectly)
	require.InDelta(t, 0, preview.OrderAmount, 1e-9)
	require.NotNil(t, preview.UpgradeBreakdown)
	require.InDelta(t, 0, preview.UpgradeBreakdown.UpgradeDelta, 1e-9)

	resp, err := paymentSvc.CreateOrder(h.ctx, service.CreateOrderRequest{
		UserID:      user.ID,
		Amount:      0,
		PaymentType: payment.TypeAlipay,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      targetPlan.ID,
		ClientIP:    "127.0.0.1",
		SrcHost:     "example.com",
	})
	require.NoError(t, err)
	require.Equal(t, payment.CreatePaymentResultCompletedDirectly, resp.ResultType)
	require.Zero(t, resp.OrderID)
	require.Equal(t, service.OrderStatusCompleted, resp.Status)
	require.Equal(t, "upgrade", resp.SubscriptionAction)

	current, err := h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, current.PlanID)
	require.Equal(t, targetPlan.ID, *current.PlanID)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 2)
	require.Equal(t, domain.SettlementStatusClosed, settlements[0].Status)
	require.Equal(t, domain.SettlementStatusEffective, settlements[1].Status)
	require.Equal(t, domain.SettlementActionUpgrade, settlements[1].ActionType)
	require.Equal(t, domain.SettlementTriggerRefDirectAction, settlements[1].TriggerRefType)
	require.Nil(t, settlements[1].TriggerRefID)
	require.NotNil(t, settlements[1].PrevSettlementID)
	require.Equal(t, settlements[0].ID, *settlements[1].PrevSettlementID)
	require.Equal(t, current.ID, *settlements[1].AfterUserSubscriptionID)
	require.InDelta(t, 0, settlements[1].ActionDeltaValue, 1e-9)
	require.InDelta(t, 160, settlements[1].AfterSettlementValue, 1e-9)
	require.InDelta(t, 40, settlements[1].WriteoffValue, 1e-9)
	require.NotEqual(t, active.ID, current.ID)
}

func TestCreateOrderZeroDeltaUpgradeCountsTowardPurchaseLimit(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, &paymentDirectSettingRepoStub{
		values: map[string]string{service.SettingPaymentEnabled: "true"},
	}, nil)
	userRepo := repository.NewUserRepository(h.client, h.db)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, userRepo, nil, nil)

	user := h.createUser(t, "direct-upgrade-limit@test.com")
	group := h.createGroup(t, "direct-upgrade-limit-group")
	groupID := group.ID
	monthly := 100.0
	basePlan := h.createPlan(t, "Direct Limit Starter", 100, 30, "day", &groupID, nil, nil, &monthly)
	targetPlan := h.createPlan(t, "Direct Limit Pro", 160, 30, "day", &groupID, nil, nil, &monthly)
	targetPlan, err := h.client.SubscriptionPlan.UpdateOneID(targetPlan.ID).
		SetPurchaseLimitPerUser(1).
		Save(h.ctx)
	require.NoError(t, err)

	_ = seedRenewedSettlementHead(t, h, user.ID, basePlan.ID, basePlan.Name, basePlan.Price)

	resp, err := paymentSvc.CreateOrder(h.ctx, service.CreateOrderRequest{
		UserID:      user.ID,
		Amount:      0,
		PaymentType: payment.TypeAlipay,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      targetPlan.ID,
		ClientIP:    "127.0.0.1",
		SrcHost:     "example.com",
	})
	require.NoError(t, err)
	require.Equal(t, payment.CreatePaymentResultCompletedDirectly, resp.ResultType)

	preview, err := paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, targetPlan.ID, payment.DefaultPaymentCurrency)
	require.NoError(t, err)
	require.Equal(t, "unavailable", preview.Action)
	require.Equal(t, "purchase_limit_reached", preview.BlockedReason)
}

func TestCreateOrderZeroDeltaUpgradeAfterAdminRenewCompletesDirectly(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	configSvc := service.NewPaymentConfigService(h.client, &paymentDirectSettingRepoStub{
		values: map[string]string{service.SettingPaymentEnabled: "true"},
	}, nil)
	userRepo := repository.NewUserRepository(h.client, h.db)
	paymentSvc := service.NewPaymentService(h.client, nil, nil, nil, h.svc, configSvc, userRepo, nil, nil)

	operator := h.createUser(t, "direct-admin-renew-operator@test.com")
	user := h.createUser(t, "direct-admin-renew-user@test.com")
	group := h.createGroup(t, "direct-admin-renew-group")
	groupID := group.ID
	monthly := 100.0
	basePlan := h.createPlan(t, "Direct Admin Base", 1, 30, "day", &groupID, nil, nil, &monthly)
	targetPlan := h.createPlan(t, "Direct Admin Pro", 2, 30, "day", &groupID, nil, nil, &monthly)
	order := createPaidSettlementSubscriptionOrder(t, h, user.ID, user.Email, basePlan.ID, basePlan.Name, basePlan.Price)

	require.NoError(t, paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID))
	_, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID:     user.ID,
		PlanID:     basePlan.ID,
		AssignedBy: operator.ID,
		Notes:      "admin renew same plan",
	})
	require.NoError(t, err)
	require.True(t, reused)

	preview, err := paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, targetPlan.ID, payment.DefaultPaymentCurrency)
	require.NoError(t, err)
	require.Equal(t, "upgrade", preview.Action)
	require.True(t, preview.CanCompleteDirectly)
	require.InDelta(t, 0, preview.OrderAmount, 0.01)

	resp, err := paymentSvc.CreateOrder(h.ctx, service.CreateOrderRequest{
		UserID:      user.ID,
		Amount:      0,
		PaymentType: payment.TypeAlipay,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      targetPlan.ID,
		ClientIP:    "127.0.0.1",
		SrcHost:     "example.com",
	})
	require.NoError(t, err)
	require.Equal(t, payment.CreatePaymentResultCompletedDirectly, resp.ResultType)

	settlements, err := h.client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(user.ID)).
		Order(dbent.Asc(subscriptionsettlementorder.FieldID)).
		All(h.ctx)
	require.NoError(t, err)
	require.Len(t, settlements, 3)
	require.Nil(t, settlements[0].PrevSettlementID)
	require.NotNil(t, settlements[1].PrevSettlementID)
	require.Equal(t, settlements[0].ID, *settlements[1].PrevSettlementID)
	require.NotNil(t, settlements[2].PrevSettlementID)
	require.Equal(t, settlements[1].ID, *settlements[2].PrevSettlementID)
	require.Equal(t, domain.SettlementActionUpgrade, settlements[2].ActionType)
	require.Equal(t, domain.SettlementStatusEffective, settlements[2].Status)
}

func seedRenewedSettlementHead(t *testing.T, h *subscriptionOpsHarness, userID, planID int64, planName string, planPrice float64) *service.UserSubscription {
	t.Helper()
	anchor := time.Now()
	now := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, anchor.Location())
	expiresAt := now.Add(60 * 24 * time.Hour)
	monthly := 100.0
	sub, err := h.client.UserSubscription.Create().
		SetUserID(userID).
		SetPlanID(planID).
		SetPlanNameSnapshot(planName).
		SetPlanPriceSnapshot(planPrice).
		SetStartsAt(now).
		SetExpiresAt(expiresAt).
		SetStatus(service.SubscriptionStatusActive).
		SetMonthlyQuotaKnives(monthly).
		SetMonthlyWindowStart(now).
		Save(h.ctx)
	require.NoError(t, err)

	_, err = h.client.SubscriptionSettlementOrder.Create().
		SetUserID(userID).
		SetOperatorUserID(userID).
		SetActionType(domain.SettlementActionRenew).
		SetActionSource(domain.SettlementActionSourceUserPurchase).
		SetStatus(domain.SettlementStatusEffective).
		SetTriggerRefType(domain.SettlementTriggerRefPaymentOrder).
		SetTriggerRefID(99).
		SetCarryInResidualValue(100).
		SetActionDeltaValue(100).
		SetAfterSettlementValue(200).
		SetWriteoffValue(0).
		SetAfterUserSubscriptionID(sub.ID).
		SetAfterPlanID(planID).
		SetAfterPlanNameSnapshot(planName).
		SetAfterPlanPriceSnapshot(planPrice).
		SetAfterValidityDaysSnapshot(30).
		SetAfterValidityUnitSnapshot("day").
		SetAfterStartsAt(now).
		SetAfterExpiresAt(expiresAt).
		SetAfterMonthlyQuotaKnivesSnapshot(monthly).
		SetAfterSubscriptionStatus(service.SubscriptionStatusActive).
		SetEffectiveAt(now).
		Save(h.ctx)
	require.NoError(t, err)

	return &service.UserSubscription{
		ID:                 sub.ID,
		UserID:             userID,
		PlanID:             &planID,
		PlanNameSnapshot:   &planName,
		PlanPriceSnapshot:  &planPrice,
		StartsAt:           now,
		ExpiresAt:          expiresAt,
		Status:             service.SubscriptionStatusActive,
		MonthlyQuotaKnives: &monthly,
		MonthlyWindowStart: &now,
		MonthlyUsedKnives:  0,
	}
}

type paymentDirectSettingRepoStub struct {
	values map[string]string
}

func (s *paymentDirectSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, nil
}

func (s *paymentDirectSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *paymentDirectSettingRepoStub) Set(context.Context, string, string) error {
	return nil
}

func (s *paymentDirectSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = s.values[key]
	}
	return out, nil
}

func (s *paymentDirectSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (s *paymentDirectSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *paymentDirectSettingRepoStub) Delete(context.Context, string) error {
	return nil
}
