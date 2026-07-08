package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// AffiliateGroupGrant records one source-order grant of temporary exclusive
// group access. It is intentionally separate from User.AllowedGroups, which
// remains the permanent access list.
type AffiliateGroupGrant struct {
	ID            int64
	UserID        int64
	GroupID       int64
	SourceUserID  *int64
	SourceOrderID int64
	PayAmount     float64
	GrantDays     int
	StartsAt      time.Time
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AffiliateGroupGrantInput struct {
	UserID        int64
	GroupID       int64
	SourceUserID  int64
	SourceOrderID int64
	PayAmount     float64
	GrantDays     int
	StartsAt      time.Time
	Now           time.Time
}

type TimedGroupGrant struct {
	GroupID   int64
	ExpiresAt time.Time
}

type AffiliateGroupGrantRepository interface {
	GrantForOrder(ctx context.Context, input AffiliateGroupGrantInput) (*AffiliateGroupGrant, bool, error)
	ListActiveByUserID(ctx context.Context, userID int64, now time.Time) ([]TimedGroupGrant, error)
	ClaimExpiredForProcessing(ctx context.Context, now time.Time, limit int) ([]int64, error)
}

type AffiliateGroupGrantService struct {
	repo                 AffiliateGroupGrantRepository
	affiliateRepo        AffiliateRepository
	settingService       *SettingService
	groupRepo            GroupRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	now                  func() time.Time
}

func NewAffiliateGroupGrantService(
	repo AffiliateGroupGrantRepository,
	affiliateRepo AffiliateRepository,
	settingService *SettingService,
	groupRepo GroupRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
) *AffiliateGroupGrantService {
	return &AffiliateGroupGrantService{
		repo:                 repo,
		affiliateRepo:        affiliateRepo,
		settingService:       settingService,
		groupRepo:            groupRepo,
		authCacheInvalidator: authCacheInvalidator,
		now:                  timezone.Now,
	}
}

func (s *AffiliateGroupGrantService) SetNowForTest(now func() time.Time) {
	if s == nil || now == nil {
		return
	}
	s.now = now
}

// ApplyForPaymentOrder grants temporary access to the configured exclusive
// group when an invitee completes a qualifying payment.
func (s *AffiliateGroupGrantService) ApplyForPaymentOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	if s == nil || s.repo == nil || s.affiliateRepo == nil || s.settingService == nil || s.groupRepo == nil || o == nil {
		return nil
	}
	if !affiliateGroupGrantOrderTypeEligible(o.OrderType) || o.PayAmount <= 0 || math.IsNaN(o.PayAmount) || math.IsInf(o.PayAmount, 0) {
		return nil
	}
	if !s.settingService.IsAffiliateEnabled(ctx) || !s.settingService.IsAffiliateGroupGrantEnabled(ctx) {
		return nil
	}

	groupID := s.settingService.GetAffiliateGroupGrantGroupID(ctx)
	if groupID <= 0 {
		return nil
	}

	now := s.now()
	inviteeSummary, err := s.affiliateRepo.EnsureUserAffiliate(ctx, o.UserID)
	if err != nil {
		return fmt.Errorf("ensure invitee affiliate profile: %w", err)
	}
	if inviteeSummary == nil || inviteeSummary.InviterID == nil || *inviteeSummary.InviterID <= 0 {
		return nil
	}
	if durationDays := s.settingService.GetAffiliateRebateDurationDays(ctx); durationDays > 0 {
		if now.After(inviteeSummary.CreatedAt.AddDate(0, 0, durationDays)) {
			return nil
		}
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		slog.Warn("affiliate group grant skipped: configured group not found", "group_id", groupID, "error", err)
		return nil
	}
	if group == nil || !group.IsActive() || !group.IsExclusive {
		slog.Warn("affiliate group grant skipped: configured group is not an active exclusive group", "group_id", groupID)
		return nil
	}

	grantDays := AffiliateGroupGrantDays(o.PayAmount)
	startsAt, _ := AffiliateGroupGrantNaturalPeriod(now, grantDays)
	grant, applied, err := s.repo.GrantForOrder(ctx, AffiliateGroupGrantInput{
		UserID:        *inviteeSummary.InviterID,
		GroupID:       groupID,
		SourceUserID:  o.UserID,
		SourceOrderID: o.ID,
		PayAmount:     o.PayAmount,
		GrantDays:     grantDays,
		StartsAt:      startsAt,
		Now:           now,
	})
	if err != nil {
		return fmt.Errorf("grant affiliate exclusive group access: %w", err)
	}
	if applied && grant != nil && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, grant.UserID)
	}
	return nil
}

func affiliateGroupGrantOrderTypeEligible(orderType string) bool {
	switch orderType {
	case payment.OrderTypeBalance, payment.OrderTypeSubscription:
		return true
	default:
		return false
	}
}

// AffiliateGroupGrantDays converts actual paid amount into natural days:
// 0 < amount <= 19.99 grants 1 day, 20 grants 2 days, etc.
func AffiliateGroupGrantDays(payAmount float64) int {
	if payAmount <= 0 || math.IsNaN(payAmount) || math.IsInf(payAmount, 0) {
		return 0
	}
	days := int(math.Floor(payAmount / 10))
	if days < 1 {
		days = 1
	}
	return days
}

func AffiliateGroupGrantNaturalPeriod(now time.Time, days int) (time.Time, time.Time) {
	if days <= 0 {
		days = 1
	}
	startsAt := timezone.StartOfDay(now)
	return startsAt, startsAt.AddDate(0, 0, days)
}
