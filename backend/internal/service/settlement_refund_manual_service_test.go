package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSettlementRefundServiceUploadManualProofRejectsNonManualRequest(t *testing.T) {
	now := time.Date(2026, 6, 25, 19, 0, 0, 0, time.UTC)
	record := &SettlementRefundRequestRecord{
		ID:                   9001,
		Status:               SettlementRefundStatusSubmitted,
		ManualTransferAmount: 0,
	}
	store := &settlementRefundManualStoreStub{request: record}
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.UploadSettlementRefundManualProof(context.Background(), SettlementRefundManualProofInput{
		RefundRequestID: 9001,
		OperatorUserID:  88,
		ProofURL:        "uploads/refund/proof/9001.png",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundManualProofState)
}

func TestSettlementRefundServiceUploadManualProofRejectsInvalidState(t *testing.T) {
	now := time.Date(2026, 6, 25, 19, 0, 0, 0, time.UTC)
	record := &SettlementRefundRequestRecord{
		ID:                   9001,
		Status:               SettlementRefundStatusPreviewed,
		ManualTransferAmount: 69.5,
	}
	store := &settlementRefundManualStoreStub{request: record}
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.UploadSettlementRefundManualProof(context.Background(), SettlementRefundManualProofInput{
		RefundRequestID: 9001,
		OperatorUserID:  88,
		ProofURL:        "uploads/refund/proof/9001.png",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundManualProofState)
}

func TestSettlementRefundServiceUploadManualProofRequiresProofURL(t *testing.T) {
	now := time.Date(2026, 6, 25, 19, 0, 0, 0, time.UTC)
	record := &SettlementRefundRequestRecord{
		ID:                   9001,
		Status:               SettlementRefundStatusSubmitted,
		ManualTransferAmount: 69.5,
	}
	store := &settlementRefundManualStoreStub{request: record}
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.UploadSettlementRefundManualProof(context.Background(), SettlementRefundManualProofInput{
		RefundRequestID: 9001,
		OperatorUserID:  88,
		ProofURL:        "   ",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundManualProofRequired)
}

func TestSettlementRefundServiceUploadManualProofRejectsInvalidProofURL(t *testing.T) {
	now := time.Date(2026, 6, 25, 19, 0, 0, 0, time.UTC)
	record := &SettlementRefundRequestRecord{
		ID:                   9001,
		Status:               SettlementRefundStatusSubmitted,
		ManualTransferAmount: 69.5,
	}
	store := &settlementRefundManualStoreStub{request: record}
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.UploadSettlementRefundManualProof(context.Background(), SettlementRefundManualProofInput{
		RefundRequestID: 9001,
		OperatorUserID:  88,
		ProofURL:        "javascript:alert(1)",
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundManualProofInvalid)
}

func TestSettlementRefundServiceUploadManualProofPersistsProof(t *testing.T) {
	now := time.Date(2026, 6, 25, 19, 0, 0, 0, time.UTC)
	record := &SettlementRefundRequestRecord{
		ID:                   9001,
		Status:               SettlementRefundStatusSubmitted,
		ManualTransferAmount: 69.5,
	}
	store := &settlementRefundManualStoreStub{request: record}
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.UploadSettlementRefundManualProof(context.Background(), SettlementRefundManualProofInput{
		RefundRequestID: 9001,
		OperatorUserID:  88,
		ProofURL:        " uploads/refund/proof/9001.png ",
		AdminNote:       " transfer done ",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusManualPending, result.Status)
	require.Equal(t, "uploads/refund/proof/9001.png", result.ManualTransferProofURL)
	require.Equal(t, int64(88), result.ManualTransferOperatorUserID)
	require.Equal(t, "transfer done", result.AdminNote)
	require.NotNil(t, store.lastUpdate)
	require.Equal(t, SettlementRefundStatusSubmitted, store.lastUpdate.ExpectedStatus)
	require.Equal(t, SettlementRefundStatusManualPending, store.lastUpdate.Status)
	require.Equal(t, "uploads/refund/proof/9001.png", store.lastUpdate.ProofURL)
}

type settlementRefundManualStoreStub struct {
	request    *SettlementRefundRequestRecord
	lastUpdate *UpdateSettlementRefundManualProofInput
	updateFn   func(UpdateSettlementRefundManualProofInput) (*SettlementRefundRequestRecord, error)
}

func (s *settlementRefundManualStoreStub) CreateSettlementRefundPreview(context.Context, CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	panic("unexpected CreateSettlementRefundPreview call")
}

func (s *settlementRefundManualStoreStub) GetSettlementRefundRequest(_ context.Context, id int64) (*SettlementRefundRequestRecord, error) {
	if s.request == nil || s.request.ID != id {
		return nil, ErrSettlementRefundRequestNotFound
	}
	return cloneSettlementRefundRequestRecord(s.request), nil
}

func (s *settlementRefundManualStoreStub) UpdateSettlementRefundManualProof(_ context.Context, input UpdateSettlementRefundManualProofInput) (*SettlementRefundRequestRecord, error) {
	s.lastUpdate = &input
	if s.updateFn != nil {
		return s.updateFn(input)
	}
	record := cloneSettlementRefundRequestRecord(s.request)
	record.Status = input.Status
	record.ManualTransferProofURL = settlementRefundTestPtrString(input.ProofURL)
	record.ManualTransferProofUploadedAt = &input.UploadedAt
	record.ManualTransferOperatorUserID = &input.OperatorUserID
	record.AdminNote = input.AdminNote
	return record, nil
}
