package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newPaymentHandlerTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	dbName := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()),
	)
	db, err := sql.Open("sqlite", dbName)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestPaymentOrderResponseRechargeBonusFieldsStayAuthenticated(t *testing.T) {
	ruleID := int64(42)
	order := &dbent.PaymentOrder{
		RechargePrincipal:       100,
		RechargeBonus:           20,
		RechargeBonusRuleID:     &ruleID,
		RechargeBonusSnapshot:   map[string]any{"source": "rule"},
		RefundedBonusAmount:     20,
		RefundedPrincipalAmount: 30,
		RefundedGatewayAmount:   30,
	}

	authenticated := sanitizePaymentOrderForResponse(order)
	if authenticated == nil {
		t.Fatal("authenticated payment order response is nil")
	}
	if authenticated.RechargePrincipal != 100 || authenticated.RechargeBonus != 20 {
		t.Fatalf("unexpected recharge split: principal=%v bonus=%v", authenticated.RechargePrincipal, authenticated.RechargeBonus)
	}
	if authenticated.RechargeBonusRuleID == nil || *authenticated.RechargeBonusRuleID != ruleID {
		t.Fatalf("recharge bonus rule id = %v, want %d", authenticated.RechargeBonusRuleID, ruleID)
	}
	if authenticated.RechargeBonusSnapshot["source"] != "rule" {
		t.Fatalf("recharge bonus snapshot = %v", authenticated.RechargeBonusSnapshot)
	}
	if authenticated.RefundedBonusAmount != 20 || authenticated.RefundedPrincipalAmount != 30 || authenticated.RefundedGatewayAmount != 30 {
		t.Fatalf("unexpected refund totals: bonus=%v principal=%v gateway=%v", authenticated.RefundedBonusAmount, authenticated.RefundedPrincipalAmount, authenticated.RefundedGatewayAmount)
	}

	public := buildPublicOrderResult(order)
	encoded, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("marshal public payment order response: %v", err)
	}
	var publicFields map[string]any
	if err := json.Unmarshal(encoded, &publicFields); err != nil {
		t.Fatalf("unmarshal public payment order response: %v", err)
	}
	if _, ok := publicFields["recharge_bonus_snapshot"]; ok {
		t.Fatal("public payment order response exposes recharge bonus snapshot")
	}
	if _, ok := publicFields["recharge_bonus_rule_id"]; ok {
		t.Fatal("public payment order response exposes recharge bonus rule id")
	}
}

func TestBuildPublicCheckoutPlansIncludesAllForSalePlans(t *testing.T) {
	ctx := context.Background()
	client := newPaymentHandlerTestClient(t)
	configService := service.NewPaymentConfigService(client, nil, nil)

	dailyQuota := 10.0
	weeklyQuota := 70.0

	firstPlan, err := client.SubscriptionPlan.Create().
		SetName("Starter").
		SetDescription("starter").
		SetPrice(9.99).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetDailyQuotaKnives(dailyQuota).
		SetWeeklyQuotaKnives(weeklyQuota).
		SetFeatures("line-1\nline-2").
		SetProductName("legacy").
		SetForSale(true).
		SetSortOrder(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("create first plan: %v", err)
	}

	userLevelQuota := 25.0
	secondPlan, err := client.SubscriptionPlan.Create().
		SetName("User Level").
		SetDescription("new").
		SetPrice(19.99).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetDailyQuotaKnives(userLevelQuota).
		SetFeatures("only-one").
		SetProductName("user-level").
		SetForSale(true).
		SetSortOrder(2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create second plan: %v", err)
	}

	plans, err := configService.ListPlansForSale(ctx)
	if err != nil {
		t.Fatalf("ListPlansForSale: %v", err)
	}

	out := buildPublicCheckoutPlans(plans)
	if len(out) != 2 {
		t.Fatalf("buildPublicCheckoutPlans len = %d, want 2", len(out))
	}
	if out[0].ID != firstPlan.ID {
		t.Fatalf("first plan ID = %d, want %d", out[0].ID, firstPlan.ID)
	}
	if out[0].DailyQuotaKnives == nil || *out[0].DailyQuotaKnives != dailyQuota {
		t.Fatalf("DailyQuotaKnives = %v, want %v", out[0].DailyQuotaKnives, dailyQuota)
	}
	if out[0].WeeklyQuotaKnives == nil || *out[0].WeeklyQuotaKnives != weeklyQuota {
		t.Fatalf("WeeklyQuotaKnives = %v, want %v", out[0].WeeklyQuotaKnives, weeklyQuota)
	}
	if len(out[0].Features) != 2 {
		t.Fatalf("Features len = %d, want 2", len(out[0].Features))
	}
	if out[1].ID != secondPlan.ID {
		t.Fatalf("second plan ID = %d, want %d", out[1].ID, secondPlan.ID)
	}
	if out[1].DailyQuotaKnives == nil || *out[1].DailyQuotaKnives != userLevelQuota {
		t.Fatalf("second DailyQuotaKnives = %v, want %v", out[1].DailyQuotaKnives, userLevelQuota)
	}
	if len(out[1].Features) != 1 {
		t.Fatalf("second Features len = %d, want 1", len(out[1].Features))
	}
}
