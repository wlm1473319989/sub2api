package service

import (
	"context"
	"strconv"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var subscriptionPurchaseLimitOrderStatuses = []string{
	OrderStatusPaid,
	OrderStatusRecharging,
	OrderStatusCompleted,
	OrderStatusRefundRequested,
	OrderStatusRefunding,
	OrderStatusPartiallyRefunded,
	OrderStatusRefunded,
	OrderStatusRefundFailed,
}

type planPurchaseLimitCountOptions struct {
	CurrentOrderID *int64
}

func (s *PaymentService) ensurePlanPurchaseAllowed(ctx context.Context, userID int64, plan *dbent.SubscriptionPlan) error {
	return s.ensurePlanPurchaseAllowedWithOptions(ctx, userID, plan, planPurchaseLimitCountOptions{})
}

func (s *PaymentService) ensurePlanPurchaseAllowedForOrder(ctx context.Context, userID int64, plan *dbent.SubscriptionPlan, currentOrderID int64) error {
	return s.ensurePlanPurchaseAllowedWithOptions(ctx, userID, plan, planPurchaseLimitCountOptions{
		CurrentOrderID: &currentOrderID,
	})
}

func (s *PaymentService) ensurePlanPurchaseAllowedWithOptions(ctx context.Context, userID int64, plan *dbent.SubscriptionPlan, options planPurchaseLimitCountOptions) error {
	if plan == nil || plan.PurchaseLimitPerUser == nil {
		return nil
	}
	count, err := s.countPlanPurchasesByUser(ctx, userID, plan.ID, options)
	if err != nil {
		return err
	}
	if count < int64(*plan.PurchaseLimitPerUser) {
		return nil
	}
	return ErrSubscriptionPlanPurchaseLimitExceeded.WithMetadata(map[string]string{
		"count":   strconv.FormatInt(count, 10),
		"limit":   strconv.Itoa(*plan.PurchaseLimitPerUser),
		"plan_id": strconv.FormatInt(plan.ID, 10),
	})
}

func (s *PaymentService) countPlanPurchasesByUser(ctx context.Context, userID int64, planID int64, options planPurchaseLimitCountOptions) (int64, error) {
	if s == nil || s.entClient == nil {
		return 0, infraerrors.InternalServer("SUBSCRIPTION_PLAN_PURCHASE_LIMIT_UNAVAILABLE", "subscription purchase limit requires database access")
	}

	paymentOrderQuery := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(userID),
			paymentorder.PlanIDEQ(planID),
			paymentorder.OrderTypeEQ(payment.OrderTypeSubscription),
			paymentorder.StatusIn(subscriptionPurchaseLimitOrderStatuses...),
		)
	if options.CurrentOrderID != nil {
		paymentOrderQuery = paymentOrderQuery.Where(paymentorder.IDLT(*options.CurrentOrderID))
	}
	paymentOrderCount, err := paymentOrderQuery.Count(ctx)
	if err != nil {
		return 0, err
	}

	directActionCount, err := s.entClient.SubscriptionSettlementOrder.Query().
		Where(
			subscriptionsettlementorder.UserIDEQ(userID),
			subscriptionsettlementorder.AfterPlanIDEQ(planID),
			subscriptionsettlementorder.ActionSourceEQ(domain.SettlementActionSourceUserPurchase),
			subscriptionsettlementorder.TriggerRefTypeEQ(domain.SettlementTriggerRefDirectAction),
		).
		Count(ctx)
	if err != nil {
		return 0, err
	}

	return int64(paymentOrderCount + directActionCount), nil
}
