package history

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
)

type mem struct {
	receipts  []Receipt
	delegated []Line
	paid      []Paid
}

func (m *mem) Receipts(context.Context, string, int) ([]Receipt, error) { return m.receipts, nil }
func (m *mem) Receipt(_ context.Context, _, id string) (Receipt, error) {
	for _, r := range m.receipts {
		if r.ID == id {
			return r, nil
		}
	}
	return Receipt{}, apperr.ErrNotFound
}
func (m *mem) DelegatedLines(context.Context, string, int) ([]Line, error) { return m.delegated, nil }
func (m *mem) PaidManually(context.Context, string, int) ([]Paid, error)   { return m.paid, nil }
func (m *mem) DisplayNames(_ context.Context, uids []string) (map[string]string, error) {
	n := map[string]string{"minh": "Minh", "lan": "Lan"}
	out := map[string]string{}
	for _, u := range uids {
		out[u] = n[u]
	}
	return out, nil
}
func (m *mem) PlanNames(context.Context, string) (map[string]string, error) {
	return map[string]string{"p-be": "Mua đồ cho bé"}, nil
}

func vnd(v int64) *int64 { return &v }

var lan = authz.Principal{UID: "lan", Role: authz.RoleOwner}

func fixture() *mem {
	saved := time.Date(2026, 10, 14, 11, 15, 0, 0, time.UTC) // 18:15 giờ Việt Nam
	return &mem{
		receipts: []Receipt{
			{ID: "r1", Store: "Bách Hóa Xanh", PurchasedOn: "2026-10-14", SavedAt: saved, TotalVND: 150000, DiscountVND: 10000, SourcePlanID: "p-be",
				Lines: []Line{
					{Name: "Sữa tươi", Qty: 2, VND: vnd(60000)},
					{Name: "Gạo", Qty: 1, VND: vnd(100000), Owner: "minh"},
					{Name: "Món mờ", Qty: 1},
				}},
			{ID: "r0", Store: "Coopmart", PurchasedOn: "2026-10-01", TotalVND: 50000, Lines: []Line{{Name: "Trứng", Qty: 10, VND: vnd(50000)}}},
		},
		delegated: []Line{
			{ReceiptID: "m9", Name: "Nước mắm", Qty: 1, VND: vnd(40000), PayerUID: "minh", Store: "Winmart", PurchasedOn: "2026-10-10"},
			{ReceiptID: "m9", Name: "Giấy", Qty: 2, PayerUID: "minh", Store: "Winmart", PurchasedOn: "2026-10-10"},
		},
		paid: []Paid{{ID: "o1", RecurringID: "rec-net", Name: "Internet", PaidOn: "2026-10-05", AmountVND: 220000}},
	}
}

func TestHistory(t *testing.T) {
	got, err := New(fixture()).History(context.Background(), lan)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, e := range got {
		ids = append(ids, e.ReceiptID)
	}
	want := []string{"r1", "d-m9", "rec-o1", "r0"} // mới nhất trước
	if len(ids) != len(want) {
		t.Fatalf("ids %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids %v, want %v", ids, want)
		}
	}
	r1, d := got[0], got[1]
	if r1.PurchasedAt != "2026-10-14T11:15:00Z" || r1.PlanName != "Mua đồ cho bé" || r1.MissingPrice != 1 || len(r1.ItemNames) != 3 {
		t.Fatalf("own: %+v", r1)
	}
	if d.Kind != "delegated" || d.BuyerUID != "minh" || d.BuyerName != "Minh" || d.TotalVND != 40000 || d.MissingPrice != 1 {
		t.Fatalf("delegated: %+v", d)
	}
	if got[2].Kind != "recurring" || got[2].RecurringID != "rec-net" {
		t.Fatalf("recurring: %+v", got[2])
	}
}

func TestDetail(t *testing.T) {
	svc := New(fixture())
	d, err := svc.Detail(context.Background(), lan, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Subtotal != 160000 || d.Discount != 10000 || len(d.Plans) != 1 || d.Split == nil {
		t.Fatalf("detail: %+v", d)
	}
	if d.Split.ForOwner != 100000 || d.Split.Mine != 60000 || d.Split.OwnerName != "Minh" {
		t.Fatalf("split: %+v", d.Split)
	}
	if d.Lines[0].Tag != "mine" || *d.Lines[0].UnitPrice != 30000 || d.Lines[1].Tag != "for_owner" {
		t.Fatalf("lines: %+v", d.Lines)
	}
	if d.Lines[2].Amount != nil || d.Lines[2].UnitPrice != nil {
		t.Fatalf("món chưa rõ giá phải là null, got %+v", d.Lines[2])
	}

	plain, _ := svc.Detail(context.Background(), lan, "r0")
	if plain.Split != nil || plain.Lines[0].Tag != "" {
		t.Fatalf("hóa đơn không mua hộ không có split: %+v", plain)
	}

	dd, err := svc.Detail(context.Background(), lan, "d-m9")
	if err != nil || len(dd.Lines) != 2 || dd.Subtotal != 40000 || dd.BuyerName != "Minh" {
		t.Fatalf("delegated detail: %+v %v", dd, err)
	}
	for _, id := range []string{"nope", "d-nope", "../x", ""} {
		if _, err := svc.Detail(context.Background(), lan, id); apperr.From(err).Code != apperr.CodeNotFound {
			t.Errorf("%q: got %v", id, err)
		}
	}
	if _, err := svc.History(context.Background(), authz.Principal{}); err == nil {
		t.Fatal("chưa đăng nhập phải bị chặn")
	}
}
