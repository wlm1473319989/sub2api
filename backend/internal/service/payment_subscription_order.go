package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	subscriptionActionPurchase = "purchase"
	subscriptionActionRenew    = "renew"
	subscriptionActionUpgrade  = "upgrade"
	subscriptionActionRefund   = "refund"
)

var (
	ErrSubscriptionOrderActionInvalid   = infraerrors.BadRequest("SUBSCRIPTION_ORDER_ACTION_INVALID", "active subscription only supports renewal or upgrade")
	ErrUpgradePaymentNotRequired        = infraerrors.BadRequest("UPGRADE_PAYMENT_NOT_REQUIRED", "upgrade does not require additional payment")
	ErrActiveSubscriptionPlanUnresolved = infraerrors.Conflict("ACTIVE_SUBSCRIPTION_PLAN_UNRESOLVED", "active subscription plan could not be resolved during migration")
)

type subscriptionOrderDecision struct {
	Plan               *dbent.SubscriptionPlan
	ActiveSubscription *UserSubscription
	Action             string
	OrderAmount        float64
	UpgradeBreakdown   *UpgradeResidualBreakdown
}

func (s *PaymentService) prepareSubscriptionOrderDecision(ctx context.Context, userID int64, planID int64) (*subscriptionOrderDecision, error) {
	if planID == 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription order requires a plan")
	}
	plan, err := s.configService.GetPlan(ctx, planID)
	if err != nil || !plan.ForSale {
		return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
	}

	active, err := s.subscriptionSvc.GetActiveSubscriptionByUser(ctx, userID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			return &subscriptionOrderDecision{
				Plan:        plan,
				Action:      subscriptionActionPurchase,
				OrderAmount: plan.Price,
			}, nil
		}
		return nil, err
	}

	currentPlanID, currentPrice, err := s.resolveActiveSubscriptionReference(ctx, active)
	if err != nil {
		return nil, err
	}

	if subscriptionOrderMatchesRenewPlan(active, currentPlanID, plan) {
		return &subscriptionOrderDecision{
			Plan:               plan,
			ActiveSubscription: active,
			Action:             subscriptionActionRenew,
			OrderAmount:        plan.Price,
		}, nil
	}

	resolvedCurrentPrice := 0.0
	if currentPrice != nil {
		resolvedCurrentPrice = *currentPrice
	}
	if resolvedCurrentPrice > 0 && plan.Price <= resolvedCurrentPrice {
		return nil, ErrSubscriptionOrderActionInvalid
	}

	upgradeAmount := plan.Price
	var breakdown *UpgradeResidualBreakdown
	if resolvedCurrentPrice > 0 {
		breakdown, err = s.calculateUpgradeOrderDelta(ctx, active, resolvedCurrentPrice, plan)
		if err != nil {
			return nil, err
		}
		upgradeAmount = breakdown.UpgradeDelta
	}
	if upgradeAmount <= 0 {
		return nil, ErrUpgradePaymentNotRequired
	}

	return &subscriptionOrderDecision{
		Plan:               plan,
		ActiveSubscription: active,
		Action:             subscriptionActionUpgrade,
		OrderAmount:        upgradeAmount,
		UpgradeBreakdown:   breakdown,
	}, nil
}

func subscriptionOrderMatchesRenewPlan(active *UserSubscription, currentPlanID *int64, plan *dbent.SubscriptionPlan) bool {
	if active == nil || plan == nil {
		return false
	}
	if active.PlanID != nil {
		return *active.PlanID == plan.ID
	}
	return currentPlanID != nil && *currentPlanID == plan.ID
}

func (s *PaymentService) resolveActiveSubscriptionReference(ctx context.Context, active *UserSubscription) (*int64, *float64, error) {
	if active == nil {
		return nil, nil, nil
	}
	planID := active.PlanID
	price := active.PlanPriceSnapshot
	if planID != nil && price != nil {
		return planID, price, nil
	}

	if s == nil || s.entClient == nil {
		return planID, price, nil
	}
	query := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(active.UserID),
			paymentorder.OrderTypeEQ(payment.OrderTypeSubscription),
			paymentorder.StatusIn(OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted),
		).
		Order(dbent.Desc(paymentorder.FieldCreatedAt))
	if active.PlanID != nil {
		query = query.Where(paymentorder.PlanIDEQ(*active.PlanID))
	} else {
		return planID, price, nil
	}
	order, err := query.First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return planID, price, nil
		}
		return nil, nil, fmt.Errorf("resolve active subscription order: %w", err)
	}
	if planID == nil {
		planID = order.PlanID
	}
	if price == nil {
		switch {
		case order.SubscriptionPlanPriceSnapshot != nil:
			price = order.SubscriptionPlanPriceSnapshot
		case order.PlanID != nil:
			plan, planErr := s.configService.GetPlan(ctx, *order.PlanID)
			if planErr == nil && plan != nil {
				price = copyFloat64Pointer(&plan.Price)
			}
		}
	}
	return planID, price, nil
}

func (s *PaymentService) calculateUpgradeOrderDelta(ctx context.Context, active *UserSubscription, currentPrice float64, targetPlan *dbent.SubscriptionPlan) (*UpgradeResidualBreakdown, error) {
	_ = ctx
	if active == nil || targetPlan == nil {
		return nil, ErrActiveSubscriptionPlanUnresolved
	}
	if active.DailyQuotaKnives == nil && active.WeeklyQuotaKnives == nil && active.MonthlyQuotaKnives == nil {
		return nil, ErrActiveSubscriptionPlanUnresolved
	}

	input := UpgradeResidualInput{
		Now:                time.Now(),
		StartsAt:           active.StartsAt,
		ExpiresAt:          active.ExpiresAt,
		PlanPrice:          currentPrice,
		TargetPlanPrice:    targetPlan.Price,
		DailyQuotaKnives:   active.DailyQuotaKnives,
		WeeklyQuotaKnives:  active.WeeklyQuotaKnives,
		MonthlyQuotaKnives: active.MonthlyQuotaKnives,
		DailyUsedKnives:    active.DailyUsedKnives,
		WeeklyUsedKnives:   active.WeeklyUsedKnives,
		MonthlyUsedKnives:  active.MonthlyUsedKnives,
		DailyWindowStart:   active.DailyWindowStart,
		WeeklyWindowStart:  active.WeeklyWindowStart,
		MonthlyWindowStart: active.MonthlyWindowStart,
	}
	breakdown, err := CalculateUpgradeResidual(input)
	if err != nil {
		return nil, err
	}
	return breakdown, nil
}
