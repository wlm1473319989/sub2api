package service

import (
	"math"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const settlementRefundValuePrecision int32 = 8

type SettlementRefundPaymentOrderCandidate struct {
	PaymentOrderID         int64
	OrderAmount            float64
	PayAmount              float64
	AlreadyRefundedAmount  float64
	GatewayRefundedAmount  float64
	Currency               string
	RefundChannelAvailable bool
	UnavailableReason      string
}

type SettlementRefundOrderAllocation struct {
	PaymentOrderID         int64
	OrderAmount            float64
	PayAmount              float64
	AlreadyRefundedAmount  float64
	RefundableOrderAmount  float64
	AllocatedRefundValue   float64
	GatewayRefundAmount    float64
	Currency               string
	RefundChannelAvailable bool
	SkippedReason          string
}

type SettlementRefundAllocationResult struct {
	RefundResidualValue    float64
	AllocatedRefundValue   float64
	GatewayRefundableTotal float64
	ManualTransferAmount   float64
	Currency               string
	Allocations            []SettlementRefundOrderAllocation
}

func allocateSettlementRefundAcrossOrders(refundResidualValue float64, currency string, candidates []SettlementRefundPaymentOrderCandidate) SettlementRefundAllocationResult {
	result := SettlementRefundAllocationResult{
		RefundResidualValue: roundSettlementRefundValue(refundResidualValue),
		Currency:            strings.TrimSpace(currency),
		Allocations:         make([]SettlementRefundOrderAllocation, 0, len(candidates)),
	}
	remainingResidual := result.RefundResidualValue
	if remainingResidual <= 0 {
		result.ManualTransferAmount = 0
		return result
	}

	for _, candidate := range candidates {
		allocation := settlementRefundAllocationFromCandidate(candidate)
		if remainingResidual <= 0 {
			allocation.SkippedReason = "residual_already_allocated"
			result.Allocations = append(result.Allocations, allocation)
			continue
		}
		if !candidate.RefundChannelAvailable {
			allocation.SkippedReason = settlementRefundSkippedReason(candidate.UnavailableReason, "refund_channel_unavailable")
			result.Allocations = append(result.Allocations, allocation)
			continue
		}
		if !settlementRefundCurrencyMatches(currency, candidate.Currency) {
			allocation.SkippedReason = "currency_mismatch"
			result.Allocations = append(result.Allocations, allocation)
			continue
		}
		if candidate.OrderAmount <= 0 || candidate.PayAmount <= 0 {
			allocation.SkippedReason = "invalid_order_amount"
			result.Allocations = append(result.Allocations, allocation)
			continue
		}

		remainingPayAmount := remainingRefundableAmount(candidate.PayAmount, candidate.GatewayRefundedAmount)
		if allocation.RefundableOrderAmount <= 0 || remainingPayAmount <= 0 {
			allocation.SkippedReason = "no_refundable_amount"
			result.Allocations = append(result.Allocations, allocation)
			continue
		}

		businessAmountByGatewayCap := reverseGatewayRefundAmount(candidate.OrderAmount, candidate.PayAmount, remainingPayAmount)
		allocated := minPositiveSettlementRefundValue(remainingResidual, allocation.RefundableOrderAmount, businessAmountByGatewayCap)
		if allocated <= 0 {
			allocation.SkippedReason = "no_refundable_amount"
			result.Allocations = append(result.Allocations, allocation)
			continue
		}

		gatewayAmount := calculateGatewayRefundAmount(candidate.OrderAmount, candidate.PayAmount, allocated, candidate.Currency)
		if gatewayAmount-remainingPayAmount > paymentAmountToleranceForCurrency(candidate.Currency) {
			gatewayAmount = remainingPayAmount
		}

		allocation.AllocatedRefundValue = roundSettlementRefundValue(allocated)
		allocation.GatewayRefundAmount = roundGatewayRefundValue(gatewayAmount, candidate.Currency)
		result.AllocatedRefundValue = roundSettlementRefundValue(result.AllocatedRefundValue + allocation.AllocatedRefundValue)
		result.GatewayRefundableTotal = roundGatewayRefundValue(result.GatewayRefundableTotal+allocation.GatewayRefundAmount, candidate.Currency)
		remainingResidual = roundSettlementRefundValue(remainingResidual - allocation.AllocatedRefundValue)
		result.Allocations = append(result.Allocations, allocation)
	}

	result.ManualTransferAmount = roundSettlementRefundValue(remainingResidual)
	if result.ManualTransferAmount < 0 {
		result.ManualTransferAmount = 0
	}
	return result
}

func settlementRefundAllocationFromCandidate(candidate SettlementRefundPaymentOrderCandidate) SettlementRefundOrderAllocation {
	return SettlementRefundOrderAllocation{
		PaymentOrderID:         candidate.PaymentOrderID,
		OrderAmount:            candidate.OrderAmount,
		PayAmount:              candidate.PayAmount,
		AlreadyRefundedAmount:  candidate.AlreadyRefundedAmount,
		RefundableOrderAmount:  remainingRefundableAmount(candidate.OrderAmount, candidate.AlreadyRefundedAmount),
		Currency:               strings.TrimSpace(candidate.Currency),
		RefundChannelAvailable: candidate.RefundChannelAvailable,
	}
}

func settlementRefundSkippedReason(reason, fallback string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	return fallback
}

func settlementRefundCurrencyMatches(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	return expected == "" || actual == "" || strings.EqualFold(expected, actual)
}

func remainingRefundableAmount(total, used float64) float64 {
	if total <= 0 {
		return 0
	}
	remaining := total - math.Max(used, 0)
	if remaining <= 0 {
		return 0
	}
	return roundSettlementRefundValue(remaining)
}

func reverseGatewayRefundAmount(orderAmount, payAmount, gatewayAmount float64) float64 {
	if orderAmount <= 0 || payAmount <= 0 || gatewayAmount <= 0 {
		return 0
	}
	return decimal.NewFromFloat(gatewayAmount).
		Mul(decimal.NewFromFloat(orderAmount)).
		Div(decimal.NewFromFloat(payAmount)).
		Round(settlementRefundValuePrecision).
		InexactFloat64()
}

func minPositiveSettlementRefundValue(values ...float64) float64 {
	min := 0.0
	for _, value := range values {
		if value <= 0 {
			return 0
		}
		if min == 0 || value < min {
			min = value
		}
	}
	return roundSettlementRefundValue(min)
}

func roundSettlementRefundValue(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0
	}
	return decimal.NewFromFloat(value).Round(settlementRefundValuePrecision).InexactFloat64()
}

func roundGatewayRefundValue(value float64, currency string) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0
	}
	return decimal.NewFromFloat(value).Round(int32(payment.CurrencyMaxFractionDigits(currency))).InexactFloat64()
}
