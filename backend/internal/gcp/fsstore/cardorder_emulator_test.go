package fsstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/order"
)

// Webhook Braintree không mang uid: CardOrder tìm chủ đơn qua sổ payments theo mã giao dịch.
func TestCardOrderEmulator(t *testing.T) {
	s, _, uid := emulator(t)
	ctx := context.Background()
	now := time.Now()
	txn := "BT-" + uid
	o := order.Order{ID: "ord-" + uid, State: order.StateAwaitingPayment, Merchant: "Sandbox", TotalVND: 100_000, AmountUSD: "4.00",
		Provider: order.ProviderBraintree, Refunds: []order.Refund{}, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateOrder(ctx, uid, o); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CardOrder(ctx, txn); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("before payment: %v", err)
	}
	o.State, o.ProviderOrderID, o.ProviderTxnID, o.PaidAt = order.StatePaid, txn, txn, &now
	if err := s.RecordPayment(ctx, uid, o, time.UTC); err != nil {
		t.Fatal(err)
	}
	gotUID, gotID, err := s.CardOrder(ctx, txn)
	if err != nil || gotUID != uid || gotID != o.ID {
		t.Fatalf("%q %q %v", gotUID, gotID, err)
	}
}
