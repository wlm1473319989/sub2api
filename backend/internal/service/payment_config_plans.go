package service

import (
	"context"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var validPlanValidityUnits = map[string]string{
	"day":    "day",
	"days":   "day",
	"week":   "week",
	"weeks":  "week",
	"month":  "month",
	"months": "month",
	"year":   "year",
	"years":  "year",
}

// validatePlanRequired checks that all required fields for a plan are provided.
func validatePlanRequired(name string, groupID *int64, price float64, validityDays int, validityUnit string, originalPrice, dailyQuotaKnives, weeklyQuotaKnives, monthlyQuotaKnives *float64) error {
	if strings.TrimSpace(name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if groupID != nil && *groupID <= 0 {
		return infraerrors.BadRequest("PLAN_GROUP_REQUIRED", "group is required")
	}
	if price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if validityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if strings.TrimSpace(validityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if _, err := normalizePlanValidityUnit(validityUnit); err != nil {
		return err
	}
	if originalPrice != nil && *originalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	for _, quota := range []*float64{dailyQuotaKnives, weeklyQuotaKnives, monthlyQuotaKnives} {
		if quota != nil && *quota < 0 {
			return infraerrors.BadRequest("PLAN_QUOTA_INVALID", "quota knives must be >= 0")
		}
	}
	if groupID == nil &&
		normalizeOptionalQuotaKnives(dailyQuotaKnives) == nil &&
		normalizeOptionalQuotaKnives(weeklyQuotaKnives) == nil &&
		normalizeOptionalQuotaKnives(monthlyQuotaKnives) == nil {
		return infraerrors.BadRequest("PLAN_QUOTA_REQUIRED", "user-level plan requires at least one quota")
	}
	return nil
}

// validatePlanPatch validates only the non-nil fields in a patch update.
func validatePlanPatch(req UpdatePlanRequest) error {
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if req.ClearGroupID && req.GroupID != nil {
		return infraerrors.BadRequest("PLAN_GROUP_CONFLICT", "group_id and clear_group_id cannot be set together")
	}
	if req.GroupID != nil && *req.GroupID <= 0 {
		return infraerrors.BadRequest("PLAN_GROUP_REQUIRED", "group is required")
	}
	if req.Price != nil && *req.Price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if req.ValidityDays != nil && *req.ValidityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if req.ValidityUnit != nil && strings.TrimSpace(*req.ValidityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if req.ValidityUnit != nil {
		if _, err := normalizePlanValidityUnit(*req.ValidityUnit); err != nil {
			return err
		}
	}
	if req.OriginalPrice != nil && *req.OriginalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	for _, quota := range []*float64{req.DailyQuotaKnives, req.WeeklyQuotaKnives, req.MonthlyQuotaKnives} {
		if quota != nil && *quota < 0 {
			return infraerrors.BadRequest("PLAN_QUOTA_INVALID", "quota knives must be >= 0")
		}
	}
	return nil
}

func normalizePlanValidityUnit(unit string) (string, error) {
	normalized, ok := validPlanValidityUnits[strings.ToLower(strings.TrimSpace(unit))]
	if !ok {
		return "", infraerrors.BadRequest("PLAN_VALIDITY_UNIT_INVALID", "validity unit must be day/week/month/year")
	}
	return normalized, nil
}

func normalizePlanValidityUnitValue(unit string) string {
	normalized, err := normalizePlanValidityUnit(unit)
	if err != nil {
		return strings.TrimSpace(unit)
	}
	return normalized
}

func normalizeOptionalQuotaKnives(quota *float64) *float64 {
	if quota == nil {
		return nil
	}
	value := *quota
	if value == 0 {
		return nil
	}
	return &value
}

func effectiveQuotaKnives(current, patch *float64) *float64 {
	if patch == nil {
		return current
	}
	return normalizeOptionalQuotaKnives(patch)
}

func normalizePlanEntity(plan *dbent.SubscriptionPlan) *dbent.SubscriptionPlan {
	if plan == nil {
		return nil
	}
	plan.ValidityUnit = normalizePlanValidityUnitValue(plan.ValidityUnit)
	return plan
}

func normalizePlanEntities(plans []*dbent.SubscriptionPlan) []*dbent.SubscriptionPlan {
	for i := range plans {
		normalizePlanEntity(plans[i])
	}
	return plans
}

// --- Plan CRUD ---

// PlanGroupInfo holds the group details needed for subscription plan display.
type PlanGroupInfo struct {
	Platform        string   `json:"platform"`
	Name            string   `json:"name"`
	RateMultiplier  float64  `json:"rate_multiplier"`
	DailyLimitUSD   *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD  *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`
	ModelScopes     []string `json:"supported_model_scopes"`
}

// GetGroupPlatformMap returns a map of group_id → platform for the given plans.
func (s *PaymentConfigService) GetGroupPlatformMap(ctx context.Context, plans []*dbent.SubscriptionPlan) map[int64]string {
	info := s.GetGroupInfoMap(ctx, plans)
	m := make(map[int64]string, len(info))
	for id, gi := range info {
		m[id] = gi.Platform
	}
	return m
}

// GetGroupInfoMap returns a map of group_id → PlanGroupInfo for the given plans.
func (s *PaymentConfigService) GetGroupInfoMap(ctx context.Context, plans []*dbent.SubscriptionPlan) map[int64]PlanGroupInfo {
	ids := make([]int64, 0, len(plans))
	seen := make(map[int64]bool)
	for _, p := range plans {
		if p.GroupID == nil {
			continue
		}
		if !seen[*p.GroupID] {
			seen[*p.GroupID] = true
			ids = append(ids, *p.GroupID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	groups, err := s.entClient.Group.Query().Where(group.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil
	}
	m := make(map[int64]PlanGroupInfo, len(groups))
	for _, g := range groups {
		m[int64(g.ID)] = PlanGroupInfo{
			Platform:        g.Platform,
			Name:            g.Name,
			RateMultiplier:  g.RateMultiplier,
			DailyLimitUSD:   g.DailyLimitUsd,
			WeeklyLimitUSD:  g.WeeklyLimitUsd,
			MonthlyLimitUSD: g.MonthlyLimitUsd,
			ModelScopes:     g.SupportedModelScopes,
		}
	}
	return m
}

func (s *PaymentConfigService) ListPlans(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	plans, err := s.entClient.SubscriptionPlan.Query().Order(subscriptionplan.BySortOrder()).All(ctx)
	if err != nil {
		return nil, err
	}
	return normalizePlanEntities(plans), nil
}

func (s *PaymentConfigService) ListPlansForSale(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	plans, err := s.entClient.SubscriptionPlan.Query().Where(subscriptionplan.ForSaleEQ(true)).Order(subscriptionplan.BySortOrder()).All(ctx)
	if err != nil {
		return nil, err
	}
	return normalizePlanEntities(plans), nil
}

func (s *PaymentConfigService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*dbent.SubscriptionPlan, error) {
	if err := validatePlanRequired(req.Name, req.GroupID, req.Price, req.ValidityDays, req.ValidityUnit, req.OriginalPrice, req.DailyQuotaKnives, req.WeeklyQuotaKnives, req.MonthlyQuotaKnives); err != nil {
		return nil, err
	}
	validityUnit, _ := normalizePlanValidityUnit(req.ValidityUnit)
	dailyQuota := normalizeOptionalQuotaKnives(req.DailyQuotaKnives)
	weeklyQuota := normalizeOptionalQuotaKnives(req.WeeklyQuotaKnives)
	monthlyQuota := normalizeOptionalQuotaKnives(req.MonthlyQuotaKnives)
	b := s.entClient.SubscriptionPlan.Create().
		SetNillableGroupID(req.GroupID).SetName(req.Name).SetDescription(req.Description).
		SetPrice(req.Price).SetValidityDays(req.ValidityDays).SetValidityUnit(validityUnit).
		SetNillableDailyQuotaKnives(dailyQuota).
		SetNillableWeeklyQuotaKnives(weeklyQuota).
		SetNillableMonthlyQuotaKnives(monthlyQuota).
		SetFeatures(req.Features).SetProductName(req.ProductName).
		SetForSale(req.ForSale).SetSortOrder(req.SortOrder)
	if req.OriginalPrice != nil {
		b.SetOriginalPrice(*req.OriginalPrice)
	}
	plan, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return normalizePlanEntity(plan), nil
}

// UpdatePlan updates a subscription plan by ID (patch semantics).
// NOTE: This function exceeds 30 lines due to per-field nil-check patch update boilerplate
// plus a validation guard for non-nil fields.
func (s *PaymentConfigService) UpdatePlan(ctx context.Context, id int64, req UpdatePlanRequest) (*dbent.SubscriptionPlan, error) {
	if err := validatePlanPatch(req); err != nil {
		return nil, err
	}
	current, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, infraerrors.NotFound("PLAN_NOT_FOUND", "subscription plan not found")
	}
	effectiveGroupID := current.GroupID
	if req.ClearGroupID {
		effectiveGroupID = nil
	} else if req.GroupID != nil {
		value := *req.GroupID
		effectiveGroupID = &value
	}
	if effectiveGroupID == nil &&
		effectiveQuotaKnives(current.DailyQuotaKnives, req.DailyQuotaKnives) == nil &&
		effectiveQuotaKnives(current.WeeklyQuotaKnives, req.WeeklyQuotaKnives) == nil &&
		effectiveQuotaKnives(current.MonthlyQuotaKnives, req.MonthlyQuotaKnives) == nil {
		return nil, infraerrors.BadRequest("PLAN_QUOTA_REQUIRED", "user-level plan requires at least one quota")
	}
	u := s.entClient.SubscriptionPlan.UpdateOneID(id)
	if req.ClearGroupID {
		u.ClearGroupID()
	} else if req.GroupID != nil {
		u.SetGroupID(*req.GroupID)
	}
	if req.Name != nil {
		u.SetName(*req.Name)
	}
	if req.Description != nil {
		u.SetDescription(*req.Description)
	}
	if req.Price != nil {
		u.SetPrice(*req.Price)
	}
	if req.OriginalPrice != nil {
		u.SetOriginalPrice(*req.OriginalPrice)
	}
	if req.ValidityDays != nil {
		u.SetValidityDays(*req.ValidityDays)
	}
	if req.ValidityUnit != nil {
		validityUnit, _ := normalizePlanValidityUnit(*req.ValidityUnit)
		u.SetValidityUnit(validityUnit)
	}
	if req.DailyQuotaKnives != nil {
		if normalized := normalizeOptionalQuotaKnives(req.DailyQuotaKnives); normalized == nil {
			u.ClearDailyQuotaKnives()
		} else {
			u.SetDailyQuotaKnives(*normalized)
		}
	}
	if req.WeeklyQuotaKnives != nil {
		if normalized := normalizeOptionalQuotaKnives(req.WeeklyQuotaKnives); normalized == nil {
			u.ClearWeeklyQuotaKnives()
		} else {
			u.SetWeeklyQuotaKnives(*normalized)
		}
	}
	if req.MonthlyQuotaKnives != nil {
		if normalized := normalizeOptionalQuotaKnives(req.MonthlyQuotaKnives); normalized == nil {
			u.ClearMonthlyQuotaKnives()
		} else {
			u.SetMonthlyQuotaKnives(*normalized)
		}
	}
	if req.Features != nil {
		u.SetFeatures(*req.Features)
	}
	if req.ProductName != nil {
		u.SetProductName(*req.ProductName)
	}
	if req.ForSale != nil {
		u.SetForSale(*req.ForSale)
	}
	if req.SortOrder != nil {
		u.SetSortOrder(*req.SortOrder)
	}
	plan, err := u.Save(ctx)
	if err != nil {
		return nil, err
	}
	return normalizePlanEntity(plan), nil
}

func (s *PaymentConfigService) DeletePlan(ctx context.Context, id int64) error {
	count, err := s.countPendingOrdersByPlan(ctx, id)
	if err != nil {
		return fmt.Errorf("check pending orders: %w", err)
	}
	if count > 0 {
		return infraerrors.Conflict("PENDING_ORDERS",
			fmt.Sprintf("this plan has %d in-progress orders and cannot be deleted — wait for orders to complete first", count))
	}
	return s.entClient.SubscriptionPlan.DeleteOneID(id).Exec(ctx)
}

// GetPlan returns a subscription plan by ID.
func (s *PaymentConfigService) GetPlan(ctx context.Context, id int64) (*dbent.SubscriptionPlan, error) {
	plan, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, infraerrors.NotFound("PLAN_NOT_FOUND", "subscription plan not found")
	}
	return normalizePlanEntity(plan), nil
}
