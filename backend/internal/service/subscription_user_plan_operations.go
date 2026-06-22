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

var (
	ErrSubscriptionPlanRequired          = infraerrors.BadRequest("SUBSCRIPTION_PLAN_REQUIRED", "subscription plan is required")
	ErrActiveSubscriptionExists          = infraerrors.Conflict("ACTIVE_SUBSCRIPTION_EXISTS", "user already has an active subscription")
	ErrActiveSubscriptionRequired        = infraerrors.Conflict("ACTIVE_SUBSCRIPTION_REQUIRED", "user must have an active subscription")
	ErrActiveSubscriptionSnapshotMissing = infraerrors.Conflict("ACTIVE_SUBSCRIPTION_SNAPSHOT_MISSING", "active subscription snapshot is incomplete")
	ErrSubscriptionPlanActionInvalid     = infraerrors.BadRequest("SUBSCRIPTION_PLAN_ACTION_INVALID", "active subscription only supports renewal or upgrade")
	ErrRenewPlanMismatch                 = infraerrors.BadRequest("RENEW_PLAN_MISMATCH", "renewal plan must match the active subscription")
	ErrUpgradePlanPriceInvalid           = infraerrors.BadRequest("UPGRADE_PLAN_PRICE_INVALID", "upgrade target plan price must be higher than the active subscription")
	ErrRefundOrderRequired               = infraerrors.BadRequest("REFUND_ORDER_REQUIRED", "refund requires a subscription order id")
	ErrRefundOrderNotLatest              = infraerrors.Conflict("REFUND_ORDER_NOT_LATEST", "refund order must be the latest order for the active subscription")
)

type PurchaseNewPlanInput struct {
	UserID     int64
	Plan       *dbent.SubscriptionPlan
	AssignedBy int64
	Notes      string
}

type RenewActivePlanInput struct {
	UserID int64
	Plan   *dbent.SubscriptionPlan
	Notes  string
}

type UpgradeActivePlanInput struct {
	UserID     int64
	TargetPlan *dbent.SubscriptionPlan
	AssignedBy int64
	Notes      string
}

type RefundActivePlanInput struct {
	UserID  int64
	OrderID int64
	Notes   string
}

type UpgradeActivePlanResult struct {
	Previous *UserSubscription
	Current  *UserSubscription
}

type RefundActivePlanResult struct {
	Subscription *UserSubscription
	OrderID      int64
}

func (s *SubscriptionService) PurchaseNewPlan(ctx context.Context, input *PurchaseNewPlanInput) (*UserSubscription, error) {
	if input == nil || input.Plan == nil {
		return nil, ErrSubscriptionPlanRequired
	}
	active, err := s.userSubRepo.GetActiveByUserID(ctx, input.UserID)
	if err == nil && active != nil {
		return nil, ErrActiveSubscriptionExists
	}
	if err != nil && !errorsIsSubscriptionNotFound(err) {
		return nil, err
	}

	sub, err := s.createPlanSnapshotSubscription(ctx, input.UserID, input.Plan, input.AssignedBy, input.Notes, time.Now())
	if err != nil {
		return nil, err
	}
	s.invalidateSubscriptionCaches(input.UserID, sub.GroupID)
	return sub, nil
}

func (s *SubscriptionService) RenewActivePlan(ctx context.Context, input *RenewActivePlanInput) (*UserSubscription, error) {
	if input == nil || input.Plan == nil {
		return nil, ErrSubscriptionPlanRequired
	}
	active, err := s.userSubRepo.GetActiveByUserID(ctx, input.UserID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			return nil, ErrActiveSubscriptionRequired
		}
		return nil, err
	}
	if !subscriptionMatchesRenewalPlan(active, input.Plan) {
		return nil, ErrRenewPlanMismatch
	}

	validityDays, err := subscriptionPlanTotalValidityDays(input.Plan)
	if err != nil {
		return nil, err
	}
	newExpiresAt := clipSubscriptionExpiry(active.ExpiresAt.AddDate(0, 0, validityDays))
	renewed := *active
	renewed.ExpiresAt = newExpiresAt
	renewed.Notes = appendSubscriptionNotes(active.Notes, input.Notes)

	if err := s.userSubRepo.Update(ctx, &renewed); err != nil {
		return nil, err
	}
	s.invalidateSubscriptionCaches(input.UserID, active.GroupID)
	return s.userSubRepo.GetByID(ctx, active.ID)
}

func (s *SubscriptionService) UpgradeActivePlan(ctx context.Context, input *UpgradeActivePlanInput) (*UpgradeActivePlanResult, error) {
	if input == nil || input.TargetPlan == nil {
		return nil, ErrSubscriptionPlanRequired
	}
	active, err := s.userSubRepo.GetActiveByUserID(ctx, input.UserID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			return nil, ErrActiveSubscriptionRequired
		}
		return nil, err
	}

	currentPrice, err := activeSubscriptionPrice(active)
	if err != nil {
		return nil, err
	}
	if input.TargetPlan.Price <= currentPrice {
		return nil, ErrUpgradePlanPriceInvalid
	}

	now := time.Now()
	var newSubscriptionID int64
	if err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		newSub, createErr := s.createPlanSnapshotSubscription(txCtx, input.UserID, input.TargetPlan, input.AssignedBy, input.Notes, now)
		if createErr != nil {
			return createErr
		}
		newSubscriptionID = newSub.ID

		superseded := *active
		superseded.Status = SubscriptionStatusSuperseded
		superseded.SupersededByID = &newSubscriptionID
		superseded.Notes = appendSubscriptionNotes(active.Notes, input.Notes)
		return s.userSubRepo.Update(txCtx, &superseded)
	}); err != nil {
		return nil, err
	}

	s.invalidateSubscriptionCaches(input.UserID, active.GroupID)
	current, err := s.userSubRepo.GetByID(ctx, newSubscriptionID)
	if err != nil {
		return nil, err
	}
	previous, err := s.userSubRepo.GetByID(ctx, active.ID)
	if err != nil {
		return nil, err
	}
	return &UpgradeActivePlanResult{
		Previous: previous,
		Current:  current,
	}, nil
}

func (s *SubscriptionService) RefundActivePlan(ctx context.Context, input *RefundActivePlanInput) (*RefundActivePlanResult, error) {
	if input == nil || input.OrderID <= 0 {
		return nil, ErrRefundOrderRequired
	}
	if s.entClient == nil {
		return nil, infraerrors.InternalServer("SUBSCRIPTION_ENT_CLIENT_REQUIRED", "subscription refund requires database access")
	}
	active, err := s.userSubRepo.GetActiveByUserID(ctx, input.UserID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			return nil, ErrActiveSubscriptionRequired
		}
		return nil, err
	}

	latestOrder, err := s.latestSubscriptionOrderForActive(ctx, input.UserID, active)
	if err != nil {
		return nil, err
	}
	if latestOrder.ID != input.OrderID {
		return nil, ErrRefundOrderNotLatest
	}

	refundAt := time.Now()
	refunded := *active
	refunded.Status = SubscriptionStatusRefunded
	refunded.ExpiresAt = refundAt
	refunded.Notes = appendSubscriptionNotes(active.Notes, input.Notes)
	if err := s.userSubRepo.Update(ctx, &refunded); err != nil {
		return nil, err
	}
	s.invalidateSubscriptionCaches(input.UserID, active.GroupID)
	sub, err := s.userSubRepo.GetByID(ctx, active.ID)
	if err != nil {
		return nil, err
	}
	return &RefundActivePlanResult{
		Subscription: sub,
		OrderID:      latestOrder.ID,
	}, nil
}

func (s *SubscriptionService) createPlanSnapshotSubscription(ctx context.Context, userID int64, plan *dbent.SubscriptionPlan, assignedBy int64, notes string, now time.Time) (*UserSubscription, error) {
	if plan == nil {
		return nil, ErrSubscriptionPlanRequired
	}
	validityDays, err := subscriptionPlanTotalValidityDays(plan)
	if err != nil {
		return nil, err
	}
	expiresAt := clipSubscriptionExpiry(now.AddDate(0, 0, validityDays))

	planID := plan.ID
	planName := plan.Name
	planPrice := plan.Price
	sub := &UserSubscription{
		UserID:             userID,
		GroupID:            resolveSubscriptionPersistenceGroupID(plan),
		PlanID:             &planID,
		PlanNameSnapshot:   copyStringPointer(&planName),
		PlanPriceSnapshot:  copyFloat64Pointer(&planPrice),
		StartsAt:           now,
		ExpiresAt:          expiresAt,
		Status:             SubscriptionStatusActive,
		DailyQuotaKnives:   copyFloat64Pointer(plan.DailyQuotaKnives),
		WeeklyQuotaKnives:  copyFloat64Pointer(plan.WeeklyQuotaKnives),
		MonthlyQuotaKnives: copyFloat64Pointer(plan.MonthlyQuotaKnives),
		AssignedAt:         now,
		Notes:              notes,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if assignedBy > 0 {
		sub.AssignedBy = &assignedBy
	}
	if err := s.userSubRepo.Create(ctx, sub); err != nil {
		return nil, err
	}
	return s.userSubRepo.GetByID(ctx, sub.ID)
}

func subscriptionMatchesRenewalPlan(active *UserSubscription, plan *dbent.SubscriptionPlan) bool {
	if active == nil || plan == nil {
		return false
	}
	return active.PlanID != nil && *active.PlanID == plan.ID
}

func activeSubscriptionPrice(active *UserSubscription) (float64, error) {
	if active == nil || active.PlanPriceSnapshot == nil {
		return 0, ErrActiveSubscriptionSnapshotMissing
	}
	return *active.PlanPriceSnapshot, nil
}

func subscriptionPlanTotalValidityDays(plan *dbent.SubscriptionPlan) (int, error) {
	if plan == nil {
		return 0, ErrSubscriptionPlanRequired
	}
	if plan.ValidityDays <= 0 {
		return 0, infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "plan validity days must be > 0")
	}
	days := plan.ValidityDays
	switch normalizePlanValidityUnitValue(plan.ValidityUnit) {
	case "week":
		days *= 7
	case "month":
		days *= 30
	case "year":
		days *= 365
	}
	if days > MaxValidityDays {
		days = MaxValidityDays
	}
	return days, nil
}

func clipSubscriptionExpiry(expiresAt time.Time) time.Time {
	if expiresAt.After(MaxExpiresAt) {
		return MaxExpiresAt
	}
	return expiresAt
}

func resolveSubscriptionPersistenceGroupID(plan *dbent.SubscriptionPlan) int64 {
	if plan != nil && plan.GroupID != nil && *plan.GroupID > 0 {
		return *plan.GroupID
	}
	return 0
}

func copyFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func copyStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func (s *SubscriptionService) invalidateSubscriptionCaches(userID int64, _ ...int64) {
	s.InvalidateSubCache(userID)
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID)
		}()
	}
}

func (s *SubscriptionService) latestSubscriptionOrderForActive(ctx context.Context, userID int64, active *UserSubscription) (*dbent.PaymentOrder, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.InternalServer("SUBSCRIPTION_ENT_CLIENT_REQUIRED", "subscription order lookup requires database access")
	}
	query := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(userID),
			paymentorder.OrderTypeEQ(payment.OrderTypeSubscription),
			paymentorder.StatusIn(
				OrderStatusPaid,
				OrderStatusRecharging,
				OrderStatusCompleted,
				OrderStatusRefundRequested,
				OrderStatusRefunding,
				OrderStatusRefundFailed,
			),
		).
		Order(dbent.Desc(paymentorder.FieldCreatedAt))

	if active != nil && active.PlanID != nil {
		query = query.Where(paymentorder.PlanIDEQ(*active.PlanID))
	} else {
		return nil, ErrRefundOrderNotLatest
	}

	order, err := query.First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrRefundOrderNotLatest
		}
		return nil, fmt.Errorf("query latest subscription order: %w", err)
	}
	return order, nil
}

func errorsIsSubscriptionNotFound(err error) bool {
	return err != nil && infraerrors.Reason(err) == infraerrors.Reason(ErrSubscriptionNotFound)
}
