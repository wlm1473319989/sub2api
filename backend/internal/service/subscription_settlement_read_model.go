package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type AdminSubscriptionDetail struct {
	Subscription          *UserSubscription
	CurrentSettlementHead *SubscriptionSettlementOrderView
	SettlementHistory     []SubscriptionSettlementOrderView
}

type SubscriptionSettlementOrderView struct {
	ID                              int64
	UserID                          int64
	PrevSettlementID                *int64
	ActionType                      string
	ActionSource                    string
	Status                          string
	TriggerRefType                  string
	TriggerRefID                    *int64
	OperatorUserID                  int64
	ActionNote                      *string
	CarryInResidualValue            float64
	ActionDeltaValue                float64
	AfterSettlementValue            float64
	RefundResidualValue             *float64
	WriteoffValue                   float64
	AfterUserSubscriptionID         *int64
	AfterPlanID                     *int64
	AfterPlanNameSnapshot           *string
	AfterPlanPriceSnapshot          *float64
	AfterValidityDaysSnapshot       *int
	AfterValidityUnitSnapshot       *string
	AfterStartsAt                   *time.Time
	AfterExpiresAt                  *time.Time
	AfterDailyQuotaKnivesSnapshot   *float64
	AfterWeeklyQuotaKnivesSnapshot  *float64
	AfterMonthlyQuotaKnivesSnapshot *float64
	AfterSubscriptionStatus         string
	EffectiveAt                     time.Time
	ClosedAt                        *time.Time
	CreatedAt                       time.Time
	UpdatedAt                       time.Time
}

func (s *SubscriptionService) GetAdminSubscriptionDetail(ctx context.Context, subscriptionID int64) (*AdminSubscriptionDetail, error) {
	subscription, err := s.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if s.entClient == nil {
		return nil, ErrSettlementEntClientRequired
	}

	client := s.entClient
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}

	settlements, err := client.SubscriptionSettlementOrder.Query().
		Where(subscriptionsettlementorder.UserIDEQ(subscription.UserID)).
		Order(
			dbent.Asc(subscriptionsettlementorder.FieldEffectiveAt),
			dbent.Asc(subscriptionsettlementorder.FieldID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query subscription settlement history: %w", err)
	}

	history := make([]SubscriptionSettlementOrderView, 0, len(settlements))
	var current *SubscriptionSettlementOrderView
	for _, settlement := range settlements {
		view := subscriptionSettlementOrderViewFromEnt(settlement)
		history = append(history, view)
		if settlement.Status == domain.SettlementStatusEffective {
			currentView := view
			current = &currentView
		}
	}

	return &AdminSubscriptionDetail{
		Subscription:          subscription,
		CurrentSettlementHead: current,
		SettlementHistory:     history,
	}, nil
}

func subscriptionSettlementOrderViewFromEnt(settlement *dbent.SubscriptionSettlementOrder) SubscriptionSettlementOrderView {
	if settlement == nil {
		return SubscriptionSettlementOrderView{}
	}
	return SubscriptionSettlementOrderView{
		ID:                              settlement.ID,
		UserID:                          settlement.UserID,
		PrevSettlementID:                settlement.PrevSettlementID,
		ActionType:                      settlement.ActionType,
		ActionSource:                    settlement.ActionSource,
		Status:                          settlement.Status,
		TriggerRefType:                  settlement.TriggerRefType,
		TriggerRefID:                    settlement.TriggerRefID,
		OperatorUserID:                  settlement.OperatorUserID,
		ActionNote:                      settlement.ActionNote,
		CarryInResidualValue:            settlement.CarryInResidualValue,
		ActionDeltaValue:                settlement.ActionDeltaValue,
		AfterSettlementValue:            settlement.AfterSettlementValue,
		RefundResidualValue:             settlement.RefundResidualValue,
		WriteoffValue:                   settlement.WriteoffValue,
		AfterUserSubscriptionID:         settlement.AfterUserSubscriptionID,
		AfterPlanID:                     settlement.AfterPlanID,
		AfterPlanNameSnapshot:           settlement.AfterPlanNameSnapshot,
		AfterPlanPriceSnapshot:          settlement.AfterPlanPriceSnapshot,
		AfterValidityDaysSnapshot:       settlement.AfterValidityDaysSnapshot,
		AfterValidityUnitSnapshot:       settlement.AfterValidityUnitSnapshot,
		AfterStartsAt:                   settlement.AfterStartsAt,
		AfterExpiresAt:                  settlement.AfterExpiresAt,
		AfterDailyQuotaKnivesSnapshot:   settlement.AfterDailyQuotaKnivesSnapshot,
		AfterWeeklyQuotaKnivesSnapshot:  settlement.AfterWeeklyQuotaKnivesSnapshot,
		AfterMonthlyQuotaKnivesSnapshot: settlement.AfterMonthlyQuotaKnivesSnapshot,
		AfterSubscriptionStatus:         settlement.AfterSubscriptionStatus,
		EffectiveAt:                     settlement.EffectiveAt,
		ClosedAt:                        settlement.ClosedAt,
		CreatedAt:                       settlement.CreatedAt,
		UpdatedAt:                       settlement.UpdatedAt,
	}
}
