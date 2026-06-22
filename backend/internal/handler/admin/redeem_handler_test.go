package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCreateAndRedeemHandler creates a RedeemHandler with a non-nil (but minimal)
// RedeemService so that CreateAndRedeem's nil guard passes and we can test the
// parameter-validation layer that runs before any service call.
func newCreateAndRedeemHandler() *RedeemHandler {
	return &RedeemHandler{
		adminService:  newStubAdminService(),
		redeemService: &service.RedeemService{},
	}
}

// postCreateAndRedeemValidation calls CreateAndRedeem and returns the response
// status code. If validation passes and the request reaches the minimal service
// stub, a panic is expected and reported as code 0.
func postCreateAndRedeemValidation(t *testing.T, handler *RedeemHandler, body any) (code int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	jsonBytes, err := json.Marshal(body)
	require.NoError(t, err)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/admin/redeem-codes/create-and-redeem", bytes.NewReader(jsonBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		if r := recover(); r != nil {
			code = 0
		}
	}()
	handler.CreateAndRedeem(c)
	return w.Code
}

func TestCreateAndRedeem_TypeDefaultsToBalance(t *testing.T) {
	h := newCreateAndRedeemHandler()
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":    "test-balance-default",
		"value":   10.0,
		"user_id": 1,
	})

	assert.NotEqual(t, http.StatusBadRequest, code)
}

func TestCreateAndRedeem_SubscriptionRequiresPlanID(t *testing.T) {
	h := newCreateAndRedeemHandler()
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":    "test-sub-no-plan",
		"type":    "subscription",
		"value":   29.9,
		"user_id": 1,
	})

	assert.Equal(t, http.StatusBadRequest, code)
}

func TestCreateAndRedeem_SubscriptionValidParamsPassValidation(t *testing.T) {
	planID := int64(5)
	h := newCreateAndRedeemHandler()
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":    "test-sub-valid",
		"type":    "subscription",
		"value":   29.9,
		"user_id": 1,
		"plan_id": planID,
	})

	assert.NotEqual(t, http.StatusBadRequest, code)
}

func TestCreateAndRedeem_BalanceIgnoresSubscriptionFields(t *testing.T) {
	h := newCreateAndRedeemHandler()
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":    "test-balance-no-extras",
		"type":    "balance",
		"value":   50.0,
		"user_id": 1,
	})

	assert.NotEqual(t, http.StatusBadRequest, code)
}

func TestResolveRedeemCodeExpiresAt_FromDays(t *testing.T) {
	days := 3
	expiresAt, err := resolveRedeemCodeExpiresAt(nil, &days)
	require.NoError(t, err)
	require.NotNil(t, expiresAt)
	require.WithinDuration(t, time.Now().UTC().AddDate(0, 0, days), *expiresAt, 2*time.Second)
}

func TestResolveRedeemCodeExpiresAt_RejectsPastAbsoluteTime(t *testing.T) {
	past := time.Now().UTC().Add(-time.Minute)
	expiresAt, err := resolveRedeemCodeExpiresAt(&past, nil)
	require.Error(t, err)
	require.Nil(t, expiresAt)
}

func TestResolveRedeemCodeExpiresAt_RejectsNonPositiveDays(t *testing.T) {
	days := 0
	expiresAt, err := resolveRedeemCodeExpiresAt(nil, &days)
	require.Error(t, err)
	require.Nil(t, expiresAt)
}

func TestResolveRedeemCodeExpiresAt_RejectsConflictingInputs(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	days := 3
	expiresAt, err := resolveRedeemCodeExpiresAt(&future, &days)
	require.Error(t, err)
	require.Nil(t, expiresAt)
}
