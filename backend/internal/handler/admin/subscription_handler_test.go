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
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(h.ctx)
	require.NoError(t, err)
	return group
}

func (h *subscriptionHandlerHarness) createPlan(t *testing.T, name string, groupID int64) *dbent.SubscriptionPlan {
	t.Helper()
	plan, err := h.client.SubscriptionPlan.Create().
		SetName(name).
		SetDescription(name).
		SetPrice(19.9).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetFeatures("").
		SetProductName(name).
		SetForSale(true).
		SetGroupID(groupID).
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
