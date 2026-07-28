package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrSubscriptionRefundOrderRequiresCurrentSettlement = infraerrors.Conflict("SUBSCRIPTION_REFUND_REQUIRES_CURRENT_SETTLEMENT", "subscription refund must be requested from the current subscription settlement")
)

// --- Refund Flow ---

// getOrderProviderInstance looks up the provider instance that processed this order.
// For legacy orders without provider_instance_id, it resolves only when the
// historical instance is uniquely identifiable from the stored order fields.
func (s *PaymentService) getOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return s.resolveUniqueLegacyOrderProviderInstance(ctx, o)
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, nil
	}
	return s.entClient.PaymentProviderInstance.Get(ctx, instID)
}

// getRefundOrderProviderInstance resolves the provider instance for refund paths.
// Refunds must be pinned to an explicit historical binding, so legacy
// "best-effort" provider guessing is intentionally not allowed here.
func (s *PaymentService) getRefundOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return nil, nil
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("order %d refund provider instance id is invalid: %s", o.ID, instIDStr)
	}
	inst, err := s.entClient.PaymentProviderInstance.Get(ctx, instID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, fmt.Errorf("order %d refund provider instance %s is missing", o.ID, instIDStr)
		}
		return nil, err
	}
	return inst, nil
}

func (s *PaymentService) resolveUniqueLegacyOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	paymentType := payment.GetBasePaymentType(strings.TrimSpace(o.PaymentType))
	providerKey := strings.TrimSpace(psStringValue(o.ProviderKey))
	if providerKey != "" {
		instances, err := s.entClient.PaymentProviderInstance.Query().
			Where(paymentproviderinstance.ProviderKeyEQ(providerKey)).
			All(ctx)
		if err != nil {
			return nil, err
		}
		matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
		if len(matched) == 1 {
			return matched[0], nil
		}
		return nil, nil
	}

	if paymentType == "" {
		return nil, nil
	}

	instances, err := s.entClient.PaymentProviderInstance.Query().
		All(ctx)
	if err != nil {
		return nil, err
	}

	matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
	if len(matched) == 1 {
		return matched[0], nil
	}
	return nil, nil
}

func psFilterLegacyOrderProviderInstances(orderPaymentType string, instances []*dbent.PaymentProviderInstance) []*dbent.PaymentProviderInstance {
	if len(instances) == 0 {
		return nil
	}
	if strings.TrimSpace(orderPaymentType) == "" {
		return instances
	}
	var matched []*dbent.PaymentProviderInstance
	for _, inst := range instances {
		if psLegacyOrderMatchesInstance(orderPaymentType, inst) {
			matched = append(matched, inst)
		}
	}
	return matched
}

func psLegacyOrderMatchesInstance(orderPaymentType string, inst *dbent.PaymentProviderInstance) bool {
	if inst == nil {
		return false
	}

	baseType := payment.GetBasePaymentType(strings.TrimSpace(orderPaymentType))
	instanceProviderKey := strings.TrimSpace(inst.ProviderKey)
	if baseType == "" {
		return false
	}

	if baseType == payment.TypeStripe {
		return instanceProviderKey == payment.TypeStripe
	}
	if instanceProviderKey == payment.TypeStripe {
		return false
	}
	if instanceProviderKey == baseType {
		return true
	}
	return payment.InstanceSupportsType(inst.SupportedTypes, baseType)
}

func (s *PaymentService) RequestRefund(ctx context.Context, oid, uid int64, reason string) error {
	o, err := s.validateRefundRequest(ctx, oid, uid)
	if err != nil {
		return err
	}
	nr := strings.TrimSpace(reason)
	refundAmount := remainingOrderRecoveryAmount(o)
	if o.OrderType == payment.OrderTypeBalance {
		u, err := s.userRepo.GetByID(ctx, o.UserID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if u.Balance < refundAmount {
			return infraerrors.BadRequest("BALANCE_NOT_ENOUGH", "refund amount exceeds balance")
		}
	} else if o.OrderType == payment.OrderTypeSubscription {
		preview, err := s.PreviewRefund(ctx, oid, 0, nr, false, true)
		if err != nil {
			return err
		}
		if preview.RequireForce {
			msg := strings.TrimSpace(preview.Warning)
			if msg == "" {
				msg = "subscription refund requires admin review"
			}
			return infraerrors.Conflict("REFUND_REQUIRES_ADMIN_REVIEW", msg)
		}
		if preview.RefundAmount > 0 {
			refundAmount = preview.RefundAmount
		}
	}
	now := time.Now()
	by := fmt.Sprintf("%d", uid)
	requestStatuses := []string{OrderStatusCompleted}
	if isRechargeBonusRefundOrder(o) {
		requestStatuses = append(requestStatuses, OrderStatusPartiallyRefunded)
	}
	update := s.entClient.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(oid),
			paymentorder.UserIDEQ(uid),
			paymentorder.StatusIn(requestStatuses...),
			paymentorder.OrderTypeEQ(o.OrderType),
		).
		SetStatus(OrderStatusRefundRequested).
		SetRefundRequestedAt(now).
		SetRefundRequestReason(nr).
		SetRefundRequestedBy(by)
	if !isRechargeBonusRefundOrder(o) {
		update.SetRefundAmount(refundAmount)
	}
	c, err := update.Save(ctx)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if c == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}
	s.writeAuditLog(ctx, oid, "REFUND_REQUESTED", fmt.Sprintf("user:%d", uid), map[string]any{"amount": refundAmount, "reason": nr, "orderType": o.OrderType})
	return nil
}

func (s *PaymentService) validateRefundRequestBase(ctx context.Context, oid, uid int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != uid {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if o.OrderType != payment.OrderTypeBalance && o.OrderType != payment.OrderTypeSubscription {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only balance or subscription orders can request refund")
	}
	if o.Status != OrderStatusCompleted && !(o.Status == OrderStatusPartiallyRefunded && isRechargeBonusRefundOrder(o)) {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "only completed orders can request refund")
	}
	return o, nil
}

func (s *PaymentService) validateUserRefundProvider(ctx context.Context, o *dbent.PaymentOrder) error {
	if o == nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	// Check provider instance allows user refund
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil || inst == nil {
		return infraerrors.Forbidden("USER_REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.AllowUserRefund {
		return infraerrors.Forbidden("USER_REFUND_DISABLED", "user refund is not enabled for this provider")
	}
	return nil
}

func (s *PaymentService) validateRefundRequest(ctx context.Context, oid, uid int64) (*dbent.PaymentOrder, error) {
	o, err := s.validateRefundRequestBase(ctx, oid, uid)
	if err != nil {
		return nil, err
	}
	if err := s.validateUserRefundProvider(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *PaymentService) ResolveSubscriptionRefundTarget(ctx context.Context, orderID, userID int64) (*dbent.PaymentOrder, *UserSubscription, error) {
	order, err := s.validateRefundRequestBase(ctx, orderID, userID)
	if err != nil {
		return nil, nil, err
	}
	if order.OrderType != payment.OrderTypeSubscription {
		if err := s.validateUserRefundProvider(ctx, order); err != nil {
			return nil, nil, err
		}
	}
	return s.resolveSubscriptionRefundTargetOrder(ctx, order)
}

func (s *PaymentService) ResolveAdminSubscriptionRefundTarget(ctx context.Context, orderID int64) (*dbent.PaymentOrder, *UserSubscription, error) {
	order, err := s.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	return s.resolveSubscriptionRefundTargetOrder(ctx, order)
}

func (s *PaymentService) resolveSubscriptionRefundTargetOrder(ctx context.Context, order *dbent.PaymentOrder) (*dbent.PaymentOrder, *UserSubscription, error) {
	if order == nil {
		return nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if order.OrderType != payment.OrderTypeSubscription {
		return order, nil, nil
	}
	if s.subscriptionSvc == nil {
		return nil, nil, infraerrors.InternalServer("SUBSCRIPTION_SERVICE_REQUIRED", "subscription service is required")
	}

	active, err := s.subscriptionSvc.GetActiveSubscriptionByUser(ctx, order.UserID)
	if err != nil {
		if errorsIsSubscriptionNotFound(err) {
			return nil, nil, ErrSubscriptionRefundOrderRequiresCurrentSettlement
		}
		return nil, nil, err
	}
	if active == nil {
		return nil, nil, ErrSubscriptionRefundOrderRequiresCurrentSettlement
	}

	settlementSvc := s.settlementSvc
	if settlementSvc == nil && s.entClient != nil {
		settlementSvc = NewSettlementService(s.entClient)
	}
	if settlementSvc == nil {
		return nil, nil, ErrSettlementEntClientRequired
	}

	head, err := settlementSvc.GetEffectiveHead(ctx, order.UserID, time.Now())
	if err != nil {
		return nil, nil, err
	}
	if head == nil {
		return nil, nil, ErrSubscriptionRefundOrderRequiresCurrentSettlement
	}
	if head.AfterUserSubscriptionID == nil || *head.AfterUserSubscriptionID != active.ID {
		return nil, nil, ErrSubscriptionRefundOrderRequiresCurrentSettlement
	}
	if head.ActionSource != domain.SettlementActionSourceUserPurchase ||
		head.TriggerRefType != domain.SettlementTriggerRefPaymentOrder ||
		head.TriggerRefID == nil ||
		*head.TriggerRefID != order.ID {
		return nil, nil, ErrSubscriptionRefundOrderRequiresCurrentSettlement
	}

	return order, active, nil
}

func (s *PaymentService) PrepareRefund(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPlan, *RefundResult, error) {
	p, earlyResult, err := s.prepareRefundPlan(ctx, oid, amt, reason, force, deduct)
	if err != nil {
		return nil, nil, err
	}
	if earlyResult != nil {
		return nil, earlyResult, nil
	}
	return p, nil, nil
}

func (s *PaymentService) PreviewRefund(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPreview, error) {
	p, earlyResult, err := s.prepareRefundPlan(ctx, oid, amt, reason, force, deduct)
	if err != nil {
		return nil, err
	}
	preview := refundPreviewFromPlan(p, earlyResult)
	if preview != nil {
		preview.AffiliateReward = s.RefundAffiliateRewardInfo(ctx, oid)
	}
	return preview, nil
}

func (s *PaymentService) PreviewUserRefund(ctx context.Context, oid, uid int64) (*RefundPreview, error) {
	o, err := s.validateRefundRequest(ctx, oid, uid)
	if err != nil {
		return nil, err
	}
	if o.OrderType == payment.OrderTypeBalance {
		u, err := s.userRepo.GetByID(ctx, o.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if u.Balance < remainingOrderRecoveryAmount(o) {
			return nil, infraerrors.BadRequest("BALANCE_NOT_ENOUGH", "refund amount exceeds balance")
		}
	}
	return s.PreviewRefund(ctx, oid, 0, "", false, true)
}

func remainingOrderRecoveryAmount(o *dbent.PaymentOrder) float64 {
	if o == nil {
		return 0
	}
	if !isRechargeBonusRefundOrder(o) {
		return o.Amount
	}
	return math.Max(0, decimal.NewFromFloat(o.Amount).Sub(decimal.NewFromFloat(o.RefundAmount)).Round(2).InexactFloat64())
}

func (s *PaymentService) prepareRefundPlan(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPlan, *RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	ok := []string{OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed, OrderStatusPartiallyRefunded}
	if !psSliceContains(ok, o.Status) {
		return nil, nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	if math.IsNaN(amt) || math.IsInf(amt, 0) {
		return nil, nil, infraerrors.BadRequest("INVALID_AMOUNT", "invalid refund amount")
	}
	orderCurrency := PaymentOrderCurrency(o)
	p := &RefundPlan{OrderID: oid, Order: o, Reason: strings.TrimSpace(reason), Force: force, DeductBalance: deduct, DeductionType: payment.DeductionTypeNone}
	if isRechargeBonusRefundOrder(o) {
		if err := prepareRechargeBonusRefundAmounts(p, amt, orderCurrency); err != nil {
			return nil, nil, err
		}
	} else {
		if amt <= 0 {
			amt = o.Amount
		}
		if amt-o.Amount > paymentAmountToleranceForCurrency(orderCurrency) {
			return nil, nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds recharge")
		}
		p.RefundAmount = amt
		p.GatewayAmount = calculateGatewayRefundAmount(o.Amount, o.PayAmount, amt, orderCurrency)
	}
	if p.GatewayAmount > paymentAmountToleranceForCurrency(orderCurrency) {
		inst, instErr := s.getRefundOrderProviderInstance(ctx, o)
		if instErr != nil {
			slog.Warn("refund: provider instance lookup failed", "orderID", oid, "error", instErr)
			return nil, nil, infraerrors.InternalServer("PROVIDER_LOOKUP_FAILED", "failed to look up payment provider for this order")
		}
		if inst == nil || !inst.RefundEnabled {
			return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this order")
		}
	}
	rr := strings.TrimSpace(reason)
	if rr == "" && o.RefundRequestReason != nil {
		rr = *o.RefundRequestReason
	}
	if rr == "" {
		rr = fmt.Sprintf("refund order:%d", o.ID)
	}
	p.Reason = rr
	if deduct {
		if er := s.prepDeduct(ctx, o, p, force); er != nil {
			return p, er, nil
		}
	}
	return p, nil, nil
}

func isRechargeBonusRefundOrder(o *dbent.PaymentOrder) bool {
	return o != nil && o.OrderType == payment.OrderTypeBalance && o.RechargePrincipal > 0 && o.RechargeBonusSnapshot != nil
}

func prepareRechargeBonusRefundAmounts(p *RefundPlan, requested float64, currency string) error {
	o := p.Order
	remaining := decimal.NewFromFloat(o.Amount).Sub(decimal.NewFromFloat(o.RefundAmount)).Round(2).InexactFloat64()
	if requested <= 0 {
		requested = remaining
	}
	if requested <= 0 || requested-remaining > paymentAmountToleranceForCurrency(currency) {
		return infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "recovery amount exceeds remaining credited balance")
	}
	requested = decimal.NewFromFloat(requested).Round(2).InexactFloat64()
	bonusRemaining := math.Max(0, decimal.NewFromFloat(o.RechargeBonus).Sub(decimal.NewFromFloat(o.RefundedBonusAmount)).Round(2).InexactFloat64())
	bonusRecovery := math.Min(requested, bonusRemaining)
	principalRefund := decimal.NewFromFloat(requested).Sub(decimal.NewFromFloat(bonusRecovery)).Round(2).InexactFloat64()
	cumulativePrincipal := decimal.NewFromFloat(o.RefundedPrincipalAmount).Add(decimal.NewFromFloat(principalRefund)).Round(2).InexactFloat64()
	targetGateway := calculateGatewayRefundAmount(o.RechargePrincipal, o.PayAmount, cumulativePrincipal, currency)
	gatewayAmount := decimal.NewFromFloat(targetGateway).Sub(decimal.NewFromFloat(o.RefundedGatewayAmount)).Round(int32(payment.CurrencyMaxFractionDigits(currency))).InexactFloat64()
	if gatewayAmount < 0 {
		gatewayAmount = 0
	}
	p.RechargeBonusRefund = true
	p.RecoveryAmount = requested
	p.BonusRecoveryAmount = bonusRecovery
	p.PrincipalRefundAmount = principalRefund
	p.RefundAmount = requested
	p.GatewayAmount = gatewayAmount
	p.CumulativeRecoveryAmount = decimal.NewFromFloat(o.RefundAmount).Add(decimal.NewFromFloat(requested)).Round(2).InexactFloat64()
	p.CumulativeBonusAmount = decimal.NewFromFloat(o.RefundedBonusAmount).Add(decimal.NewFromFloat(bonusRecovery)).Round(2).InexactFloat64()
	p.CumulativePrincipalAmount = cumulativePrincipal
	p.CumulativeGatewayAmount = decimal.NewFromFloat(o.RefundedGatewayAmount).Add(decimal.NewFromFloat(gatewayAmount)).Round(int32(payment.CurrencyMaxFractionDigits(currency))).InexactFloat64()
	return nil
}

func refundPreviewFromPlan(p *RefundPlan, earlyResult *RefundResult) *RefundPreview {
	if p == nil {
		return nil
	}
	preview := &RefundPreview{
		OrderAmount:           p.Order.Amount,
		PayAmount:             p.Order.PayAmount,
		RefundAmount:          p.RefundAmount,
		GatewayAmount:         p.GatewayAmount,
		Currency:              PaymentOrderCurrency(p.Order),
		DeductionType:         p.DeductionType,
		BalanceToDeduct:       p.BalanceToDeduct,
		SubDaysToDeduct:       p.SubDaysToDeduct,
		SettlementHead:        refundSettlementHeadInfo(p.SettlementHead, p.SettlementResidual),
		RecoveryAmount:        p.RecoveryAmount,
		BonusRecoveryAmount:   p.BonusRecoveryAmount,
		PrincipalRefundAmount: p.PrincipalRefundAmount,
	}
	if p.RechargeBonusRefund {
		preview.RefundAmount = p.RecoveryAmount
		preview.RemainingAmount = math.Max(0, p.Order.Amount-p.CumulativeRecoveryAmount)
	}
	if earlyResult != nil {
		preview.Warning = earlyResult.Warning
		preview.RequireForce = earlyResult.RequireForce
		if earlyResult.BalanceDeducted > 0 {
			preview.BalanceToDeduct = earlyResult.BalanceDeducted
		}
		if earlyResult.SubDaysDeducted > 0 {
			preview.SubDaysToDeduct = earlyResult.SubDaysDeducted
		}
		if earlyResult.SettlementHead != nil {
			preview.SettlementHead = earlyResult.SettlementHead
			if earlyResult.SettlementHead.RefundResidualValue > 0 {
				preview.RefundAmount = earlyResult.SettlementHead.RefundResidualValue
				preview.GatewayAmount = calculateGatewayRefundAmount(p.Order.Amount, p.Order.PayAmount, preview.RefundAmount, PaymentOrderCurrency(p.Order))
			}
		}
	}
	return preview
}

func (s *PaymentService) RefundAffiliateRewardInfo(ctx context.Context, orderID int64) *RefundAffiliateRewardInfo {
	if s == nil || s.entClient == nil || orderID <= 0 {
		return nil
	}

	info := &RefundAffiliateRewardInfo{}
	if err := s.loadRefundAffiliateRebateInfo(ctx, orderID, info); err != nil {
		slog.Warn("refund preview: failed to load affiliate rebate info", "orderID", orderID, "error", err)
	}
	if err := s.loadRefundAffiliateGroupGrantInfo(ctx, orderID, info); err != nil {
		slog.Warn("refund preview: failed to load affiliate group grant info", "orderID", orderID, "error", err)
	}

	if !info.HasReward {
		return nil
	}
	return info
}

func (s *PaymentService) loadRefundAffiliateRebateInfo(ctx context.Context, orderID int64, info *RefundAffiliateRewardInfo) error {
	if info == nil {
		return nil
	}

	amountExpr := "COALESCE(SUM(amount), 0)"
	placeholder := "?"
	if paymentAuditDialect(s.entClient) == "postgres" {
		amountExpr = "COALESCE(SUM(amount), 0)::double precision"
		placeholder = "$1"
	}
	rows, err := s.entClient.QueryContext(ctx, fmt.Sprintf(`
SELECT %s, MIN(user_id), COUNT(*)
FROM user_affiliate_ledger
WHERE action = 'accrue'
  AND source_order_id = %s`, amountExpr, placeholder), orderID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var amount float64
	var inviterID sql.NullInt64
	var count int64
	if rows.Next() {
		if err := rows.Scan(&amount, &inviterID, &count); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		info.HasReward = true
		info.RebateApplied = true
		info.RebateAmount = amount
		if inviterID.Valid {
			v := inviterID.Int64
			info.InviterID = &v
		}
		return nil
	}

	return s.loadRefundAffiliateRebateAuditFallback(ctx, orderID, info)
}

func (s *PaymentService) loadRefundAffiliateRebateAuditFallback(ctx context.Context, orderID int64, info *RefundAffiliateRewardInfo) error {
	if info == nil {
		return nil
	}
	placeholder := "?"
	if paymentAuditDialect(s.entClient) == "postgres" {
		placeholder = "$1"
	}
	rows, err := s.entClient.QueryContext(ctx, fmt.Sprintf(`
SELECT detail
FROM payment_audit_logs
WHERE order_id = %s
  AND action = 'AFFILIATE_REBATE_APPLIED'
LIMIT 1`, placeholder), strconv.FormatInt(orderID, 10))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var detail sql.NullString
	if rows.Next() {
		if err := rows.Scan(&detail); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !detail.Valid {
		return nil
	}

	info.HasReward = true
	info.RebateApplied = true
	var payload map[string]any
	if err := json.Unmarshal([]byte(detail.String), &payload); err == nil {
		if v, ok := payload["rebateAmount"].(float64); ok {
			info.RebateAmount = v
		}
	}
	return nil
}

func (s *PaymentService) loadRefundAffiliateGroupGrantInfo(ctx context.Context, orderID int64, info *RefundAffiliateRewardInfo) error {
	if info == nil {
		return nil
	}

	daysExpr := "COALESCE(SUM(grant_days), 0)"
	placeholder := "?"
	if paymentAuditDialect(s.entClient) == "postgres" {
		daysExpr = "COALESCE(SUM(grant_days), 0)::bigint"
		placeholder = "$1"
	}
	rows, err := s.entClient.QueryContext(ctx, fmt.Sprintf(`
SELECT %s, MAX(expires_at), COUNT(*)
FROM affiliate_group_grants
WHERE source_order_id = %s`, daysExpr, placeholder), orderID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var days int64
	var expiresAt sql.NullTime
	var count int64
	if rows.Next() {
		if err := rows.Scan(&days, &expiresAt, &count); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}

	info.HasReward = true
	info.GroupGrantApplied = true
	info.GroupGrantDays = int(days)
	if expiresAt.Valid {
		v := expiresAt.Time
		info.GroupGrantExpiresAt = &v
	}
	return nil
}

func (s *PaymentService) prepDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, force bool) *RefundResult {
	if o.OrderType == payment.OrderTypeSubscription {
		p.DeductionType = payment.DeductionTypeSubscription
		active, err := s.subscriptionSvc.GetActiveSubscriptionByUser(ctx, o.UserID)
		if err == nil && active != nil {
			if result, handled := s.prepSettlementDeduct(ctx, o, p, active, force); handled {
				return result
			}
			latestOrder, latestErr := s.subscriptionSvc.latestSubscriptionOrderForActive(ctx, o.UserID, active)
			if latestErr == nil && latestOrder != nil && latestOrder.ID == o.ID {
				snapshot := *active
				p.SubscriptionID = active.ID
				p.SubscriptionSnapshot = &snapshot
				return nil
			}
		}
		if !force {
			return &RefundResult{Success: false, Warning: "cannot find matching active subscription for refund, use force", RequireForce: true}
		}
		return nil
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot fetch user balance, use force", RequireForce: true}
		}
		return nil
	}
	p.DeductionType = payment.DeductionTypeBalance
	deductAmount := p.RefundAmount
	if p.RechargeBonusRefund {
		deductAmount = p.RecoveryAmount
	}
	p.BalanceToDeduct = math.Min(deductAmount, u.Balance)
	if p.BalanceToDeduct+1e-9 < deductAmount && !force {
		return &RefundResult{Success: false, Warning: "user balance is insufficient to recover the refund amount; use force to continue", RequireForce: true, BalanceDeducted: p.BalanceToDeduct}
	}
	return nil
}

func (s *PaymentService) prepSettlementDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, active *UserSubscription, force bool) (*RefundResult, bool) {
	settlementSvc := s.settlementSvc
	if settlementSvc == nil && s.entClient != nil {
		settlementSvc = NewSettlementService(s.entClient)
	}
	if settlementSvc == nil {
		return nil, false
	}

	head, err := settlementSvc.GetEffectiveHead(ctx, o.UserID, time.Now())
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot load subscription settlement head for refund, use force", RequireForce: true}, true
		}
		return nil, false
	}
	if head == nil {
		return nil, false
	}
	settlementResidual := settlementResidualValue(active, settlementResidualBasisValue(head, active, head.AfterSettlementValue))
	settlementInfo := refundSettlementHeadInfo(head, settlementResidual)
	if head.ActionSource != domain.SettlementActionSourceUserPurchase ||
		head.TriggerRefType != domain.SettlementTriggerRefPaymentOrder ||
		head.TriggerRefID == nil ||
		*head.TriggerRefID != o.ID {
		if !force {
			return &RefundResult{Success: false, Warning: "refund order must match current subscription settlement head, use force", RequireForce: true, SettlementHead: settlementInfo}, true
		}
		return nil, false
	}
	if head.AfterUserSubscriptionID != nil && *head.AfterUserSubscriptionID != active.ID {
		if !force {
			return &RefundResult{Success: false, Warning: "active subscription does not match current settlement head, use force", RequireForce: true, SettlementHead: settlementInfo}, true
		}
		return nil, false
	}

	snapshot := *active
	p.SubscriptionID = active.ID
	p.SubscriptionSnapshot = &snapshot
	p.SettlementHead = head
	p.SettlementResidual = settlementResidual
	if p.SettlementResidual > 0 {
		p.RefundAmount = p.SettlementResidual
		p.GatewayAmount = calculateGatewayRefundAmount(o.Amount, o.PayAmount, p.SettlementResidual, PaymentOrderCurrency(o))
	}
	return nil, true
}

func refundSettlementHeadInfo(head *dbent.SubscriptionSettlementOrder, residual float64) *RefundSettlementHeadInfo {
	if head == nil {
		return nil
	}
	return &RefundSettlementHeadInfo{
		HeadID:               head.ID,
		ActionSource:         head.ActionSource,
		TriggerRefType:       head.TriggerRefType,
		TriggerRefID:         copyInt64Pointer(head.TriggerRefID),
		CurrentResidualValue: residual,
		RefundResidualValue:  residual,
	}
}

func (s *PaymentService) ExecuteRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed, OrderStatusPartiallyRefunded)).SetStatus(OrderStatusRefunding).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		// Skip balance deduction on retry if previous attempt already deducted
		// but failed to roll back (REFUND_ROLLBACK_FAILED in audit log).
		if !s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
			if err := s.userRepo.DeductBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
				s.restoreStatus(ctx, p)
				return nil, fmt.Errorf("deduction: %w", err)
			}
		} else {
			slog.Warn("skipping balance deduction on retry (previous rollback failed)", "orderID", p.OrderID)
			p.BalanceToDeduct = 0
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubscriptionSnapshot != nil && p.SubscriptionID > 0 {
		if !s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
			_, err := s.subscriptionSvc.RefundActivePlan(ctx, &RefundActivePlanInput{
				UserID:  p.Order.UserID,
				OrderID: p.OrderID,
				Notes:   fmt.Sprintf("refund order %d", p.OrderID),
			})
			if err != nil {
				s.restoreStatus(ctx, p)
				return nil, fmt.Errorf("refund active subscription: %w", err)
			}
		}
	}
	if err := s.gwRefund(ctx, p); err != nil {
		return s.handleGwFail(ctx, p, err)
	}
	return s.markRefundOk(ctx, p)
}

func (s *PaymentService) gwRefund(ctx context.Context, p *RefundPlan) error {
	if p.GatewayAmount <= paymentAmountToleranceForCurrency(PaymentOrderCurrency(p.Order)) {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_NO_GATEWAY_AMOUNT", "admin", map[string]any{"detail": "bonus recovery only"})
		return nil
	}
	if p.Order.PaymentTradeNo == "" {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_NO_TRADE_NO", "admin", map[string]any{"detail": "skipped"})
		return nil
	}

	// Use the exact provider instance that created this order, not a random one
	// from the registry. Each instance has its own merchant credentials.
	prov, err := s.getRefundProvider(ctx, p.Order)
	if err != nil {
		return fmt.Errorf("get refund provider: %w", err)
	}
	if err := validateProviderSnapshotMetadata(p.Order, prov.ProviderKey(), providerMerchantIdentityMetadata(prov)); err != nil {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_PROVIDER_METADATA_MISMATCH", "admin", map[string]any{
			"detail": err.Error(),
		})
		return err
	}
	resp, err := prov.Refund(ctx, payment.RefundRequest{
		TradeNo: p.Order.PaymentTradeNo,
		OrderID: p.Order.OutTradeNo,
		Amount:  formatGatewayRefundAmount(p.GatewayAmount, p.Order),
		Reason:  p.Reason,
	})
	if err != nil {
		return err
	}
	return validateRefundProviderResponse(resp)
}

func formatGatewayRefundAmount(amount float64, order *dbent.PaymentOrder) string {
	return payment.FormatAmountForCurrency(amount, PaymentOrderCurrency(order))
}

func validateRefundProviderResponse(resp *payment.RefundResponse) error {
	if resp == nil {
		return fmt.Errorf("payment refund response missing")
	}
	status := strings.TrimSpace(resp.Status)
	switch status {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded, payment.ProviderStatusPending:
		return nil
	case payment.ProviderStatusFailed:
		return fmt.Errorf("payment refund failed: status %s", status)
	default:
		return fmt.Errorf("payment refund returned unknown status: %s", status)
	}
}

// getRefundProvider creates a provider using the order's original instance config.
// Delegates to getOrderProvider which handles instance lookup and fallback.
func (s *PaymentService) getRefundProvider(ctx context.Context, o *dbent.PaymentOrder) (payment.Provider, error) {
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("refund provider instance is unavailable for order %d", o.ID)
	}
	return s.createProviderFromInstance(ctx, inst)
}

func (s *PaymentService) handleGwFail(ctx context.Context, p *RefundPlan, gErr error) (*RefundResult, error) {
	if s.RollbackRefund(ctx, p, gErr) {
		s.restoreStatus(ctx, p)
		s.writeAuditLog(ctx, p.OrderID, "REFUND_GATEWAY_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
		return &RefundResult{Success: false, Warning: "gateway failed: " + psErrMsg(gErr) + ", rolled back"}, nil
	}
	now := time.Now()
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	s.writeAuditLog(ctx, p.OrderID, "REFUND_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
	return nil, infraerrors.InternalServer("REFUND_FAILED", psErrMsg(gErr))
}

func (s *PaymentService) markRefundOk(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	if p.SettlementHead != nil {
		return s.markRefundOkWithSettlement(ctx, p)
	}
	if p.RechargeBonusRefund {
		return s.markRechargeBonusRefundOk(ctx, p)
	}
	fs := OrderStatusRefunded
	if p.RefundAmount < p.Order.Amount {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	_, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(fs).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).SetRefundAt(now).SetForceRefund(p.Force).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "admin", map[string]any{"refundAmount": p.RefundAmount, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "force": p.Force})
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct, SettlementHead: refundSettlementHeadInfo(p.SettlementHead, p.SettlementResidual)}, nil
}

func (s *PaymentService) markRechargeBonusRefundOk(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	status := OrderStatusPartiallyRefunded
	if p.CumulativeRecoveryAmount+1e-9 >= p.Order.Amount {
		status = OrderStatusRefunded
	}
	now := time.Now()
	_, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).
		SetStatus(status).
		SetRefundAmount(p.CumulativeRecoveryAmount).
		SetRefundedBonusAmount(p.CumulativeBonusAmount).
		SetRefundedPrincipalAmount(p.CumulativePrincipalAmount).
		SetRefundedGatewayAmount(p.CumulativeGatewayAmount).
		SetRefundReason(p.Reason).
		SetRefundAt(now).
		SetForceRefund(p.Force).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark recharge bonus refund: %w", err)
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "admin", map[string]any{
		"recoveryAmount": p.RecoveryAmount, "bonusRecovered": p.BonusRecoveryAmount,
		"principalRefunded": p.PrincipalRefundAmount, "gatewayRefunded": p.GatewayAmount,
		"balanceDeducted": p.BalanceToDeduct, "force": p.Force,
	})
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct}, nil
}

func (s *PaymentService) markRefundOkWithSettlement(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin refund settlement tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	fs := OrderStatusRefunded
	if p.RefundAmount < p.Order.Amount {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	_, err = tx.Client().PaymentOrder.UpdateOneID(p.OrderID).
		SetStatus(fs).
		SetRefundAmount(p.RefundAmount).
		SetRefundReason(p.Reason).
		SetRefundAt(now).
		SetForceRefund(p.Force).
		Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}

	afterSub, err := s.subscriptionSvc.userSubRepo.GetByID(txCtx, p.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("load refunded subscription: %w", err)
	}
	refundResidual := p.SettlementResidual
	if refundResidual <= 0 && p.SubscriptionSnapshot != nil {
		refundResidual = settlementResidualValue(p.SubscriptionSnapshot, settlementResidualBasisValue(p.SettlementHead, p.SubscriptionSnapshot, p.SettlementHead.AfterSettlementValue))
	}
	triggerRefID := copyInt64Pointer(p.SettlementHead.TriggerRefID)
	if _, err = NewSettlementService(s.entClient).CreateSettlementOrder(txCtx, SettlementOrderInput{
		UserID:                  p.Order.UserID,
		OperatorUserID:          p.Order.UserID,
		ActionType:              domain.SettlementActionRefund,
		ActionSource:            p.SettlementHead.ActionSource,
		TriggerRefType:          p.SettlementHead.TriggerRefType,
		TriggerRefID:            triggerRefID,
		ActionNote:              p.Reason,
		CarryInResidualValue:    refundResidual,
		ActionDeltaValue:        -refundResidual,
		AfterSettlementValue:    0,
		RefundResidualValue:     &refundResidual,
		WriteoffValue:           0,
		AfterUserSubscription:   afterSub,
		AfterSubscriptionStatus: domain.SubscriptionStatusRefunded,
		EffectiveAt:             now,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit refund settlement tx: %w", err)
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "admin", map[string]any{"refundAmount": p.RefundAmount, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "force": p.Force})
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct, SettlementHead: refundSettlementHeadInfo(p.SettlementHead, refundResidual)}, nil
}

func (s *PaymentService) RollbackRefund(ctx context.Context, p *RefundPlan, gErr error) bool {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		if err := s.userRepo.UpdateBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
			slog.Error("[CRITICAL] rollback failed", "orderID", p.OrderID, "amount", p.BalanceToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "balanceDeducted": p.BalanceToDeduct})
			return false
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubscriptionSnapshot != nil && p.SubscriptionID > 0 {
		if err := s.subscriptionSvc.userSubRepo.Update(ctx, p.SubscriptionSnapshot); err != nil {
			slog.Error("[CRITICAL] subscription snapshot rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subscriptionId": p.SubscriptionID})
			return false
		}
		s.subscriptionSvc.invalidateSubscriptionCaches(p.Order.UserID)
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, p.SubDaysToDeduct); err != nil {
			slog.Error("[CRITICAL] subscription rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subDaysDeducted": p.SubDaysToDeduct})
			return false
		}
	}
	return true
}

func (s *PaymentService) restoreStatus(ctx context.Context, p *RefundPlan) {
	rs := OrderStatusCompleted
	if p.Order.Status == OrderStatusRefundRequested || p.Order.Status == OrderStatusPartiallyRefunded {
		rs = p.Order.Status
	}
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(rs).Save(ctx)
}
