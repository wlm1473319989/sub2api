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

func TestBuildPublicCheckoutPlansFiltersUserLevelPlans(t *testing.T) {
	ctx := context.Background()
	client := newPaymentHandlerTestClient(t)
	configService := service.NewPaymentConfigService(client, nil, nil)

	group, err := client.Group.Create().SetName("legacy").Save(ctx)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	dailyQuota := 10.0
	weeklyQuota := 70.0

	legacyPlan, err := client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("Legacy").
		SetDescription("legacy").
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
		t.Fatalf("create legacy plan: %v", err)
	}

	userLevelQuota := 25.0
	if _, err := client.SubscriptionPlan.Create().
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
		Save(ctx); err != nil {
		t.Fatalf("create user-level plan: %v", err)
	}

	plans, err := configService.ListPlansForSale(ctx)
	if err != nil {
		t.Fatalf("ListPlansForSale: %v", err)
	}

	out := buildPublicCheckoutPlans(ctx, configService, plans)
	if len(out) != 1 {
		t.Fatalf("buildPublicCheckoutPlans len = %d, want 1", len(out))
	}
	if out[0].ID != legacyPlan.ID {
		t.Fatalf("plan ID = %d, want %d", out[0].ID, legacyPlan.ID)
	}
	if out[0].DailyQuotaKnives == nil || *out[0].DailyQuotaKnives != dailyQuota {
		t.Fatalf("DailyQuotaKnives = %v, want %v", out[0].DailyQuotaKnives, dailyQuota)
	}
	if out[0].WeeklyQuotaKnives == nil || *out[0].WeeklyQuotaKnives != weeklyQuota {
		t.Fatalf("WeeklyQuotaKnives = %v, want %v", out[0].WeeklyQuotaKnives, weeklyQuota)
	}
	if out[0].GroupID == nil || *out[0].GroupID != group.ID {
		t.Fatalf("GroupID = %v, want %d", out[0].GroupID, group.ID)
	}
	if len(out[0].Features) != 2 {
		t.Fatalf("Features len = %d, want 2", len(out[0].Features))
	}
}
