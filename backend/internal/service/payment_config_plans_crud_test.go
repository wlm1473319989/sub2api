package service

import (
	"context"
	"testing"
)

func ptrPlanStr(s string) *string     { return &s }
func ptrPlanFloat(f float64) *float64 { return &f }

func TestPaymentConfigServiceCreatePlan_UserLevelQuotas(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentConfigService{entClient: client}

	dailyQuota := 12.5
	monthlyQuota := 300.0

	plan, err := svc.CreatePlan(ctx, CreatePlanRequest{
		Name:               "User Level",
		Description:        "quota plan",
		Price:              9.99,
		ValidityDays:       30,
		ValidityUnit:       "days",
		DailyQuotaKnives:   &dailyQuota,
		MonthlyQuotaKnives: &monthlyQuota,
		ForSale:            true,
		SortOrder:          3,
	})
	if err != nil {
		t.Fatalf("CreatePlan returned error: %v", err)
	}

	if plan.ValidityUnit != "day" {
		t.Fatalf("ValidityUnit = %q, want day", plan.ValidityUnit)
	}
	if plan.DailyQuotaKnives == nil || *plan.DailyQuotaKnives != dailyQuota {
		t.Fatalf("DailyQuotaKnives = %v, want %v", plan.DailyQuotaKnives, dailyQuota)
	}
	if plan.MonthlyQuotaKnives == nil || *plan.MonthlyQuotaKnives != monthlyQuota {
		t.Fatalf("MonthlyQuotaKnives = %v, want %v", plan.MonthlyQuotaKnives, monthlyQuota)
	}
}

func TestPaymentConfigServiceUpdatePlan_ReplacesQuotaAndNormalizesUnit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentConfigService{entClient: client}

	created, err := client.SubscriptionPlan.Create().
		SetName("Starter").
		SetDescription("starter plan").
		SetPrice(19.99).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetDailyQuotaKnives(20).
		SetFeatures("").
		SetProductName("").
		SetForSale(false).
		SetSortOrder(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	weeklyQuota := 88.0
	updated, err := svc.UpdatePlan(ctx, created.ID, UpdatePlanRequest{
		DailyQuotaKnives:  ptrPlanFloat(0),
		WeeklyQuotaKnives: &weeklyQuota,
		ValidityUnit:      ptrPlanStr("months"),
	})
	if err != nil {
		t.Fatalf("UpdatePlan returned error: %v", err)
	}

	if updated.DailyQuotaKnives != nil {
		t.Fatalf("DailyQuotaKnives = %v, want nil", *updated.DailyQuotaKnives)
	}
	if updated.WeeklyQuotaKnives == nil || *updated.WeeklyQuotaKnives != weeklyQuota {
		t.Fatalf("WeeklyQuotaKnives = %v, want %v", updated.WeeklyQuotaKnives, weeklyQuota)
	}
	if updated.ValidityUnit != "month" {
		t.Fatalf("ValidityUnit = %q, want month", updated.ValidityUnit)
	}
}

func TestPaymentConfigServiceListPlans_NormalizesLegacyUnits(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentConfigService{entClient: client}

	if _, err := client.SubscriptionPlan.Create().
		SetName("Legacy Unit").
		SetDescription("legacy").
		SetPrice(5.5).
		SetValidityDays(7).
		SetValidityUnit("weeks").
		SetFeatures("").
		SetProductName("").
		SetForSale(true).
		SetSortOrder(9).
		Save(ctx); err != nil {
		t.Fatalf("create plan: %v", err)
	}

	plans, err := svc.ListPlans(ctx)
	if err != nil {
		t.Fatalf("ListPlans returned error: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("ListPlans len = %d, want 1", len(plans))
	}
	if plans[0].ValidityUnit != "week" {
		t.Fatalf("ValidityUnit = %q, want week", plans[0].ValidityUnit)
	}
}
