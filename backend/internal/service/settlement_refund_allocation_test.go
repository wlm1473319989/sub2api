package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocateSettlementRefundAcrossOrdersSingleOrder(t *testing.T) {
	result := allocateSettlementRefundAcrossOrders(60, "CNY", []SettlementRefundPaymentOrderCandidate{
		{
			PaymentOrderID:         1001,
			OrderAmount:            100,
			PayAmount:              80,
			Currency:               "CNY",
			RefundChannelAvailable: true,
		},
	})

	require.InDelta(t, 60, result.AllocatedRefundValue, 1e-9)
	require.InDelta(t, 48, result.GatewayRefundableTotal, 1e-9)
	require.InDelta(t, 0, result.ManualTransferAmount, 1e-9)
	require.Len(t, result.Allocations, 1)
	require.InDelta(t, 60, result.Allocations[0].AllocatedRefundValue, 1e-9)
	require.InDelta(t, 48, result.Allocations[0].GatewayRefundAmount, 1e-9)
	require.Empty(t, result.Allocations[0].SkippedReason)
}

func TestAllocateSettlementRefundAcrossOrdersLeavesManualTransferRemainder(t *testing.T) {
	result := allocateSettlementRefundAcrossOrders(120, "CNY", []SettlementRefundPaymentOrderCandidate{
		{
			PaymentOrderID:         1001,
			OrderAmount:            100,
			PayAmount:              80,
			Currency:               "CNY",
			RefundChannelAvailable: true,
		},
	})

	require.InDelta(t, 100, result.AllocatedRefundValue, 1e-9)
	require.InDelta(t, 80, result.GatewayRefundableTotal, 1e-9)
	require.InDelta(t, 20, result.ManualTransferAmount, 1e-9)
}

func TestAllocateSettlementRefundAcrossOrdersCapsByRemainingGatewayPayAmount(t *testing.T) {
	result := allocateSettlementRefundAcrossOrders(50, "CNY", []SettlementRefundPaymentOrderCandidate{
		{
			PaymentOrderID:         1001,
			OrderAmount:            100,
			PayAmount:              80,
			GatewayRefundedAmount:  60,
			Currency:               "CNY",
			RefundChannelAvailable: true,
		},
	})

	require.InDelta(t, 25, result.AllocatedRefundValue, 1e-9)
	require.InDelta(t, 20, result.GatewayRefundableTotal, 1e-9)
	require.InDelta(t, 25, result.ManualTransferAmount, 1e-9)
	require.InDelta(t, 25, result.Allocations[0].AllocatedRefundValue, 1e-9)
	require.InDelta(t, 20, result.Allocations[0].GatewayRefundAmount, 1e-9)
}

func TestAllocateSettlementRefundAcrossOrdersSkipsUnavailableAndMismatchedCurrency(t *testing.T) {
	result := allocateSettlementRefundAcrossOrders(70, "CNY", []SettlementRefundPaymentOrderCandidate{
		{
			PaymentOrderID:         1001,
			OrderAmount:            100,
			PayAmount:              100,
			Currency:               "CNY",
			RefundChannelAvailable: false,
			UnavailableReason:      "provider_refund_disabled",
		},
		{
			PaymentOrderID:         1002,
			OrderAmount:            100,
			PayAmount:              100,
			Currency:               "USD",
			RefundChannelAvailable: true,
		},
		{
			PaymentOrderID:         1003,
			OrderAmount:            100,
			PayAmount:              100,
			Currency:               "CNY",
			RefundChannelAvailable: true,
		},
	})

	require.Len(t, result.Allocations, 3)
	require.Equal(t, "provider_refund_disabled", result.Allocations[0].SkippedReason)
	require.Equal(t, "currency_mismatch", result.Allocations[1].SkippedReason)
	require.Empty(t, result.Allocations[2].SkippedReason)
	require.InDelta(t, 70, result.AllocatedRefundValue, 1e-9)
	require.InDelta(t, 70, result.GatewayRefundableTotal, 1e-9)
	require.InDelta(t, 0, result.ManualTransferAmount, 1e-9)
}
