package service

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type subscriptionGrantSettlementContextKey struct{}

type subscriptionGrantSettlementMeta struct {
	ActionSource    string
	TriggerRefType  string
	TriggerRefID    int64
	HasTriggerRefID bool
	OperatorUserID  int64
}

func contextWithExchangeCodeSettlement(ctx context.Context, redeemCodeID int64) context.Context {
	return context.WithValue(ctx, subscriptionGrantSettlementContextKey{}, subscriptionGrantSettlementMeta{
		ActionSource:    domain.SettlementActionSourceExchangeCode,
		TriggerRefType:  domain.SettlementTriggerRefRedeemCode,
		TriggerRefID:    redeemCodeID,
		HasTriggerRefID: true,
	})
}

func subscriptionGrantSettlementFromContext(ctx context.Context) (subscriptionGrantSettlementMeta, bool) {
	meta, ok := ctx.Value(subscriptionGrantSettlementContextKey{}).(subscriptionGrantSettlementMeta)
	return meta, ok
}

func (s *SubscriptionService) prepareGrantSettlement(ctx context.Context, userID int64) (subscriptionGrantSettlementMeta, bool, *SettlementService, *dbent.SubscriptionSettlementOrder, error) {
	meta, ok := subscriptionGrantSettlementFromContext(ctx)
	if !ok {
		return subscriptionGrantSettlementMeta{}, false, nil, nil, nil
	}
	if s == nil || s.entClient == nil {
		return meta, true, nil, nil, ErrSettlementEntClientRequired
	}
	settlementSvc := NewSettlementService(s.entClient)
	head, err := settlementSvc.GetEffectiveHead(ctx, userID, time.Now())
	if err != nil {
		return meta, true, settlementSvc, nil, err
	}
	return meta, true, settlementSvc, head, nil
}

func (s *SubscriptionService) createGrantSettlementOrder(
	ctx context.Context,
	settlementSvc *SettlementService,
	meta subscriptionGrantSettlementMeta,
	head *dbent.SubscriptionSettlementOrder,
	userID int64,
	action string,
	plan *dbent.SubscriptionPlan,
	activeBefore *UserSubscription,
	afterSub *UserSubscription,
	note string,
) error {
	if settlementSvc == nil {
		return nil
	}
	if afterSub == nil {
		return ErrSettlementSubscriptionMissing
	}

	carryIn := 0.0
	if activeBefore != nil {
		fallbackBasis := plan.Price
		if activeBefore.PlanPriceSnapshot != nil {
			fallbackBasis = *activeBefore.PlanPriceSnapshot
		}
		carryIn = settlementResidualValue(activeBefore, settlementResidualBasisValue(head, activeBefore, fallbackBasis))
	}

	actionDelta := plan.Price
	afterSettlement := plan.Price
	writeoff := 0.0
	switch action {
	case subscriptionActionPurchase:
		carryIn = 0
	case subscriptionActionRenew:
		afterSettlement = carryIn + actionDelta
	case subscriptionActionUpgrade:
		actionDelta = plan.Price - carryIn
		if actionDelta < 0 {
			actionDelta = 0
		}
		if carryIn > plan.Price {
			writeoff = carryIn - plan.Price
		}
	default:
		return ErrSubscriptionPlanActionInvalid
	}

	var triggerRefID *int64
	if meta.HasTriggerRefID {
		id := meta.TriggerRefID
		triggerRefID = &id
	}
	operatorUserID := meta.OperatorUserID
	if operatorUserID <= 0 {
		operatorUserID = userID
	}

	_, err := settlementSvc.CreateSettlementOrder(ctx, SettlementOrderInput{
		UserID:                  userID,
		OperatorUserID:          operatorUserID,
		ActionType:              action,
		ActionSource:            meta.ActionSource,
		TriggerRefType:          meta.TriggerRefType,
		TriggerRefID:            triggerRefID,
		ActionNote:              note,
		CarryInResidualValue:    carryIn,
		ActionDeltaValue:        actionDelta,
		AfterSettlementValue:    afterSettlement,
		WriteoffValue:           writeoff,
		AfterUserSubscription:   afterSub,
		AfterPlan:               plan,
		AfterSubscriptionStatus: afterSub.Status,
		EffectiveAt:             time.Now(),
	})
	return err
}
