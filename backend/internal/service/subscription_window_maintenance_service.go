package service

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
)

const (
	subscriptionWindowMaintenanceLeaderLockKey = "subscription:window:maintenance:leader"
	subscriptionWindowMaintenanceLeaderLockTTL = 30 * time.Minute
	subscriptionWindowMaintenanceTimeout       = 5 * time.Minute
	subscriptionWindowMaintenancePageSize      = 200
	subscriptionWindowMaintenanceHour          = 2
)

// SubscriptionWindowMaintenanceService proactively resets expired subscription
// usage windows once per day. The request-time lazy maintenance remains as the
// fallback for the interval before this job runs.
type SubscriptionWindowMaintenanceService struct {
	userSubRepo    UserSubscriptionRepository
	subscriptionSV *SubscriptionService

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	now      func() time.Time

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewSubscriptionWindowMaintenanceService(userSubRepo UserSubscriptionRepository, subscriptionSV *SubscriptionService) *SubscriptionWindowMaintenanceService {
	return &SubscriptionWindowMaintenanceService{
		userSubRepo:    userSubRepo,
		subscriptionSV: subscriptionSV,
		stopCh:         make(chan struct{}),
		now:            time.Now,
		instanceID:     uuid.NewString(),
	}
}

func (s *SubscriptionWindowMaintenanceService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *SubscriptionWindowMaintenanceService) Start() {
	if s == nil || s.userSubRepo == nil || s.subscriptionSV == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			now := s.now()
			delay := nextSubscriptionWindowMaintenanceRun(now).Sub(now)
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

func (s *SubscriptionWindowMaintenanceService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func nextSubscriptionWindowMaintenanceRun(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), subscriptionWindowMaintenanceHour, 0, 0, 0, now.Location())
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func (s *SubscriptionWindowMaintenanceService) runOnce() {
	if s == nil || s.userSubRepo == nil || s.subscriptionSV == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), subscriptionWindowMaintenanceTimeout)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, subscriptionWindowMaintenanceLeaderLockKey, s.instanceID, subscriptionWindowMaintenanceLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	var scanned, maintained, failed int64
	for page := 1; ; page++ {
		subs, pag, err := s.userSubRepo.List(
			ctx,
			pagination.PaginationParams{Page: page, PageSize: subscriptionWindowMaintenancePageSize},
			nil,
			SubscriptionStatusActive,
			"expires_at",
			"asc",
		)
		if err != nil {
			log.Printf("[SubscriptionWindowMaintenance] List active subscriptions failed: %v", err)
			return
		}

		scanned += int64(len(subs))
		for i := range subs {
			sub := &subs[i]
			if !sub.NeedsDailyReset() && !sub.NeedsWeeklyReset() && !sub.NeedsMonthlyReset() {
				continue
			}
			if err := s.subscriptionSV.CheckAndResetWindows(ctx, sub); err != nil {
				failed++
				log.Printf("[SubscriptionWindowMaintenance] Reset windows failed: subscription=%d user=%d err=%v", sub.ID, sub.UserID, err)
				continue
			}
			maintained++
		}

		if pag == nil || len(subs) == 0 || page >= pag.Pages {
			break
		}
	}

	if maintained > 0 || failed > 0 {
		log.Printf("[SubscriptionWindowMaintenance] scanned=%d maintained=%d failed=%d", scanned, maintained, failed)
	}
}
