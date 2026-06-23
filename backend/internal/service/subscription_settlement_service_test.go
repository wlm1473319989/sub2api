package service

import (
	"context"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

type settlementServiceHarness struct {
	ctx    context.Context
	client *dbent.Client
	svc    *SettlementService
}

func newSettlementServiceHarness(t *testing.T) *settlementServiceHarness {
	t.Helper()

	client := newPaymentConfigServiceTestClient(t)
	return &settlementServiceHarness{
		ctx:    context.Background(),
		client: client,
		svc:    NewSettlementService(client),
	}
}

func (h *settlementServiceHarness) createSettlementUser(t *testing.T, email string) *dbent.User {
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

func (h *settlementServiceHarness) createSettlementPlan(t *testing.T, name string, price float64) *dbent.SubscriptionPlan {
	t.Helper()
	plan, err := h.client.SubscriptionPlan.Create().
		SetName(name).
		SetDescription(name).
		SetPrice(price).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetFeatures("").
		SetProductName(name).
		SetForSale(true).
		Save(h.ctx)
	require.NoError(t, err)
	return plan
}

func (h *settlementServiceHarness) createSettlementHead(t *testing.T, user *dbent.User, plan *dbent.SubscriptionPlan, status string, subscriptionStatus string, expiresAt time.Time) *dbent.SubscriptionSettlementOrder {
	t.Helper()
	order, err := h.client.SubscriptionSettlementOrder.Create().
		SetUserID(user.ID).
		SetOperatorUserID(user.ID).
		SetActionType(domain.SettlementActionPurchase).
		SetActionSource(domain.SettlementActionSourceUserPurchase).
		SetStatus(status).
		SetTriggerRefType(domain.SettlementTriggerRefPaymentOrder).
		SetCarryInResidualValue(0).
		SetActionDeltaValue(plan.Price).
		SetAfterSettlementValue(plan.Price).
		SetWriteoffValue(0).
		SetAfterPlanID(plan.ID).
		SetAfterPlanNameSnapshot(plan.Name).
		SetAfterPlanPriceSnapshot(plan.Price).
		SetAfterValidityDaysSnapshot(plan.ValidityDays).
		SetAfterValidityUnitSnapshot(plan.ValidityUnit).
		SetAfterStartsAt(expiresAt.Add(-30 * 24 * time.Hour)).
		SetAfterExpiresAt(expiresAt).
		SetAfterSubscriptionStatus(subscriptionStatus).
		Save(h.ctx)
	require.NoError(t, err)
	return order
}

func (h *settlementServiceHarness) createSettlementSubscription(t *testing.T, user *dbent.User, plan *dbent.SubscriptionPlan, startsAt, expiresAt time.Time) *UserSubscription {
	t.Helper()
	planID := plan.ID
	planName := plan.Name
	planPrice := plan.Price
	sub, err := h.client.UserSubscription.Create().
		SetUserID(user.ID).
		SetPlanID(plan.ID).
		SetPlanNameSnapshot(plan.Name).
		SetPlanPriceSnapshot(plan.Price).
		SetStartsAt(startsAt).
		SetExpiresAt(expiresAt).
		SetStatus(domain.SubscriptionStatusActive).
		Save(h.ctx)
	require.NoError(t, err)
	return &UserSubscription{
		ID:                sub.ID,
		UserID:            user.ID,
		PlanID:            &planID,
		PlanNameSnapshot:  &planName,
		PlanPriceSnapshot: &planPrice,
		StartsAt:          startsAt,
		ExpiresAt:         expiresAt,
		Status:            domain.SubscriptionStatusActive,
	}
}

func TestSettlementService_GetEffectiveHead(t *testing.T) {
	h := newSettlementServiceHarness(t)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	user := h.createSettlementUser(t, "settlement-head@example.com")
	plan := h.createSettlementPlan(t, "Starter", 100)

	head, err := h.svc.GetEffectiveHead(h.ctx, user.ID, now)
	require.NoError(t, err)
	require.Nil(t, head)

	closed := h.createSettlementHead(t, user, plan, domain.SettlementStatusClosed, domain.SubscriptionStatusActive, now.Add(24*time.Hour))
	require.Equal(t, domain.SettlementStatusClosed, closed.Status)
	head, err = h.svc.GetEffectiveHead(h.ctx, user.ID, now)
	require.NoError(t, err)
	require.Nil(t, head)

	effective := h.createSettlementHead(t, user, plan, domain.SettlementStatusEffective, domain.SubscriptionStatusActive, now.Add(24*time.Hour))
	head, err = h.svc.GetEffectiveHead(h.ctx, user.ID, now)
	require.NoError(t, err)
	require.NotNil(t, head)
	require.Equal(t, effective.ID, head.ID)
}

func TestSettlementService_DeterminePlanAction(t *testing.T) {
	h := newSettlementServiceHarness(t)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	user := h.createSettlementUser(t, "settlement-action@example.com")
	basePlan := h.createSettlementPlan(t, "Starter", 100)
	samePlan := basePlan
	higherPlan := h.createSettlementPlan(t, "Pro", 160)
	lowerPlan := h.createSettlementPlan(t, "Basic", 50)

	decision, err := h.svc.DeterminePlanAction(nil, basePlan)
	require.NoError(t, err)
	require.Equal(t, domain.SettlementActionPurchase, decision.Action)

	head := h.createSettlementHead(t, user, basePlan, domain.SettlementStatusEffective, domain.SubscriptionStatusActive, now.Add(24*time.Hour))

	decision, err = h.svc.DeterminePlanAction(head, samePlan)
	require.NoError(t, err)
	require.Equal(t, domain.SettlementActionRenew, decision.Action)
	require.NotNil(t, decision.CurrentPlanID)
	require.Equal(t, basePlan.ID, *decision.CurrentPlanID)

	decision, err = h.svc.DeterminePlanAction(head, higherPlan)
	require.NoError(t, err)
	require.Equal(t, domain.SettlementActionUpgrade, decision.Action)

	decision, err = h.svc.DeterminePlanAction(head, lowerPlan)
	require.NoError(t, err)
	require.Equal(t, subscriptionActionUnavailable, decision.Action)
	require.Equal(t, subscriptionPreviewBlockedReasonDowngradeOrSwitch, decision.BlockedReason)
}

func TestSettlementService_DeterminePlanActionRequiresHeadPriceForSwitch(t *testing.T) {
	h := newSettlementServiceHarness(t)
	targetPlan := h.createSettlementPlan(t, "Pro", 160)
	head := &dbent.SubscriptionSettlementOrder{
		AfterPlanID:            nil,
		AfterPlanPriceSnapshot: nil,
	}

	_, err := h.svc.DeterminePlanAction(head, targetPlan)
	require.ErrorIs(t, err, ErrSettlementHeadIncomplete)
}

func TestSettlementService_CreateSettlementOrderClosesPreviousHead(t *testing.T) {
	h := newSettlementServiceHarness(t)
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	user := h.createSettlementUser(t, "settlement-create@example.com")
	plan := h.createSettlementPlan(t, "Starter", 100)
	prev := h.createSettlementHead(t, user, plan, domain.SettlementStatusEffective, domain.SubscriptionStatusActive, now.Add(24*time.Hour))
	afterSub := h.createSettlementSubscription(t, user, plan, now, now.Add(30*24*time.Hour))
	triggerRefID := int64(42)

	created, err := h.svc.CreateSettlementOrder(h.ctx, SettlementOrderInput{
		UserID:                  user.ID,
		OperatorUserID:          user.ID,
		ActionType:              domain.SettlementActionRenew,
		ActionSource:            domain.SettlementActionSourceUserPurchase,
		TriggerRefType:          domain.SettlementTriggerRefPaymentOrder,
		TriggerRefID:            &triggerRefID,
		ActionNote:              "payment order 42",
		CarryInResidualValue:    20,
		ActionDeltaValue:        100,
		AfterSettlementValue:    120,
		AfterUserSubscription:   afterSub,
		AfterPlan:               plan,
		AfterSubscriptionStatus: domain.SubscriptionStatusActive,
		EffectiveAt:             now,
	})
	require.NoError(t, err)
	require.Equal(t, domain.SettlementStatusEffective, created.Status)
	require.NotNil(t, created.PrevSettlementID)
	require.Equal(t, prev.ID, *created.PrevSettlementID)
	require.Equal(t, afterSub.ID, *created.AfterUserSubscriptionID)
	require.Equal(t, plan.ID, *created.AfterPlanID)
	require.InDelta(t, 20, created.CarryInResidualValue, 1e-9)
	require.InDelta(t, 120, created.AfterSettlementValue, 1e-9)

	reloadedPrev, err := h.client.SubscriptionSettlementOrder.Get(h.ctx, prev.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SettlementStatusClosed, reloadedPrev.Status)
	require.NotNil(t, reloadedPrev.ClosedAt)

	head, err := h.svc.GetEffectiveHead(h.ctx, user.ID, now)
	require.NoError(t, err)
	require.NotNil(t, head)
	require.Equal(t, created.ID, head.ID)
}
