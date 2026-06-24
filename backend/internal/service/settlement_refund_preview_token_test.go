package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewSettlementRefundPreviewWindowUsesTwoMinuteTTL(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

	window := newSettlementRefundPreviewWindow(now)

	require.Equal(t, now, window.IssuedAt)
	require.Equal(t, now.Add(2*time.Minute), window.ExpiresAt)
	require.False(t, settlementRefundPreviewExpired(now.Add(2*time.Minute), window.ExpiresAt))
	require.True(t, settlementRefundPreviewExpired(now.Add(2*time.Minute+time.Nanosecond), window.ExpiresAt))
}

func TestSettlementRefundPreviewExpiredTreatsMissingExpiryAsExpired(t *testing.T) {
	require.True(t, settlementRefundPreviewExpired(time.Now(), time.Time{}))
}

func TestSettlementRefundPreviewTokenHashVerification(t *testing.T) {
	token, tokenHash, err := newSettlementRefundPreviewToken()
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Len(t, tokenHash, 64)

	require.True(t, verifySettlementRefundPreviewToken(token, tokenHash))
	require.True(t, verifySettlementRefundPreviewToken(" "+token+" ", tokenHash))
	require.False(t, verifySettlementRefundPreviewToken(token+"x", tokenHash))
	require.False(t, verifySettlementRefundPreviewToken(token, ""))
	require.False(t, verifySettlementRefundPreviewToken("", tokenHash))
}
