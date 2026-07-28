package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/rechargebonusrule"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	RechargeBonusTypeFixed      = "fixed"
	RechargeBonusTypePercentage = "percentage"

	rechargeBonusSourceRule       = "rule"
	rechargeBonusSourceMultiplier = "legacy_multiplier"
	rechargeBonusSourceNone       = "none"
)

type RechargeBonusRuleRequest struct {
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
	Priority   int        `json:"priority"`
	MinAmount  float64    `json:"min_amount"`
	MaxAmount  *float64   `json:"max_amount"`
	BonusType  string     `json:"bonus_type"`
	BonusValue float64    `json:"bonus_value"`
	StartsAt   *time.Time `json:"starts_at"`
	EndsAt     *time.Time `json:"ends_at"`
	Notes      *string    `json:"notes"`
}

type RechargeBonusSummary struct {
	ID         int64   `json:"id,omitempty"`
	Name       string  `json:"name"`
	BonusType  string  `json:"bonus_type"`
	BonusValue float64 `json:"bonus_value"`
	Source     string  `json:"source"`
}

type RechargeQuote struct {
	Principal      float64               `json:"principal"`
	Bonus          float64               `json:"bonus"`
	CreditedAmount float64               `json:"credited_amount"`
	FeeRate        float64               `json:"fee_rate"`
	FeeAmount      float64               `json:"fee_amount"`
	PayAmount      float64               `json:"pay_amount"`
	Currency       string                `json:"currency"`
	Rule           *RechargeBonusSummary `json:"rule,omitempty"`
	QuoteToken     string                `json:"quote_token,omitempty"`
	ExpiresAt      time.Time             `json:"expires_at,omitempty"`

	ruleID   *int64
	snapshot map[string]any
}

func (s *PaymentConfigService) ListRechargeBonusRules(ctx context.Context) ([]*dbent.RechargeBonusRule, error) {
	rules, err := s.entClient.RechargeBonusRule.Query().
		Order(dbent.Desc(rechargebonusrule.FieldPriority), dbent.Asc(rechargebonusrule.FieldMinAmount), dbent.Asc(rechargebonusrule.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recharge bonus rules: %w", err)
	}
	return rules, nil
}

func (s *PaymentConfigService) CreateRechargeBonusRule(ctx context.Context, req RechargeBonusRuleRequest) (*dbent.RechargeBonusRule, error) {
	req = normalizeRechargeBonusRuleRequest(req)
	if err := validateRechargeBonusRuleRequest(req); err != nil {
		return nil, err
	}
	if err := s.validateRechargeBonusRuleConflict(ctx, 0, req); err != nil {
		return nil, err
	}
	b := s.entClient.RechargeBonusRule.Create().
		SetName(req.Name).
		SetEnabled(req.Enabled).
		SetPriority(req.Priority).
		SetMinAmount(req.MinAmount).
		SetNillableMaxAmount(req.MaxAmount).
		SetBonusType(req.BonusType).
		SetBonusValue(req.BonusValue).
		SetNillableStartsAt(req.StartsAt).
		SetNillableEndsAt(req.EndsAt).
		SetNillableNotes(req.Notes)
	rule, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create recharge bonus rule: %w", err)
	}
	return rule, nil
}

func (s *PaymentConfigService) UpdateRechargeBonusRule(ctx context.Context, id int64, req RechargeBonusRuleRequest) (*dbent.RechargeBonusRule, error) {
	if _, err := s.entClient.RechargeBonusRule.Get(ctx, id); err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("RECHARGE_BONUS_RULE_NOT_FOUND", "recharge bonus rule not found")
		}
		return nil, fmt.Errorf("get recharge bonus rule: %w", err)
	}
	req = normalizeRechargeBonusRuleRequest(req)
	if err := validateRechargeBonusRuleRequest(req); err != nil {
		return nil, err
	}
	if err := s.validateRechargeBonusRuleConflict(ctx, id, req); err != nil {
		return nil, err
	}
	b := s.entClient.RechargeBonusRule.UpdateOneID(id).
		SetName(req.Name).
		SetEnabled(req.Enabled).
		SetPriority(req.Priority).
		SetMinAmount(req.MinAmount).
		SetBonusType(req.BonusType).
		SetBonusValue(req.BonusValue)
	if req.MaxAmount == nil {
		b.ClearMaxAmount()
	} else {
		b.SetMaxAmount(*req.MaxAmount)
	}
	if req.StartsAt == nil {
		b.ClearStartsAt()
	} else {
		b.SetStartsAt(*req.StartsAt)
	}
	if req.EndsAt == nil {
		b.ClearEndsAt()
	} else {
		b.SetEndsAt(*req.EndsAt)
	}
	if req.Notes == nil {
		b.ClearNotes()
	} else {
		b.SetNotes(*req.Notes)
	}
	rule, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update recharge bonus rule: %w", err)
	}
	return rule, nil
}

func (s *PaymentConfigService) DeleteRechargeBonusRule(ctx context.Context, id int64) error {
	used, err := s.entClient.PaymentOrder.Query().Where(paymentorder.RechargeBonusRuleIDEQ(id)).Exist(ctx)
	if err != nil {
		return fmt.Errorf("check recharge bonus rule usage: %w", err)
	}
	if used {
		return infraerrors.Conflict("RECHARGE_BONUS_RULE_IN_USE", "used recharge bonus rules can only be disabled")
	}
	if err := s.entClient.RechargeBonusRule.DeleteOneID(id).Exec(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return infraerrors.NotFound("RECHARGE_BONUS_RULE_NOT_FOUND", "recharge bonus rule not found")
		}
		return fmt.Errorf("delete recharge bonus rule: %w", err)
	}
	return nil
}

func normalizeRechargeBonusRuleRequest(req RechargeBonusRuleRequest) RechargeBonusRuleRequest {
	req.Name = strings.TrimSpace(req.Name)
	req.BonusType = strings.ToLower(strings.TrimSpace(req.BonusType))
	if req.Notes != nil {
		notes := strings.TrimSpace(*req.Notes)
		if notes == "" {
			req.Notes = nil
		} else {
			req.Notes = &notes
		}
	}
	return req
}

func validateRechargeBonusRuleRequest(req RechargeBonusRuleRequest) error {
	if req.Name == "" || len(req.Name) > 100 {
		return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_RULE", "rule name is required and must not exceed 100 characters")
	}
	if invalidPositiveAmount(req.MinAmount) {
		return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_RULE", "minimum amount must be greater than 0")
	}
	if req.MaxAmount != nil && (math.IsNaN(*req.MaxAmount) || math.IsInf(*req.MaxAmount, 0) || *req.MaxAmount < req.MinAmount) {
		return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_RULE", "maximum amount must be greater than or equal to minimum amount")
	}
	if invalidPositiveAmount(req.BonusValue) {
		return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_RULE", "bonus value must be greater than 0")
	}
	if req.BonusType != RechargeBonusTypeFixed && req.BonusType != RechargeBonusTypePercentage {
		return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_RULE", "bonus type must be fixed or percentage")
	}
	if req.BonusType == RechargeBonusTypePercentage && req.BonusValue > 100 {
		return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_RULE", "percentage bonus must not exceed 100")
	}
	if req.StartsAt != nil && req.EndsAt != nil && !req.EndsAt.After(*req.StartsAt) {
		return infraerrors.BadRequest("INVALID_RECHARGE_BONUS_RULE", "end time must be after start time")
	}
	return nil
}

func invalidPositiveAmount(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0) || value <= 0
}

func (s *PaymentConfigService) validateRechargeBonusRuleConflict(ctx context.Context, id int64, req RechargeBonusRuleRequest) error {
	if !req.Enabled {
		return nil
	}
	q := s.entClient.RechargeBonusRule.Query().Where(
		rechargebonusrule.EnabledEQ(true),
		rechargebonusrule.PriorityEQ(req.Priority),
	)
	if id > 0 {
		q.Where(rechargebonusrule.IDNEQ(id))
	}
	rules, err := q.All(ctx)
	if err != nil {
		return fmt.Errorf("query conflicting recharge bonus rules: %w", err)
	}
	for _, other := range rules {
		if amountRangesOverlap(req.MinAmount, req.MaxAmount, other.MinAmount, other.MaxAmount) && timeRangesOverlap(req.StartsAt, req.EndsAt, other.StartsAt, other.EndsAt) {
			return infraerrors.Conflict("RECHARGE_BONUS_RULE_CONFLICT", "an enabled rule with the same priority overlaps this amount and time range").
				WithMetadata(map[string]string{"conflicting_rule_id": strconv.FormatInt(other.ID, 10)})
		}
	}
	return nil
}

func amountRangesOverlap(aMin float64, aMax *float64, bMin float64, bMax *float64) bool {
	return (aMax == nil || bMin <= *aMax) && (bMax == nil || aMin <= *bMax)
}

func timeRangesOverlap(aStart, aEnd, bStart, bEnd *time.Time) bool {
	return (aEnd == nil || bStart == nil || !bStart.After(*aEnd)) && (bEnd == nil || aStart == nil || !aStart.After(*bEnd))
}

func (s *PaymentService) PreviewRecharge(ctx context.Context, amount float64, paymentType string) (*RechargeQuote, error) {
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	if !cfg.Enabled || cfg.BalanceDisabled {
		return nil, infraerrors.Forbidden("BALANCE_PAYMENT_DISABLED", "balance recharge is disabled")
	}
	req := CreateOrderRequest{Amount: amount, PaymentType: paymentType, OrderType: payment.OrderTypeBalance}
	if _, _, err := s.validateOrderInput(ctx, req, cfg); err != nil {
		return nil, err
	}
	quote, err := s.calculateRechargeQuote(ctx, amount, paymentType, cfg)
	if err != nil {
		return nil, err
	}
	claims := rechargeQuoteClaimsFromQuote(quote, paymentType)
	token, err := s.paymentResume().CreateRechargeQuoteToken(claims)
	if err != nil {
		return nil, err
	}
	issuedClaims, err := s.paymentResume().ParseRechargeQuoteToken(token)
	if err != nil {
		return nil, err
	}
	quote.QuoteToken = token
	quote.ExpiresAt = time.Unix(issuedClaims.ExpiresAt, 0)
	return quote, nil
}

func (s *PaymentService) calculateRechargeQuote(ctx context.Context, amount float64, paymentType string, cfg *PaymentConfig) (*RechargeQuote, error) {
	principal := decimal.NewFromFloat(amount).Round(2).InexactFloat64()
	rule, err := s.matchRechargeBonusRule(ctx, principal, time.Now())
	if err != nil {
		return nil, err
	}
	bonus := 0.0
	var summary *RechargeBonusSummary
	var ruleID *int64
	snapshot := map[string]any{"schema_version": 1, "source": rechargeBonusSourceNone}
	if rule != nil {
		bonus = calculateRechargeRuleBonus(principal, rule.BonusType, rule.BonusValue)
		id := rule.ID
		ruleID = &id
		summary = &RechargeBonusSummary{ID: rule.ID, Name: rule.Name, BonusType: rule.BonusType, BonusValue: rule.BonusValue, Source: rechargeBonusSourceRule}
		snapshot = rechargeBonusRuleSnapshot(rule)
	} else {
		multiplier := normalizeBalanceRechargeMultiplier(cfg.BalanceRechargeMultiplier)
		credited := calculateCreditedBalance(principal, multiplier)
		bonus = decimal.NewFromFloat(credited).Sub(decimal.NewFromFloat(principal)).Round(2).InexactFloat64()
		if bonus > 0 {
			summary = &RechargeBonusSummary{Name: "Global recharge multiplier", BonusType: RechargeBonusTypePercentage, BonusValue: decimal.NewFromFloat(multiplier).Sub(decimal.NewFromInt(1)).Mul(decimal.NewFromInt(100)).InexactFloat64(), Source: rechargeBonusSourceMultiplier}
			snapshot = map[string]any{"schema_version": 1, "source": rechargeBonusSourceMultiplier, "multiplier": multiplier}
		}
	}
	credited := decimal.NewFromFloat(principal).Add(decimal.NewFromFloat(bonus)).Round(2).InexactFloat64()
	currency, err := s.configService.ValidateMethodCurrencyConsistency(ctx, paymentType)
	if err != nil {
		return nil, err
	}
	_, payAmount, err := calculateCreateOrderPayAmount(principal, cfg.RechargeFeeRate, currency)
	if err != nil {
		return nil, err
	}
	feeAmount := decimal.NewFromFloat(payAmount).Sub(decimal.NewFromFloat(principal)).Round(int32(payment.CurrencyMaxFractionDigits(currency))).InexactFloat64()
	return &RechargeQuote{
		Principal: principal, Bonus: bonus, CreditedAmount: credited, FeeRate: cfg.RechargeFeeRate,
		FeeAmount: feeAmount, PayAmount: payAmount, Currency: currency, Rule: summary,
		ruleID: ruleID, snapshot: snapshot,
	}, nil
}

func (s *PaymentService) matchRechargeBonusRule(ctx context.Context, amount float64, now time.Time) (*dbent.RechargeBonusRule, error) {
	rules, err := s.entClient.RechargeBonusRule.Query().Where(rechargebonusrule.EnabledEQ(true)).
		Order(dbent.Desc(rechargebonusrule.FieldPriority), dbent.Asc(rechargebonusrule.FieldMinAmount), dbent.Asc(rechargebonusrule.FieldID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("match recharge bonus rule: %w", err)
	}
	for _, rule := range rules {
		if amount < rule.MinAmount || (rule.MaxAmount != nil && amount > *rule.MaxAmount) {
			continue
		}
		if rule.StartsAt != nil && now.Before(*rule.StartsAt) {
			continue
		}
		if rule.EndsAt != nil && now.After(*rule.EndsAt) {
			continue
		}
		return rule, nil
	}
	return nil, nil
}

func calculateRechargeRuleBonus(principal float64, bonusType string, value float64) float64 {
	bonus := decimal.NewFromFloat(value)
	if bonusType == RechargeBonusTypePercentage {
		bonus = decimal.NewFromFloat(principal).Mul(bonus).Div(decimal.NewFromInt(100))
	}
	return bonus.Round(2).InexactFloat64()
}

func rechargeBonusRuleSnapshot(rule *dbent.RechargeBonusRule) map[string]any {
	return map[string]any{
		"schema_version": 1, "source": rechargeBonusSourceRule, "rule_id": rule.ID,
		"rule_name": rule.Name, "bonus_type": rule.BonusType, "bonus_value": rule.BonusValue,
		"priority": rule.Priority, "rule_updated_at": rule.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func rechargeQuoteClaimsFromQuote(quote *RechargeQuote, paymentType string) RechargeQuoteClaims {
	ruleID := int64(0)
	ruleVersion := ""
	source := rechargeBonusSourceNone
	if quote.ruleID != nil {
		ruleID = *quote.ruleID
	}
	if quote.snapshot != nil {
		if v, ok := quote.snapshot["source"].(string); ok {
			source = v
		}
		if v, ok := quote.snapshot["rule_updated_at"].(string); ok {
			ruleVersion = v
		}
	}
	return RechargeQuoteClaims{
		PaymentType: NormalizeVisibleMethod(paymentType), Principal: formatQuoteAmount(quote.Principal),
		Bonus: formatQuoteAmount(quote.Bonus), CreditedAmount: formatQuoteAmount(quote.CreditedAmount),
		PayAmount: payment.FormatAmountForCurrency(quote.PayAmount, quote.Currency), FeeRate: strconv.FormatFloat(quote.FeeRate, 'f', -1, 64),
		Currency: quote.Currency, Source: source, RuleID: ruleID, RuleVersion: ruleVersion,
	}
}

func formatQuoteAmount(value float64) string {
	return decimal.NewFromFloat(value).Round(2).StringFixed(2)
}

func (s *PaymentService) validateRechargeQuoteToken(token, paymentType string, quote *RechargeQuote) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	claims, err := s.paymentResume().ParseRechargeQuoteToken(token)
	if err != nil {
		return err
	}
	expected := rechargeQuoteClaimsFromQuote(quote, paymentType)
	if !rechargeQuoteClaimsEqual(*claims, expected) {
		return infraerrors.Conflict("RECHARGE_QUOTE_CHANGED", "recharge promotion or payment amount has changed; refresh the quote")
	}
	return nil
}

func rechargeQuoteClaimsEqual(a, b RechargeQuoteClaims) bool {
	return a.PaymentType == b.PaymentType && a.Principal == b.Principal && a.Bonus == b.Bonus &&
		a.CreditedAmount == b.CreditedAmount && a.PayAmount == b.PayAmount && a.FeeRate == b.FeeRate &&
		a.Currency == b.Currency && a.Source == b.Source && a.RuleID == b.RuleID && a.RuleVersion == b.RuleVersion
}
