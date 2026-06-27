package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/stretchr/testify/require"
)

type settlementRefundAuditSinkStub struct {
	events []*logger.LogEvent
}

func (s *settlementRefundAuditSinkStub) WriteLogEvent(event *logger.LogEvent) {
	if event == nil {
		return
	}
	cloned := *event
	if event.Fields != nil {
		cloned.Fields = make(map[string]any, len(event.Fields))
		for key, value := range event.Fields {
			cloned.Fields[key] = value
		}
	}
	s.events = append(s.events, &cloned)
}

func TestSettlementRefundServiceAuditSettlementRefundEventWritesSinkEvent(t *testing.T) {
	sink := &settlementRefundAuditSinkStub{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	service := &SettlementRefundService{}
	record := &SettlementRefundRequestRecord{
		ID:                    9001,
		UserID:                11,
		SubscriptionID:        22,
		SettlementID:          33,
		ExpectedSettlementID:  33,
		Status:                SettlementRefundStatusSubmitted,
		RefundMode:            SettlementRefundModeHybrid,
		Currency:              "CNY",
		RefundResidualValue:   168.5,
		GatewayRefundableTotal: 99,
		ManualTransferAmount:  69.5,
	}

	service.auditSettlementRefundEvent(context.Background(), "submitted", record, map[string]any{
		"subscription_status": SubscriptionStatusSuspended,
	})

	require.Len(t, sink.events, 1)
	event := sink.events[0]
	require.Equal(t, "info", event.Level)
	require.Equal(t, "audit.subscription_refund", event.Component)
	require.Equal(t, "subscription refund submitted", event.Message)
	require.Equal(t, "submitted", event.Fields["action"])
	require.Equal(t, int64(9001), event.Fields["refund_request_id"])
	require.Equal(t, SubscriptionStatusSuspended, event.Fields["subscription_status"])
}

func TestSettlementRefundAuditFieldsFromGatewayResultIncludesSummary(t *testing.T) {
	fields := settlementRefundAuditFieldsFromGatewayResult(&SettlementRefundGatewayResult{
		Status:               SettlementRefundStatusManualPending,
		SucceededAllocations: 1,
		FailedAllocations:    1,
		SkippedAllocations:   0,
		GatewayRefundedTotal: 99,
		ManualTransferAmount: 69.5,
		Allocations: []SettlementRefundGatewayAllocationResult{
			{
				AllocationID:        9101,
				PaymentOrderID:      1001,
				Status:              SettlementRefundAllocationStatusSucceeded,
				GatewayRefundAmount: 99,
			},
			{
				AllocationID:        9102,
				PaymentOrderID:      1002,
				Status:              SettlementRefundAllocationStatusFailed,
				FailedReason:        "gateway unavailable",
			},
		},
	})

	require.Equal(t, SettlementRefundStatusManualPending, fields["result_status"])
	require.Equal(t, 1, fields["succeeded_allocations"])
	require.Equal(t, 1, fields["failed_allocations"])
	require.Equal(t, 99.0, fields["gateway_refunded_total"])
	require.Equal(t, []string{
		"9101:1001:succeeded:",
		"9102:1002:failed:gateway unavailable",
	}, fields["allocation_summaries"])
}
