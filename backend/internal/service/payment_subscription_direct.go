package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *PaymentService) completeSubscriptionDirectAction(ctx context.Context, req CreateOrderRequest, plan *dbent.SubscriptionPlan, decision *subscriptionOrderDecision) (*CreateOrderResponse, error) {
	if decision == nil || !decision.CanCompleteDirectly {
		return nil, infraerrors.BadRequest("SUBSCRIPTION_DIRECT_ACTION_UNAVAILABLE", "subscription action cannot complete directly")
	}
	if decision.Action != subscriptionActionUpgrade {
		return nil, infraerrors.BadRequest("SUBSCRIPTION_DIRECT_ACTION_UNAVAILABLE", "only zero-delta upgrades can complete directly")
	}
	if s.entClient == nil || s.subscriptionSvc == nil {
		return nil, infraerrors.InternalServer("SUBSCRIPTION_FULFILLMENT_UNAVAILABLE", "subscription direct action requires database access")
	}
	if plan == nil {
		plan = decision.Plan
	}
	if plan == nil {
		return nil, ErrSettlementTargetPlanMissing
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin subscription direct action tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	txClient := tx.Client()
	now := time.Now()
	if err := s.ensurePlanPurchaseAllowed(txCtx, req.UserID, plan); err != nil {
		return nil, err
	}

	activeBefore, err := s.subscriptionSvc.GetActiveSubscriptionByUser(txCtx, req.UserID)
	if err != nil {
		return nil, err
	}
	var openHead *dbent.SubscriptionSettlementOrder
	if s.settlementSvc != nil {
		openHead, err = s.settlementSvc.getOpenHead(txCtx, txClient, req.UserID)
		if err != nil {
			return nil, err
		}
	}

	result, err := s.subscriptionSvc.UpgradeActivePlan(txCtx, &UpgradeActivePlanInput{
		UserID:     req.UserID,
		TargetPlan: plan,
		Notes:      "direct subscription action",
	})
	if err != nil {
		return nil, fmt.Errorf("apply direct subscription upgrade: %w", err)
	}
	if result == nil || result.Current == nil {
		return nil, infraerrors.InternalServer("SUBSCRIPTION_FULFILLMENT_UNAVAILABLE", "subscription direct action did not produce a resulting subscription")
	}

	if s.settlementSvc != nil {
		residualBasis := settlementResidualBasisValue(openHead, activeBefore, plan.Price)
		carryIn := settlementResidualValue(activeBefore, residualBasis)
		writeoff := 0.0
		if carryIn > plan.Price {
			writeoff = carryIn - plan.Price
		}
		if _, err = s.settlementSvc.CreateSettlementOrder(txCtx, SettlementOrderInput{
			UserID:                  req.UserID,
			OperatorUserID:          req.UserID,
			ActionType:              domain.SettlementActionUpgrade,
			ActionSource:            domain.SettlementActionSourceUserPurchase,
			TriggerRefType:          domain.SettlementTriggerRefDirectAction,
			ActionNote:              "direct subscription action",
			CarryInResidualValue:    carryIn,
			ActionDeltaValue:        0,
			AfterSettlementValue:    plan.Price,
			WriteoffValue:           writeoff,
			AfterUserSubscription:   result.Current,
			AfterPlan:               plan,
			AfterSubscriptionStatus: result.Current.Status,
			EffectiveAt:             now,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit subscription direct action tx: %w", err)
	}

	return &CreateOrderResponse{
		Amount:             0,
		PayAmount:          0,
		FeeRate:            0,
		Status:             OrderStatusCompleted,
		ResultType:         payment.CreatePaymentResultCompletedDirectly,
		PaymentType:        req.PaymentType,
		SubscriptionAction: decision.Action,
		UpgradeBreakdown:   decision.UpgradeBreakdown,
		ExpiresAt:          result.Current.ExpiresAt,
	}, nil
}
