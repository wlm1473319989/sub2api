package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestValidateOrderInputRejectsCustomBalanceAmountWhenDisabled(t *testing.T) {
	svc := &PaymentService{}
	cfg := &PaymentConfig{
		AllowCustomRechargeAmount: false,
		MinAmount:                 1,
		MaxAmount:                 5000,
	}

	_, _, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{
		OrderType: payment.OrderTypeBalance,
		Amount:    88,
	}, cfg)
	if err == nil {
		t.Fatal("expected custom recharge amount to be rejected")
	}
	if appErr := infraerrors.FromError(err); appErr.Reason != "CUSTOM_RECHARGE_AMOUNT_DISABLED" {
		t.Fatalf("reason = %q, want CUSTOM_RECHARGE_AMOUNT_DISABLED", appErr.Reason)
	}
}

func TestValidateOrderInputAcceptsPresetBalanceAmountWhenCustomDisabled(t *testing.T) {
	svc := &PaymentService{}
	cfg := &PaymentConfig{
		AllowCustomRechargeAmount: false,
		MinAmount:                 1,
		MaxAmount:                 5000,
	}

	_, _, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{
		OrderType: payment.OrderTypeBalance,
		Amount:    200,
	}, cfg)
	if err != nil {
		t.Fatalf("expected preset recharge amount to be accepted, got %v", err)
	}
}

func TestValidateOrderInputAcceptsCustomBalanceAmountWhenEnabled(t *testing.T) {
	svc := &PaymentService{}
	cfg := &PaymentConfig{
		AllowCustomRechargeAmount: true,
		MinAmount:                 1,
		MaxAmount:                 5000,
	}

	_, _, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{
		OrderType: payment.OrderTypeBalance,
		Amount:    88,
	}, cfg)
	if err != nil {
		t.Fatalf("expected custom recharge amount to be accepted, got %v", err)
	}
}
