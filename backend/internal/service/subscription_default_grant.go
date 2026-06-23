package service

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *SubscriptionService) GrantConfiguredSubscription(ctx context.Context, userID int64, item DefaultSubscriptionSetting, notes string) (*UserSubscription, bool, error) {
	if item.PlanID > 0 {
		plan, err := s.resolveDefaultGrantPlan(ctx, item.PlanID)
		if err != nil {
			return nil, false, err
		}
		settlementMeta, hasSettlement, settlementSvc, settlementHead, err := s.prepareGrantSettlement(ctx, userID)
		if err != nil {
			return nil, false, err
		}

		active, err := s.userSubRepo.GetActiveByUserID(ctx, userID)
		if err != nil {
			if errorsIsSubscriptionNotFound(err) {
				if hasSettlement && settlementHead != nil {
					return nil, false, ErrActiveSubscriptionRequired
				}
				sub, purchaseErr := s.PurchaseNewPlan(ctx, &PurchaseNewPlanInput{
					UserID: userID,
					Plan:   plan,
					Notes:  notes,
				})
				if purchaseErr != nil {
					return nil, false, purchaseErr
				}
				if err := s.createGrantSettlementOrder(ctx, settlementSvc, settlementMeta, settlementHead, userID, subscriptionActionPurchase, plan, nil, sub, notes); err != nil {
					return nil, false, err
				}
				return sub, false, nil
			}
			return nil, false, err
		}

		settlementAction := ""
		if hasSettlement && settlementHead != nil {
			decision, decisionErr := settlementSvc.DeterminePlanAction(settlementHead, plan)
			if decisionErr != nil {
				return nil, false, decisionErr
			}
			switch decision.Action {
			case subscriptionActionRenew, subscriptionActionUpgrade:
				settlementAction = decision.Action
			default:
				return nil, false, ErrSubscriptionPlanActionInvalid
			}
		}

		currentPlanID, currentPrice, resolveErr := s.resolveActiveGrantReference(ctx, active)
		if resolveErr != nil {
			return nil, false, resolveErr
		}

		if settlementAction == subscriptionActionRenew || (settlementAction == "" && currentPlanID != nil && *currentPlanID == plan.ID) {
			sub, renewErr := s.RenewActivePlan(ctx, &RenewActivePlanInput{
				UserID: userID,
				Plan:   plan,
				Notes:  notes,
			})
			if renewErr != nil {
				return nil, true, renewErr
			}
			if err := s.createGrantSettlementOrder(ctx, settlementSvc, settlementMeta, settlementHead, userID, subscriptionActionRenew, plan, active, sub, notes); err != nil {
				return nil, true, err
			}
			return sub, true, nil
		}

		if settlementAction == subscriptionActionUpgrade || (settlementAction == "" && currentPrice != nil && plan.Price > *currentPrice) {
			result, upgradeErr := s.UpgradeActivePlan(ctx, &UpgradeActivePlanInput{
				UserID:     userID,
				TargetPlan: plan,
				Notes:      notes,
			})
			if upgradeErr != nil {
				return nil, false, upgradeErr
			}
			if err := s.createGrantSettlementOrder(ctx, settlementSvc, settlementMeta, settlementHead, userID, subscriptionActionUpgrade, plan, active, result.Current, notes); err != nil {
				return nil, false, err
			}
			return result.Current, false, nil
		}

		if hasSettlement {
			return nil, false, ErrSubscriptionPlanActionInvalid
		}
		return active, true, nil
	}

	return nil, false, infraerrors.BadRequest("PLAN_ID_REQUIRED", "plan_id is required")
}

func (s *SubscriptionService) AssignUserLevelSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error) {
	if input == nil {
		return nil, false, ErrSubscriptionNilInput
	}
	if input.PlanID <= 0 {
		return nil, false, infraerrors.BadRequest("PLAN_ID_REQUIRED", "plan_id is required")
	}

	plan, err := s.resolveDefaultGrantPlan(ctx, input.PlanID)
	if err != nil {
		return nil, false, err
	}

	settlementMeta := subscriptionGrantSettlementMeta{
		ActionSource:   domain.SettlementActionSourceSubscriptionAssign,
		TriggerRefType: domain.SettlementTriggerRefAdminAssignment,
		OperatorUserID: input.AssignedBy,
	}
	settlementSvc := NewSettlementService(s.entClient)
	settlementHead, err := settlementSvc.GetEffectiveHead(ctx, input.UserID, time.Now())
	if err != nil {
		return nil, false, err
	}

	active, err := s.userSubRepo.GetActiveByUserID(ctx, input.UserID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			if settlementHead != nil {
				return nil, false, ErrActiveSubscriptionRequired
			}
			sub, purchaseErr := s.PurchaseNewPlan(ctx, &PurchaseNewPlanInput{
				UserID:     input.UserID,
				Plan:       plan,
				AssignedBy: input.AssignedBy,
				Notes:      input.Notes,
			})
			if purchaseErr != nil {
				return nil, false, purchaseErr
			}
			if err := s.createGrantSettlementOrder(ctx, settlementSvc, settlementMeta, settlementHead, input.UserID, subscriptionActionPurchase, plan, nil, sub, input.Notes); err != nil {
				return nil, false, err
			}
			return sub, false, nil
		}
		return nil, false, err
	}

	settlementAction := ""
	if settlementHead != nil {
		decision, decisionErr := settlementSvc.DeterminePlanAction(settlementHead, plan)
		if decisionErr != nil {
			return nil, false, decisionErr
		}
		switch decision.Action {
		case subscriptionActionRenew, subscriptionActionUpgrade:
			settlementAction = decision.Action
		default:
			return nil, false, ErrSubscriptionPlanActionInvalid
		}
	}

	currentPlanID, currentPrice, resolveErr := s.resolveActiveGrantReference(ctx, active)
	if resolveErr != nil {
		return nil, false, resolveErr
	}

	if settlementAction == subscriptionActionRenew || (settlementAction == "" && currentPlanID != nil && *currentPlanID == plan.ID) {
		sub, renewErr := s.RenewActivePlan(ctx, &RenewActivePlanInput{
			UserID: input.UserID,
			Plan:   plan,
			Notes:  input.Notes,
		})
		if renewErr != nil {
			return nil, true, renewErr
		}
		if err := s.createGrantSettlementOrder(ctx, settlementSvc, settlementMeta, settlementHead, input.UserID, subscriptionActionRenew, plan, active, sub, input.Notes); err != nil {
			return nil, true, err
		}
		return sub, true, nil
	}

	if settlementAction == subscriptionActionUpgrade || (settlementAction == "" && currentPrice != nil && plan.Price > *currentPrice) {
		result, upgradeErr := s.UpgradeActivePlan(ctx, &UpgradeActivePlanInput{
			UserID:     input.UserID,
			TargetPlan: plan,
			AssignedBy: input.AssignedBy,
			Notes:      input.Notes,
		})
		if upgradeErr != nil {
			return nil, false, upgradeErr
		}
		if err := s.createGrantSettlementOrder(ctx, settlementSvc, settlementMeta, settlementHead, input.UserID, subscriptionActionUpgrade, plan, active, result.Current, input.Notes); err != nil {
			return nil, false, err
		}
		return result.Current, false, nil
	}

	return nil, false, ErrSubscriptionPlanActionInvalid
}

func (s *SubscriptionService) resolveDefaultGrantPlan(ctx context.Context, planID int64) (*dbent.SubscriptionPlan, error) {
	if s == nil || s.entClient == nil {
		return nil, ErrSubscriptionPlanRequired
	}
	plan, err := s.entClient.SubscriptionPlan.Get(ctx, planID)
	if err != nil {
		return nil, ErrSubscriptionPlanRequired.WithCause(err)
	}
	return normalizePlanEntity(plan), nil
}

func (s *SubscriptionService) resolveActiveGrantReference(ctx context.Context, active *UserSubscription) (*int64, *float64, error) {
	if active == nil {
		return nil, nil, nil
	}
	if active.PlanID != nil && active.PlanPriceSnapshot != nil {
		return active.PlanID, active.PlanPriceSnapshot, nil
	}
	if active.PlanID == nil {
		return nil, active.PlanPriceSnapshot, nil
	}
	if s == nil || s.entClient == nil {
		return active.PlanID, active.PlanPriceSnapshot, nil
	}

	query := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(active.UserID),
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
	if active.PlanID != nil {
		query = query.Where(paymentorder.PlanIDEQ(*active.PlanID))
	}

	order, err := query.First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return active.PlanID, active.PlanPriceSnapshot, nil
		}
		return nil, nil, err
	}

	planID := active.PlanID
	if planID == nil {
		planID = order.PlanID
	}
	price := active.PlanPriceSnapshot
	if price == nil {
		switch {
		case order.SubscriptionPlanPriceSnapshot != nil:
			price = order.SubscriptionPlanPriceSnapshot
		case order.PlanID != nil:
			plan, planErr := s.entClient.SubscriptionPlan.Get(ctx, *order.PlanID)
			if planErr == nil {
				price = copyFloat64Pointer(&plan.Price)
			}
		}
	}
	return planID, price, nil
}
