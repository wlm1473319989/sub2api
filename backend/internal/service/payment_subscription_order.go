package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	subscriptionActionPurchase    = "purchase"
	subscriptionActionRenew       = "renew"
	subscriptionActionUpgrade     = "upgrade"
	subscriptionActionUnavailable = "unavailable"
	subscriptionActionRefund      = "refund"
)

var (
	ErrSubscriptionOrderActionInvalid        = infraerrors.BadRequest("SUBSCRIPTION_ORDER_ACTION_INVALID", "active subscription only supports renewal or upgrade")
	ErrUpgradePaymentNotRequired             = infraerrors.BadRequest("UPGRADE_PAYMENT_NOT_REQUIRED", "upgrade does not require additional payment")
	ErrActiveSubscriptionPlanUnresolved      = infraerrors.Conflict("ACTIVE_SUBSCRIPTION_PLAN_UNRESOLVED", "active subscription plan could not be resolved during migration")
	ErrSubscriptionPlanPurchaseLimitExceeded = infraerrors.Conflict("SUBSCRIPTION_PLAN_PURCHASE_LIMIT_EXCEEDED", "purchase limit reached for this subscription plan")
)

type subscriptionOrderDecision struct {
	Plan                *dbent.SubscriptionPlan
	ActiveSubscription  *UserSubscription
	Action              string
	OrderAmount         float64
	UpgradeBreakdown    *UpgradeResidualBreakdown
	CanCompleteDirectly bool
}

const (
	subscriptionPreviewBlockedReasonDowngradeOrSwitch    = "downgrade_or_switch_not_supported"
	subscriptionPreviewBlockedReasonUpgradeNoPayment     = "upgrade_payment_not_required"
	subscriptionPreviewBlockedReasonPurchaseLimitReached = "purchase_limit_reached"
)

type SubscriptionOrderPreview struct {
	Action              string                    `json:"action"`
	OrderAmount         float64                   `json:"order_amount"`
	CurrentPlan         *SubscriptionPreviewPlan  `json:"current_plan,omitempty"`
	TargetPlan          *SubscriptionPreviewPlan  `json:"target_plan,omitempty"`
	UpgradeBreakdown    *UpgradeResidualBreakdown `json:"upgrade_breakdown,omitempty"`
	BlockedReason       string                    `json:"blocked_reason,omitempty"`
	CanCompleteDirectly bool                      `json:"can_complete_directly"`
}

type SubscriptionPreviewPlan struct {
	ID           *int64     `json:"id,omitempty"`
	Name         string     `json:"name,omitempty"`
	Price        *float64   `json:"price,omitempty"`
	ValidityDays *int       `json:"validity_days,omitempty"`
	ValidityUnit string     `json:"validity_unit,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

func (s *PaymentService) prepareSubscriptionOrderDecision(ctx context.Context, userID int64, planID int64, currency string) (*subscriptionOrderDecision, error) {
	if planID == 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription order requires a plan")
	}
	plan, err := s.configService.GetPlan(ctx, planID)
	if err != nil || !plan.ForSale {
		return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
	}

	if decision, usedSettlementHead, settlementErr := s.prepareSubscriptionOrderDecisionFromSettlementHead(ctx, userID, plan, currency); usedSettlementHead || settlementErr != nil {
		return decision, settlementErr
	}

	active, err := s.subscriptionSvc.GetActiveSubscriptionByUser(ctx, userID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			decision := &subscriptionOrderDecision{
				Plan:        plan,
				Action:      subscriptionActionPurchase,
				OrderAmount: plan.Price,
			}
			if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
				return nil, err
			}
			return decision, nil
		}
		return nil, err
	}

	currentPlanID, currentPrice, err := s.resolveActiveSubscriptionReference(ctx, active)
	if err != nil {
		return nil, err
	}

	if subscriptionOrderMatchesRenewPlan(active, currentPlanID, plan) {
		decision := &subscriptionOrderDecision{
			Plan:               plan,
			ActiveSubscription: active,
			Action:             subscriptionActionRenew,
			OrderAmount:        plan.Price,
		}
		if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
			return nil, err
		}
		return decision, nil
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
		breakdown = roundUpgradeBreakdownForCurrency(breakdown, currency)
		upgradeAmount = breakdown.UpgradeDelta
	}
	if upgradeAmount <= 0 {
		decision := &subscriptionOrderDecision{
			Plan:                plan,
			ActiveSubscription:  active,
			Action:              subscriptionActionUpgrade,
			OrderAmount:         0,
			UpgradeBreakdown:    breakdown,
			CanCompleteDirectly: true,
		}
		if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
			return nil, err
		}
		return decision, nil
	}

	decision := &subscriptionOrderDecision{
		Plan:               plan,
		ActiveSubscription: active,
		Action:             subscriptionActionUpgrade,
		OrderAmount:        upgradeAmount,
		UpgradeBreakdown:   breakdown,
	}
	if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
		return nil, err
	}
	return decision, nil
}

func (s *PaymentService) prepareSubscriptionOrderDecisionFromSettlementHead(ctx context.Context, userID int64, plan *dbent.SubscriptionPlan, currency string) (*subscriptionOrderDecision, bool, error) {
	if s == nil || s.settlementSvc == nil {
		return nil, false, nil
	}

	head, err := s.settlementSvc.GetEffectiveHead(ctx, userID, time.Now())
	if err != nil {
		return nil, true, err
	}
	if head == nil {
		return nil, false, nil
	}

	settlementDecision, err := s.settlementSvc.DeterminePlanAction(head, plan)
	if err != nil {
		return nil, true, err
	}

	switch settlementDecision.Action {
	case subscriptionActionPurchase:
		decision := &subscriptionOrderDecision{
			Plan:        plan,
			Action:      subscriptionActionPurchase,
			OrderAmount: plan.Price,
		}
		if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
			return nil, true, err
		}
		return decision, true, nil
	case subscriptionActionRenew:
		active, activeErr := s.subscriptionSvc.GetActiveSubscriptionByUser(ctx, userID)
		if activeErr != nil {
			return nil, true, activeErr
		}
		decision := &subscriptionOrderDecision{
			Plan:               plan,
			ActiveSubscription: active,
			Action:             subscriptionActionRenew,
			OrderAmount:        plan.Price,
		}
		if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
			return nil, true, err
		}
		return decision, true, nil
	case subscriptionActionUpgrade:
		active, activeErr := s.subscriptionSvc.GetActiveSubscriptionByUser(ctx, userID)
		if activeErr != nil {
			return nil, true, activeErr
		}
		if settlementDecision.CurrentPlanPrice == nil {
			return nil, true, ErrSettlementHeadIncomplete
		}
		residualBasis := settlementResidualBasisValue(head, active, *settlementDecision.CurrentPlanPrice)
		breakdown, calcErr := s.calculateUpgradeOrderDelta(ctx, active, residualBasis, plan)
		if calcErr != nil {
			return nil, true, calcErr
		}
		breakdown = roundUpgradeBreakdownForCurrency(breakdown, currency)
		if breakdown.UpgradeDelta <= 0 {
			decision := &subscriptionOrderDecision{
				Plan:                plan,
				ActiveSubscription:  active,
				Action:              subscriptionActionUpgrade,
				OrderAmount:         0,
				UpgradeBreakdown:    breakdown,
				CanCompleteDirectly: true,
			}
			if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
				return nil, true, err
			}
			return decision, true, nil
		}
		decision := &subscriptionOrderDecision{
			Plan:               plan,
			ActiveSubscription: active,
			Action:             subscriptionActionUpgrade,
			OrderAmount:        breakdown.UpgradeDelta,
			UpgradeBreakdown:   breakdown,
		}
		if err := s.ensurePlanPurchaseAllowed(ctx, userID, plan); err != nil {
			return nil, true, err
		}
		return decision, true, nil
	case subscriptionActionUnavailable:
		return nil, true, ErrSubscriptionOrderActionInvalid
	default:
		return nil, true, infraerrors.BadRequest("SUBSCRIPTION_ORDER_ACTION_INVALID", "unsupported subscription settlement action")
	}
}

func (s *PaymentService) PreviewSubscriptionOrder(ctx context.Context, userID int64, planID int64, currency string) (*SubscriptionOrderPreview, error) {
	decision, err := s.prepareSubscriptionOrderDecision(ctx, userID, planID, currency)
	if err == nil {
		return s.buildSubscriptionOrderPreview(ctx, decision.Action, decision.OrderAmount, decision.Plan, decision.ActiveSubscription, decision.UpgradeBreakdown, decision.CanCompleteDirectly, "")
	}

	if !errors.Is(err, ErrSubscriptionOrderActionInvalid) &&
		!errors.Is(err, ErrUpgradePaymentNotRequired) &&
		!errors.Is(err, ErrSubscriptionPlanPurchaseLimitExceeded) {
		return nil, err
	}

	plan, planErr := s.configService.GetPlan(ctx, planID)
	if planErr != nil || !plan.ForSale {
		return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
	}

	active, activeErr := s.subscriptionSvc.GetActiveSubscriptionByUser(ctx, userID)
	switch {
	case activeErr == nil:
	case errorsIsSubscriptionNotFound(activeErr):
		active = nil
	default:
		return nil, activeErr
	}

	return s.buildSubscriptionOrderPreview(
		ctx,
		subscriptionActionUnavailable,
		0,
		plan,
		active,
		nil,
		false,
		subscriptionPreviewBlockedReason(err),
	)
}

func settlementResidualBasisValue(head *dbent.SubscriptionSettlementOrder, active *UserSubscription, fallback float64) float64 {
	if head != nil && head.AfterSettlementValue > 0 {
		return head.AfterSettlementValue
	}
	if active != nil && active.PlanPriceSnapshot != nil && *active.PlanPriceSnapshot > 0 {
		return *active.PlanPriceSnapshot
	}
	return fallback
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

func (s *PaymentService) buildSubscriptionOrderPreview(
	ctx context.Context,
	action string,
	orderAmount float64,
	targetPlan *dbent.SubscriptionPlan,
	active *UserSubscription,
	upgradeBreakdown *UpgradeResidualBreakdown,
	canCompleteDirectly bool,
	blockedReason string,
) (*SubscriptionOrderPreview, error) {
	preview := &SubscriptionOrderPreview{
		Action:              action,
		OrderAmount:         orderAmount,
		TargetPlan:          buildSubscriptionPreviewTargetPlan(targetPlan),
		UpgradeBreakdown:    upgradeBreakdown,
		BlockedReason:       blockedReason,
		CanCompleteDirectly: canCompleteDirectly,
	}
	currentPlan, err := s.buildSubscriptionPreviewCurrentPlan(ctx, active)
	if err != nil {
		return nil, err
	}
	preview.CurrentPlan = currentPlan
	return preview, nil
}

func buildSubscriptionPreviewTargetPlan(plan *dbent.SubscriptionPlan) *SubscriptionPreviewPlan {
	if plan == nil {
		return nil
	}
	planID := plan.ID
	price := plan.Price
	validityDays := plan.ValidityDays
	return &SubscriptionPreviewPlan{
		ID:           &planID,
		Name:         plan.Name,
		Price:        &price,
		ValidityDays: &validityDays,
		ValidityUnit: plan.ValidityUnit,
	}
}

func (s *PaymentService) buildSubscriptionPreviewCurrentPlan(ctx context.Context, active *UserSubscription) (*SubscriptionPreviewPlan, error) {
	if active == nil {
		return nil, nil
	}

	currentPlanID, currentPrice, err := s.resolveActiveSubscriptionReference(ctx, active)
	if err != nil {
		return nil, err
	}

	preview := &SubscriptionPreviewPlan{
		ID:        currentPlanID,
		Price:     currentPrice,
		ExpiresAt: copyTimePointer(&active.ExpiresAt),
	}
	if active.PlanNameSnapshot != nil && strings.TrimSpace(*active.PlanNameSnapshot) != "" {
		preview.Name = strings.TrimSpace(*active.PlanNameSnapshot)
	}

	if currentPlanID != nil {
		plan, planErr := s.configService.GetPlan(ctx, *currentPlanID)
		if planErr == nil && plan != nil {
			if preview.Name == "" {
				preview.Name = plan.Name
			}
			if preview.Price == nil {
				preview.Price = copyFloat64Pointer(&plan.Price)
			}
			validityDays := plan.ValidityDays
			preview.ValidityDays = &validityDays
			preview.ValidityUnit = plan.ValidityUnit
		}
	}

	if preview.Name == "" {
		preview.Name = "Current Subscription"
	}
	return preview, nil
}

func subscriptionPreviewBlockedReason(err error) string {
	switch {
	case errors.Is(err, ErrUpgradePaymentNotRequired):
		return subscriptionPreviewBlockedReasonUpgradeNoPayment
	case errors.Is(err, ErrSubscriptionPlanPurchaseLimitExceeded):
		return subscriptionPreviewBlockedReasonPurchaseLimitReached
	case errors.Is(err, ErrSubscriptionOrderActionInvalid):
		return subscriptionPreviewBlockedReasonDowngradeOrSwitch
	default:
		return ""
	}
}

func copyTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	v := *value
	return &v
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
