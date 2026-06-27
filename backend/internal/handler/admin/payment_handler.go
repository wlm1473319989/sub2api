package admin

import (
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles admin payment management.
type PaymentHandler struct {
	paymentService          *service.PaymentService
	configService           *service.PaymentConfigService
	settlementRefundService *service.SettlementRefundService
}

// NewPaymentHandler creates a new admin PaymentHandler.
func NewPaymentHandler(paymentService *service.PaymentService, configService *service.PaymentConfigService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		configService:  configService,
	}
}

func (h *PaymentHandler) SetSettlementRefundService(settlementRefundService *service.SettlementRefundService) {
	h.settlementRefundService = settlementRefundService
}

// --- Dashboard ---

// GetDashboard returns payment dashboard statistics.
// GET /api/v1/admin/payment/dashboard
func (h *PaymentHandler) GetDashboard(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}
	stats, err := h.paymentService.GetDashboardStats(c.Request.Context(), days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// --- Orders ---

// ListOrders returns a paginated list of all payment orders.
// GET /api/v1/admin/payment/orders
func (h *PaymentHandler) ListOrders(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if uid := c.Query("user_id"); uid != "" {
		if v, err := strconv.ParseInt(uid, 10, 64); err == nil {
			userID = v
		}
	}
	orders, total, err := h.paymentService.AdminListOrders(c.Request.Context(), userID, service.OrderListParams{
		Page:        page,
		PageSize:    pageSize,
		Status:      c.Query("status"),
		OrderType:   c.Query("order_type"),
		PaymentType: c.Query("payment_type"),
		Keyword:     c.Query("keyword"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, sanitizeAdminPaymentOrdersForResponse(orders), int64(total), page, pageSize)
}

// GetOrderDetail returns detailed information about a single order.
// GET /api/v1/admin/payment/orders/:id
func (h *PaymentHandler) GetOrderDetail(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	order, err := h.paymentService.GetOrderByID(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	auditLogs, _ := h.paymentService.GetOrderAuditLogs(c.Request.Context(), orderID)
	response.Success(c, gin.H{"order": sanitizeAdminPaymentOrderForResponse(order), "auditLogs": auditLogs})
}

// CancelOrder cancels a pending order (admin).
// POST /api/v1/admin/payment/orders/:id/cancel
func (h *PaymentHandler) CancelOrder(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	msg, err := h.paymentService.AdminCancelOrder(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": msg})
}

// RetryFulfillment retries fulfillment for a paid order.
// POST /api/v1/admin/payment/orders/:id/retry
func (h *PaymentHandler) RetryFulfillment(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.paymentService.RetryFulfillment(c.Request.Context(), orderID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "fulfillment retried"})
}

func sanitizeAdminPaymentOrdersForResponse(orders []*dbent.PaymentOrder) []*dbent.PaymentOrder {
	if len(orders) == 0 {
		return orders
	}
	out := make([]*dbent.PaymentOrder, 0, len(orders))
	for _, order := range orders {
		out = append(out, sanitizeAdminPaymentOrderForResponse(order))
	}
	return out
}

func sanitizeAdminPaymentOrderForResponse(order *dbent.PaymentOrder) *dbent.PaymentOrder {
	if order == nil {
		return nil
	}
	cloned := *order
	cloned.ProviderSnapshot = nil
	return &cloned
}

// AdminProcessRefundRequest is the request body for admin refund processing.
type AdminProcessRefundRequest struct {
	Amount         float64                                      `json:"amount"`
	Reason         string                                       `json:"reason"`
	Force          bool                                         `json:"force"`
	DeductBalance  bool                                         `json:"deduct_balance"`
	PreviewID      int64                                        `json:"preview_id"`
	PreviewToken   string                                       `json:"preview_token"`
	ManualTransfer *legacySettlementRefundManualTransferRequest `json:"manual_transfer"`
}

type legacySettlementRefundManualTransferRequest struct {
	ReceiverType           string `json:"receiver_type"`
	ReceiverName           string `json:"receiver_name"`
	ReceiverAccount        string `json:"receiver_account"`
	ReceiverQRCodeImageURL string `json:"receiver_qr_image_url"`
	Remark                 string `json:"remark"`
}

// PreviewRefund previews a refund for an order without mutating state (admin).
// POST /api/v1/admin/payment/orders/:id/refund-preview
func (h *PaymentHandler) PreviewRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req AdminProcessRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, subscription, err := h.paymentService.ResolveAdminSubscriptionRefundTarget(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if order != nil && order.OrderType == "subscription" {
		if h == nil || h.settlementRefundService == nil {
			response.InternalError(c, "Settlement refund service is unavailable")
			return
		}
		preview, previewErr := h.settlementRefundService.PreviewSettlementRefund(c.Request.Context(), service.SettlementRefundPreviewInput{
			SubscriptionID: subscription.ID,
			UserID:         order.UserID,
			Reason:         req.Reason,
		})
		if previewErr != nil {
			response.ErrorFrom(c, previewErr)
			return
		}
		response.Success(c, buildLegacySubscriptionRefundPreviewResponse(order, preview))
		return
	}

	preview, err := h.paymentService.PreviewRefund(c.Request.Context(), orderID, req.Amount, req.Reason, req.Force, req.DeductBalance)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}

// ProcessRefund processes a refund for an order (admin).
// POST /api/v1/admin/payment/orders/:id/refund
func (h *PaymentHandler) ProcessRefund(c *gin.Context) {
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req AdminProcessRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	order, subscription, err := h.paymentService.ResolveAdminSubscriptionRefundTarget(c.Request.Context(), orderID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if order != nil && order.OrderType == "subscription" {
		if h == nil || h.settlementRefundService == nil {
			response.InternalError(c, "Settlement refund service is unavailable")
			return
		}
		if req.PreviewID <= 0 || strings.TrimSpace(req.PreviewToken) == "" {
			response.ErrorFrom(c, service.ErrSettlementRefundSubmitInput)
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
		submitResult, submitErr := h.settlementRefundService.SubmitSettlementRefund(c.Request.Context(), service.SettlementRefundSubmitInput{
			SubscriptionID: subscription.ID,
			UserID:         order.UserID,
			PreviewID:      req.PreviewID,
			PreviewToken:   req.PreviewToken,
			Reason:         req.Reason,
			ManualTransfer: manualTransfer,
		})
		if submitErr != nil {
			response.ErrorFrom(c, submitErr)
			return
		}
		gatewayResult, gatewayErr := h.settlementRefundService.ProcessSettlementRefundGateway(c.Request.Context(), service.SettlementRefundGatewayInput{
			RefundRequestID: submitResult.RefundRequestID,
			OperatorUserID:  getAdminIDFromContext(c),
		})
		if gatewayErr != nil {
			response.ErrorFrom(c, gatewayErr)
			return
		}
		if gatewayResult.FailedAllocations > 0 || gatewayResult.Status == service.SettlementRefundStatusFailed {
			response.Success(c, gin.H{
				"success":                false,
				"warning":                "gateway refund processing failed",
				"require_force":          false,
				"refund_request_id":      submitResult.RefundRequestID,
				"refund_status":          gatewayResult.Status,
				"gateway_refunded_total": gatewayResult.GatewayRefundedTotal,
				"manual_transfer_amount": gatewayResult.ManualTransferAmount,
				"failed_allocations":     gatewayResult.FailedAllocations,
			})
			return
		}
		if service.SettlementRefundManualTransferRequired(gatewayResult.ManualTransferAmount, submitResult.Currency) {
			response.Success(c, gin.H{
				"success":                false,
				"warning":                "manual transfer required",
				"require_force":          false,
				"refund_request_id":      submitResult.RefundRequestID,
				"refund_status":          gatewayResult.Status,
				"gateway_refunded_total": gatewayResult.GatewayRefundedTotal,
				"manual_transfer_amount": gatewayResult.ManualTransferAmount,
			})
			return
		}
		completeResult, completeErr := h.settlementRefundService.CompleteSettlementRefund(c.Request.Context(), service.SettlementRefundCompleteInput{
			RefundRequestID: submitResult.RefundRequestID,
			OperatorUserID:  getAdminIDFromContext(c),
		})
		if completeErr != nil {
			response.ErrorFrom(c, completeErr)
			return
		}
		response.Success(c, gin.H{
			"success":               true,
			"refund_request_id":     submitResult.RefundRequestID,
			"settlement_order_id":   completeResult.SettlementOrderID,
			"subscription_status":   completeResult.SubscriptionStatus,
			"refund_residual_value": completeResult.RefundResidualValue,
		})
		return
	}

	plan, earlyResult, err := h.paymentService.PrepareRefund(c.Request.Context(), orderID, req.Amount, req.Reason, req.Force, req.DeductBalance)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if earlyResult != nil {
		response.Success(c, earlyResult)
		return
	}

	result, err := h.paymentService.ExecuteRefund(c.Request.Context(), plan)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func buildLegacySubscriptionRefundPreviewResponse(order *dbent.PaymentOrder, preview *service.SettlementRefundPreview) gin.H {
	return gin.H{
		"order_amount":                       order.Amount,
		"pay_amount":                         order.PayAmount,
		"refund_amount":                      preview.RefundResidualValue,
		"gateway_amount":                     preview.GatewayRefundableTotal,
		"currency":                           preview.Currency,
		"deduction_type":                     "subscription",
		"preview_id":                         preview.PreviewID,
		"preview_token":                      preview.PreviewToken,
		"preview_issued_at":                  preview.PreviewIssuedAt,
		"preview_expires_at":                 preview.PreviewExpiresAt,
		"preview_ttl_seconds":                preview.PreviewTTLSeconds,
		"subscription_id":                    preview.SubscriptionID,
		"user_id":                            preview.UserID,
		"settlement_id":                      preview.SettlementID,
		"expected_settlement_id":             preview.ExpectedSettlementID,
		"action_source":                      preview.ActionSource,
		"trigger_ref_type":                   preview.TriggerRefType,
		"trigger_ref_id":                     preview.TriggerRefID,
		"plan_name":                          preview.PlanName,
		"subscription_expires_at":            preview.SubscriptionExpiresAt,
		"after_settlement_value":             preview.AfterSettlementValue,
		"theoretical_full_max_knives":        preview.TheoreticalFullMaxKnives,
		"residual_quota_knives":              preview.ResidualQuotaKnives,
		"unit_cost":                          preview.UnitCost,
		"refund_mode":                        preview.RefundMode,
		"refund_residual_value":              preview.RefundResidualValue,
		"gateway_refundable_total":           preview.GatewayRefundableTotal,
		"manual_transfer_amount":             preview.ManualTransferAmount,
		"manual_transfer_required":           preview.ManualTransferRequired,
		"after_submit_subscription_status":   preview.AfterSubmitSubscriptionStatus,
		"after_complete_subscription_status": preview.AfterCompleteSubscriptionStatus,
		"allocations":                        preview.Allocations,
		"settlement_head": gin.H{
			"head_id":                preview.SettlementID,
			"action_source":          preview.ActionSource,
			"trigger_ref_type":       preview.TriggerRefType,
			"trigger_ref_id":         preview.TriggerRefID,
			"current_residual_value": preview.AfterSettlementValue,
			"refund_residual_value":  preview.RefundResidualValue,
		},
	}
}

// --- Subscription Plans ---

// ListPlans returns all subscription plans.
// GET /api/v1/admin/payment/plans
func (h *PaymentHandler) ListPlans(c *gin.Context) {
	plans, err := h.configService.ListPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}

// CreatePlan creates a new subscription plan.
// POST /api/v1/admin/payment/plans
func (h *PaymentHandler) CreatePlan(c *gin.Context) {
	var req service.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.configService.CreatePlan(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, plan)
}

// UpdatePlan updates an existing subscription plan.
// PUT /api/v1/admin/payment/plans/:id
func (h *PaymentHandler) UpdatePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.configService.UpdatePlan(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plan)
}

// DeletePlan deletes a subscription plan.
// DELETE /api/v1/admin/payment/plans/:id
func (h *PaymentHandler) DeletePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.configService.DeletePlan(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

// --- Provider Instances ---

// ListProviders returns all payment provider instances.
// GET /api/v1/admin/payment/providers
func (h *PaymentHandler) ListProviders(c *gin.Context) {
	providers, err := h.configService.ListProviderInstancesWithConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, providers)
}

// CreateProvider creates a new payment provider instance.
// POST /api/v1/admin/payment/providers
func (h *PaymentHandler) CreateProvider(c *gin.Context) {
	var req service.CreateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.CreateProviderInstance(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Created(c, inst)
}

// UpdateProvider updates an existing payment provider instance.
// PUT /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) UpdateProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.UpdateProviderInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	inst, err := h.configService.UpdateProviderInstance(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, inst)
}

// DeleteProvider deletes a payment provider instance.
// DELETE /api/v1/admin/payment/providers/:id
func (h *PaymentHandler) DeleteProvider(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.configService.DeleteProviderInstance(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.paymentService.RefreshProviders(c.Request.Context())
	response.Success(c, gin.H{"message": "deleted"})
}

// parseIDParam parses an int64 path parameter.
// Returns the parsed ID and true on success; on failure it writes a BadRequest response and returns false.
func parseIDParam(c *gin.Context, paramName string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(paramName), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid "+paramName)
		return 0, false
	}
	return id, true
}

// --- Config ---

// GetConfig returns the payment configuration (admin view).
// GET /api/v1/admin/payment/config
func (h *PaymentHandler) GetConfig(c *gin.Context) {
	cfg, err := h.configService.GetPaymentConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig updates the payment configuration.
// PUT /api/v1/admin/payment/config
func (h *PaymentHandler) UpdateConfig(c *gin.Context) {
	var req service.UpdatePaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.configService.UpdatePaymentConfig(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "updated"})
}
