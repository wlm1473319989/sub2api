package handler

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SubscriptionSummaryItem represents a subscription item in summary
type SubscriptionSummaryItem struct {
	ID                 int64    `json:"id"`
	DisplayName        string   `json:"display_name"`
	PlanID             *int64   `json:"plan_id,omitempty"`
	PlanNameSnapshot   *string  `json:"plan_name_snapshot,omitempty"`
	Status             string   `json:"status"`
	DailyUsedUSD       float64  `json:"daily_used_usd,omitempty"`
	DailyQuotaKnives   *float64 `json:"daily_quota_knives,omitempty"`
	DailyUsedKnives    float64  `json:"daily_used_knives,omitempty"`
	WeeklyUsedUSD      float64  `json:"weekly_used_usd,omitempty"`
	WeeklyQuotaKnives  *float64 `json:"weekly_quota_knives,omitempty"`
	WeeklyUsedKnives   float64  `json:"weekly_used_knives,omitempty"`
	MonthlyUsedUSD     float64  `json:"monthly_used_usd,omitempty"`
	MonthlyQuotaKnives *float64 `json:"monthly_quota_knives,omitempty"`
	MonthlyUsedKnives  float64  `json:"monthly_used_knives,omitempty"`
	ExpiresAt          *string  `json:"expires_at,omitempty"`
}

// SubscriptionProgressInfo represents subscription with progress info
type SubscriptionProgressInfo struct {
	Subscription *dto.UserSubscription         `json:"subscription"`
	Progress     *service.SubscriptionProgress `json:"progress"`
}

// SubscriptionHandler handles user subscription operations
type SubscriptionHandler struct {
	subscriptionService *service.SubscriptionService
}

// NewSubscriptionHandler creates a new user subscription handler
func NewSubscriptionHandler(subscriptionService *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{
		subscriptionService: subscriptionService,
	}
}

// List handles listing current user's subscriptions
// GET /api/v1/subscriptions
func (h *SubscriptionHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	subscriptions, err := h.subscriptionService.ListUserSubscriptions(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.UserSubscription, 0, len(subscriptions))
	for i := range subscriptions {
		out = append(out, *dto.UserSubscriptionFromService(&subscriptions[i]))
	}
	response.Success(c, out)
}

// GetActive handles getting current user's active subscriptions
// GET /api/v1/subscriptions/active
func (h *SubscriptionHandler) GetActive(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	subscriptions, err := h.subscriptionService.ListActiveUserSubscriptions(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.UserSubscription, 0, len(subscriptions))
	for i := range subscriptions {
		out = append(out, *dto.UserSubscriptionFromService(&subscriptions[i]))
	}
	response.Success(c, out)
}

// GetProgress handles getting subscription progress for current user
// GET /api/v1/subscriptions/progress
func (h *SubscriptionHandler) GetProgress(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	// Get all active subscriptions with progress
	subscriptions, err := h.subscriptionService.ListActiveUserSubscriptions(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result := make([]SubscriptionProgressInfo, 0, len(subscriptions))
	for i := range subscriptions {
		sub := &subscriptions[i]
		progress, err := h.subscriptionService.GetSubscriptionProgress(c.Request.Context(), sub.ID)
		if err != nil {
			// Skip subscriptions with errors
			continue
		}
		result = append(result, SubscriptionProgressInfo{
			Subscription: dto.UserSubscriptionFromService(sub),
			Progress:     progress,
		})
	}

	response.Success(c, result)
}

// GetSummary handles getting a summary of current user's subscription status
// GET /api/v1/subscriptions/summary
func (h *SubscriptionHandler) GetSummary(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	// Get all active subscriptions
	subscriptions, err := h.subscriptionService.ListActiveUserSubscriptions(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	var totalUsed float64
	items := make([]SubscriptionSummaryItem, 0, len(subscriptions))

	for _, sub := range subscriptions {
		item := SubscriptionSummaryItem{
			ID:                 sub.ID,
			PlanID:             sub.PlanID,
			PlanNameSnapshot:   sub.PlanNameSnapshot,
			Status:             sub.Status,
			DailyUsedUSD:       sub.DailyUsageUSD,
			DailyQuotaKnives:   sub.DailyQuotaKnives,
			DailyUsedKnives:    sub.DailyUsedKnives,
			WeeklyUsedUSD:      sub.WeeklyUsageUSD,
			WeeklyQuotaKnives:  sub.WeeklyQuotaKnives,
			WeeklyUsedKnives:   sub.WeeklyUsedKnives,
			MonthlyUsedUSD:     sub.MonthlyUsageUSD,
			MonthlyQuotaKnives: sub.MonthlyQuotaKnives,
			MonthlyUsedKnives:  sub.MonthlyUsedKnives,
		}

		if sub.PlanNameSnapshot != nil {
			item.DisplayName = strings.TrimSpace(*sub.PlanNameSnapshot)
		}

		if item.DisplayName == "" {
			item.DisplayName = fmt.Sprintf("Subscription #%d", sub.ID)
		}

		// Format expiration time
		if !sub.ExpiresAt.IsZero() {
			formatted := sub.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
			item.ExpiresAt = &formatted
		}

		// Track total usage (use monthly as the most comprehensive)
		totalUsed += sub.MonthlyUsageUSD

		items = append(items, item)
	}

	summary := struct {
		ActiveCount   int                       `json:"active_count"`
		TotalUsedUSD  float64                   `json:"total_used_usd"`
		Subscriptions []SubscriptionSummaryItem `json:"subscriptions"`
	}{
		ActiveCount:   len(subscriptions),
		TotalUsedUSD:  totalUsed,
		Subscriptions: items,
	}

	response.Success(c, summary)
}
