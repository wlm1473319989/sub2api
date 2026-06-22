package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type subscriptionOpsHarness struct {
	ctx    context.Context
	db     *sql.DB
	client *dbent.Client
	svc    *service.SubscriptionService
}

func newSubscriptionOpsHarness(t *testing.T) *subscriptionOpsHarness {
	t.Helper()

	dbName := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()),
	)
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	groupRepo := repository.NewGroupRepository(client, db)
	userSubRepo := repository.NewUserSubscriptionRepository(client)
	svc := service.NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)

	return &subscriptionOpsHarness{
		ctx:    context.Background(),
		db:     db,
		client: client,
		svc:    svc,
	}
}

func (h *subscriptionOpsHarness) createUser(t *testing.T, email string) *dbent.User {
	t.Helper()
	user, err := h.client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(h.ctx)
	require.NoError(t, err)
	return user
}

func (h *subscriptionOpsHarness) createGroup(t *testing.T, name string) *dbent.Group {
	t.Helper()
	group, err := h.client.Group.Create().
		SetName(name).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(h.ctx)
	require.NoError(t, err)
	return group
}

func (h *subscriptionOpsHarness) createPlan(t *testing.T, name string, price float64, validityDays int, validityUnit string, groupID *int64, daily, weekly, monthly *float64) *dbent.SubscriptionPlan {
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

func (h *subscriptionOpsHarness) createSubscriptionOrder(t *testing.T, userID int64, planID *int64, groupID *int64, createdAt time.Time) *dbent.PaymentOrder {
	t.Helper()
	outTradeNo := fmt.Sprintf("sub2_%d", createdAt.UnixNano())
	builder := h.client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail("user@example.com").
		SetUserName("user").
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode(outTradeNo).
		SetOutTradeNo(outTradeNo).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetStatus(service.OrderStatusCompleted).
		SetExpiresAt(createdAt.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetCreatedAt(createdAt).
		SetUpdatedAt(createdAt)
	if planID != nil {
		builder.SetPlanID(*planID)
	}
	if groupID != nil {
		builder.SetSubscriptionGroupID(*groupID)
	}
	order, err := builder.Save(h.ctx)
	require.NoError(t, err)
	return order
}

func TestPurchaseNewPlanCreatesSnapshotSubscription(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "purchase@test.com")
	group := h.createGroup(t, "purchase-group")
	daily := 10.0
	monthly := 300.0
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "days", &groupID, &daily, nil, &monthly)

	sub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   plan,
		Notes:  "purchase",
	})
	require.NoError(t, err)
	require.NotNil(t, sub.PlanID)
	require.Equal(t, plan.ID, *sub.PlanID)
	require.Equal(t, group.ID, sub.GroupID)
	require.Equal(t, service.SubscriptionStatusActive, sub.Status)
	require.NotNil(t, sub.PlanNameSnapshot)
	require.Equal(t, plan.Name, *sub.PlanNameSnapshot)
	require.NotNil(t, sub.PlanPriceSnapshot)
	require.Equal(t, plan.Price, *sub.PlanPriceSnapshot)
	require.NotNil(t, sub.DailyQuotaKnives)
	require.Equal(t, daily, *sub.DailyQuotaKnives)
	require.NotNil(t, sub.MonthlyQuotaKnives)
	require.Equal(t, monthly, *sub.MonthlyQuotaKnives)
}

func TestPurchaseNewPlanRejectsExistingActiveSubscription(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "active-exists@test.com")
	group := h.createGroup(t, "group-a")
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, nil)

	_, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	_, err = h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.ErrorIs(t, err, service.ErrActiveSubscriptionExists)
}

func TestRenewActivePlanExtendsWithoutResettingUsage(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "renew@test.com")
	group := h.createGroup(t, "renew-group")
	groupID := group.ID
	daily := 10.0
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, &daily, nil, nil)

	sub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan, Notes: "seed"})
	require.NoError(t, err)

	dailyStart := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	_, err = h.client.UserSubscription.UpdateOneID(sub.ID).
		SetDailyWindowStart(dailyStart).
		SetDailyUsageUsd(3.5).
		SetDailyUsedKnives(2.25).
		SetNotes("seed").
		Save(h.ctx)
	require.NoError(t, err)

	before, err := h.svc.GetByID(h.ctx, sub.ID)
	require.NoError(t, err)

	renewed, err := h.svc.RenewActivePlan(h.ctx, &service.RenewActivePlanInput{
		UserID: user.ID,
		Plan:   plan,
		Notes:  "renew",
	})
	require.NoError(t, err)
	require.Equal(t, before.DailyUsageUSD, renewed.DailyUsageUSD)
	require.Equal(t, before.DailyUsedKnives, renewed.DailyUsedKnives)
	require.NotNil(t, renewed.DailyWindowStart)
	require.Equal(t, *before.DailyWindowStart, *renewed.DailyWindowStart)
	require.True(t, renewed.ExpiresAt.After(before.ExpiresAt))
	require.Contains(t, renewed.Notes, "seed")
	require.Contains(t, renewed.Notes, "renew")
}

func TestRenewActivePlanRejectsMismatchedPlan(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "renew-mismatch@test.com")
	group := h.createGroup(t, "renew-mismatch-group")
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, nil)
	otherPlan := h.createPlan(t, "Pro", 39.99, 30, "day", &groupID, nil, nil, nil)

	_, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	_, err = h.svc.RenewActivePlan(h.ctx, &service.RenewActivePlanInput{UserID: user.ID, Plan: otherPlan})
	require.ErrorIs(t, err, service.ErrRenewPlanMismatch)
}

func TestUpgradeActivePlanSupersedesActiveSubscription(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "upgrade@test.com")
	group := h.createGroup(t, "upgrade-group")
	groupID := group.ID
	daily := 10.0
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, &daily, nil, nil)
	targetDaily := 30.0
	targetPlan := h.createPlan(t, "Pro", 49.99, 30, "month", nil, &targetDaily, nil, nil)

	active, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	result, err := h.svc.UpgradeActivePlan(h.ctx, &service.UpgradeActivePlanInput{
		UserID:     user.ID,
		TargetPlan: targetPlan,
		Notes:      "upgrade",
	})
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusSuperseded, result.Previous.Status)
	require.NotNil(t, result.Previous.SupersededByID)
	require.Equal(t, result.Current.ID, *result.Previous.SupersededByID)
	require.Equal(t, service.SubscriptionStatusActive, result.Current.Status)
	require.NotNil(t, result.Current.PlanID)
	require.Equal(t, targetPlan.ID, *result.Current.PlanID)
	require.Equal(t, active.GroupID, result.Current.GroupID, "nil-group target should reuse active group for legacy persistence")
	require.NotNil(t, result.Current.DailyQuotaKnives)
	require.Equal(t, targetDaily, *result.Current.DailyQuotaKnives)

	current, err := h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, result.Current.ID, current.ID)
}

func TestUpgradeActivePlanRejectsNonHigherPrice(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "upgrade-price@test.com")
	group := h.createGroup(t, "upgrade-price-group")
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, nil)
	targetPlan := h.createPlan(t, "Cheaper", 9.99, 30, "day", &groupID, nil, nil, nil)

	_, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	_, err = h.svc.UpgradeActivePlan(h.ctx, &service.UpgradeActivePlanInput{UserID: user.ID, TargetPlan: targetPlan})
	require.ErrorIs(t, err, service.ErrUpgradePlanPriceInvalid)
}

func TestRefundActivePlanRequiresLatestOrder(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "refund-old@test.com")
	group := h.createGroup(t, "refund-group")
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, nil)

	sub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	older := h.createSubscriptionOrder(t, user.ID, sub.PlanID, &groupID, time.Now().Add(-2*time.Hour))
	_ = h.createSubscriptionOrder(t, user.ID, sub.PlanID, &groupID, time.Now().Add(-time.Hour))

	_, err = h.svc.RefundActivePlan(h.ctx, &service.RefundActivePlanInput{UserID: user.ID, OrderID: older.ID})
	require.ErrorIs(t, err, service.ErrRefundOrderNotLatest)
}

func TestRefundActivePlanMarksSubscriptionRefunded(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "refund@test.com")
	group := h.createGroup(t, "refund-group-2")
	groupID := group.ID
	plan := h.createPlan(t, "Starter", 19.99, 30, "day", &groupID, nil, nil, nil)

	sub, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{UserID: user.ID, Plan: plan})
	require.NoError(t, err)

	order := h.createSubscriptionOrder(t, user.ID, sub.PlanID, &groupID, time.Now().Add(-time.Minute))

	result, err := h.svc.RefundActivePlan(h.ctx, &service.RefundActivePlanInput{
		UserID:  user.ID,
		OrderID: order.ID,
		Notes:   "refund",
	})
	require.NoError(t, err)
	require.Equal(t, order.ID, result.OrderID)
	require.Equal(t, service.SubscriptionStatusRefunded, result.Subscription.Status)
	require.Contains(t, result.Subscription.Notes, "refund")

	_, err = h.svc.GetActiveSubscriptionByUser(h.ctx, user.ID)
	require.Error(t, err)
	require.Equal(t, infraerrors.Reason(service.ErrSubscriptionNotFound), infraerrors.Reason(err))
}

func TestAssignUserLevelSubscriptionPurchasesWhenNoActiveSubscription(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "assign-plan-new@test.com")
	group := h.createGroup(t, "assign-plan-group")
	groupID := group.ID
	daily := 12.0
	plan := h.createPlan(t, "Assign Starter", 29.9, 30, "day", &groupID, &daily, nil, nil)

	sub, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID: user.ID,
		PlanID: plan.ID,
		Notes:  "admin assign",
	})
	require.NoError(t, err)
	require.False(t, reused)
	require.NotNil(t, sub.PlanID)
	require.Equal(t, plan.ID, *sub.PlanID)
	require.Equal(t, service.SubscriptionStatusActive, sub.Status)
	require.NotNil(t, sub.PlanNameSnapshot)
	require.Equal(t, plan.Name, *sub.PlanNameSnapshot)
}

func TestAssignUserLevelSubscriptionRenewsMatchingActivePlan(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "assign-plan-renew@test.com")
	group := h.createGroup(t, "assign-plan-renew-group")
	groupID := group.ID
	plan := h.createPlan(t, "Assign Renew", 19.9, 30, "day", &groupID, nil, nil, nil)

	seed, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   plan,
		Notes:  "seed",
	})
	require.NoError(t, err)

	sub, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID: user.ID,
		PlanID: plan.ID,
		Notes:  "renew via assign",
	})
	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, seed.ID, sub.ID)
	require.True(t, sub.ExpiresAt.After(seed.ExpiresAt))
}

func TestAssignUserLevelSubscriptionUpgradesHigherPricedPlan(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "assign-plan-upgrade@test.com")
	group := h.createGroup(t, "assign-plan-upgrade-group")
	groupID := group.ID
	basePlan := h.createPlan(t, "Assign Base", 19.9, 30, "day", &groupID, nil, nil, nil)
	targetPlan := h.createPlan(t, "Assign Pro", 49.9, 30, "day", &groupID, nil, nil, nil)

	active, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   basePlan,
		Notes:  "seed",
	})
	require.NoError(t, err)

	sub, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID: user.ID,
		PlanID: targetPlan.ID,
		Notes:  "upgrade via assign",
	})
	require.NoError(t, err)
	require.False(t, reused)
	require.NotEqual(t, active.ID, sub.ID)
	require.NotNil(t, sub.PlanID)
	require.Equal(t, targetPlan.ID, *sub.PlanID)

	previous, err := h.svc.GetByID(h.ctx, active.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusSuperseded, previous.Status)
}

func TestAssignUserLevelSubscriptionRejectsDifferentPlanWithoutUpgrade(t *testing.T) {
	h := newSubscriptionOpsHarness(t)
	user := h.createUser(t, "assign-plan-invalid@test.com")
	group := h.createGroup(t, "assign-plan-invalid-group")
	groupID := group.ID
	activePlan := h.createPlan(t, "Assign Active", 49.9, 30, "day", &groupID, nil, nil, nil)
	targetPlan := h.createPlan(t, "Assign Lower", 29.9, 30, "day", &groupID, nil, nil, nil)

	_, err := h.svc.PurchaseNewPlan(h.ctx, &service.PurchaseNewPlanInput{
		UserID: user.ID,
		Plan:   activePlan,
		Notes:  "seed",
	})
	require.NoError(t, err)

	sub, reused, err := h.svc.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID: user.ID,
		PlanID: targetPlan.ID,
		Notes:  "invalid via assign",
	})
	require.ErrorIs(t, err, service.ErrSubscriptionPlanActionInvalid)
	require.False(t, reused)
	require.Nil(t, sub)
}
