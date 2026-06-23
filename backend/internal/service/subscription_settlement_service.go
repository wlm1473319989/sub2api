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
	ErrSettlementEntClientRequired = infraerrors.InternalServer("SETTLEMENT_ENT_CLIENT_REQUIRED", "settlement service requires database access")
	ErrSettlementTargetPlanMissing = infraerrors.BadRequest("SETTLEMENT_TARGET_PLAN_MISSING", "settlement action requires a target plan")
	ErrSettlementHeadIncomplete    = infraerrors.Conflict("SETTLEMENT_HEAD_INCOMPLETE", "effective settlement head is missing plan snapshot")
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

func NewSettlementService(entClient *dbent.Client) *SettlementService {
	return &SettlementService{entClient: entClient}
}

func (s *SettlementService) GetEffectiveHead(ctx context.Context, userID int64, now time.Time) (*dbent.SubscriptionSettlementOrder, error) {
	if s == nil || s.entClient == nil {
		return nil, ErrSettlementEntClientRequired
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "user id is required")
	}
	if now.IsZero() {
		now = time.Now()
	}

	head, err := s.entClient.SubscriptionSettlementOrder.Query().
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
