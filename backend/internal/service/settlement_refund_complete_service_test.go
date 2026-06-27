package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSettlementRefundServiceCompleteRequiresManualProof(t *testing.T) {
	now := time.Date(2026, 6, 25, 20, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.Status = SubscriptionStatusSuspended
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceUserPurchase)
	record := &SettlementRefundRequestRecord{
		ID:                   9001,
		UserID:               active.UserID,
		SubscriptionID:       active.ID,
		ExpectedSettlementID: head.ID,
		Status:               SettlementRefundStatusSubmitted,
		RefundResidualValue:  168.5,
		ManualTransferAmount: 69.5,
		Allocations: []SettlementRefundAllocationRecord{
			{Status: SettlementRefundAllocationStatusSucceeded},
		},
	}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	store := &settlementRefundCompleteStoreStub{request: record}
	service := &SettlementRefundService{
		subscription: &SubscriptionService{userSubRepo: repo},
		settlement:   &SettlementService{},
		requestStore: store,
		now:          func() time.Time { return now },
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return cloneSettlementHead(head), nil
		},
	}

	result, err := service.CompleteSettlementRefund(context.Background(), SettlementRefundCompleteInput{
		RefundRequestID: record.ID,
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundCompleteManualRequired)
}

func TestSettlementRefundServiceCompleteRefundsSubscriptionAndCreatesSettlement(t *testing.T) {
	now := time.Date(2026, 6, 25, 20, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.Status = SubscriptionStatusSuspended
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	reason := "refund complete"
	record := &SettlementRefundRequestRecord{
		ID:                     9001,
		UserID:                 active.UserID,
		SubscriptionID:         active.ID,
		ExpectedSettlementID:   head.ID,
		Status:                 SettlementRefundStatusManualPending,
		Reason:                 &reason,
		RefundResidualValue:    168.5,
		ManualTransferAmount:   69.5,
		ManualTransferProofURL: ptrString("uploads/refund/proof/9001.png"),
		Allocations: []SettlementRefundAllocationRecord{
			{Status: SettlementRefundAllocationStatusSkipped},
		},
	}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	store := &settlementRefundCompleteStoreStub{request: record}
	var capturedSettlementInput *SettlementOrderInput
	service := &SettlementRefundService{
		subscription: &SubscriptionService{userSubRepo: repo},
		settlement:   &SettlementService{},
		requestStore: store,
		now:          func() time.Time { return now },
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return cloneSettlementHead(head), nil
		},
		createSettlementOrder: func(_ context.Context, input SettlementOrderInput) (*dbent.SubscriptionSettlementOrder, error) {
			capturedSettlementInput = &input
			return &dbent.SubscriptionSettlementOrder{
				ID:                   9301,
				ActionType:           input.ActionType,
				ActionSource:         input.ActionSource,
				TriggerRefType:       input.TriggerRefType,
				AfterSettlementValue: input.AfterSettlementValue,
			}, nil
		},
	}

	result, err := service.CompleteSettlementRefund(context.Background(), SettlementRefundCompleteInput{
		RefundRequestID: record.ID,
		OperatorUserID:  77,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusCompleted, result.Status)
	require.Equal(t, SubscriptionStatusRefunded, result.SubscriptionStatus)
	require.Equal(t, int64(9301), result.SettlementOrderID)
	require.NotNil(t, capturedSettlementInput)
	require.Equal(t, domain.SettlementActionRefund, capturedSettlementInput.ActionType)
	require.Equal(t, domain.SettlementActionSourceExchangeCode, capturedSettlementInput.ActionSource)
	require.Equal(t, domain.SubscriptionStatusRefunded, capturedSettlementInput.AfterSubscriptionStatus)
	require.InDelta(t, -168.5, capturedSettlementInput.ActionDeltaValue, 1e-9)

	updated, err := repo.GetByID(context.Background(), active.ID)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusRefunded, updated.Status)
	require.Equal(t, now, updated.ExpiresAt)
	require.NotNil(t, store.lastComplete)
	require.Equal(t, SettlementRefundStatusManualPending, store.lastComplete.ExpectedStatus)
}

func TestSettlementRefundServiceCompleteDoesNotRequireManualProofForSmallRemainder(t *testing.T) {
	now := time.Date(2026, 6, 25, 20, 0, 0, 0, time.UTC)
	active := settlementRefundPreviewTestActiveSubscription()
	active.Status = SubscriptionStatusSuspended
	head := settlementRefundPreviewTestSettlementHead(active.UserID, active.ID, domain.SettlementActionSourceExchangeCode)
	record := &SettlementRefundRequestRecord{
		ID:                   9011,
		UserID:               active.UserID,
		SubscriptionID:       active.ID,
		ExpectedSettlementID: head.ID,
		Status:               SettlementRefundStatusGatewayProcessing,
		Currency:             "CNY",
		RefundResidualValue:  19.7046,
		ManualTransferAmount: 0.0046,
		Allocations: []SettlementRefundAllocationRecord{
			{Status: SettlementRefundAllocationStatusSucceeded},
		},
	}
	repo := newSubscriptionUserSubRepoStub()
	repo.seed(active)
	store := &settlementRefundCompleteStoreStub{request: record}
	service := &SettlementRefundService{
		subscription: &SubscriptionService{userSubRepo: repo},
		settlement:   &SettlementService{},
		requestStore: store,
		now:          func() time.Time { return now },
		loadEffectiveHead: func(context.Context, int64, time.Time) (*dbent.SubscriptionSettlementOrder, error) {
			return cloneSettlementHead(head), nil
		},
		createSettlementOrder: func(_ context.Context, input SettlementOrderInput) (*dbent.SubscriptionSettlementOrder, error) {
			return &dbent.SubscriptionSettlementOrder{
				ID:                   9311,
				ActionType:           input.ActionType,
				ActionSource:         input.ActionSource,
				TriggerRefType:       input.TriggerRefType,
				AfterSettlementValue: input.AfterSettlementValue,
			}, nil
		},
	}

	result, err := service.CompleteSettlementRefund(context.Background(), SettlementRefundCompleteInput{
		RefundRequestID: record.ID,
		OperatorUserID:  77,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusCompleted, result.Status)
}

type settlementRefundCompleteStoreStub struct {
	request      *SettlementRefundRequestRecord
	lastComplete *CompleteSettlementRefundRequestInput
	completeFn   func(CompleteSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error)
}

func (s *settlementRefundCompleteStoreStub) CreateSettlementRefundPreview(context.Context, CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	panic("unexpected CreateSettlementRefundPreview call")
}

func (s *settlementRefundCompleteStoreStub) GetSettlementRefundRequest(_ context.Context, id int64) (*SettlementRefundRequestRecord, error) {
	if s.request == nil || s.request.ID != id {
		return nil, ErrSettlementRefundRequestNotFound
	}
	return cloneSettlementRefundRequestRecord(s.request), nil
}

func (s *settlementRefundCompleteStoreStub) CompleteSettlementRefundRequest(_ context.Context, input CompleteSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	s.lastComplete = &input
	if s.completeFn != nil {
		return s.completeFn(input)
	}
	record := cloneSettlementRefundRequestRecord(s.request)
	record.Status = SettlementRefundStatusCompleted
	record.CompletedAt = &input.CompletedAt
	return record, nil
}
