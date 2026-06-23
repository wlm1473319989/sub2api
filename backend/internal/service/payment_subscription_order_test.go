package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
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
		ID:             group.ID,
		Name:           group.Name,
		Status:         group.Status,
		Platform:       group.Platform,
		RateMultiplier: group.RateMultiplier,
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
		return nil, ErrGroupNotFound
	}
	cp := *group
	return &cp, nil
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
	_ = daily
	_ = weekly
	_ = monthly
	builder := h.client.Group.Create().
		SetName(name).
		SetStatus(StatusActive)
	group, err := builder.Save(h.ctx)
	require.NoError(t, err)
	h.groupRepo.byID[group.ID] = groupToService(group)
	return group
}

func (h *paymentSubscriptionHarness) createPlan(t *testing.T, name string, price float64, validityDays int, validityUnit string, groupID *int64, daily, weekly, monthly *float64) *dbent.SubscriptionPlan {
	t.Helper()
	_ = groupID
	builder := h.client.SubscriptionPlan.Create().
		SetName(name).
		SetDescription(name).
		SetPrice(price).
		SetValidityDays(validityDays).
		SetValidityUnit(validityUnit).
		SetFeatures("").
		SetProductName(name).
		SetForSale(true)
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
	_ = group
	sub := &UserSubscription{
		UserID:     user.ID,
		StartsAt:   startsAt,
		ExpiresAt:  expiresAt,
		Status:     SubscriptionStatusActive,
		AssignedAt: startsAt,
		Notes:      "legacy",
		CreatedAt:  startsAt,
		UpdatedAt:  startsAt,
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

func TestPrepareSubscriptionOrderDecision_UsesSettlementHeadWhenPresent(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	h.paymentSvc.settlementSvc = NewSettlementService(h.client)
	user := h.createUser(t, "settlement-upgrade@example.com")
	group := h.createGroup(t, "settlement-upgrade-group", nil, nil, floatPtr(100))
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 100, 30, "day", &groupID, nil, nil, floatPtr(100))
	targetPlan := h.createPlan(t, "Pro", 160, 30, "day", &groupID, nil, nil, floatPtr(200))

	sub, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)
	stored := h.userSubRepo.byID[sub.ID]
	require.NotNil(t, stored)
	stored.PlanPriceSnapshot = floatPtr(999)

	settlementHarness := &settlementServiceHarness{
		ctx:    h.ctx,
		client: h.client,
		svc:    h.paymentSvc.settlementSvc,
	}
	_ = settlementHarness.createSettlementHead(t, user, plan, domain.SettlementStatusEffective, domain.SubscriptionStatusActive, time.Now().Add(24*time.Hour))

	decision, err := h.paymentSvc.prepareSubscriptionOrderDecision(h.ctx, user.ID, targetPlan.ID)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionUpgrade, decision.Action)
	require.NotNil(t, decision.UpgradeBreakdown)
	require.Greater(t, decision.OrderAmount, 0.0)
}

func TestPrepareSubscriptionOrderDecision_LegacyActiveWithoutPlanFallsBackToUpgrade(t *testing.T) {
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
	require.Equal(t, subscriptionActionUpgrade, decision.Action)
	require.Nil(t, decision.UpgradeBreakdown)
	require.InDelta(t, plan.Price, decision.OrderAmount, 1e-9)
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

func TestPreviewSubscriptionOrder_Upgrade(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "preview-upgrade@example.com")
	group := h.createGroup(t, "preview-upgrade-group", nil, nil, floatPtr(100))
	groupID := group.ID
	basePlan := h.createPlan(t, "Starter", 100, 30, "day", &groupID, nil, nil, floatPtr(100))
	targetPlan := h.createPlan(t, "Pro", 160, 30, "day", &groupID, nil, nil, floatPtr(200))

	sub, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: basePlan})
	require.NoError(t, err)
	monthlyStart := startOfDay(time.Now().Add(-20 * 24 * time.Hour))
	stored := h.userSubRepo.byID[sub.ID]
	require.NotNil(t, stored)
	stored.StartsAt = monthlyStart
	stored.ExpiresAt = monthlyStart.Add(30 * 24 * time.Hour)
	stored.MonthlyWindowStart = &monthlyStart
	stored.MonthlyUsedKnives = 40

	preview, err := h.paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, targetPlan.ID)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionUpgrade, preview.Action)
	require.InDelta(t, 100, preview.OrderAmount, 1e-9)
	require.NotNil(t, preview.CurrentPlan)
	require.Equal(t, basePlan.Name, preview.CurrentPlan.Name)
	require.NotNil(t, preview.TargetPlan)
	require.Equal(t, targetPlan.Name, preview.TargetPlan.Name)
	require.NotNil(t, preview.UpgradeBreakdown)
	require.InDelta(t, 100, preview.UpgradeBreakdown.UpgradeDelta, 1e-9)
}

func TestPreviewSubscriptionOrder_LowerPriceReturnsUnavailable(t *testing.T) {
	h := newPaymentSubscriptionHarness(t)
	user := h.createUser(t, "preview-unavailable@example.com")
	group := h.createGroup(t, "preview-unavailable-group", nil, nil, floatPtr(100))
	groupID := group.ID
	activePlan := h.createPlan(t, "Starter", 50, 30, "day", &groupID, nil, nil, floatPtr(100))
	lowerPlan := h.createPlan(t, "Lower", 30, 30, "day", &groupID, nil, nil, floatPtr(100))

	_, err := h.subscriptionSvc.PurchaseNewPlan(h.ctx, &PurchaseNewPlanInput{UserID: user.ID, Plan: activePlan})
	require.NoError(t, err)

	preview, err := h.paymentSvc.PreviewSubscriptionOrder(h.ctx, user.ID, lowerPlan.ID)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionUnavailable, preview.Action)
	require.Equal(t, subscriptionPreviewBlockedReasonDowngradeOrSwitch, preview.BlockedReason)
	require.Equal(t, 0.0, preview.OrderAmount)
	require.NotNil(t, preview.CurrentPlan)
	require.Equal(t, activePlan.Name, preview.CurrentPlan.Name)
	require.NotNil(t, preview.TargetPlan)
	require.Equal(t, lowerPlan.Name, preview.TargetPlan.Name)
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
