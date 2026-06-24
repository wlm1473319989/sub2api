package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
)

const settlementRefundPreviewTTL = 2 * time.Minute
const settlementRefundPreviewTokenBytes = 32

type settlementRefundPreviewWindow struct {
	IssuedAt  time.Time
	ExpiresAt time.Time
}

func newSettlementRefundPreviewWindow(now time.Time) settlementRefundPreviewWindow {
	if now.IsZero() {
		now = time.Now()
	}
	return settlementRefundPreviewWindow{
		IssuedAt:  now,
		ExpiresAt: now.Add(settlementRefundPreviewTTL),
	}
}

func settlementRefundPreviewExpired(now, expiresAt time.Time) bool {
	if expiresAt.IsZero() {
		return true
	}
	if now.IsZero() {
		now = time.Now()
	}
	return now.After(expiresAt)
}

func newSettlementRefundPreviewToken() (token string, tokenHash string, err error) {
	raw := make([]byte, settlementRefundPreviewTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashSettlementRefundPreviewToken(token), nil
}

func hashSettlementRefundPreviewToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func verifySettlementRefundPreviewToken(token, expectedHash string) bool {
	token = strings.TrimSpace(token)
	expectedHash = strings.TrimSpace(expectedHash)
	if token == "" || expectedHash == "" {
		return false
	}
	actualHash := hashSettlementRefundPreviewToken(token)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}
