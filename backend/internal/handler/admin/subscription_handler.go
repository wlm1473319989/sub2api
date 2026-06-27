package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// toResponsePagination converts pagination.PaginationResult to response.PaginationResult
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

// SubscriptionHandler handles admin subscription management
type SubscriptionHandler struct {
	subscriptionService     *service.SubscriptionService
	settlementRefundService *service.SettlementRefundService
}

// NewSubscriptionHandler creates a new admin subscription handler
func NewSubscriptionHandler(subscriptionService *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{
		subscriptionService: subscriptionService,
	}
}

// SetSettlementRefundService injects the subscription settlement refund service.
func (h *SubscriptionHandler) SetSettlementRefundService(settlementRefundService *service.SettlementRefundService) {
	h.settlementRefundService = settlementRefundService
}

// AssignSubscriptionRequest represents assign subscription request
type AssignSubscriptionRequest struct {
	UserID       int64  `json:"user_id" binding:"required"`
	PlanID       int64  `json:"plan_id" binding:"required"`
	ValidityDays int    `json:"validity_days" binding:"omitempty,max=36500"` // max 100 years
	Notes        string `json:"notes"`
}

// BulkAssignSubscriptionRequest represents bulk assign subscription request
type BulkAssignSubscriptionRequest struct {
	UserIDs      []int64 `json:"user_ids" binding:"required,min=1"`
	PlanID       int64   `json:"plan_id" binding:"required"`
	ValidityDays int     `json:"validity_days" binding:"omitempty,max=36500"` // max 100 years
	Notes        string  `json:"notes"`
}

// AdjustSubscriptionRequest represents adjust subscription request (extend or shorten)
type AdjustSubscriptionRequest struct {
	Days int `json:"days" binding:"required,min=-36500,max=36500"` // negative to shorten, positive to extend
}

// BulkAdjustSubscriptionRequest represents bulk adjust subscription request.
type BulkAdjustSubscriptionRequest struct {
	SubscriptionIDs []int64 `json:"subscription_ids" binding:"required,min=1"`
	Days            int     `json:"days" binding:"required,min=-36500,max=36500"`
}

// BulkResetSubscriptionQuotaRequest represents bulk reset quota request.
type BulkResetSubscriptionQuotaRequest struct {
	SubscriptionIDs []int64 `json:"subscription_ids" binding:"required,min=1"`
	Daily           bool    `json:"daily"`
	Weekly          bool    `json:"weekly"`
	Monthly         bool    `json:"monthly"`
}

// List handles listing all subscriptions with pagination and filters
// GET /api/v1/admin/subscriptions
func (h *SubscriptionHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)

	// Parse optional filters
	var userID *int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			userID = &id
		}
	}
	status := c.Query("status")

	// Parse sorting parameters
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	subscriptions, pagination, err := h.subscriptionService.List(c.Request.Context(), page, pageSize, userID, status, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	applySettlementRefundMarkersToSubscriptions(c, h.settlementRefundService, subscriptions)

	out := make([]dto.AdminUserSubscription, 0, len(subscriptions))
	for i := range subscriptions {
		out = append(out, *dto.UserSubscriptionFromServiceAdmin(&subscriptions[i]))
	}
	response.PaginatedWithResult(c, out, toResponsePagination(pagination))
}

// GetByID handles getting a subscription by ID
// GET /api/v1/admin/subscriptions/:id
func (h *SubscriptionHandler) GetByID(c *gin.Context) {
	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}

	detail, err := h.subscriptionService.GetAdminSubscriptionDetail(c.Request.Context(), subscriptionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.AdminUserSubscriptionDetailFromService(detail))
}

func applySettlementRefundMarkersToSubscriptions(c *gin.Context, settlementRefundService *service.SettlementRefundService, subscriptions []service.UserSubscription) {
	if settlementRefundService == nil || len(subscriptions) == 0 {
		return
	}

	subscriptionIDs := make([]int64, 0, len(subscriptions))
	for i := range subscriptions {
		subscriptionIDs = append(subscriptionIDs, subscriptions[i].ID)
	}
	markers, err := settlementRefundService.GetActiveRefundMarkersBySubscriptionIDs(c.Request.Context(), subscriptionIDs)
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

// GetProgress handles getting subscription usage progress
// GET /api/v1/admin/subscriptions/:id/progress
func (h *SubscriptionHandler) GetProgress(c *gin.Context) {
	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}

	progress, err := h.subscriptionService.GetSubscriptionProgress(c.Request.Context(), subscriptionID)
	if err != nil {
		response.NotFound(c, "Subscription not found")
		return
	}

	response.Success(c, progress)
}

// Assign handles assigning a subscription to a user
// POST /api/v1/admin/subscriptions/assign
func (h *SubscriptionHandler) Assign(c *gin.Context) {
	var req AssignSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.PlanID <= 0 {
		response.BadRequest(c, "plan_id is required")
		return
	}

	// Get admin user ID from context
	adminID := getAdminIDFromContext(c)

	subscription, err := h.subscriptionService.AssignSubscription(c.Request.Context(), &service.AssignSubscriptionInput{
		UserID:       req.UserID,
		PlanID:       req.PlanID,
		ValidityDays: req.ValidityDays,
		AssignedBy:   adminID,
		Notes:        req.Notes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserSubscriptionFromServiceAdmin(subscription))
}

// BulkAssign handles bulk assigning subscriptions to multiple users
// POST /api/v1/admin/subscriptions/bulk-assign
func (h *SubscriptionHandler) BulkAssign(c *gin.Context) {
	var req BulkAssignSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.PlanID <= 0 {
		response.BadRequest(c, "plan_id is required")
		return
	}

	// Get admin user ID from context
	adminID := getAdminIDFromContext(c)

	result, err := h.subscriptionService.BulkAssignSubscription(c.Request.Context(), &service.BulkAssignSubscriptionInput{
		UserIDs:      req.UserIDs,
		PlanID:       req.PlanID,
		ValidityDays: req.ValidityDays,
		AssignedBy:   adminID,
		Notes:        req.Notes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.BulkAssignResultFromService(result))
}

// BulkExtend handles adjusting multiple subscriptions (extend or shorten).
// POST /api/v1/admin/subscriptions/bulk-extend
func (h *SubscriptionHandler) BulkExtend(c *gin.Context) {
	var req BulkAdjustSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.subscriptions.bulk_extend", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		result, err := h.subscriptionService.BulkAdjustSubscription(ctx, &service.BulkAdjustSubscriptionInput{
			SubscriptionIDs: req.SubscriptionIDs,
			Days:            req.Days,
		})
		if err != nil {
			return nil, err
		}
		return dto.BulkAdjustResultFromService(result), nil
	})
}

// BulkResetQuota handles resetting usage quota windows for multiple subscriptions.
// POST /api/v1/admin/subscriptions/bulk-reset-quota
func (h *SubscriptionHandler) BulkResetQuota(c *gin.Context) {
	var req BulkResetSubscriptionQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if !req.Daily && !req.Weekly && !req.Monthly {
		response.BadRequest(c, "At least one of 'daily', 'weekly', or 'monthly' must be true")
		return
	}

	executeAdminIdempotentJSON(c, "admin.subscriptions.bulk_reset_quota", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		result, err := h.subscriptionService.BulkResetQuota(ctx, &service.BulkResetSubscriptionQuotaInput{
			SubscriptionIDs: req.SubscriptionIDs,
			Daily:           req.Daily,
			Weekly:          req.Weekly,
			Monthly:         req.Monthly,
		})
		if err != nil {
			return nil, err
		}
		return dto.BulkResetSubscriptionQuotaResultFromService(result), nil
	})
}

// Extend handles adjusting a subscription (extend or shorten)
// POST /api/v1/admin/subscriptions/:id/extend
func (h *SubscriptionHandler) Extend(c *gin.Context) {
	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}

	var req AdjustSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	idempotencyPayload := struct {
		SubscriptionID int64                     `json:"subscription_id"`
		Body           AdjustSubscriptionRequest `json:"body"`
	}{
		SubscriptionID: subscriptionID,
		Body:           req,
	}
	executeAdminIdempotentJSON(c, "admin.subscriptions.extend", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		subscription, execErr := h.subscriptionService.ExtendSubscription(ctx, subscriptionID, req.Days)
		if execErr != nil {
			return nil, execErr
		}
		return dto.UserSubscriptionFromServiceAdmin(subscription), nil
	})
}

// ResetSubscriptionQuotaRequest represents the reset quota request
type ResetSubscriptionQuotaRequest struct {
	Daily   bool `json:"daily"`
	Weekly  bool `json:"weekly"`
	Monthly bool `json:"monthly"`
}

// ResetQuota resets daily, weekly, and/or monthly usage for a subscription.
// POST /api/v1/admin/subscriptions/:id/reset-quota
func (h *SubscriptionHandler) ResetQuota(c *gin.Context) {
	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}
	var req ResetSubscriptionQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if !req.Daily && !req.Weekly && !req.Monthly {
		response.BadRequest(c, "At least one of 'daily', 'weekly', or 'monthly' must be true")
		return
	}
	sub, err := h.subscriptionService.AdminResetQuota(c.Request.Context(), subscriptionID, req.Daily, req.Weekly, req.Monthly)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UserSubscriptionFromServiceAdmin(sub))
}

// Revoke handles revoking a subscription
// DELETE /api/v1/admin/subscriptions/:id
func (h *SubscriptionHandler) Revoke(c *gin.Context) {
	subscriptionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}

	_, err = h.subscriptionService.RevokeSubscription(c.Request.Context(), &service.RevokeSubscriptionInput{
		SubscriptionID: subscriptionID,
		OperatorUserID: getAdminIDFromContext(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Subscription revoked successfully"})
}

// ListRefundRequests returns settlement refund requests for admin review.
// GET /api/v1/admin/subscription-refund-requests
func (h *SubscriptionHandler) ListRefundRequests(c *gin.Context) {
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	page, pageSize := response.ParsePagination(c)
	var userID *int64
	if userIDStr := strings.TrimSpace(c.Query("user_id")); userIDStr != "" {
		if id, err := strconv.ParseInt(userIDStr, 10, 64); err == nil && id > 0 {
			userID = &id
		}
	}
	var subscriptionID *int64
	if subscriptionIDStr := strings.TrimSpace(c.Query("subscription_id")); subscriptionIDStr != "" {
		if id, err := strconv.ParseInt(subscriptionIDStr, 10, 64); err == nil && id > 0 {
			subscriptionID = &id
		}
	}
	filter := &service.SettlementRefundListFilter{
		UserID:        userID,
		SubscriptionID: subscriptionID,
		Status:        strings.TrimSpace(c.Query("status")),
	}

	items, paginationResult, err := h.settlementRefundService.ListSettlementRefundRequests(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminSubscriptionRefundRequest, 0, len(items))
	for i := range items {
		out = append(out, *dto.AdminSubscriptionRefundRequestFromService(&items[i]))
	}
	response.PaginatedWithResult(c, out, toResponsePagination(paginationResult))
}

// GetRefundRequest returns a single settlement refund request for admin review.
// GET /api/v1/admin/subscription-refund-requests/:id
func (h *SubscriptionHandler) GetRefundRequest(c *gin.Context) {
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	refundRequestID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	view, err := h.settlementRefundService.GetSettlementRefundRequestView(c.Request.Context(), refundRequestID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminSubscriptionRefundRequestFromService(view))
}

type SettlementRefundManualProofRequest struct {
	ProofURL  string `json:"proof_url"`
	AdminNote string `json:"admin_note"`
}

type SettlementRefundCancelRequest struct {
	AdminNote string `json:"admin_note"`
}

// UploadRefundProof records manual transfer proof for a settlement refund request.
// POST /api/v1/admin/subscriptions/refund-requests/:id/manual-proof
func (h *SubscriptionHandler) UploadRefundProof(c *gin.Context) {
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	refundRequestID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req SettlementRefundManualProofRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	proofURL := strings.TrimSpace(req.ProofURL)
	if proofURL == "" {
		response.BadRequest(c, "proof_url is required")
		return
	}

	result, err := h.settlementRefundService.UploadSettlementRefundManualProof(c.Request.Context(), service.SettlementRefundManualProofInput{
		RefundRequestID: refundRequestID,
		OperatorUserID:  getAdminIDFromContext(c),
		ProofURL:        proofURL,
		AdminNote:       req.AdminNote,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ProcessRefundGateway runs the gateway refund step for a settlement refund request.
// POST /api/v1/admin/subscriptions/refund-requests/:id/gateway-process
func (h *SubscriptionHandler) ProcessRefundGateway(c *gin.Context) {
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	refundRequestID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	result, err := h.settlementRefundService.ProcessSettlementRefundGateway(c.Request.Context(), service.SettlementRefundGatewayInput{
		RefundRequestID: refundRequestID,
		OperatorUserID:  getAdminIDFromContext(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// CompleteRefund finalizes a settlement refund request and creates the refund settlement order.
// POST /api/v1/admin/subscriptions/refund-requests/:id/complete
func (h *SubscriptionHandler) CompleteRefund(c *gin.Context) {
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	refundRequestID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	result, err := h.settlementRefundService.CompleteSettlementRefund(c.Request.Context(), service.SettlementRefundCompleteInput{
		RefundRequestID: refundRequestID,
		OperatorUserID:  getAdminIDFromContext(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// CancelRefund cancels a settlement refund request and restores the subscription state.
// POST /api/v1/admin/subscriptions/refund-requests/:id/cancel
func (h *SubscriptionHandler) CancelRefund(c *gin.Context) {
	if h == nil || h.settlementRefundService == nil {
		response.InternalError(c, "Settlement refund service is unavailable")
		return
	}

	refundRequestID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req SettlementRefundCancelRequest
	if c.Request != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}

	result, err := h.settlementRefundService.CancelSettlementRefund(c.Request.Context(), service.SettlementRefundCancelInput{
		RefundRequestID: refundRequestID,
		OperatorUserID:  getAdminIDFromContext(c),
		AdminNote:       req.AdminNote,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListByUser handles listing subscriptions for a specific user
// GET /api/v1/admin/users/:id/subscriptions
func (h *SubscriptionHandler) ListByUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	subscriptions, err := h.subscriptionService.ListUserSubscriptions(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminUserSubscription, 0, len(subscriptions))
	for i := range subscriptions {
		out = append(out, *dto.UserSubscriptionFromServiceAdmin(&subscriptions[i]))
	}
	response.Success(c, out)
}

// Helper function to get admin ID from context
func getAdminIDFromContext(c *gin.Context) int64 {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}
