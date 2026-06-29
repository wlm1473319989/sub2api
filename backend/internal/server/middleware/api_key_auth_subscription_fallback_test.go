//go:build unit

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuth_AllowsSubscriptionFallbackToBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := 1.0
	group := &service.Group{
		ID:       42,
		Name:     "sub",
		Status:   service.StatusActive,
		Hydrated: true,
	}
	user := &service.User{
		ID:          7,
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		Balance:     10,
		Concurrency: 3,
	}
	apiKey := &service.APIKey{
		ID:     100,
		UserID: user.ID,
		Key:    "fallback-balance",
		Status: service.StatusActive,
		User:   user,
		Group:  group,
	}
	apiKey.GroupID = &group.ID

	apiKeyService := service.NewAPIKeyService(&stubApiKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	}, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeStandard})

	now := time.Now()
	sub := &service.UserSubscription{
		ID:               55,
		UserID:           user.ID,
		Status:           service.SubscriptionStatusActive,
		ExpiresAt:        now.Add(24 * time.Hour),
		DailyWindowStart: &now,
		DailyQuotaKnives: &limit,
		DailyUsageUSD:    10,
		DailyUsedKnives:  10,
	}
	subscriptionService := service.NewSubscriptionService(nil, &stubUserSubscriptionRepo{
		getActive: func(ctx context.Context, userID int64) (*service.UserSubscription, error) {
			clone := *sub
			return &clone, nil
		},
		updateStatus:   func(ctx context.Context, subscriptionID int64, status string) error { return nil },
		activateWindow: func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetDaily:     func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetWeekly:    func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetMonthly:   func(ctx context.Context, id int64, start time.Time) error { return nil },
	}, nil, nil, &config.Config{RunMode: config.RunModeStandard})
	t.Cleanup(subscriptionService.Stop)

	router := newAuthTestRouter(apiKeyService, subscriptionService, &config.Config{RunMode: config.RunModeStandard})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-api-key", apiKey.Key)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestAPIKeyAuthGoogle_AllowsSubscriptionFallbackToBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := 1.0
	group := &service.Group{
		ID:       77,
		Name:     "gemini-sub",
		Status:   service.StatusActive,
		Platform: service.PlatformGemini,
		Hydrated: true,
	}
	user := &service.User{
		ID:          999,
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		Balance:     10,
		Concurrency: 3,
	}
	apiKey := &service.APIKey{
		ID:     501,
		UserID: user.ID,
		Key:    "google-sub-fallback",
		Status: service.StatusActive,
		User:   user,
		Group:  group,
	}
	apiKey.GroupID = &group.ID

	apiKeyService := newTestAPIKeyService(fakeAPIKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	})

	now := time.Now()
	sub := &service.UserSubscription{
		ID:               601,
		UserID:           user.ID,
		Status:           service.SubscriptionStatusActive,
		ExpiresAt:        now.Add(24 * time.Hour),
		DailyWindowStart: &now,
		DailyQuotaKnives: &limit,
		DailyUsageUSD:    10,
		DailyUsedKnives:  10,
	}
	subscriptionService := service.NewSubscriptionService(nil, fakeGoogleSubscriptionRepo{
		getActive: func(ctx context.Context, userID int64) (*service.UserSubscription, error) {
			clone := *sub
			return &clone, nil
		},
		updateStatus:   func(ctx context.Context, subscriptionID int64, status string) error { return nil },
		activateWindow: func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetDaily:     func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetWeekly:    func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetMonthly:   func(ctx context.Context, id int64, start time.Time) error { return nil },
	}, nil, nil, &config.Config{RunMode: config.RunModeStandard})
	t.Cleanup(subscriptionService.Stop)

	r := gin.New()
	r.Use(APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, &config.Config{RunMode: config.RunModeStandard}))
	r.GET("/v1beta/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/v1beta/test", nil)
	req.Header.Set("x-goog-api-key", apiKey.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIKeyAuthGoogle_PreservesSubscriptionErrorWhenNeitherQuotaNorBalanceAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limit := 1.0
	group := &service.Group{
		ID:       77,
		Name:     "gemini-sub",
		Status:   service.StatusActive,
		Platform: service.PlatformGemini,
		Hydrated: true,
	}
	user := &service.User{
		ID:          999,
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		Balance:     0,
		Concurrency: 3,
	}
	apiKey := &service.APIKey{
		ID:     501,
		UserID: user.ID,
		Key:    "google-sub-no-fallback",
		Status: service.StatusActive,
		User:   user,
		Group:  group,
	}
	apiKey.GroupID = &group.ID

	apiKeyService := newTestAPIKeyService(fakeAPIKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			return &clone, nil
		},
	})

	now := time.Now()
	sub := &service.UserSubscription{
		ID:               601,
		UserID:           user.ID,
		Status:           service.SubscriptionStatusActive,
		ExpiresAt:        now.Add(24 * time.Hour),
		DailyWindowStart: &now,
		DailyQuotaKnives: &limit,
		DailyUsageUSD:    10,
		DailyUsedKnives:  10,
	}
	subscriptionService := service.NewSubscriptionService(nil, fakeGoogleSubscriptionRepo{
		getActive: func(ctx context.Context, userID int64) (*service.UserSubscription, error) {
			clone := *sub
			return &clone, nil
		},
		updateStatus:   func(ctx context.Context, subscriptionID int64, status string) error { return nil },
		activateWindow: func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetDaily:     func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetWeekly:    func(ctx context.Context, id int64, start time.Time) error { return nil },
		resetMonthly:   func(ctx context.Context, id int64, start time.Time) error { return nil },
	}, nil, nil, &config.Config{RunMode: config.RunModeStandard})
	t.Cleanup(subscriptionService.Stop)

	r := gin.New()
	r.Use(APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, &config.Config{RunMode: config.RunModeStandard}))
	r.GET("/v1beta/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/v1beta/test", nil)
	req.Header.Set("x-goog-api-key", apiKey.Key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	var resp googleErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "RESOURCE_EXHAUSTED", resp.Error.Status)
	require.Contains(t, resp.Error.Message, "daily usage limit exceeded")
}
