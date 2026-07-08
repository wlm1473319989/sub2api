package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

const (
	affiliateGroupGrantExpiryLeaderLockKey = "affiliate:group_grant:expiry:leader"
	affiliateGroupGrantExpiryLeaderLockTTL = 10 * time.Minute
	affiliateGroupGrantExpiryTimeout       = 5 * time.Minute
	affiliateGroupGrantExpiryBatchSize     = 500
	affiliateGroupGrantExpiryHour          = 0
	affiliateGroupGrantExpiryMinute        = 5
)

// AffiliateGroupGrantExpiryService clears API-key auth caches for users whose
// temporary group grants have expired. The request path still enforces
// expires_at > now; this job is a cache-cleanup fallback.
type AffiliateGroupGrantExpiryService struct {
	repo                 AffiliateGroupGrantRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	now      func() time.Time

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewAffiliateGroupGrantExpiryService(repo AffiliateGroupGrantRepository, authCacheInvalidator APIKeyAuthCacheInvalidator) *AffiliateGroupGrantExpiryService {
	return &AffiliateGroupGrantExpiryService{
		repo:                 repo,
		authCacheInvalidator: authCacheInvalidator,
		stopCh:               make(chan struct{}),
		now:                  timezone.Now,
		instanceID:           uuid.NewString(),
	}
}

func (s *AffiliateGroupGrantExpiryService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *AffiliateGroupGrantExpiryService) SetNowForTest(now func() time.Time) {
	if s == nil || now == nil {
		return
	}
	s.now = now
}

func (s *AffiliateGroupGrantExpiryService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.runOnce()
		for {
			now := s.now()
			delay := NextAffiliateGroupGrantExpiryRun(now).Sub(now)
			if delay < 0 {
				delay = 0
			}
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
				s.runOnce()
			case <-s.stopCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
		}
	}()
}

func (s *AffiliateGroupGrantExpiryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func NextAffiliateGroupGrantExpiryRun(now time.Time) time.Time {
	loc := timezone.Location()
	now = now.In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), affiliateGroupGrantExpiryHour, affiliateGroupGrantExpiryMinute, 0, 0, loc)
	if !now.Before(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func (s *AffiliateGroupGrantExpiryService) runOnce() {
	if s == nil || s.repo == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), affiliateGroupGrantExpiryTimeout)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, affiliateGroupGrantExpiryLeaderLockKey, s.instanceID, affiliateGroupGrantExpiryLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	var invalidated int64
	for {
		userIDs, err := s.repo.ClaimExpiredForProcessing(ctx, s.now(), affiliateGroupGrantExpiryBatchSize)
		if err != nil {
			slog.Error("[AffiliateGroupGrantExpiry] failed to claim expired grants", "error", err)
			return
		}
		if len(userIDs) == 0 {
			break
		}
		for _, userID := range userIDs {
			if userID <= 0 || s.authCacheInvalidator == nil {
				continue
			}
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
			invalidated++
		}
	}
	if invalidated > 0 {
		slog.Info("[AffiliateGroupGrantExpiry] invalidated auth caches for expired grants", "users", invalidated)
	}
}
