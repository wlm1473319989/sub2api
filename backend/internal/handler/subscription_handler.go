package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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

func toResponsePagination(p *pagination.PaginationResult) *response.PaginationResult {
	if p == nil {
		return nil
	}
	return &response.PaginationResult{
		Total:    p.Total,
		Page:     p.Page,
		PageSize: p.PageSize,
		Pages:    p.Pages,
	}
}

// SubscriptionProgressInfo represents subscription with progress info
type SubscriptionProgressInfo struct {
	Subscription *dto.UserSubscription         `json:"subscription"`
	Progress     *service.SubscriptionProgress `json:"progress"`
}

// SubscriptionHandler handles user subscription operations
type SubscriptionHandler struct {
	subscriptionService      *service.SubscriptionService
	settlementRefundService  *service.SettlementRefundService
}

// NewSubscriptionHandler creates a new user subscription handler
func NewSubscriptionHandler(subscriptionService *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{
		subscriptionService: subscriptionService,
	}
}

// SetSettlementRefundService injects the subscription settlement refund service.
func (h *SubscriptionHandler) SetSettlementRefundService(settlementRefundService *service.SettlementRefundService) {
	h.settlementRefundService = settlementRefundService
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
	applySettlementRefundMarkersToSubscriptions(c.Request.Context(), h.settlementRefundService, subscriptions)

	out := make([]dto.UserSubscription, 0, len(subscriptions))
	for i := range subscriptions {
		out = append(out, *dto.UserSubscriptionFromService(&subscriptions[i]))
	}
	response.Success(c, out)
}

// GetLedger handles listing current user's subscription settlement chain.
// GET /api/v1/subscriptions/ledger
func (h *SubscriptionHandler) GetLedger(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	settlements, err := h.subscriptionService.ListUserSettlementHistory(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.SubscriptionSettlementOrder, 0, len(settlements))
	for i := range settlements {
		out = append(out, *dto.SubscriptionSettlementOrderFromService(&settlements[i]))
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
	applySettlementRefundMarkersToSubscriptions(c.Request.Context(), h.settlementRefundService, subscriptions)

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
	applySettlementRefundMarkersToSubscriptions(c.Request.Context(), h.settlementRefundService, subscriptions)

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

func applySettlementRefundMarkersToSubscriptions(ctx context.Context, settlementRefundService *service.SettlementRefundService, subscriptions []service.UserSubscription) {
	if settlementRefundService == nil || len(subscriptions) == 0 {
		return
	}

	subscriptionIDs := make([]int64, 0, len(subscriptions))
	for i := range subscriptions {
		subscriptionIDs = append(subscriptionIDs, subscriptions[i].ID)
	}
	markers, err := settlementRefundService.GetActiveRefundMarkersBySubscriptionIDs(ctx, subscriptionIDs)
	if err != nil {
		return
	}
	for i := range subscriptions {
		marker, ok := markers[subscriptions[i].ID]
		if !ok {
			continue
		}
		subscriptions[i].RefundFreezeActive = true
		subscriptions[i].ActiveRefundRequestID = &marker.RefundRequestID
		status := marker.Status
		subscriptions[i].ActiveRefundStatus = &status
	}
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

type SettlementRefundPreviewRequest struct {
	Reason string `json:"reason"`
}

type SettlementRefundManualTransferRequest struct {
	ReceiverType           string `json:"receiver_type"`
	ReceiverName           string `json:"receiver_name"`
	ReceiverAccount        string `json:"receiver_account"`
	ReceiverQRCodeImageURL string `json:"receiver_qr_image_url"`
	Remark                 string `json:"remark"`
}

type SettlementRefundSubmitRequest struct {
	PreviewID      int64                            `json:"preview_id" binding:"required"`
	PreviewToken   string                           `json:"preview_token" binding:"required"`
	Reason         string                           `json:"reason"`
	ManualTransfer *SettlementRefundManualTransferRequest `json:"manual_transfer"`
}

// PreviewRefund previews a settlement refund for the current user's subscription.
// POST /api/v1/subscriptions/:id/refund-preview
func (h *SubscriptionHandler) PreviewRefund(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}

	var req SettlementRefundPreviewRequest
	if c.Request != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}

	preview, err := h.settlementRefundService.PreviewSettlementRefund(c.Request.Context(), service.SettlementRefundPreviewInput{
		SubscriptionID: subscriptionID,
		UserID:         subject.UserID,
		Reason:         req.Reason,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}

// RequestRefund submits a settlement refund request for the current user's subscription.
// POST /api/v1/subscriptions/:id/refund-request
func (h *SubscriptionHandler) RequestRefund(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}

	var req SettlementRefundSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var manualTransfer *service.ManualTransferInput
	if req.ManualTransfer != nil {
		manualTransfer = &service.ManualTransferInput{
			ReceiverType:           req.ManualTransfer.ReceiverType,
			ReceiverName:           req.ManualTransfer.ReceiverName,
			ReceiverAccount:        req.ManualTransfer.ReceiverAccount,
			ReceiverQRCodeImageURL: req.ManualTransfer.ReceiverQRCodeImageURL,
			Remark:                 req.ManualTransfer.Remark,
		}
	}

	result, err := h.settlementRefundService.SubmitSettlementRefund(c.Request.Context(), service.SettlementRefundSubmitInput{
		SubscriptionID: subscriptionID,
		UserID:         subject.UserID,
		PreviewID:      req.PreviewID,
		PreviewToken:   req.PreviewToken,
		Reason:         req.Reason,
		ManualTransfer: manualTransfer,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListRefundRequests lists the current user's settlement refund requests.
// GET /api/v1/subscription-refund-requests
func (h *SubscriptionHandler) ListRefundRequests(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	page, pageSize := response.ParsePagination(c)
	items, paginationResult, err := h.settlementRefundService.ListUserSettlementRefundRequests(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.SubscriptionRefundRequest, 0, len(items))
	for i := range items {
		out = append(out, *dto.SubscriptionRefundRequestFromService(&items[i]))
	}
	response.PaginatedWithResult(c, out, toResponsePagination(paginationResult))
}

// GetRefundRequest returns a single settlement refund request owned by the current user.
// GET /api/v1/subscription-refund-requests/:id
func (h *SubscriptionHandler) GetRefundRequest(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	refundRequestID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	view, err := h.settlementRefundService.GetUserSettlementRefundRequestView(c.Request.Context(), subject.UserID, refundRequestID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SubscriptionRefundRequestFromService(view))
}
