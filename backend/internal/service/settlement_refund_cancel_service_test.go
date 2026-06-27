package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSettlementRefundServiceCancelRejectsAfterPayout(t *testing.T) {
	now := time.Date(2026, 6, 25, 21, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.Status = SubscriptionStatusSuspended
	record := &SettlementRefundRequestRecord{
		ID:                     9001,
		UserID:                 active.UserID,
		SubscriptionID:         active.ID,
		Status:                 SettlementRefundStatusManualPending,
		OriginalSubscriptionStatus: ptrString(SubscriptionStatusActive),
		OriginalSubscriptionExpiresAt: timePtr(now.Add(24 * time.Hour)),
		ManualTransferProofURL: ptrString("uploads/refund/proof/9001.png"),
	}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	store := &settlementRefundCancelStoreStub{request: record}
	service := &SettlementRefundService{
		subscription: &SubscriptionService{userSubRepo: repo},
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.CancelSettlementRefund(context.Background(), SettlementRefundCancelInput{
		RefundRequestID: record.ID,
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundCancelAfterPayout)
}

func TestSettlementRefundServiceCancelRestoresActiveSubscription(t *testing.T) {
	now := time.Date(2026, 6, 25, 21, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.Status = SubscriptionStatusSuspended
	expiresAt := now.Add(24 * time.Hour)
	record := &SettlementRefundRequestRecord{
		ID:                            9001,
		UserID:                        active.UserID,
		SubscriptionID:                active.ID,
		Status:                        SettlementRefundStatusSubmitted,
		OriginalSubscriptionStatus:    ptrString(SubscriptionStatusActive),
		OriginalSubscriptionExpiresAt: &expiresAt,
		Allocations: []SettlementRefundAllocationRecord{
			{Status: SettlementRefundAllocationStatusSkipped},
		},
	}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	store := &settlementRefundCancelStoreStub{request: record}
	service := &SettlementRefundService{
		subscription: &SubscriptionService{userSubRepo: repo},
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.CancelSettlementRefund(context.Background(), SettlementRefundCancelInput{
		RefundRequestID: record.ID,
		AdminNote:       "cancel before payout",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusCancelled, result.Status)
	require.Equal(t, SubscriptionStatusActive, result.SubscriptionStatus)

	updated, err := repo.GetByID(context.Background(), active.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusActive, updated.Status)
	require.Equal(t, expiresAt, updated.ExpiresAt)
	require.NotNil(t, store.lastCancel)
	require.Equal(t, SettlementRefundStatusSubmitted, store.lastCancel.ExpectedStatus)
}

func TestSettlementRefundServiceCancelRestoresExpiredSubscription(t *testing.T) {
	now := time.Date(2026, 6, 25, 21, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.Status = SubscriptionStatusSuspended
	expiredAt := now.Add(-time.Minute)
	record := &SettlementRefundRequestRecord{
		ID:                            9001,
		UserID:                        active.UserID,
		SubscriptionID:                active.ID,
		Status:                        SettlementRefundStatusSubmitted,
		OriginalSubscriptionStatus:    ptrString(SubscriptionStatusActive),
		OriginalSubscriptionExpiresAt: &expiredAt,
	}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	store := &settlementRefundCancelStoreStub{request: record}
	service := &SettlementRefundService{
		subscription: &SubscriptionService{userSubRepo: repo},
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.CancelSettlementRefund(context.Background(), SettlementRefundCancelInput{
		RefundRequestID: record.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SubscriptionStatusExpired, result.SubscriptionStatus)
}

func TestSettlementRefundServiceCancelAllowsFailedWithoutPayoutEvidence(t *testing.T) {
	now := time.Date(2026, 6, 25, 21, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.Status = SubscriptionStatusSuspended
	expiresAt := now.Add(24 * time.Hour)
	record := &SettlementRefundRequestRecord{
		ID:                            9001,
		UserID:                        active.UserID,
		SubscriptionID:                active.ID,
		Status:                        SettlementRefundStatusFailed,
		OriginalSubscriptionStatus:    ptrString(SubscriptionStatusActive),
		OriginalSubscriptionExpiresAt: &expiresAt,
		Allocations: []SettlementRefundAllocationRecord{
			{Status: SettlementRefundAllocationStatusFailed},
		},
	}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	store := &settlementRefundCancelStoreStub{request: record}
	service := &SettlementRefundService{
		subscription: &SubscriptionService{userSubRepo: repo},
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.CancelSettlementRefund(context.Background(), SettlementRefundCancelInput{
		RefundRequestID: record.ID,
		AdminNote:       "cancel failed refund",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusCancelled, result.Status)
	require.Equal(t, SubscriptionStatusActive, result.SubscriptionStatus)
}

type settlementRefundCancelStoreStub struct {
	request    *SettlementRefundRequestRecord
	lastCancel *CancelSettlementRefundRequestInput
	cancelFn   func(CancelSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error)
}

func (s *settlementRefundCancelStoreStub) CreateSettlementRefundPreview(context.Context, CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	panic("unexpected CreateSettlementRefundPreview call")
}

func (s *settlementRefundCancelStoreStub) GetSettlementRefundRequest(_ context.Context, id int64) (*SettlementRefundRequestRecord, error) {
	if s.request == nil || s.request.ID != id {
		return nil, ErrSettlementRefundRequestNotFound
	}
	return cloneSettlementRefundRequestRecord(s.request), nil
}

func (s *settlementRefundCancelStoreStub) CancelSettlementRefundRequest(_ context.Context, input CancelSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	s.lastCancel = &input
	if s.cancelFn != nil {
		return s.cancelFn(input)
	}
	record := cloneSettlementRefundRequestRecord(s.request)
	record.Status = SettlementRefundStatusCancelled
	record.CancelledAt = &input.CancelledAt
	record.AdminNote = input.AdminNote
	return record, nil
}

func timePtr(v time.Time) *time.Time {
	return &v
}
