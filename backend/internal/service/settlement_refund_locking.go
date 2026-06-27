package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *SettlementRefundService) loadLockedSubscriptionByID(ctx context.Context, subscriptionID int64) (*UserSubscription, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		sub, err := tx.Client().UserSubscription.Query().
			Where(usersubscription.IDEQ(subscriptionID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if dbent.IsNotFound(err) {
				return nil, ErrSubscriptionNotFound
			}
			return nil, fmt.Errorf("lock settlement refund subscription %d: %w", subscriptionID, err)
		}
		return userSubscriptionEntityToService(sub), nil
	}

	if s == nil || s.subscription == nil || s.subscription.userSubRepo == nil {
		return nil, infraerrors.InternalServer("SUBSCRIPTION_SERVICE_REQUIRED", "subscription service is required")
	}
	return s.subscription.userSubRepo.GetByID(ctx, subscriptionID)
}

func (s *SettlementRefundService) loadLockedEffectiveHead(ctx context.Context, userID int64, now time.Time) (*dbent.SubscriptionSettlementOrder, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if now.IsZero() {
			now = time.Now()
		}
		head, err := tx.Client().SubscriptionSettlementOrder.Query().
			Where(
				subscriptionsettlementorder.UserIDEQ(userID),
				subscriptionsettlementorder.StatusEQ(domain.SettlementStatusEffective),
				subscriptionsettlementorder.AfterSubscriptionStatusEQ(domain.SubscriptionStatusActive),
				subscriptionsettlementorder.AfterExpiresAtGT(now),
			).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if dbent.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("lock settlement refund effective head: %w", err)
		}
		return head, nil
	}

	return s.previewLoadEffectiveHead(ctx, userID, now)
}
