package handler

import (
	"context"
	"database/sql"
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
