package admin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func postSubscriptionAssignValidation(t *testing.T, route string, body any, handlerFunc gin.HandlerFunc) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	c.Request, _ = http.NewRequest(http.MethodPost, route, bytes.NewReader(jsonBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		if recover() != nil {
			w.Code = 0
		}
	}()
	handlerFunc(c)
	return w.Code
}

type subscriptionHandlerHarness struct {
	ctx     context.Context
	db      *sql.DB
	client  *dbent.Client
	handler *SubscriptionHandler
}

func newSubscriptionHandlerHarness(t *testing.T) *subscriptionHandlerHarness {
	t.Helper()

	dbName := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()),
	)
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	groupRepo := repository.NewGroupRepository(client, db)
	userSubRepo := repository.NewUserSubscriptionRepository(client)
	svc := service.NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)

	return &subscriptionHandlerHarness{
		ctx:     context.Background(),
		db:      db,
		client:  client,
		handler: NewSubscriptionHandler(svc),
	}
}

func (h *subscriptionHandlerHarness) createUser(t *testing.T, email string) *dbent.User {
	t.Helper()
	user, err := h.client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(h.ctx)
	require.NoError(t, err)
	return user
}

func (h *subscriptionHandlerHarness) createGroup(t *testing.T, name string) *dbent.Group {
	t.Helper()
	group, err := h.client.Group.Create().
		SetName(name).
		SetStatus(service.StatusActive).
		Save(h.ctx)
	require.NoError(t, err)
	return group
}

func (h *subscriptionHandlerHarness) createPlan(t *testing.T, name string, groupID int64) *dbent.SubscriptionPlan {
	t.Helper()
	_ = groupID
	plan, err := h.client.SubscriptionPlan.Create().
		SetName(name).
		SetDescription(name).
		SetPrice(19.9).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetFeatures("").
		SetProductName(name).
		SetForSale(true).
		Save(h.ctx)
	require.NoError(t, err)
	return plan
}

func TestSubscriptionAssignRequiresPlanID(t *testing.T) {
	h := &SubscriptionHandler{}
	code := postSubscriptionAssignValidation(t, "/api/v1/admin/subscriptions/assign", map[string]any{
		"user_id":       1,
		"validity_days": 30,
	}, h.Assign)

	assert.Equal(t, http.StatusBadRequest, code)
}

func TestSubscriptionAssignAcceptsPlanIDWithoutGroupID(t *testing.T) {
	h := newSubscriptionHandlerHarness(t)
	user := h.createUser(t, "assign-handler@example.com")
	group := h.createGroup(t, "assign-handler-group")
	plan := h.createPlan(t, "Assign Handler Plan", group.ID)
	code := postSubscriptionAssignValidation(t, "/api/v1/admin/subscriptions/assign", map[string]any{
		"user_id":       user.ID,
		"plan_id":       plan.ID,
		"validity_days": 30,
	}, h.handler.Assign)

	assert.Equal(t, http.StatusOK, code)
}

func TestSubscriptionGetByIDIncludesSettlementChain(t *testing.T) {
	h := newSubscriptionHandlerHarness(t)
	operator := h.createUser(t, "detail-settlement-operator@example.com")
	user := h.createUser(t, "detail-settlement-user@example.com")
	group := h.createGroup(t, "detail-settlement-group")
	plan := h.createPlan(t, "Detail Settlement Plan", group.ID)

	sub, reused, err := h.handler.subscriptionService.AssignUserLevelSubscription(h.ctx, &service.AssignSubscriptionInput{
		UserID:     user.ID,
		PlanID:     plan.ID,
		AssignedBy: operator.ID,
		Notes:      "detail settlement",
	})
	require.NoError(t, err)
	require.False(t, reused)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", sub.ID)}}
	c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/subscriptions/%d", sub.ID), nil)

	h.handler.GetByID(c)

	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		Code int `json:"code"`
		Data struct {
			ID                    int64 `json:"id"`
			CurrentSettlementHead *struct {
				ActionType              string `json:"action_type"`
				ActionSource            string `json:"action_source"`
				Status                  string `json:"status"`
				AfterUserSubscriptionID *int64 `json:"after_user_subscription_id"`
			} `json:"current_settlement_head"`
			SettlementHistory []struct {
				ActionType   string `json:"action_type"`
				ActionSource string `json:"action_source"`
				Status       string `json:"status"`
			} `json:"settlement_history"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, sub.ID, payload.Data.ID)
	require.NotNil(t, payload.Data.CurrentSettlementHead)
	require.Equal(t, domain.SettlementActionPurchase, payload.Data.CurrentSettlementHead.ActionType)
	require.Equal(t, domain.SettlementActionSourceSubscriptionAssign, payload.Data.CurrentSettlementHead.ActionSource)
	require.Equal(t, domain.SettlementStatusEffective, payload.Data.CurrentSettlementHead.Status)
	require.NotNil(t, payload.Data.CurrentSettlementHead.AfterUserSubscriptionID)
	require.Equal(t, sub.ID, *payload.Data.CurrentSettlementHead.AfterUserSubscriptionID)
	require.Len(t, payload.Data.SettlementHistory, 1)
	require.Equal(t, domain.SettlementActionSourceSubscriptionAssign, payload.Data.SettlementHistory[0].ActionSource)
}

func TestSubscriptionBulkAssignRequiresPlanID(t *testing.T) {
	h := &SubscriptionHandler{}
	code := postSubscriptionAssignValidation(t, "/api/v1/admin/subscriptions/bulk-assign", map[string]any{
		"user_ids":      []int64{1, 2},
		"validity_days": 30,
	}, h.BulkAssign)

	assert.Equal(t, http.StatusBadRequest, code)
}

func TestSubscriptionBulkAssignAcceptsPlanIDWithoutGroupID(t *testing.T) {
	h := newSubscriptionHandlerHarness(t)
	user1 := h.createUser(t, "bulk-assign-handler-1@example.com")
	user2 := h.createUser(t, "bulk-assign-handler-2@example.com")
	group := h.createGroup(t, "bulk-assign-handler-group")
	plan := h.createPlan(t, "Bulk Assign Handler Plan", group.ID)
	code := postSubscriptionAssignValidation(t, "/api/v1/admin/subscriptions/bulk-assign", map[string]any{
		"user_ids":      []int64{user1.ID, user2.ID},
		"plan_id":       plan.ID,
		"validity_days": 30,
	}, h.handler.BulkAssign)

	assert.Equal(t, http.StatusOK, code)
}
