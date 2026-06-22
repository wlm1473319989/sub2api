package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type userSubscriptionHandlerHarness struct {
	ctx     context.Context
	db      *sql.DB
	client  *dbent.Client
	handler *SubscriptionHandler
}

func newUserSubscriptionHandlerHarness(t *testing.T) *userSubscriptionHandlerHarness {
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

	return &userSubscriptionHandlerHarness{
		ctx:     context.Background(),
		db:      db,
		client:  client,
		handler: NewSubscriptionHandler(svc),
	}
}

func (h *userSubscriptionHandlerHarness) createUser(t *testing.T, email string) *dbent.User {
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

func (h *userSubscriptionHandlerHarness) createPlanOnlySubscription(t *testing.T, userID int64, planName string) *dbent.UserSubscription {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	sub, err := h.client.UserSubscription.Create().
		SetUserID(userID).
		SetPlanID(88).
		SetPlanNameSnapshot(planName).
		SetStartsAt(now).
		SetExpiresAt(now.Add(30 * 24 * time.Hour)).
		SetStatus(service.SubscriptionStatusActive).
		Save(h.ctx)
	require.NoError(t, err)
	return sub
}

func newAuthenticatedTestContext(method, path string, userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	return c, w
}

func TestSubscriptionHandlerGetSummaryUsesDisplayName(t *testing.T) {
	h := newUserSubscriptionHandlerHarness(t)
	user := h.createUser(t, "summary-display@example.com")
	h.createPlanOnlySubscription(t, user.ID, "Starter Plan")

	c, w := newAuthenticatedTestContext(http.MethodGet, "/api/v1/subscriptions/summary", user.ID)
	h.handler.GetSummary(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Subscriptions []map[string]any `json:"subscriptions"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Subscriptions, 1)
	assert.Equal(t, "Starter Plan", resp.Data.Subscriptions[0]["display_name"])
	_, hasGroupName := resp.Data.Subscriptions[0]["group_name"]
	assert.False(t, hasGroupName)
	_, hasGroupID := resp.Data.Subscriptions[0]["group_id"]
	assert.False(t, hasGroupID)
}

func TestSubscriptionHandlerGetProgressUsesDisplayName(t *testing.T) {
	h := newUserSubscriptionHandlerHarness(t)
	user := h.createUser(t, "progress-display@example.com")
	h.createPlanOnlySubscription(t, user.ID, "Starter Plan")

	c, w := newAuthenticatedTestContext(http.MethodGet, "/api/v1/subscriptions/progress", user.ID)
	h.handler.GetProgress(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int `json:"code"`
		Data []struct {
			Progress map[string]any `json:"progress"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "Starter Plan", resp.Data[0].Progress["display_name"])
	_, hasGroupName := resp.Data[0].Progress["group_name"]
	assert.False(t, hasGroupName)
}
