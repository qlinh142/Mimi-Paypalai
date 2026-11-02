package plan

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/item"
)

type memStore struct {
	lines  map[string]Line
	state  State
	prices []domain.LastPrice
	asked  []string
}

func (m *memStore) OpenLines(context.Context, string) ([]Line, error) {
	var out []Line
	for _, l := range m.lines {
		out = append(out, l)
	}
	return out, nil
}
func (m *memStore) AddLine(_ context.Context, _ string, l Line) error { m.lines[l.Key] = l; return nil }
func (m *memStore) UpdateLine(_ context.Context, _ string, l Line) error {
	m.lines[l.Key] = l
	return nil
}
func (m *memStore) RemoveLine(_ context.Context, _ string, k string) error {
	delete(m.lines, k)
	return nil
}
func (m *memStore) State(context.Context, string) (State, error) { return m.state, nil }
func (m *memStore) SetState(_ context.Context, _ string, s State) (int64, error) {
	if s.BaseRev != nil && *s.BaseRev != m.state.Rev {
		return 0, apperr.New(apperr.CodeConflict, "conflict")
	}
	s.Rev, s.BaseRev = m.state.Rev+1, nil
	m.state = s
	return s.Rev, nil
}
func (m *memStore) Prices(_ context.Context, _ string, groups []string) ([]domain.LastPrice, error) {
	m.asked = append(m.asked, groups...)
	want := map[string]bool{}
	for _, g := range groups {
		want[g] = true
	}
	var out []domain.LastPrice
	for _, p := range m.prices {
		if want[p.GroupKey] {
			out = append(out, p)
		}
	}
	return out, nil
}

type memItems map[string]item.Record

func (m memItems) GetItem(_ context.Context, _ string, k string) (item.Record, error) {
	r, ok := m[k]
	if !ok {
		return item.Record{}, apperr.ErrNotFound
	}
	return r, nil
}
func (m memItems) ListItems(context.Context, string) ([]item.Record, error) {
	var out []item.Record
	for _, r := range m {
		out = append(out, r)
	}
	return out, nil
}

func date(s string) domain.Date { d, _ := domain.ParseDate(s); return d }

func setup() (*Service, *memStore) {
	now := time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)
	items := memItems{
		"sua":   {GroupKey: "sua", Name: "Sữa tươi", CategoryKey: "sua", Prediction: domain.Prediction{OutOn: date("2026-10-16"), PurchaseCount: 5, Source: "predicted"}},
		"gao":   {GroupKey: "gao", Name: "Gạo", CategoryKey: "do-kho", Prediction: domain.Prediction{OutOn: date("2026-11-20"), PurchaseCount: 4, Source: "predicted"}},
		"keo":   {GroupKey: "keo", Name: "Kẹo", CategoryKey: "banh-keo"},
		"trung": {GroupKey: "trung", Name: "Trứng", CategoryKey: "tuoi-song"},
	}
	st := &memStore{lines: map[string]Line{}, prices: []domain.LastPrice{
		{GroupKey: "sua", Store: "Co.op", Unit: 32_000}, {GroupKey: "sua", Store: "Bách Hóa Xanh", Unit: 30_000},
		{GroupKey: "gao", Store: "Co.op", Unit: 120_000}, {GroupKey: "gao", Store: "Bách Hóa Xanh", Unit: 140_000},
		{GroupKey: "trung", Store: "Bách Hóa Xanh", Unit: 36_000},
	}}
	return New(st, items, func() time.Time { return now }), st
}

var owner = authz.Principal{UID: "u1", Role: authz.RoleOwner, Timezone: "Asia/Ho_Chi_Minh", LeadDays: 7}

func TestCart(t *testing.T) {
	ctx := context.Background()
	svc, st := setup()

	t.Run("empty cart encodes empty list and null estimate", func(t *testing.T) {
		c, err := svc.Get(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(c)
		if !strings.Contains(string(b), `"items":[]`) || !strings.Contains(string(b), `"estimate":null`) {
			t.Fatalf("got %s", b)
		}
		if len(st.asked) != 0 {
			t.Fatal("must not query prices for an empty cart")
		}
	})
	t.Run("add is idempotent and returns the cart", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			c, err := svc.Add(ctx, owner, "sua")
			if err != nil || c.ItemCount != 1 {
				t.Fatalf("err=%v count=%d", err, c.ItemCount)
			}
		}
		c, _ := svc.Add(ctx, owner, "gao")
		if c.ItemCount != 2 {
			t.Fatalf("count %d", c.ItemCount)
		}
	})
	t.Run("cart shows stock, cheapest price and estimate", func(t *testing.T) {
		c, err := svc.Get(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		var sua CartItem
		for _, it := range c.Items {
			if it.Key == "sua" {
				sua = it
			}
		}
		if sua.Status != domain.StatusBuyNow || sua.UnitPrice == nil || *sua.UnitPrice != 30_000 || sua.BestStore != "Bách Hóa Xanh" {
			t.Fatalf("sua %+v", sua)
		}
		// Một nơi (Co.op): 32k + 120k = 152k. Tách: 30k + 120k = 150k, tiết kiệm 2k < 10k nên khuyên một nơi.
		if c.Estimate == nil || *c.Estimate != 152_000 || c.MissingPrice != 0 {
			t.Fatalf("estimate %v missing %d", c.Estimate, c.MissingPrice)
		}
	})
	t.Run("item without price is counted as missing", func(t *testing.T) {
		if _, err := svc.Add(ctx, owner, "keo"); err != nil {
			t.Fatal(err)
		}
		c, _ := svc.Get(ctx, owner)
		if c.MissingPrice != 1 || *c.Estimate != 152_000 {
			t.Fatalf("missing %d estimate %v", c.MissingPrice, c.Estimate)
		}
		n, est, miss, err := svc.Summary(ctx, owner)
		if err != nil || n != 3 || *est != 152_000 || miss != 1 {
			t.Fatalf("summary %d %v %d %v", n, est, miss, err)
		}
	})
	t.Run("prices recommend split when saving reaches the threshold", func(t *testing.T) {
		if _, err := svc.Add(ctx, owner, "trung"); err != nil {
			t.Fatal(err)
		}
		pr, err := svc.Prices(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		// Một nơi: Bách Hóa Xanh có đủ 3 món, 30k + 140k + 36k = 206k.
		// Tách: sữa, trứng ở Bách Hóa Xanh (66k) + gạo ở Co.op (120k) = 186k, tiết kiệm 20k ≥ 10k nên khuyên tách.
		if pr.Recommend != "split" || pr.SingleStore == nil || pr.SingleStore.Store != "Bách Hóa Xanh" ||
			pr.SingleStore.Total != 206_000 || len(pr.Split) != 2 || pr.SplitTotal != 186_000 || pr.Saving != 20_000 {
			t.Fatalf("prices %+v", pr)
		}
		if strings.Join(pr.MissingPrice, ",") != "keo" || pr.SaleOffers == nil {
			t.Fatalf("missing %v offers %v", pr.MissingPrice, pr.SaleOffers)
		}
	})
	t.Run("remove", func(t *testing.T) {
		if err := svc.Remove(ctx, owner, "keo"); err != nil {
			t.Fatal(err)
		}
		if err := svc.Remove(ctx, owner, "keo"); err != nil { // lần hai vẫn thành công
			t.Fatal(err)
		}
		if _, ok := st.lines["keo"]; ok {
			t.Fatal("not removed")
		}
	})
}

func TestRules(t *testing.T) {
	ctx := context.Background()
	svc, st := setup()
	member := authz.Principal{UID: "u2", Role: authz.RoleMember, HouseholdID: "h1"}

	if _, err := svc.Add(ctx, owner, "khong-co"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("unknown item err=%v", err)
	}
	if _, err := svc.Add(ctx, owner, "../x"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("invalid key err=%v", err)
	}
	if _, err := svc.Get(ctx, member); err != nil {
		t.Fatalf("member reads own plan err=%v", err)
	}
	if _, err := svc.Add(ctx, authz.Principal{UID: "x"}, "sua"); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("unknown role write err=%v", err)
	}
	for i := 0; i < MaxCartLines; i++ {
		st.lines[string(rune('a'+i%26))+strings.Repeat("x", i/26+1)] = Line{}
	}
	var ae *apperr.Error
	if _, err := svc.Add(ctx, owner, "sua"); !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
		t.Fatalf("full cart err=%v", err)
	}
}

func TestState(t *testing.T) {
	ctx := context.Background()
	svc, st := setup()
	h := 9
	got, err := svc.SetState(ctx, owner, State{BuyDate: "2026-10-18", BuyHour: &h, TripNote: "  Mua ở chợ  "})
	if err != nil || got.TripNote != "Mua ở chợ" || st.state.BuyDate != "2026-10-18" {
		t.Fatalf("err=%v got=%+v", err, got)
	}
	if _, err := svc.SetState(ctx, owner, State{}); err != nil { // xóa hết là hợp lệ
		t.Fatal(err)
	}
	h24 := 24
	bad := []State{
		{BuyDate: "18/10/2026"},
		{BuyDate: "2020-01-01"},
		{BuyHour: &h24},
		{TripNote: strings.Repeat("a", 121)},
		{TripNote: "a\x00b"},
	}
	for i, s := range bad {
		var ae *apperr.Error
		if _, err := svc.SetState(ctx, owner, s); !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
			t.Errorf("case %d err=%v", i, err)
		}
	}
}

func TestPlanLineDetails(t *testing.T) {
	ctx := context.Background()
	svc, st := setup()
	if _, err := svc.Add(ctx, owner, "sua"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(ctx, owner, "gao"); err != nil {
		t.Fatal(err)
	}
	if l := st.lines["sua"]; l.Source != SourceForecast || l.NeedBy != "2026-10-16" || l.Qty != 1 {
		t.Fatalf("forecast line %+v", l)
	}
	if l := st.lines["gao"]; l.Source != SourceCheck || l.NeedBy != "" {
		t.Fatalf("check line %+v", l)
	}

	qty, unit, need, prio := 2.0, "Hộp", "2026-10-18", "high"
	c, err := svc.UpdateLine(ctx, owner, "sua", LinePatch{Qty: &qty, Unit: &unit, NeedBy: &need, Priority: &prio})
	if err != nil {
		t.Fatal(err)
	}
	var sua CartItem
	for _, it := range c.Items {
		if it.Key == "sua" {
			sua = it
		}
	}
	if sua.Qty != 2 || sua.Unit != "hop" || sua.NeedBy != "2026-10-18" || sua.Priority != "high" {
		t.Fatalf("updated %+v", sua)
	}

	zero, badPrio, past := 0.0, "urgent", "2020-01-01"
	for i, p := range []LinePatch{{Qty: &zero}, {Priority: &badPrio}, {NeedBy: &past}} {
		if _, err := svc.UpdateLine(ctx, owner, "sua", p); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if _, err := svc.UpdateLine(ctx, owner, "keo", LinePatch{Qty: &qty}); err == nil {
		t.Fatal("line not in cart accepted")
	}
}

func TestPhase(t *testing.T) {
	today, _ := domain.ParseDate("2026-10-20")
	cases := []struct {
		st   State
		want string
	}{
		{State{}, PhasePlanning},
		{State{BuyDate: "2026-10-25"}, PhasePlanning},
		{State{BuyDate: "2026-10-20"}, PhaseShopping},
		{State{BuyDate: "2026-10-18", ReconciledOn: "2026-10-18"}, PhaseReconciled},
		{State{ReconciledOn: "2026-10-10"}, PhaseReconciled},
		// Đặt ngày mua mới sau lần đối chiếu: quay lại lập kế hoạch.
		{State{BuyDate: "2026-10-25", ReconciledOn: "2026-10-18"}, PhasePlanning},
	}
	for _, c := range cases {
		if got := c.st.phase(today); got != c.want {
			t.Errorf("%+v: %s, want %s", c.st, got, c.want)
		}
	}
}

func TestSetStateMinuteAndIgnoresReconciled(t *testing.T) {
	svc, _ := setup()
	h, m := 9, 30
	got, err := svc.SetState(context.Background(), owner, State{BuyDate: "2026-10-18", BuyHour: &h, BuyMinute: &m, ReconciledOn: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BuyMinute == nil || *got.BuyMinute != 30 || got.ReconciledOn != "" {
		t.Fatalf("%+v", got)
	}
	bad := 60
	if _, err := svc.SetState(context.Background(), owner, State{BuyHour: &h, BuyMinute: &bad}); err == nil {
		t.Fatal("minute 60 accepted")
	}
}

func TestSetStateRevConflict(t *testing.T) {
	svc, _ := setup()
	ctx := context.Background()
	first, err := svc.SetState(ctx, owner, State{TripNote: "máy A"})
	if err != nil || first.Rev != 1 {
		t.Fatalf("%+v %v", first, err)
	}
	base := first.Rev
	second, err := svc.SetState(ctx, owner, State{TripNote: "máy A lần 2", BaseRev: &base})
	if err != nil || second.Rev != 2 {
		t.Fatalf("%+v %v", second, err)
	}
	// Máy B còn giữ rev 1: không được ghi đè.
	_, err = svc.SetState(ctx, owner, State{TripNote: "máy B", BaseRev: &base})
	if err == nil || apperr.From(err).Code != apperr.CodeConflict {
		t.Fatalf("want conflict, got %v", err)
	}
}
