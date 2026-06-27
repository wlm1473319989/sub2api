package service

import (
	"math"

	"github.com/shopspring/decimal"
)

const settlementAmountPrecision int32 = 4

func roundSettlementAmountValue(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	rounded := decimal.NewFromFloat(value).Round(settlementAmountPrecision).InexactFloat64()
	if rounded == 0 {
		return 0
	}
	return rounded
}

func roundSettlementAmountPointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	rounded := roundSettlementAmountValue(*value)
	return &rounded
}
