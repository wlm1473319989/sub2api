package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateSettlementRefundFee(t *testing.T) {
	tests := []struct {
		name  string
		gross float64
		fee   float64
		net   float64
	}{
		{name: "five percent below cap", gross: 100, fee: 5, net: 95},
		{name: "cap applies", gross: 500, fee: 10, net: 490},
		{name: "zero and negative values", gross: 0, fee: 0, net: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.fee, CalculateSettlementRefundFee(tt.gross))
			net, fee := settlementRefundNetAmount(tt.gross)
			require.Equal(t, tt.fee, fee)
			require.Equal(t, tt.net, net)
		})
	}
}
