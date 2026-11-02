package order

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/paypal"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// ---- fakes ----

type memStore struct {
	mandate   Mandate
	orders    map[string]Order
	payments  int
	delivered []Order
	refunds   []Refund
}

func (m *memStore) Mandate(context.Context, string) (Mandate, error) { return m.mandate, nil }
func (m *memStore) SetMandate(_ context.Context, _ string, v Mandate) error {
	m.mandate = v
	return nil
}
func (m *memStore) CreateOrder(_ context.Context, _ string, o Order) error {
	m.orders[o.ID] = o
	return nil
}
func (m *memStore) GetOrder(_ context.Context, _, id string) (Order, error) {
	o, ok := m.orders[id]
	if !ok {
		return Order{}, apperr.ErrNotFound
	}
	return o, nil
}
func (m *memStore) ListOrders(context.Context, string, int) ([]Order, error) {
	var out []Order
	for _, o := range m.orders {
		out = append(out, o)
	}
	return out, nil
}
func (m *memStore) UpdateOrder(_ context.Context, _ string, o Order, from State) error {
	if m.orders[o.ID].State != from {
		return apperr.New(apperr.CodeInvalidState, "changed")
	}
	m.orders[o.ID] = o
	return nil
}
func (m *memStore) RecordPayment(_ context.Context, _ string, o Order, _ *time.Location) error {
	m.payments++
	m.orders[o.ID] = o
	return nil
}
func (m *memStore) ApplyDelivery(_ context.Context, _ string, o Order, _ *time.Location) error {
	m.delivered = append(m.delivered, o)
	m.orders[o.ID] = o
	return nil
}
func (m *memStore) RecordRefund(_ context.Context, _ string, o Order, r Refund, _ *time.Location) error {
	m.refunds = append(m.refunds, r)
	m.orders[o.ID] = o
	return nil
}

type planLines []plan.Line

func (p planLines) OpenLines(context.Context, string) ([]plan.Line, error) { return p, nil }

type fakeVerifier struct{ obs []Observed }

func (f fakeVerifier) VerifyDelivery(context.Context, []receipt.Image, []Expected) ([]Observed, error) {
	return f.obs, nil
}

var owner = authz.Principal{UID: "u1", Role: authz.RoleOwner, Timezone: "Asia/Ho_Chi_Minh"}

func setup(t *testing.T, m Mandate, lines planLines) (*Service, *memStore) {
	t.Helper()
	now := time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)
	st := &memStore{mandate: m, orders: map[string]Order{}}
	svc, err := New(st, lines, NewSandboxCatalog(now), paypal.NewFake(), fakeVerifier{}, 25_000,
		slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return svc, st
}

func code(err error) apperr.Code {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

var enabled = Mandate{Enabled: true, Categories: []string{CategoryGrocery, CategoryHousehold}, PriceTolerancePct: 10, MaxOrderVND: 1_000_000}

// ---- tests ----

func TestToUSD(t *testing.T) {
	for _, c := range []struct {
		vnd  int64
		want string
	}{{100_000, "4.00"}, {12_345, "0.49"}, {12_500, "0.50"}, {0, "0.00"}} {
		if got := ToUSD(c.vnd, 25_000); got != c.want {
			t.Errorf("ToUSD(%d) = %s want %s", c.vnd, got, c.want)
		}
	}
}

func TestBasketAndMandate(t *testing.T) {
	ctx := context.Background()
	lines := planLines{
		{Key: "trung", Name: "Trứng", Qty: 12, Unit: "qua", NeedBy: "2026-10-18"},
		{Key: "sua-tuoi", Name: "Sữa tươi", Qty: 2, Unit: "hop"},
		{Key: "khong-ban", Name: "Món lạ", Qty: 1},
	}

	t.Run("resolves packs, prices, ETA, missing products", func(t *testing.T) {
		svc, _ := setup(t, enabled, lines)
		b, err := svc.Basket(ctx, owner, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(b.Lines) != 2 || len(b.Missing) != 1 || b.Missing[0] != "khong-ban" {
			t.Fatalf("basket %+v", b)
		}
		var trung Line
		for _, l := range b.Lines {
			if l.Key == "trung" {
				trung = l
			}
		}
		// 12 quả, vỉ 10 quả: 2 vỉ.
		if trung.Qty != 2 || trung.TotalVND != 68_000 || trung.ETA != "2026-10-17" {
			t.Fatalf("trung %+v", trung)
		}
		if b.TotalVND != 68_000+64_000 || b.AmountUSD != "5.28" || b.Decision.Result != Allow {
			t.Fatalf("total %d usd %s decision %+v", b.TotalVND, b.AmountUSD, b.Decision)
		}
	})

	t.Run("price above tolerance asks", func(t *testing.T) {
		svc, _ := setup(t, enabled, planLines{{Key: "dau-an", Name: "Dầu ăn", Qty: 1, Unit: "l"}})
		b, _ := svc.Basket(ctx, owner, nil)
		if b.Decision.Result != Ask || b.Decision.Reasons[0].Code != "price_guard" {
			t.Fatalf("decision %+v", b.Decision)
		}
	})

	t.Run("item outside plan is blocked", func(t *testing.T) {
		svc, _ := setup(t, enabled, lines)
		b, _ := svc.Basket(ctx, owner, []string{"trung", "gao"})
		if b.Decision.Result != Block {
			t.Fatalf("decision %+v", b.Decision)
		}
	})

	t.Run("category not authorized is blocked", func(t *testing.T) {
		m := enabled
		m.Categories = []string{CategoryGrocery}
		svc, _ := setup(t, m, planLines{{Key: "nuoc-giat", Name: "Nước giặt", Qty: 1}})
		b, _ := svc.Basket(ctx, owner, nil)
		if b.Decision.Result != Block || b.Decision.Reasons[0].Code != "category" {
			t.Fatalf("decision %+v", b.Decision)
		}
	})

	t.Run("budget and delivery guards ask, mandate off asks", func(t *testing.T) {
		m := enabled
		m.MaxOrderVND = 50_000
		svc, _ := setup(t, m, planLines{{Key: "gao", Name: "Gạo", Qty: 5, Unit: "kg", NeedBy: "2026-10-17"}})
		b, _ := svc.Basket(ctx, owner, nil)
		codes := map[string]bool{}
		for _, r := range b.Decision.Reasons {
			codes[r.Code] = true
		}
		if b.Decision.Result != Ask || !codes["budget_order"] || !codes["delivery_guard"] {
			t.Fatalf("decision %+v", b.Decision)
		}
		svc2, _ := setup(t, DefaultMandate(), planLines{{Key: "trung", Name: "Trứng", Qty: 10, Unit: "qua"}})
		b2, _ := svc2.Basket(ctx, owner, nil)
		if b2.Decision.Result != Ask || b2.Decision.Reasons[0].Code != "mandate_off" {
			t.Fatalf("mandate off %+v", b2.Decision)
		}
	})

	t.Run("max authorized price", func(t *testing.T) {
		if MaxAuthorizedPrice(100_000, 15) != 115_000 {
			t.Fatal("formula")
		}
	})
}

func TestOnlineFlow(t *testing.T) {
	ctx := context.Background()
	lines := planLines{
		{Key: "trung", Name: "Trứng", Qty: 20, Unit: "qua", CategoryKey: "tuoi-song"},
		{Key: "sua-tuoi", Name: "Sữa tươi", Qty: 2, Unit: "hop", CategoryKey: "sua"},
	}
	svc, st := setup(t, enabled, lines)

	o, err := svc.Checkout(ctx, owner, CheckoutRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if o.State != StateAwaitingPayment || o.PayPalOrderID == "" || o.TotalVND != 132_000 {
		t.Fatalf("checkout %+v", o)
	}
	if _, err := svc.Advance(ctx, owner, o.ID); code(err) != apperr.CodeInvalidState {
		t.Fatalf("advance before payment: %v", err)
	}

	o, err = svc.Capture(ctx, owner, o.ID)
	if err != nil || o.State != StatePaid || o.CaptureID == "" || st.payments != 1 {
		t.Fatalf("capture %v %+v", err, o)
	}
	again, err := svc.Capture(ctx, owner, o.ID)
	if err != nil || again.CaptureID != o.CaptureID || st.payments != 1 {
		t.Fatalf("capture must be idempotent: %v payments=%d", err, st.payments)
	}

	// Payment không phải hàng đã nhận: Verify bị từ chối trước DELIVERED.
	if _, err := svc.Verify(ctx, owner, o.ID, VerifyRequest{}); code(err) != apperr.CodeInvalidState {
		t.Fatalf("verify before delivered: %v", err)
	}
	for _, want := range []State{StateOrdered, StateShipped, StateDelivered} {
		if o, err = svc.Advance(ctx, owner, o.ID); err != nil || o.State != want {
			t.Fatalf("advance to %s: %v %s", want, err, o.State)
		}
	}

	// Trứng: đặt 2 vỉ, nhận 1 vỉ. Sữa đủ.
	o, err = svc.Verify(ctx, owner, o.ID, VerifyRequest{Actual: map[string]float64{"trung": 1, "sua-tuoi": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if o.State != StateVerified || o.SuggestedRefundVND != 34_000 || len(st.delivered) != 1 {
		t.Fatalf("verify %+v", o)
	}
	for _, l := range o.Lines {
		if (l.Key == "trung" && l.Match != "partial") || (l.Key == "sua-tuoi" && l.Match != "match") {
			t.Fatalf("match %+v", l)
		}
	}

	if _, err := svc.Refund(ctx, owner, o.ID, RefundRequest{Reason: "bogus"}); code(err) != apperr.CodeValidation {
		t.Fatalf("bad reason: %v", err)
	}
	o, err = svc.Refund(ctx, owner, o.ID, RefundRequest{Reason: "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if o.RefundedVND != 34_000 || o.NetVND != 98_000 || len(o.Refunds) != 1 || o.Refunds[0].AmountUSD != "1.36" || o.SuggestedRefundVND != 0 {
		t.Fatalf("refund %+v", o)
	}
	if _, err := svc.Refund(ctx, owner, o.ID, RefundRequest{Reason: "other", AmountVND: 200_000}); code(err) != apperr.CodeValidation {
		t.Fatalf("over-refund: %v", err)
	}
}

func TestCheckoutGuards(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup(t, enabled, planLines{{Key: "dau-an", Name: "Dầu ăn", Qty: 1, Unit: "l"}})
	if _, err := svc.Checkout(ctx, owner, CheckoutRequest{}); code(err) != apperr.CodeApprovalRequired {
		t.Fatalf("ask without approve: %v", err)
	}
	o, err := svc.Checkout(ctx, owner, CheckoutRequest{Approve: true})
	if err != nil || !o.ApprovedByUser {
		t.Fatalf("approved ask: %v %+v", err, o)
	}
	svc2, _ := setup(t, enabled, planLines{{Key: "trung", Name: "Trứng", Qty: 10, Unit: "qua"}})
	if _, err := svc2.Checkout(ctx, owner, CheckoutRequest{Keys: []string{"gao"}}); code(err) != apperr.CodeMandateBlocked {
		t.Fatalf("blocked: %v", err)
	}
	member := authz.Principal{UID: "m", Role: authz.RoleMember}
	if _, err := svc2.Checkout(ctx, member, CheckoutRequest{}); code(err) != apperr.CodeForbidden {
		t.Fatalf("member: %v", err)
	}
	if _, err := svc2.Get(ctx, owner, "../x"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}

// Vấn đề 44: đơn phải ghi lại tỷ giá đã dùng, lấy từ đâu và của ngày nào.
func TestCheckoutRecordsRateInfo(t *testing.T) {
	lines := planLines{{Key: "trung", Name: "Trứng", Qty: 20, Unit: "qua", CategoryKey: "tuoi-song"}}
	svc, _ := setup(t, enabled, lines)
	svc.WithRateInfo("vietcombank-ban-ra", "2026-11-01")
	o, err := svc.Checkout(context.Background(), owner, CheckoutRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if o.VNDPerUSD != 25_000 || o.RateSource != "vietcombank-ban-ra" || o.RateAsOf != "2026-11-01" {
		t.Fatalf("rate info %d %q %q", o.VNDPerUSD, o.RateSource, o.RateAsOf)
	}
}
