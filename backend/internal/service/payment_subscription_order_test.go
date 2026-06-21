package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type paymentSubscriptionHarness struct {
	ctx             context.Context
	client          *dbent.Client
	groupRepo       *paymentSubscriptionGroupRepoStub
	userSubRepo     *subscriptionUserSubRepoStub
	subscriptionSvc *SubscriptionService
	paymentSvc      *PaymentService
}

type paymentSubscriptionGroupRepoStub struct {
	groupRepoNoop
	byID map[int64]*Group
}

func groupToService(group *dbent.Group) *Group {
	if group == nil {
		return nil
	}
	return &Group{
		ID:               group.ID,
		Name:             group.Name,
		Status:           group.Status,
		Platform:         group.Platform,
		RateMultiplier:   group.RateMultiplier,
		SubscriptionType: group.SubscriptionType,
		DailyLimitUSD:    copyFloat64Pointer(group.DailyLimitUsd),
		WeeklyLimitUSD:   copyFloat64Pointer(group.WeeklyLimitUsd),
		MonthlyLimitUSD:  copyFloat64Pointer(group.MonthlyLimitUsd),
	}
}

func userToService(user *dbent.User) *User {
	if user == nil {
		return nil
	}
	return &User{
		ID:       user.ID,
		Email:    user.Email,
		Username: user.Username,
		Status:   user.Status,
		Role:     user.Role,
	}
}

func newPaymentSubscriptionGroupRepoStub() *paymentSubscriptionGroupRepoStub {
	return &paymentSubscriptionGroupRepoStub{byID: make(map[int64]*Group)}
}

func (s *paymentSubscriptionGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	group := s.byID[id]
	if group == nil {
		return nil, ErrGroupNotSubscriptionType
	}
	cp := *group
	return &cp, nil
}

func (s *subscriptionUserSubRepoStub) GetActiveByUserID(_ context.Context, userID int64) (*UserSubscription, error) {
	var active *UserSubscription
	for _, sub := range s.byID {
		if sub == nil || sub.UserID != userID || sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(time.Now()) {
			continue
		}
		if active != nil {
			return nil, ErrMultipleActiveSubscriptions
		}
		cp := *sub
		active = &cp
	}
	if active == nil {
		return nil, ErrSubscriptionNotFound
	}
	return active, nil
}

func (s *subscriptionUserSubRepoStub) HasActiveByUserID(_ context.Context, userID int64) (bool, error) {
	sub, err := s.GetActiveByUserID(context.Background(), userID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return sub != nil, nil
}

func newPaymentSubscriptionHarness(t *testing.T) *paymentSubscriptionHarness {
	t.Helper()

	client := newPaymentConfigServiceTestClient(t)
	groupRepo := newPaymentSubscriptionGroupRepoStub()
	userSubRepo := newSubscriptionUserSubRepoStub()
	subscriptionSvc := NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	configSvc := NewPaymentConfigService(client, nil, nil)
	paymentSvc := &PaymentService{
		entClient:       client,
		configService:   configSvc,
		groupRepo:       groupRepo,
		subscriptionSvc: subscriptionSvc,
	}

	return &paymentSubscriptionHarness{
		ctx:             context.Background(),
		client:          client,
		groupRepo:       groupRepo,
		userSubRepo:     userSubRepo,
		subscriptionSvc: subscriptionSvc,
		paymentSvc:      paymentSvc,
	}
}

func (h *paymentSubscriptionHarness) createUser(t *testing.T, email string) *dbent.User {
	t.Helper()
	user, err := h.client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetStatus(StatusActive).
		SetRole(RoleUser).
		SetUsername(strings.TrimSuffix(email, "@example.com")).
		Save(h.ctx)
	require.NoError(t, err)
	return user
}

func (h *paymentSubscriptionHarness) createGroup(t *testing.T, name string, daily, weekly, monthly *float64) *dbent.Group {
	t.Helper()
	builder := h.client.Group.Create().
		SetName(name).
		SetStatus(StatusActive).
		SetSubscriptionType(SubscriptionTypeSubscription)
	if daily != nil {
		builder.SetDailyLimitUsd(*daily)
	}
	if weekly != nil {
		builder.SetWeeklyLimitUsd(*weekly)
	}
	if monthly != nil {
		builder.SetMonthlyLimitUsd(*monthly)
	}
	group, err := builder.Save(h.ctx)
	require.NoError(t, err)
	h.groupRepo.byID[group.ID] = groupToService(group)
	return group
}

func (h *paymentSubscriptionHarness) createPlan(t *testing.T, name string, price float64, validityDays int, validityUnit string, groupID *int64, daily, weekly, monthly *float64) *dbent.SubscriptionPlan {
	t.Helper()
	builder := h.client.SubscriptionPlan.Create().
		SetName(name).
		SetDescription(name).
		SetPrice(price).
		SetValidityDays(validityDays).
		SetValidityUnit(validityUnit).
		SetFeatures("").
		SetProductName(name).
		SetForSale(true)
	if groupID != nil {
		builder.SetGroupID(*groupID)
	}
	if daily != nil {
		builder.SetDailyQuotaKnives(*daily)
	}
	if weekly != nil {
		builder.SetWeeklyQuotaKnives(*weekly)
	}
	if monthly != nil {
		builder.SetMonthlyQuotaKnives(*monthly)
	}
	plan, err := builder.Save(h.ctx)
	require.NoError(t, err)
	return plan
}

func (h *paymentSubscriptionHarness) seedLegacyActiveSubscription(t *testing.T, user *dbent.User, group *dbent.Group, startsAt, expiresAt time.Time) *UserSubscription {
	t.Helper()
	sub := &UserSubscription{
		UserID:     user.ID,
		GroupID:    group.ID,
		StartsAt:   startsAt,
		ExpiresAt:  expiresAt,
		Status:     SubscriptionStatusActive,
		AssignedAt: startsAt,
		Notes:      "legacy",
		CreatedAt:  startsAt,
		UpdatedAt:  startsAt,
		Group:      groupToService(group),
		User:       userToService(user),
	}
	require.NoError(t, h.subscriptionSvc.userSubRepo.Create(h.ctx, sub))
	return sub
}

func (h *paymentSubscriptionHarness) createSubscriptionOrder(t *testing.T, user *dbent.User, plan *dbent.SubscriptionPlan, action string, status string, createdAt time.Time) *dbent.PaymentOrder {
	t.Helper()
	orderNo := fmt.Sprintf("sub2_%d", createdAt.UnixNano())
	builder := h.client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(plan.Price).
		SetPayAmount(plan.Price).
		SetFeeRate(0).
		SetRechargeCode(orderNo).
		SetOutTradeNo(orderNo).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetStatus(status).
		SetExpiresAt(createdAt.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetPlanID(plan.ID).
		SetSubscriptionAction(action).
		SetSubscriptionPlanNameSnapshot(plan.Name).
		SetSubscriptionPlanPriceSnapshot(plan.Price).
		SetSubscriptionValidityDaysSnapshot(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)).
		SetCreatedAt(createdAt).
		SetUpdatedAt(createdAt)
	if plan.GroupID != nil {
		builder.SetSubscriptionGroupID(*plan.GroupID)
	}
	if plan.DailyQuotaKnives != nil {
		builder.SetSubscriptionDailyQuotaKnivesSnapshot(*plan.DailyQuotaKnives)
	}
	if plan.WeeklyQuotaKnives != nil {
		builder.SetSubscriptionWeeklyQuotaKnivesSnapshot(*plan.WeeklyQuotaKnives)
	}
	if plan.MonthlyQuotaKnives != nil {
		builder.SetSubscriptionMonthlyQuotaKnivesSnapshot(*plan.MonthlyQuotaKnives)
	}
	order, err := builder.Save(h.ctx)
	require.NoError(t, err)
	return order
}

func TestPrepareSubscriptionOrderDecision_NoActivePurchase(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "purchase@example.com")
	group := h.createGroup(t, "purchase-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, floatPtr(100))

	decision, err := h.paymentSvc.prepareSubscriptionOrderDecision(h.ctx, user.ID, plan.ID)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionPurchase, decision.Action)
	require.Equal(t, plan.ID, decision.Plan.ID)
	require.Nil(t, decision.ActiveSubscription)
	require.InDelta(t, plan.Price, decision.OrderAmount, 1e-9)
}

func TestPrepareSubscriptionOrderDecision_RenewSamePlan(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "renew@example.com")
	group := h.createGroup(t, "renew-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, floatPtr(100))

	_, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	decision, err := h.paymentSvc.prepareSubscriptionOrderDecision(h.ctx, user.ID, plan.ID)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionRenew, decision.Action)
	require.NotNil(t, decision.ActiveSubscription)
	require.InDelta(t, plan.Price, decision.OrderAmount, 1e-9)
}

func TestPrepareSubscriptionOrderDecision_UpgradeUsesResidualDelta(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "upgrade@example.com")
	group := h.createGroup(t, "upgrade-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 100, 30, "day", &groupID, nil, nil, floatPtr(100))
	targetPlan := h.createPlan(t, "Pro", 160, 30, "day", &groupID, nil, nil, floatPtr(200))

	sub, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)
	monthlyStart := startOfDay(time.Now().Add(-20 * 24 * time.Hour))
	stored := h.userSubRepo.byID[sub.ID]
	require.NotNil(t, stored)
	stored.StartsAt = monthlyStart
	stored.ExpiresAt = monthlyStart.Add(30 * 24 * time.Hour)
	stored.MonthlyWindowStart = &monthlyStart
	stored.MonthlyUsedKnives = 40

	decision, err := h.paymentSvc.prepareSubscriptionOrderDecision(h.ctx, user.ID, targetPlan.ID)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionUpgrade, decision.Action)
	require.NotNil(t, decision.UpgradeBreakdown)
	require.InDelta(t, 100, decision.OrderAmount, 1e-9)
}

func TestPrepareSubscriptionOrderDecision_LegacyActiveUsesLatestOrderForRenew(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "legacy-renew@example.com")
	group := h.createGroup(t, "legacy-group", floatPtr(10), nil, nil)
	groupID := group.ID
	plan := h.createPlan(t, "Legacy Starter", 29.99, 30, "day", &groupID, floatPtr(10), nil, nil)

	now := time.Now()
	legacySub := h.seedLegacyActiveSubscription(t, user, group, now.Add(-5*24*time.Hour), now.Add(10*24*time.Hour))
	require.Nil(t, legacySub.PlanID)

	_ = h.createSubscriptionOrder(t, user, plan, subscriptionActionPurchase, OrderStatusCompleted, now.Add(-24*time.Hour))

	decision, err := h.paymentSvc.prepareSubscriptionOrderDecision(h.ctx, user.ID, plan.ID)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionRenew, decision.Action)
}

func TestPrepareSubscriptionOrderDecision_LowerPriceRejected(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "invalid-upgrade@example.com")
	group := h.createGroup(t, "invalid-upgrade-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 50, 30, "day", &groupID, nil, nil, floatPtr(100))
	lowerPlan := h.createPlan(t, "Lower", 30, 30, "day", &groupID, nil, nil, floatPtr(100))

	_, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	_, err = h.paymentSvc.prepareSubscriptionOrderDecision(h.ctx, user.ID, lowerPlan.ID)
	require.ErrorIs(t, err, ErrSubscriptionOrderActionInvalid)
}

func TestCreateOrderInTx_WritesSubscriptionActionSnapshot(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "snapshot-order@example.com")
	group := h.createGroup(t, "snapshot-order-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Snapshot", 88, 1, "month", &groupID, floatPtr(10), nil, floatPtr(100))

	decision := &subscriptionOrderDecision{
		Plan:        plan,
		Action:      subscriptionActionPurchase,
		OrderAmount: plan.Price,
	}
	order, err := h.paymentSvc.createOrderInTx(
		h.ctx,
		CreateOrderRequest{
			UserID:      user.ID,
			PaymentType: payment.TypeAlipay,
			OrderType:   payment.OrderTypeSubscription,
			ClientIP:    "127.0.0.1",
			SrcHost:     "example.com",
		},
		&User{ID: user.ID, Email: user.Email, Username: user.Username},
		plan,
		decision,
		&PaymentConfig{MaxPendingOrders: 3, OrderTimeoutMin: 30},
		plan.Price,
		plan.Price,
		0,
		plan.Price,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, order.PlanID)
	require.Equal(t, plan.ID, *order.PlanID)
	require.Equal(t, subscriptionActionPurchase, derefString(order.SubscriptionAction))
	require.Equal(t, plan.Name, derefString(order.SubscriptionPlanNameSnapshot))
	require.NotNil(t, order.SubscriptionPlanPriceSnapshot)
	require.Equal(t, plan.Price, *order.SubscriptionPlanPriceSnapshot)
	require.NotNil(t, order.SubscriptionValidityDaysSnapshot)
	require.Equal(t, 30, *order.SubscriptionValidityDaysSnapshot)
	require.NotNil(t, order.SubscriptionDailyQuotaKnivesSnapshot)
	require.Equal(t, 10.0, *order.SubscriptionDailyQuotaKnivesSnapshot)
	require.NotNil(t, order.SubscriptionMonthlyQuotaKnivesSnapshot)
	require.Equal(t, 100.0, *order.SubscriptionMonthlyQuotaKnivesSnapshot)
	require.Nil(t, order.SubscriptionGroupID)
	require.Nil(t, order.SubscriptionDays)
}

func TestExecuteSubscriptionFulfillment_PurchaseAction(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "fulfill-purchase@example.com")
	group := h.createGroup(t, "fulfill-purchase-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, floatPtr(100))
	order := h.createSubscriptionOrder(t, user, plan, subscriptionActionPurchase, OrderStatusPaid, time.Now())

	err := h.paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID)
	require.NoError(t, err)

	reloaded, err := h.client.PaymentOrder.Get(h.ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)

	active, err := h.subscriptionSvc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, active.PlanID)
	require.Equal(t, plan.ID, *active.PlanID)
}

func TestExecuteSubscriptionFulfillment_RenewAction(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "fulfill-renew@example.com")
	group := h.createGroup(t, "fulfill-renew-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, floatPtr(100))
	active, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)
	beforeExpiry := active.ExpiresAt
	order := h.createSubscriptionOrder(t, user, plan, subscriptionActionRenew, OrderStatusPaid, time.Now())

	err = h.paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID)
	require.NoError(t, err)

	renewed, err := h.subscriptionSvc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.True(t, renewed.ExpiresAt.After(beforeExpiry))
}

func TestExecuteSubscriptionFulfillment_UpgradeAction(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "fulfill-upgrade@example.com")
	group := h.createGroup(t, "fulfill-upgrade-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, floatPtr(100))
	targetPlan := h.createPlan(t, "Pro", 49.99, 30, "day", &groupID, nil, nil, floatPtr(200))
	active, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)
	order := h.createSubscriptionOrder(t, user, targetPlan, subscriptionActionUpgrade, OrderStatusPaid, time.Now())

	err = h.paymentSvc.ExecuteSubscriptionFulfillment(h.ctx, order.ID)
	require.NoError(t, err)

	current, err := h.subscriptionSvc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, current.PlanID)
	require.Equal(t, targetPlan.ID, *current.PlanID)

	previous, err := h.subscriptionSvc.GetByID(h.ctx, active.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusSuperseded, previous.Status)
	require.NotNil(t, previous.SupersededByID)
	require.Equal(t, current.ID, *previous.SupersededByID)
}
