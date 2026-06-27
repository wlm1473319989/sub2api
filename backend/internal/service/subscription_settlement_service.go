package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSettlementEntClientRequired   = infraerrors.InternalServer("SETTLEMENT_ENT_CLIENT_REQUIRED", "settlement service requires database access")
	ErrSettlementTargetPlanMissing   = infraerrors.BadRequest("SETTLEMENT_TARGET_PLAN_MISSING", "settlement action requires a target plan")
	ErrSettlementHeadIncomplete      = infraerrors.Conflict("SETTLEMENT_HEAD_INCOMPLETE", "effective settlement head is missing plan snapshot")
	ErrSettlementSubscriptionMissing = infraerrors.BadRequest("SETTLEMENT_SUBSCRIPTION_MISSING", "settlement action requires a resulting subscription")
)

type SettlementService struct {
	entClient *dbent.Client
}

type SettlementActionDecision struct {
	Action           string
	CurrentHead      *dbent.SubscriptionSettlementOrder
	TargetPlan       *dbent.SubscriptionPlan
	CurrentPlanID    *int64
	CurrentPlanPrice *float64
	BlockedReason    string
}

type SettlementOrderInput struct {
	UserID                  int64
	OperatorUserID          int64
	ActionType              string
	ActionSource            string
	TriggerRefType          string
	TriggerRefID            *int64
	ActionNote              string
	CarryInResidualValue    float64
	ActionDeltaValue        float64
	AfterSettlementValue    float64
	RefundResidualValue     *float64
	WriteoffValue           float64
	AfterUserSubscription   *UserSubscription
	AfterPlan               *dbent.SubscriptionPlan
	AfterSubscriptionStatus string
	EffectiveAt             time.Time
}

func NewSettlementService(entClient *dbent.Client) *SettlementService {
	return &SettlementService{entClient: entClient}
}

func (s *SettlementService) clientFromContext(ctx context.Context) (*dbent.Client, error) {
	if s == nil || s.entClient == nil {
		return nil, ErrSettlementEntClientRequired
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client(), nil
	}
	return s.entClient, nil
}

func (s *SettlementService) GetEffectiveHead(ctx context.Context, userID int64, now time.Time) (*dbent.SubscriptionSettlementOrder, error) {
	client, err := s.clientFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "user id is required")
	}
	if now.IsZero() {
		now = time.Now()
	}

	head, err := client.SubscriptionSettlementOrder.Query().
		Where(
			subscriptionsettlementorder.UserIDEQ(userID),
			subscriptionsettlementorder.StatusEQ(domain.SettlementStatusEffective),
			subscriptionsettlementorder.AfterSubscriptionStatusEQ(domain.SubscriptionStatusActive),
			subscriptionsettlementorder.AfterExpiresAtGT(now),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query effective settlement head: %w", err)
	}
	return head, nil
}

func (s *SettlementService) CreateSettlementOrder(ctx context.Context, input SettlementOrderInput) (*dbent.SubscriptionSettlementOrder, error) {
	client, err := s.clientFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if input.UserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "user id is required")
	}
	if input.OperatorUserID <= 0 {
		input.OperatorUserID = input.UserID
	}
	if input.AfterUserSubscription == nil {
		return nil, ErrSettlementSubscriptionMissing
	}
	if input.EffectiveAt.IsZero() {
		input.EffectiveAt = time.Now()
	}
	input.CarryInResidualValue = roundSettlementAmountValue(input.CarryInResidualValue)
	input.ActionDeltaValue = roundSettlementAmountValue(input.ActionDeltaValue)
	input.AfterSettlementValue = roundSettlementAmountValue(input.AfterSettlementValue)
	input.RefundResidualValue = roundSettlementAmountPointer(input.RefundResidualValue)
	input.WriteoffValue = roundSettlementAmountValue(input.WriteoffValue)

	afterStatus := input.AfterSubscriptionStatus
	if afterStatus == "" {
		afterStatus = input.AfterUserSubscription.Status
	}
	if afterStatus == "" {
		afterStatus = domain.SubscriptionStatusActive
	}

	openHead, err := s.getOpenHead(ctx, client, input.UserID)
	if err != nil {
		return nil, err
	}
	appendPrev := settlementOrderCanAppend(openHead, input.EffectiveAt)
	if input.ActionType == domain.SettlementActionPurchase && appendPrev {
		return nil, infraerrors.Conflict("SETTLEMENT_CHAIN_ACTIVE", "purchase cannot start a new settlement chain while an active chain exists")
	}
	if settlementOrderMustAppend(input.ActionType) && !appendPrev {
		return nil, infraerrors.Conflict("SETTLEMENT_CHAIN_HEAD_MISSING", "active subscription settlement chain head is missing")
	}
	if settlementOrderRequiresActiveCursor(input.ActionType) && openHead != nil && !appendPrev {
		return nil, infraerrors.Conflict("SETTLEMENT_CHAIN_HEAD_MISSING", "active subscription settlement chain head is missing")
	}
	if openHead != nil {
		updated, updateErr := client.SubscriptionSettlementOrder.Update().
			Where(
				subscriptionsettlementorder.IDEQ(openHead.ID),
				subscriptionsettlementorder.StatusEQ(domain.SettlementStatusEffective),
			).
			SetStatus(domain.SettlementStatusClosed).
			SetClosedAt(input.EffectiveAt).
			SetUpdatedAt(input.EffectiveAt).
			Save(ctx)
		if updateErr != nil {
			return nil, fmt.Errorf("close previous settlement head: %w", updateErr)
		}
		if updated != 1 {
			return nil, infraerrors.Conflict("SETTLEMENT_HEAD_CHANGED", "effective settlement head changed during settlement creation")
		}
		if appendPrev {
			if err := s.clearStaleReverseSettlementLinks(ctx, client, input.UserID, openHead.ID, input.EffectiveAt); err != nil {
				return nil, err
			}
		}
	}

	var prevSettlementID *int64
	if appendPrev {
		prevSettlementID = &openHead.ID
	}

	builder := client.SubscriptionSettlementOrder.Create().
		SetUserID(input.UserID).
		SetOperatorUserID(input.OperatorUserID).
		SetActionType(input.ActionType).
		SetActionSource(input.ActionSource).
		SetStatus(domain.SettlementStatusEffective).
		SetTriggerRefType(input.TriggerRefType).
		SetNillableTriggerRefID(input.TriggerRefID).
		SetCarryInResidualValue(input.CarryInResidualValue).
		SetActionDeltaValue(input.ActionDeltaValue).
		SetAfterSettlementValue(input.AfterSettlementValue).
		SetNillableRefundResidualValue(input.RefundResidualValue).
		SetWriteoffValue(input.WriteoffValue).
		SetAfterUserSubscriptionID(input.AfterUserSubscription.ID).
		SetAfterStartsAt(input.AfterUserSubscription.StartsAt).
		SetAfterExpiresAt(input.AfterUserSubscription.ExpiresAt).
		SetNillableAfterDailyQuotaKnivesSnapshot(input.AfterUserSubscription.DailyQuotaKnives).
		SetNillableAfterWeeklyQuotaKnivesSnapshot(input.AfterUserSubscription.WeeklyQuotaKnives).
		SetNillableAfterMonthlyQuotaKnivesSnapshot(input.AfterUserSubscription.MonthlyQuotaKnives).
		SetAfterSubscriptionStatus(afterStatus).
		SetEffectiveAt(input.EffectiveAt)
	if input.ActionNote != "" {
		builder.SetActionNote(input.ActionNote)
	}
	if input.AfterPlan != nil {
		builder.
			SetAfterPlanID(input.AfterPlan.ID).
			SetAfterPlanNameSnapshot(input.AfterPlan.Name).
			SetAfterPlanPriceSnapshot(roundSettlementAmountValue(input.AfterPlan.Price)).
			SetAfterValidityDaysSnapshot(input.AfterPlan.ValidityDays).
			SetAfterValidityUnitSnapshot(input.AfterPlan.ValidityUnit)
	} else {
		builder.
			SetNillableAfterPlanID(input.AfterUserSubscription.PlanID).
			SetNillableAfterPlanNameSnapshot(input.AfterUserSubscription.PlanNameSnapshot).
			SetNillableAfterPlanPriceSnapshot(roundSettlementAmountPointer(input.AfterUserSubscription.PlanPriceSnapshot))
	}

	order, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create settlement order: %w", err)
	}
	if prevSettlementID != nil {
		if err := s.setSettlementPrevID(ctx, client, order.ID, *prevSettlementID); err != nil {
			return nil, err
		}
		order.PrevSettlementID = prevSettlementID
	}
	return order, nil
}

func settlementOrderCanAppend(order *dbent.SubscriptionSettlementOrder, at time.Time) bool {
	if order == nil {
		return false
	}
	if at.IsZero() {
		at = time.Now()
	}
	if order.Status != domain.SettlementStatusEffective {
		return false
	}
	if order.AfterSubscriptionStatus != domain.SubscriptionStatusActive {
		return false
	}
	return order.AfterExpiresAt != nil && order.AfterExpiresAt.After(at)
}

func settlementOrderMustAppend(actionType string) bool {
	switch actionType {
	case domain.SettlementActionRefund, domain.SettlementActionRevoke:
		return true
	default:
		return false
	}
}

func settlementOrderRequiresActiveCursor(actionType string) bool {
	switch actionType {
	case domain.SettlementActionRenew, domain.SettlementActionUpgrade:
		return true
	default:
		return false
	}
}

func (s *SettlementService) clearStaleReverseSettlementLinks(ctx context.Context, client *dbent.Client, userID, headID int64, updatedAt time.Time) error {
	_, err := client.ExecContext(
		ctx,
		`UPDATE subscription_settlement_orders
SET prev_settlement_id = NULL, updated_at = $1
WHERE user_id = $2 AND prev_settlement_id = $3 AND id < $3`,
		updatedAt,
		userID,
		headID,
	)
	if err != nil {
		return fmt.Errorf("clear stale reverse settlement links: %w", err)
	}
	return nil
}

func (s *SettlementService) setSettlementPrevID(ctx context.Context, client *dbent.Client, orderID, prevID int64) error {
	result, err := client.ExecContext(
		ctx,
		`UPDATE subscription_settlement_orders
SET prev_settlement_id = $1
WHERE id = $2`,
		prevID,
		orderID,
	)
	if err != nil {
		return fmt.Errorf("set settlement previous link: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set settlement previous link rows affected: %w", err)
	}
	if updated != 1 {
		return infraerrors.Conflict("SETTLEMENT_ORDER_CHANGED", "settlement order changed during previous link update")
	}
	return nil
}

func (s *SettlementService) getOpenHead(ctx context.Context, client *dbent.Client, userID int64) (*dbent.SubscriptionSettlementOrder, error) {
	head, err := client.SubscriptionSettlementOrder.Query().
		Where(
			subscriptionsettlementorder.UserIDEQ(userID),
			subscriptionsettlementorder.StatusEQ(domain.SettlementStatusEffective),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query open settlement head: %w", err)
	}
	return head, nil
}

func (s *SettlementService) DeterminePlanAction(head *dbent.SubscriptionSettlementOrder, targetPlan *dbent.SubscriptionPlan) (*SettlementActionDecision, error) {
	if targetPlan == nil {
		return nil, ErrSettlementTargetPlanMissing
	}

	decision := &SettlementActionDecision{
		CurrentHead: head,
		TargetPlan:  targetPlan,
	}
	if head == nil {
		decision.Action = domain.SettlementActionPurchase
		return decision, nil
	}

	decision.CurrentPlanID = copyInt64Pointer(head.AfterPlanID)
	decision.CurrentPlanPrice = copyFloat64Pointer(head.AfterPlanPriceSnapshot)
	if head.AfterPlanID != nil && *head.AfterPlanID == targetPlan.ID {
		decision.Action = domain.SettlementActionRenew
		return decision, nil
	}
	if head.AfterPlanPriceSnapshot == nil {
		return nil, ErrSettlementHeadIncomplete
	}
	if targetPlan.Price > *head.AfterPlanPriceSnapshot {
		decision.Action = domain.SettlementActionUpgrade
		return decision, nil
	}

	decision.Action = subscriptionActionUnavailable
	decision.BlockedReason = subscriptionPreviewBlockedReasonDowngradeOrSwitch
	return decision, nil
}

func copyInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}
