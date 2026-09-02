package service

import (
	"math"

	"github.com/shopspring/decimal"
)

const (
	SettlementRefundFeeRate = 0.05
	SettlementRefundFeeCap  = 10.0
)

// CalculateSettlementRefundFee applies the fixed subscription refund handling
// fee to the gross residual value. The result uses settlement precision.
func CalculateSettlementRefundFee(refundValue float64) float64 {
	if math.IsNaN(refundValue) || math.IsInf(refundValue, 0) || refundValue <= 0 {
		return 0
	}
	fee := decimal.NewFromFloat(refundValue).
		Mul(decimal.NewFromFloat(SettlementRefundFeeRate)).
		Round(settlementAmountPrecision).
		InexactFloat64()
	if fee > SettlementRefundFeeCap {
		fee = SettlementRefundFeeCap
	}
	return roundSettlementRefundValue(fee)
}

func settlementRefundNetAmount(refundValue float64) (float64, float64) {
	gross := roundSettlementRefundValue(refundValue)
	fee := CalculateSettlementRefundFee(gross)
	net := roundSettlementRefundValue(gross - fee)
	return net, fee
}
