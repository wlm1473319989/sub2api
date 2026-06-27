package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestSettlementRefundServiceProcessGatewayRefundsMovesToManualPending(t *testing.T) {
	now := time.Date(2026, 6, 25, 22, 0, 0, 0, time.UTC)
	record := cloneGatewaySettlementRefundRequestRecord(&SettlementRefundRequestRecord{
		ID:                   9001,
		UserID:               11,
		SubscriptionID:       22,
		Status:               SettlementRefundStatusSubmitted,
		ManualTransferAmount: 69.5,
		Reason:               gatewayRefundStringPtr("gateway success"),
		Allocations: []SettlementRefundAllocationRecord{
			{
				ID:                    9101,
				RefundRequestID:       9001,
				PaymentOrderID:        1001,
				AlreadyRefundedAmount: 0,
				GatewayRefundAmount:   99,
				Currency:              "CNY",
				Status:                SettlementRefundAllocationStatusPending,
			},
			{
				ID:                  9102,
				RefundRequestID:     9001,
				PaymentOrderID:      1002,
				GatewayRefundAmount: 0,
				Currency:            "CNY",
				Status:              SettlementRefundAllocationStatusSkipped,
			},
		},
	})
	store := &settlementRefundGatewayStoreStub{request: record}
	var providerCalls int
	var syncCalls int
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
		loadRefundPaymentOrder: func(_ context.Context, id int64) (*dbent.PaymentOrder, error) {
			return gatewayRefundPaymentOrder(id), nil
		},
		resolveRefundProvider: func(context.Context, *dbent.PaymentOrder) (payment.Provider, error) {
			providerCalls++
			return &settlementRefundGatewayProviderStub{
				refundFn: func(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
					return &payment.RefundResponse{RefundID: "refund-1001", Status: payment.ProviderStatusSuccess}, nil
				},
			}, nil
		},
		syncGatewayPaymentOrderRefund: func(context.Context, *dbent.PaymentOrder, *SettlementRefundRequestRecord, SettlementRefundAllocationRecord, time.Time) error {
			syncCalls++
			return nil
		},
		markGatewayPaymentOrderRefundFailed: func(context.Context, *dbent.PaymentOrder, string, time.Time) error {
			return nil
		},
	}

	result, err := service.ProcessSettlementRefundGateway(context.Background(), SettlementRefundGatewayInput{
		RefundRequestID: 9001,
		OperatorUserID:  77,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusManualPending, result.Status)
	require.Equal(t, 1, result.SucceededAllocations)
	require.Equal(t, 1, result.SkippedAllocations)
	require.Equal(t, 0, result.FailedAllocations)
	require.Equal(t, 1, providerCalls)
	require.Equal(t, 1, syncCalls)
	require.Equal(t, SettlementRefundStatusManualPending, store.request.Status)
}

func TestSettlementRefundServiceProcessGatewayRefundsMarksFailuresAndContinues(t *testing.T) {
	now := time.Date(2026, 6, 25, 22, 30, 0, 0, time.UTC)
	record := cloneGatewaySettlementRefundRequestRecord(&SettlementRefundRequestRecord{
		ID:             9002,
		UserID:         11,
		SubscriptionID: 22,
		Status:         SettlementRefundStatusSubmitted,
		Allocations: []SettlementRefundAllocationRecord{
			{
				ID:                  9201,
				RefundRequestID:     9002,
				PaymentOrderID:      1001,
				GatewayRefundAmount: 30,
				Currency:            "CNY",
				Status:              SettlementRefundAllocationStatusPending,
			},
			{
				ID:                  9202,
				RefundRequestID:     9002,
				PaymentOrderID:      1002,
				GatewayRefundAmount: 20,
				Currency:            "CNY",
				Status:              SettlementRefundAllocationStatusPending,
			},
		},
	})
	store := &settlementRefundGatewayStoreStub{request: record}
	var providerCalls int
	var failedMarks int
	var syncCalls int
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
		loadRefundPaymentOrder: func(_ context.Context, id int64) (*dbent.PaymentOrder, error) {
			return gatewayRefundPaymentOrder(id), nil
		},
		resolveRefundProvider: func(context.Context, *dbent.PaymentOrder) (payment.Provider, error) {
			providerCalls++
			return &settlementRefundGatewayProviderStub{
				refundFn: func(_ context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
					if providerCalls == 1 {
						return nil, errors.New("gateway unavailable")
					}
					return &payment.RefundResponse{RefundID: "refund-1002", Status: payment.ProviderStatusSuccess}, nil
				},
			}, nil
		},
		syncGatewayPaymentOrderRefund: func(context.Context, *dbent.PaymentOrder, *SettlementRefundRequestRecord, SettlementRefundAllocationRecord, time.Time) error {
			syncCalls++
			return nil
		},
		markGatewayPaymentOrderRefundFailed: func(context.Context, *dbent.PaymentOrder, string, time.Time) error {
			failedMarks++
			return nil
		},
	}

	result, err := service.ProcessSettlementRefundGateway(context.Background(), SettlementRefundGatewayInput{
		RefundRequestID: 9002,
		OperatorUserID:  77,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusFailed, result.Status)
	require.Equal(t, 1, result.SucceededAllocations)
	require.Equal(t, 1, result.FailedAllocations)
	require.Equal(t, 2, providerCalls)
	require.Equal(t, 1, syncCalls)
	require.Equal(t, 1, failedMarks)
	require.Equal(t, SettlementRefundStatusFailed, store.request.Status)
}

func TestSettlementRefundServiceProcessGatewayRefundsSkipsSucceededAllocations(t *testing.T) {
	now := time.Date(2026, 6, 25, 23, 0, 0, 0, time.UTC)
	record := cloneGatewaySettlementRefundRequestRecord(&SettlementRefundRequestRecord{
		ID:             9003,
		UserID:         11,
		SubscriptionID: 22,
		Status:         SettlementRefundStatusGatewayProcessing,
		Allocations: []SettlementRefundAllocationRecord{
			{
				ID:                    9301,
				RefundRequestID:       9003,
				PaymentOrderID:        1001,
				AlreadyRefundedAmount: 0,
				GatewayRefundAmount:   99,
				Currency:              "CNY",
				Status:                SettlementRefundAllocationStatusSucceeded,
				GatewayRefundTradeNo:  gatewayRefundStringPtr("refund-trade-9301"),
			},
		},
	})
	store := &settlementRefundGatewayStoreStub{request: record}
	var providerCalls int
	var syncCalls int
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
		loadRefundPaymentOrder: func(_ context.Context, id int64) (*dbent.PaymentOrder, error) {
			return gatewayRefundPaymentOrder(id), nil
		},
		resolveRefundProvider: func(context.Context, *dbent.PaymentOrder) (payment.Provider, error) {
			providerCalls++
			return &settlementRefundGatewayProviderStub{}, nil
		},
		syncGatewayPaymentOrderRefund: func(context.Context, *dbent.PaymentOrder, *SettlementRefundRequestRecord, SettlementRefundAllocationRecord, time.Time) error {
			syncCalls++
			return nil
		},
		markGatewayPaymentOrderRefundFailed: func(context.Context, *dbent.PaymentOrder, string, time.Time) error {
			return nil
		},
	}

	result, err := service.ProcessSettlementRefundGateway(context.Background(), SettlementRefundGatewayInput{
		RefundRequestID: 9003,
		OperatorUserID:  77,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusGatewayProcessing, result.Status)
	require.Equal(t, 0, providerCalls)
	require.Equal(t, 1, syncCalls)
	require.Equal(t, SettlementRefundStatusGatewayProcessing, store.request.Status)
}

func TestSettlementRefundServiceProcessGatewayRefundsRejectsRequestsWithoutGatewayWork(t *testing.T) {
	now := time.Date(2026, 6, 25, 23, 30, 0, 0, time.UTC)
	record := cloneGatewaySettlementRefundRequestRecord(&SettlementRefundRequestRecord{
		ID:                     9004,
		UserID:                 11,
		SubscriptionID:         22,
		Status:                 SettlementRefundStatusSubmitted,
		GatewayRefundableTotal: 0,
		ManualTransferAmount:   69.5,
		Allocations: []SettlementRefundAllocationRecord{
			{
				ID:                  9401,
				RefundRequestID:     9004,
				PaymentOrderID:      1001,
				GatewayRefundAmount: 0,
				Currency:            "CNY",
				Status:              SettlementRefundAllocationStatusSkipped,
			},
		},
	})
	store := &settlementRefundGatewayStoreStub{request: record}
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
	}

	result, err := service.ProcessSettlementRefundGateway(context.Background(), SettlementRefundGatewayInput{
		RefundRequestID: 9004,
		OperatorUserID:  77,
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrSettlementRefundGatewayNotRequired)
	require.Empty(t, store.requestStatusUpdates)
	require.Empty(t, store.allocationStatusUpdates)
}

func TestSettlementRefundServiceProcessGatewayRefundsDoesNotMoveToManualPendingForSmallRemainder(t *testing.T) {
	now := time.Date(2026, 6, 25, 23, 45, 0, 0, time.UTC)
	record := cloneGatewaySettlementRefundRequestRecord(&SettlementRefundRequestRecord{
		ID:                     9010,
		UserID:                 11,
		SubscriptionID:         22,
		Status:                 SettlementRefundStatusSubmitted,
		Currency:               "CNY",
		GatewayRefundableTotal: 19.7,
		ManualTransferAmount:   0.0046,
		Allocations: []SettlementRefundAllocationRecord{
			{
				ID:                    9410,
				RefundRequestID:       9010,
				PaymentOrderID:        1001,
				AlreadyRefundedAmount: 0,
				GatewayRefundAmount:   19.7,
				Currency:              "CNY",
				Status:                SettlementRefundAllocationStatusPending,
			},
		},
	})
	store := &settlementRefundGatewayStoreStub{request: record}
	service := &SettlementRefundService{
		requestStore: store,
		now:          func() time.Time { return now },
		loadRefundPaymentOrder: func(_ context.Context, id int64) (*dbent.PaymentOrder, error) {
			return gatewayRefundPaymentOrder(id), nil
		},
		resolveRefundProvider: func(context.Context, *dbent.PaymentOrder) (payment.Provider, error) {
			return &settlementRefundGatewayProviderStub{
				refundFn: func(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
					return &payment.RefundResponse{RefundID: "refund-1010", Status: payment.ProviderStatusSuccess}, nil
				},
			}, nil
		},
		syncGatewayPaymentOrderRefund: func(context.Context, *dbent.PaymentOrder, *SettlementRefundRequestRecord, SettlementRefundAllocationRecord, time.Time) error {
			return nil
		},
		markGatewayPaymentOrderRefundFailed: func(context.Context, *dbent.PaymentOrder, string, time.Time) error {
			return nil
		},
	}

	result, err := service.ProcessSettlementRefundGateway(context.Background(), SettlementRefundGatewayInput{
		RefundRequestID: 9010,
		OperatorUserID:  77,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, SettlementRefundStatusGatewayProcessing, result.Status)
	require.Equal(t, SettlementRefundStatusGatewayProcessing, store.request.Status)
}

func TestSettlementRefundManualTransferRequiredHonorsCurrencyTolerance(t *testing.T) {
	require.False(t, SettlementRefundManualTransferRequired(0.0062, "CNY"))
	require.True(t, SettlementRefundManualTransferRequired(0.01, "CNY"))
}

func TestSettlementRefundServiceDefaultSyncGatewayPaymentOrderRefundUsesAllocatedBusinessAmount(t *testing.T) {
	h := newSettlementRefundTestHarness(t)
	service := &SettlementRefundService{entClient: h.client}

	now := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	user, err := h.client.User.Create().
		SetEmail("refund-sync@test.com").
		SetPasswordHash("hash").
		SetStatus(StatusActive).
		SetRole(RoleUser).
		Save(h.ctx)
	require.NoError(t, err)

	order, err := h.client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Email).
		SetAmount(100).
		SetPayAmount(80).
		SetFeeRate(0).
		SetRechargeCode("refund-sync-1").
		SetOutTradeNo("refund-sync-1").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("trade-refund-sync-1").
		SetOrderType(payment.OrderTypeSubscription).
		SetStatus(OrderStatusCompleted).
		SetRefundAmount(20).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(h.ctx)
	require.NoError(t, err)

	err = service.defaultSyncGatewayPaymentOrderRefund(h.ctx, order, &SettlementRefundRequestRecord{ID: 9001}, SettlementRefundAllocationRecord{
		PaymentOrderID:        order.ID,
		AlreadyRefundedAmount: 20,
		AllocatedRefundValue:  30,
		GatewayRefundAmount:   24,
	}, now)
	require.NoError(t, err)

	reloaded, err := h.client.PaymentOrder.Get(h.ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.InDelta(t, 50, reloaded.RefundAmount, 1e-9)
}

type settlementRefundGatewayStoreStub struct {
	request                 *SettlementRefundRequestRecord
	requestStatusUpdates    []UpdateSettlementRefundRequestStatusInput
	allocationStatusUpdates []UpdateSettlementRefundAllocationStatusInput
}

func (s *settlementRefundGatewayStoreStub) GetSettlementRefundRequest(_ context.Context, id int64) (*SettlementRefundRequestRecord, error) {
	if s.request == nil || s.request.ID != id {
		return nil, ErrSettlementRefundRequestNotFound
	}
	return cloneGatewaySettlementRefundRequestRecord(s.request), nil
}

func (s *settlementRefundGatewayStoreStub) UpdateSettlementRefundRequestStatus(_ context.Context, input UpdateSettlementRefundRequestStatusInput) (*SettlementRefundRequestRecord, error) {
	s.requestStatusUpdates = append(s.requestStatusUpdates, input)
	if s.request == nil || s.request.ID != input.RequestID {
		return nil, ErrSettlementRefundRequestNotFound
	}
	if s.request.Status != input.ExpectedStatus {
		return nil, ErrSettlementRefundSubmitConflict
	}
	s.request.Status = input.Status
	return cloneGatewaySettlementRefundRequestRecord(s.request), nil
}

func (s *settlementRefundGatewayStoreStub) UpdateSettlementRefundAllocationStatus(_ context.Context, input UpdateSettlementRefundAllocationStatusInput) (*SettlementRefundAllocationRecord, error) {
	s.allocationStatusUpdates = append(s.allocationStatusUpdates, input)
	if s.request == nil {
		return nil, ErrSettlementRefundAllocationConflict
	}
	for idx := range s.request.Allocations {
		allocation := &s.request.Allocations[idx]
		if allocation.ID != input.AllocationID {
			continue
		}
		if allocation.Status != input.ExpectedStatus {
			return nil, ErrSettlementRefundAllocationConflict
		}
		allocation.Status = input.Status
		allocation.GatewayRefundTradeNo = input.GatewayRefundTradeNo
		allocation.FailedReason = input.FailedReason
		allocation.ProcessedAt = input.ProcessedAt
		return cloneGatewayRefundAllocationRecord(allocation), nil
	}
	return nil, ErrSettlementRefundAllocationConflict
}

type settlementRefundGatewayProviderStub struct {
	refundFn func(context.Context, payment.RefundRequest) (*payment.RefundResponse, error)
}

func (p *settlementRefundGatewayProviderStub) Name() string { return "gateway-stub" }

func (p *settlementRefundGatewayProviderStub) ProviderKey() string { return "gateway-stub" }

func (p *settlementRefundGatewayProviderStub) SupportedTypes() []payment.PaymentType { return nil }

func (p *settlementRefundGatewayProviderStub) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	return nil, nil
}

func (p *settlementRefundGatewayProviderStub) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	return nil, nil
}

func (p *settlementRefundGatewayProviderStub) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}

func (p *settlementRefundGatewayProviderStub) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	if p.refundFn != nil {
		return p.refundFn(ctx, req)
	}
	return &payment.RefundResponse{RefundID: "refund-trade", Status: payment.ProviderStatusSuccess}, nil
}

func gatewayRefundPaymentOrder(id int64) *dbent.PaymentOrder {
	return &dbent.PaymentOrder{
		ID:             id,
		Status:         OrderStatusCompleted,
		Amount:         99,
		PayAmount:      99,
		PaymentTradeNo: "trade-" + gatewayRefundInt64String(id),
		OutTradeNo:     "out-" + gatewayRefundInt64String(id),
		PaymentType:    "stripe",
		RefundAmount:   0,
	}
}

func cloneGatewaySettlementRefundRequestRecord(record *SettlementRefundRequestRecord) *SettlementRefundRequestRecord {
	if record == nil {
		return nil
	}
	cloned := *record
	if record.Reason != nil {
		value := *record.Reason
		cloned.Reason = &value
	}
	if len(record.Allocations) > 0 {
		cloned.Allocations = append([]SettlementRefundAllocationRecord(nil), record.Allocations...)
	}
	return &cloned
}

func cloneGatewayRefundAllocationRecord(record *SettlementRefundAllocationRecord) *SettlementRefundAllocationRecord {
	if record == nil {
		return nil
	}
	cloned := *record
	if record.GatewayRefundTradeNo != nil {
		value := *record.GatewayRefundTradeNo
		cloned.GatewayRefundTradeNo = &value
	}
	if record.FailedReason != nil {
		value := *record.FailedReason
		cloned.FailedReason = &value
	}
	if record.ProcessedAt != nil {
		value := *record.ProcessedAt
		cloned.ProcessedAt = &value
	}
	if record.PaymentProviderInstanceID != nil {
		value := *record.PaymentProviderInstanceID
		cloned.PaymentProviderInstanceID = &value
	}
	return &cloned
}

func gatewayRefundStringPtr(v string) *string {
	return &v
}

func gatewayRefundInt64String(v int64) string {
	return fmt.Sprintf("%d", v)
}
