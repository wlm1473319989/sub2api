package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSettlementRefundPreviewTokenFailed   = infraerrors.InternalServer("SETTLEMENT_REFUND_PREVIEW_TOKEN_FAILED", "failed to generate settlement refund preview token")
	ErrSettlementRefundPreviewCacheRequired = infraerrors.InternalServer("SETTLEMENT_REFUND_PREVIEW_CACHE_REQUIRED", "settlement refund preview cache is unavailable")
	ErrSettlementRefundZeroResidual         = infraerrors.BadRequest("SETTLEMENT_REFUND_ZERO_RESIDUAL", "settlement refund has no residual value")
	ErrSettlementRefundSubscriptionMismatch = infraerrors.Conflict("SETTLEMENT_REFUND_SUBSCRIPTION_MISMATCH", "requested subscription is not the current active subscription")
)

type SettlementRefundPreviewInput struct {
	SubscriptionID int64
	UserID         int64
	Reason         string
}

type SettlementRefundPreview struct {
	PreviewID                       int64                               `json:"preview_id"`
	PreviewToken                    string                              `json:"preview_token"`
	PreviewIssuedAt                 time.Time                           `json:"preview_issued_at"`
	PreviewExpiresAt                time.Time                           `json:"preview_expires_at"`
	PreviewTTLSeconds               int64                               `json:"preview_ttl_seconds"`
	SubscriptionID                  int64                               `json:"subscription_id"`
	UserID                          int64                               `json:"user_id"`
	SettlementID                    int64                               `json:"settlement_id"`
	ExpectedSettlementID            int64                               `json:"expected_settlement_id"`
	ActionSource                    string                              `json:"action_source"`
	TriggerRefType                  string                              `json:"trigger_ref_type"`
	TriggerRefID                    *int64                              `json:"trigger_ref_id,omitempty"`
	PlanName                        string                              `json:"plan_name"`
	SubscriptionExpiresAt           time.Time                           `json:"subscription_expires_at"`
	AfterSettlementValue            float64                             `json:"after_settlement_value"`
	TheoreticalFullMaxKnives        float64                             `json:"theoretical_full_max_knives"`
	ResidualQuotaKnives             float64                             `json:"residual_quota_knives"`
	UnitCost                        float64                             `json:"unit_cost"`
	RefundMode                      string                              `json:"refund_mode"`
	RefundResidualValue             float64                             `json:"refund_residual_value"`
	GatewayRefundableTotal          float64                             `json:"gateway_refundable_total"`
	ManualTransferAmount            float64                             `json:"manual_transfer_amount"`
	ManualTransferRequired          bool                                `json:"manual_transfer_required"`
	Currency                        string                              `json:"currency"`
	AfterSubmitSubscriptionStatus   string                              `json:"after_submit_subscription_status"`
	AfterCompleteSubscriptionStatus string                              `json:"after_complete_subscription_status"`
	Allocations                     []SettlementRefundPreviewAllocation `json:"allocations"`
}

type settlementRefundPreviewComputation struct {
	Active            *UserSubscription
	Head              *dbent.SubscriptionSettlementOrder
	ResidualBreakdown *UpgradeResidualBreakdown
	RefundMode        string
	AllocationResult  SettlementRefundAllocationResult
}

type SettlementRefundPreviewAllocation struct {
	PaymentOrderID         int64   `json:"payment_order_id"`
	OrderAmount            float64 `json:"order_amount"`
	PayAmount              float64 `json:"pay_amount"`
	PaymentType            string  `json:"payment_type,omitempty"`
	PaymentProviderKey     string  `json:"payment_provider_key,omitempty"`
	ProviderInstanceID     *int64  `json:"payment_provider_instance_id,omitempty"`
	AlreadyRefundedAmount  float64 `json:"already_refunded_amount"`
	RefundableOrderAmount  float64 `json:"refundable_order_amount"`
	AllocatedRefundValue   float64 `json:"allocated_refund_value"`
	GatewayRefundAmount    float64 `json:"gateway_refund_amount"`
	Currency               string  `json:"currency"`
	RefundChannelAvailable bool    `json:"refund_channel_available"`
	SkippedReason          string  `json:"skipped_reason,omitempty"`
}

type SettlementRefundService struct {
	entClient    *dbent.Client
	subscription *SubscriptionService
	settlement   *SettlementService
	requestStore any
	previewCache SettlementRefundPreviewCache
	paymentSvc   *PaymentService

	now                                 func() time.Time
	generatePreviewID                   func() (int64, error)
	generatePreviewToken                func() (string, string, error)
	loadActiveSubscription              func(context.Context, int64) (*UserSubscription, error)
	loadEffectiveHead                   func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error)
	loadPaymentOrderCandidates          func(context.Context, *dbent.SubscriptionSettlementOrder) ([]SettlementRefundPaymentOrderCandidate, error)
	createSettlementOrder               func(context.Context, SettlementOrderInput) (*dbent.SubscriptionSettlementOrder, error)
	loadRefundPaymentOrder              func(context.Context, int64) (*dbent.PaymentOrder, error)
	resolveRefundProvider               func(context.Context, *dbent.PaymentOrder) (payment.Provider, error)
	syncGatewayPaymentOrderRefund       func(context.Context, *dbent.PaymentOrder, *SettlementRefundRequestRecord, SettlementRefundAllocationRecord, time.Time) error
	markGatewayPaymentOrderRefundFailed func(context.Context, *dbent.PaymentOrder, string, time.Time) error
}

func NewSettlementRefundService(entClient *dbent.Client, subscriptionSvc *SubscriptionService, previewCache SettlementRefundPreviewCache) *SettlementRefundService {
	settlementSvc := NewSettlementService(entClient)
	svc := &SettlementRefundService{
		entClient:            entClient,
		subscription:         subscriptionSvc,
		settlement:           settlementSvc,
		previewCache:         previewCache,
		now:                  time.Now,
		generatePreviewID:    newSettlementRefundPreviewID,
		generatePreviewToken: newSettlementRefundPreviewToken,
	}
	if entClient != nil {
		svc.requestStore = newSettlementRefundRequestStore(entClient)
	}
	svc.loadActiveSubscription = svc.defaultLoadActiveSubscription
	svc.loadEffectiveHead = svc.defaultLoadEffectiveHead
	svc.loadPaymentOrderCandidates = svc.defaultLoadPaymentOrderCandidates
	svc.createSettlementOrder = settlementSvc.CreateSettlementOrder
	svc.syncGatewayPaymentOrderRefund = svc.defaultSyncGatewayPaymentOrderRefund
	svc.markGatewayPaymentOrderRefundFailed = svc.defaultMarkGatewayPaymentOrderRefundFailed
	return svc
}

func (s *SettlementRefundService) SetPaymentService(paymentSvc *PaymentService) {
	s.paymentSvc = paymentSvc
}

func (s *SettlementRefundService) PreviewSettlementRefund(ctx context.Context, input SettlementRefundPreviewInput) (*SettlementRefundPreview, error) {
	if input.UserID <= 0 || input.SubscriptionID <= 0 {
		return nil, ErrSettlementRefundPreviewInput
	}
	if s == nil || s.previewCache == nil {
		return nil, ErrSettlementRefundPreviewCacheRequired
	}

	now := s.previewNow()
	cached, err := s.previewCache.GetSettlementRefundPreview(ctx, input.UserID, input.SubscriptionID)
	if err != nil {
		return nil, err
	}
	if cached != nil && !settlementRefundPreviewExpired(now, cached.PreviewExpiresAt) {
		return settlementRefundPreviewFromCacheEntry(cached), nil
	}

	computation, err := s.computeSettlementRefundPreview(ctx, input)
	if err != nil {
		return nil, err
	}

	previewID, err := s.generatePreviewID()
	if err != nil {
		return nil, ErrSettlementRefundPreviewTokenFailed.WithCause(err)
	}
	previewToken, previewTokenHash, err := s.generatePreviewToken()
	if err != nil {
		return nil, ErrSettlementRefundPreviewTokenFailed.WithCause(err)
	}
	window := newSettlementRefundPreviewWindow(now)
	reason := settlementRefundNullableReason(input.Reason)
	entry := &SettlementRefundPreviewCacheEntry{
		PreviewID:               previewID,
		PreviewToken:            previewToken,
		UserID:                  input.UserID,
		SubscriptionID:          computation.Active.ID,
		SettlementID:            computation.Head.ID,
		ExpectedSettlementID:    computation.Head.ID,
		ActionSource:            computation.Head.ActionSource,
		TriggerRefType:          computation.Head.TriggerRefType,
		TriggerRefID:            copyInt64Pointer(computation.Head.TriggerRefID),
		PlanName:                settlementRefundPreviewPlanName(computation.Active, computation.Head),
		SubscriptionExpiresAt:   computation.Active.ExpiresAt,
		AfterSettlementValue:    roundSettlementAmountValue(settlementResidualBasisValue(computation.Head, computation.Active, computation.Head.AfterSettlementValue)),
		TheoreticalFullMaxKnives: computation.ResidualBreakdown.TheoreticalFullMaxKnives,
		ResidualQuotaKnives:     computation.ResidualBreakdown.ResidualQuotaKnives,
		UnitCost:                roundSettlementAmountValue(computation.ResidualBreakdown.UnitCost),
		RefundMode:              computation.RefundMode,
		Reason:                  reason,
		RefundResidualValue:     roundSettlementRefundValue(computation.AllocationResult.RefundResidualValue),
		GatewayRefundableTotal:  roundSettlementAmountValue(computation.AllocationResult.GatewayRefundableTotal),
		ManualTransferAmount:    roundSettlementRefundValue(computation.AllocationResult.ManualTransferAmount),
		Currency:                settlementRefundPreviewResponseCurrency(computation.AllocationResult.Currency),
		PreviewTokenHash:        previewTokenHash,
		PreviewFingerprint:      settlementRefundPreviewFingerprint(computation),
		PreviewIssuedAt:         window.IssuedAt,
		PreviewExpiresAt:        window.ExpiresAt,
		Allocations:             settlementRefundPreviewAllocations(computation.AllocationResult.Allocations),
	}
	if err := s.previewCache.SetSettlementRefundPreview(ctx, entry, settlementRefundPreviewTTL); err != nil {
		return nil, err
	}

	preview := settlementRefundPreviewFromCacheEntry(entry)
	return preview, nil
}

func (s *SettlementRefundService) computeSettlementRefundPreview(ctx context.Context, input SettlementRefundPreviewInput) (*settlementRefundPreviewComputation, error) {
	now := s.previewNow()
	active, err := s.previewLoadActiveSubscription(ctx, input.UserID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			return nil, ErrActiveSubscriptionRequired
		}
		return nil, err
	}
	if active == nil {
		return nil, ErrActiveSubscriptionRequired
	}
	if active.ID != input.SubscriptionID {
		return nil, ErrSettlementRefundSubscriptionMismatch
	}

	head, err := s.previewLoadEffectiveHead(ctx, input.UserID, now)
	if err != nil {
		return nil, err
	}
	if head == nil {
		return nil, ErrSettlementHeadRequired
	}
	if head.AfterUserSubscriptionID == nil || *head.AfterUserSubscriptionID != active.ID {
		return nil, ErrSettlementHeadSubscriptionMismatch
	}
	if head.ActionType == domain.SettlementActionRefund || head.ActionType == domain.SettlementActionRevoke {
		return nil, ErrSettlementRefundSourceInvalid
	}

	basisValue := roundSettlementAmountValue(settlementResidualBasisValue(head, active, head.AfterSettlementValue))
	residualBreakdown, err := CalculateUpgradeResidual(UpgradeResidualInput{
		Now:                now,
		StartsAt:           active.StartsAt,
		ExpiresAt:          active.ExpiresAt,
		PlanPrice:          basisValue,
		TargetPlanPrice:    basisValue,
		DailyQuotaKnives:   active.DailyQuotaKnives,
		WeeklyQuotaKnives:  active.WeeklyQuotaKnives,
		MonthlyQuotaKnives: active.MonthlyQuotaKnives,
		DailyUsedKnives:    active.DailyUsedKnives,
		WeeklyUsedKnives:   active.WeeklyUsedKnives,
		MonthlyUsedKnives:  active.MonthlyUsedKnives,
		DailyWindowStart:   active.DailyWindowStart,
		WeeklyWindowStart:  active.WeeklyWindowStart,
		MonthlyWindowStart: active.MonthlyWindowStart,
	})
	if err != nil {
		return nil, ErrSettlementRefundZeroResidual
	}
	refundResidualValue := roundSettlementRefundValue(residualBreakdown.ResidualValue)
	if refundResidualValue <= 0 {
		return nil, ErrSettlementRefundZeroResidual
	}

	allocationResult := SettlementRefundAllocationResult{
		RefundResidualValue: refundResidualValue,
		Currency:            payment.DefaultPaymentCurrency,
		Allocations:         make([]SettlementRefundOrderAllocation, 0),
	}
	refundMode := SettlementRefundModeEntitlementOnly
	switch head.ActionSource {
	case domain.SettlementActionSourceUserPurchase:
		if head.TriggerRefType != domain.SettlementTriggerRefPaymentOrder {
			return nil, ErrSettlementRefundSourceInvalid
		}
		candidates, candidateErr := s.previewLoadPaymentOrderCandidates(ctx, head)
		if candidateErr != nil {
			return nil, candidateErr
		}
		currency := settlementRefundPreviewCurrency(candidates)
		allocationResult = allocateSettlementRefundAcrossOrders(refundResidualValue, currency, candidates)
		refundMode = settlementRefundModeFromAllocation(allocationResult)
	case domain.SettlementActionSourceExchangeCode, domain.SettlementActionSourceSubscriptionAssign:
		refundMode = SettlementRefundModeEntitlementOnly
	default:
		return nil, ErrSettlementRefundSourceInvalid
	}

	return &settlementRefundPreviewComputation{
		Active:            active,
		Head:              head,
		ResidualBreakdown: residualBreakdown,
		RefundMode:        refundMode,
		AllocationResult:  allocationResult,
	}, nil
}

func (s *SettlementRefundService) defaultLoadActiveSubscription(ctx context.Context, userID int64) (*UserSubscription, error) {
	if s == nil || s.subscription == nil {
		return nil, infraerrors.InternalServer("SUBSCRIPTION_SERVICE_REQUIRED", "subscription service is required")
	}
	return s.subscription.GetActiveSubscriptionByUser(ctx, userID)
}

func (s *SettlementRefundService) defaultLoadEffectiveHead(ctx context.Context, userID int64, now time.Time) (*dbent.SubscriptionSettlementOrder, error) {
	if s == nil || s.settlement == nil {
		return nil, ErrSettlementEntClientRequired
	}
	return s.settlement.GetEffectiveHead(ctx, userID, now)
}

func (s *SettlementRefundService) defaultLoadPaymentOrderCandidates(ctx context.Context, head *dbent.SubscriptionSettlementOrder) ([]SettlementRefundPaymentOrderCandidate, error) {
	if head == nil {
		return nil, nil
	}
	client, err := s.settlementRefundClient(ctx)
	if err != nil {
		return nil, err
	}

	settlementChain, err := s.loadSettlementChain(ctx, client, head)
	if err != nil {
		return nil, err
	}
	orderIDs := make([]int64, 0, len(settlementChain))
	seenOrderIDs := make(map[int64]struct{}, len(settlementChain))
	for _, settlement := range settlementChain {
		if settlement == nil ||
			settlement.ActionSource != domain.SettlementActionSourceUserPurchase ||
			settlement.TriggerRefType != domain.SettlementTriggerRefPaymentOrder ||
			settlement.TriggerRefID == nil {
			continue
		}
		if _, exists := seenOrderIDs[*settlement.TriggerRefID]; exists {
			continue
		}
		seenOrderIDs[*settlement.TriggerRefID] = struct{}{}
		orderIDs = append(orderIDs, *settlement.TriggerRefID)
	}
	if len(orderIDs) == 0 {
		return nil, nil
	}

	orders, err := client.PaymentOrder.Query().
		Where(paymentorder.IDIn(orderIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query settlement refund payment orders: %w", err)
	}
	orderByID := make(map[int64]*dbent.PaymentOrder, len(orders))
	for _, order := range orders {
		orderByID[order.ID] = order
	}

	candidates := make([]SettlementRefundPaymentOrderCandidate, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order := orderByID[orderID]
		if order == nil {
			continue
		}
		candidates = append(candidates, s.settlementRefundPaymentOrderCandidate(ctx, order))
	}
	return candidates, nil
}

func (s *SettlementRefundService) settlementRefundClient(ctx context.Context) (*dbent.Client, error) {
	if s != nil && s.settlement != nil {
		return s.settlement.clientFromContext(ctx)
	}
	if s == nil || s.entClient == nil {
		return nil, ErrSettlementEntClientRequired
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client(), nil
	}
	return s.entClient, nil
}

func (s *SettlementRefundService) loadSettlementChain(ctx context.Context, client *dbent.Client, head *dbent.SubscriptionSettlementOrder) ([]*dbent.SubscriptionSettlementOrder, error) {
	if head == nil {
		return nil, nil
	}
	chain := make([]*dbent.SubscriptionSettlementOrder, 0, 4)
	visited := make(map[int64]struct{}, 4)
	current := head
	for current != nil {
		if _, exists := visited[current.ID]; exists {
			return nil, fmt.Errorf("settlement refund chain cycle detected at settlement %d", current.ID)
		}
		visited[current.ID] = struct{}{}
		chain = append(chain, current)
		if current.PrevSettlementID == nil {
			break
		}
		previous, err := client.SubscriptionSettlementOrder.Get(ctx, *current.PrevSettlementID)
		if err != nil {
			return nil, fmt.Errorf("load previous settlement %d: %w", *current.PrevSettlementID, err)
		}
		current = previous
	}
	return chain, nil
}

func (s *SettlementRefundService) settlementRefundPaymentOrderCandidate(ctx context.Context, order *dbent.PaymentOrder) SettlementRefundPaymentOrderCandidate {
	gatewayRefundedAmount := s.settlementRefundGatewayRefundedAmount(ctx, order)
	candidate := SettlementRefundPaymentOrderCandidate{
		PaymentOrderID:        order.ID,
		OrderAmount:           roundSettlementAmountValue(order.Amount),
		PayAmount:             roundSettlementAmountValue(order.PayAmount),
		AlreadyRefundedAmount: roundSettlementAmountValue(order.RefundAmount),
		GatewayRefundedAmount: roundSettlementAmountValue(gatewayRefundedAmount),
		Currency:              PaymentOrderCurrency(order),
		PaymentType:           strings.TrimSpace(order.PaymentType),
		PaymentProviderKey:    strings.TrimSpace(psStringValue(order.ProviderKey)),
	}
	if providerInstanceID, ok := settlementRefundOrderProviderInstanceID(order); ok {
		candidate.PaymentProviderInstanceID = providerInstanceID
	}
	available, reason := s.settlementRefundOrderChannelAvailability(ctx, order)
	candidate.RefundChannelAvailable = available
	candidate.UnavailableReason = reason
	return candidate
}

func (s *SettlementRefundService) settlementRefundGatewayRefundedAmount(ctx context.Context, order *dbent.PaymentOrder) float64 {
	if order == nil || order.ID <= 0 {
		return 0
	}

	client, err := s.settlementRefundClient(ctx)
	if err != nil || client == nil {
		return 0
	}

	rows, err := client.QueryContext(ctx, `
SELECT COALESCE(SUM(gateway_refund_amount), 0)::double precision
FROM subscription_refund_allocations
WHERE payment_order_id = $1
  AND status = 'succeeded'
`, order.ID)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var total sql.NullFloat64
	if rows.Next() {
		if scanErr := rows.Scan(&total); scanErr != nil {
			return 0
		}
	}
	if total.Valid {
		return roundSettlementRefundValue(total.Float64)
	}
	return 0
}

func (s *SettlementRefundService) settlementRefundOrderChannelAvailability(ctx context.Context, order *dbent.PaymentOrder) (bool, string) {
	if order == nil {
		return false, "payment_order_missing"
	}
	switch order.Status {
	case OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefunding, OrderStatusPartiallyRefunded, OrderStatusRefundFailed, OrderStatusRefunded:
	default:
		return false, "payment_order_status_invalid"
	}
	providerInstanceID, ok := settlementRefundOrderProviderInstanceID(order)
	if !ok {
		return false, "refund_provider_unavailable"
	}
	client, err := s.settlementRefundClient(ctx)
	if err != nil {
		return false, "refund_provider_unavailable"
	}
	instance, err := client.PaymentProviderInstance.Get(ctx, *providerInstanceID)
	if err != nil {
		return false, "refund_provider_unavailable"
	}
	if !instance.Enabled || !instance.RefundEnabled {
		return false, "refund_channel_unavailable"
	}
	baseType := payment.GetBasePaymentType(strings.TrimSpace(order.PaymentType))
	if baseType != "" && instance.ProviderKey != baseType && !payment.InstanceSupportsType(instance.SupportedTypes, baseType) {
		return false, "refund_channel_unavailable"
	}
	return true, ""
}

func settlementRefundOrderProviderInstanceID(order *dbent.PaymentOrder) (*int64, bool) {
	if order == nil || order.ProviderInstanceID == nil {
		return nil, false
	}
	raw := strings.TrimSpace(*order.ProviderInstanceID)
	if raw == "" {
		return nil, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, false
	}
	return &id, true
}

func settlementRefundModeFromAllocation(result SettlementRefundAllocationResult) string {
	switch {
	case result.GatewayRefundableTotal > 0 && result.ManualTransferAmount > 0:
		return SettlementRefundModeHybrid
	case result.GatewayRefundableTotal > 0:
		return SettlementRefundModeGatewayRefund
	case result.ManualTransferAmount > 0:
		return SettlementRefundModeManualTransfer
	default:
		return SettlementRefundModeManualTransfer
	}
}

func settlementRefundPreviewCurrency(candidates []SettlementRefundPaymentOrderCandidate) string {
	for _, candidate := range candidates {
		currency := strings.TrimSpace(candidate.Currency)
		if currency != "" {
			return currency
		}
	}
	return payment.DefaultPaymentCurrency
}

func settlementRefundPreviewResponseCurrency(currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return payment.DefaultPaymentCurrency
	}
	return currency
}

func settlementRefundPreviewFingerprint(computation *settlementRefundPreviewComputation) string {
	if computation == nil || computation.Active == nil || computation.Head == nil {
		return ""
	}
	parts := []string{
		strconv.FormatInt(computation.Head.ID, 10),
		strconv.FormatInt(computation.Active.ID, 10),
		strconv.FormatInt(computation.Active.UserID, 10),
		computation.RefundMode,
		settlementRefundPreviewResponseCurrency(computation.AllocationResult.Currency),
		settlementRefundFingerprintMoney(computation.AllocationResult.RefundResidualValue),
		settlementRefundFingerprintMoney(computation.AllocationResult.GatewayRefundableTotal),
		settlementRefundFingerprintMoney(computation.AllocationResult.ManualTransferAmount),
		computation.Active.Status,
		computation.Active.StartsAt.UTC().Format(time.RFC3339Nano),
		computation.Active.ExpiresAt.UTC().Format(time.RFC3339Nano),
		fmt.Sprintf("%.8f", computation.Active.DailyUsedKnives),
		fmt.Sprintf("%.8f", computation.Active.WeeklyUsedKnives),
		fmt.Sprintf("%.8f", computation.Active.MonthlyUsedKnives),
		settlementRefundFingerprintTime(computation.Active.DailyWindowStart),
		settlementRefundFingerprintTime(computation.Active.WeeklyWindowStart),
		settlementRefundFingerprintTime(computation.Active.MonthlyWindowStart),
		fmt.Sprintf("%.8f", settlementRefundFloatPointerValue(computation.Active.DailyQuotaKnives)),
		fmt.Sprintf("%.8f", settlementRefundFloatPointerValue(computation.Active.WeeklyQuotaKnives)),
		fmt.Sprintf("%.8f", settlementRefundFloatPointerValue(computation.Active.MonthlyQuotaKnives)),
	}
	for _, allocation := range computation.AllocationResult.Allocations {
		parts = append(parts,
			strconv.FormatInt(allocation.PaymentOrderID, 10),
			settlementRefundFingerprintMoney(allocation.OrderAmount),
			settlementRefundFingerprintMoney(allocation.PayAmount),
			settlementRefundFingerprintMoney(allocation.AlreadyRefundedAmount),
			settlementRefundFingerprintMoney(allocation.RefundableOrderAmount),
			settlementRefundFingerprintMoney(allocation.AllocatedRefundValue),
			settlementRefundFingerprintMoney(allocation.GatewayRefundAmount),
			settlementRefundPreviewResponseCurrency(allocation.Currency),
			strconv.FormatBool(allocation.RefundChannelAvailable),
			strings.TrimSpace(allocation.SkippedReason),
		)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func settlementRefundFingerprintTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func settlementRefundFloatPointerValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func settlementRefundFingerprintMoney(value float64) string {
	return fmt.Sprintf("%.4f", roundSettlementAmountValue(value))
}

func settlementRefundNullableReason(reason string) *string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil
	}
	return &reason
}

func settlementRefundPreviewStoreAllocations(allocations []SettlementRefundOrderAllocation) []CreateSettlementRefundAllocationInput {
	if len(allocations) == 0 {
		return nil
	}
	result := make([]CreateSettlementRefundAllocationInput, 0, len(allocations))
	for _, allocation := range allocations {
		input := CreateSettlementRefundAllocationInput{
			PaymentOrderID:            allocation.PaymentOrderID,
			PaymentProviderInstanceID: allocation.ProviderInstanceID,
			OrderAmount:               roundSettlementAmountValue(allocation.OrderAmount),
			OrderPayAmount:            roundSettlementAmountValue(allocation.PayAmount),
			AlreadyRefundedAmount:     roundSettlementAmountValue(allocation.AlreadyRefundedAmount),
			RefundableOrderAmount:     roundSettlementAmountValue(allocation.RefundableOrderAmount),
			AllocatedRefundValue:      roundSettlementRefundValue(allocation.AllocatedRefundValue),
			GatewayRefundAmount:       roundSettlementAmountValue(allocation.GatewayRefundAmount),
			Currency:                  allocation.Currency,
		}
		if allocation.GatewayRefundAmount > 0 {
			input.Status = SettlementRefundAllocationStatusPending
		} else {
			input.Status = SettlementRefundAllocationStatusSkipped
		}
		if allocation.SkippedReason != "" {
			input.FailedReason = &allocation.SkippedReason
		}
		result = append(result, input)
	}
	return result
}

func settlementRefundPreviewAllocations(allocations []SettlementRefundOrderAllocation) []SettlementRefundPreviewAllocation {
	if len(allocations) == 0 {
		return nil
	}
	result := make([]SettlementRefundPreviewAllocation, 0, len(allocations))
	for _, allocation := range allocations {
		result = append(result, SettlementRefundPreviewAllocation{
			PaymentOrderID:         allocation.PaymentOrderID,
			OrderAmount:            roundSettlementAmountValue(allocation.OrderAmount),
			PayAmount:              roundSettlementAmountValue(allocation.PayAmount),
			PaymentType:            allocation.PaymentType,
			PaymentProviderKey:     allocation.PaymentProviderKey,
			ProviderInstanceID:     allocation.ProviderInstanceID,
			AlreadyRefundedAmount:  roundSettlementAmountValue(allocation.AlreadyRefundedAmount),
			RefundableOrderAmount:  roundSettlementAmountValue(allocation.RefundableOrderAmount),
			AllocatedRefundValue:   roundSettlementRefundValue(allocation.AllocatedRefundValue),
			GatewayRefundAmount:    roundSettlementAmountValue(allocation.GatewayRefundAmount),
			Currency:               allocation.Currency,
			RefundChannelAvailable: allocation.RefundChannelAvailable,
			SkippedReason:          allocation.SkippedReason,
		})
	}
	return result
}

func settlementRefundPreviewFromCacheEntry(entry *SettlementRefundPreviewCacheEntry) *SettlementRefundPreview {
	if entry == nil {
		return nil
	}
	return &SettlementRefundPreview{
		PreviewID:                       entry.PreviewID,
		PreviewToken:                    entry.PreviewToken,
		PreviewIssuedAt:                 entry.PreviewIssuedAt,
		PreviewExpiresAt:                entry.PreviewExpiresAt,
		PreviewTTLSeconds:               int64(entry.PreviewExpiresAt.Sub(entry.PreviewIssuedAt).Seconds()),
		SubscriptionID:                  entry.SubscriptionID,
		UserID:                          entry.UserID,
		SettlementID:                    entry.SettlementID,
		ExpectedSettlementID:            entry.ExpectedSettlementID,
		ActionSource:                    entry.ActionSource,
		TriggerRefType:                  entry.TriggerRefType,
		TriggerRefID:                    copyInt64Pointer(entry.TriggerRefID),
		PlanName:                        entry.PlanName,
		SubscriptionExpiresAt:           entry.SubscriptionExpiresAt,
		AfterSettlementValue:            entry.AfterSettlementValue,
		TheoreticalFullMaxKnives:        entry.TheoreticalFullMaxKnives,
		ResidualQuotaKnives:             entry.ResidualQuotaKnives,
		UnitCost:                        entry.UnitCost,
		RefundMode:                      entry.RefundMode,
		RefundResidualValue:             entry.RefundResidualValue,
		GatewayRefundableTotal:          entry.GatewayRefundableTotal,
		ManualTransferAmount:            entry.ManualTransferAmount,
		ManualTransferRequired:          SettlementRefundManualTransferRequired(entry.ManualTransferAmount, entry.Currency),
		Currency:                        settlementRefundPreviewResponseCurrency(entry.Currency),
		AfterSubmitSubscriptionStatus:   SubscriptionStatusSuspended,
		AfterCompleteSubscriptionStatus: SubscriptionStatusRefunded,
		Allocations:                     append([]SettlementRefundPreviewAllocation(nil), entry.Allocations...),
	}
}

func (s *SettlementRefundService) previewNow() time.Time {
	if s == nil || s.now == nil {
		return time.Now()
	}
	return s.now()
}

func (s *SettlementRefundService) previewLoadActiveSubscription(ctx context.Context, userID int64) (*UserSubscription, error) {
	if s != nil && s.loadActiveSubscription != nil {
		return s.loadActiveSubscription(ctx, userID)
	}
	return s.defaultLoadActiveSubscription(ctx, userID)
}

func (s *SettlementRefundService) previewLoadEffectiveHead(ctx context.Context, userID int64, now time.Time) (*dbent.SubscriptionSettlementOrder, error) {
	if s != nil && s.loadEffectiveHead != nil {
		return s.loadEffectiveHead(ctx, userID, now)
	}
	return s.defaultLoadEffectiveHead(ctx, userID, now)
}

func (s *SettlementRefundService) previewLoadPaymentOrderCandidates(ctx context.Context, head *dbent.SubscriptionSettlementOrder) ([]SettlementRefundPaymentOrderCandidate, error) {
	if s != nil && s.loadPaymentOrderCandidates != nil {
		return s.loadPaymentOrderCandidates(ctx, head)
	}
	return s.defaultLoadPaymentOrderCandidates(ctx, head)
}

func settlementRefundPreviewPlanName(active *UserSubscription, head *dbent.SubscriptionSettlementOrder) string {
	if active != nil && active.PlanNameSnapshot != nil {
		if name := strings.TrimSpace(*active.PlanNameSnapshot); name != "" {
			return name
		}
	}
	if head != nil && head.AfterPlanNameSnapshot != nil {
		if name := strings.TrimSpace(*head.AfterPlanNameSnapshot); name != "" {
			return name
		}
	}
	return ""
}
