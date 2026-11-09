package spending

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
)

type fakeStore struct {
	months map[string]Month
	rows   []ReceiptRow
	asked  []string
}

func (f *fakeStore) Month(_ context.Context, _ string, m string) (Month, error) {
	f.asked = append(f.asked, m)
	return f.months[m], nil
}
func (f *fakeStore) MonthReceipts(_ context.Context, _ string, _ string, limit int) ([]ReceiptRow, error) {
	if len(f.rows) > limit {
		return f.rows[:limit], nil
	}
	return f.rows, nil
}

func TestBuild(t *testing.T) {
	cur := Month{SpentVND: 1_200_000, DiscountVND: 300_000, Receipts: 5, ByCategory: map[string]int64{"sua": 300_000, "hoa-pham": 900_000, "khac": 0}}
	prev := Month{SpentVND: 1_000_000}
	rows := []ReceiptRow{
		{ID: "a", PurchasedOn: "2026-10-02", TotalVND: 100, DiscountVND: 50},
		{ID: "b", PurchasedOn: "2026-10-20", TotalVND: 100, DiscountVND: 0},
		{ID: "c", PurchasedOn: "2026-10-15", TotalVND: 100, DiscountVND: 200},
		{ID: "d", PurchasedOn: "2026-10-09", TotalVND: 100, DiscountVND: 10},
		{ID: "e", PurchasedOn: "2026-10-20", TotalVND: 100, DiscountVND: 40},
	}
	r := Build("2026-10", cur, prev, rows)

	if r.SpentPrevMonthPct == nil || *r.SpentPrevMonthPct != 20 {
		t.Fatalf("prev pct %v", r.SpentPrevMonthPct)
	}
	if r.SavedRatio == nil || *r.SavedRatio != 0.2 { // 300k / 1.5tr
		t.Fatalf("saved ratio %v", r.SavedRatio)
	}
	if len(r.ByCategory) != 2 || r.ByCategory[0].Key != "hoa-pham" || r.ByCategory[0].Pct != 75 || r.ByCategory[1].Pct != 25 {
		t.Fatalf("by category %+v", r.ByCategory)
	}
	ids := func(xs []ReceiptRow) string {
		var s []string
		for _, x := range xs {
			s = append(s, x.ID)
		}
		return strings.Join(s, ",")
	}
	if got := ids(r.RecentReceipts); got != "e,b,c,d,a" { // mọi hóa đơn của tháng, mới nhất trước
		t.Fatalf("recent %s", got)
	}
	if got := ids(r.TopSavingReceipts); got != "c,a,e" {
		t.Fatalf("top saving %s", got)
	}
}

func TestBuildEmptyMonthEncodesEmptyLists(t *testing.T) {
	r := Build("2026-10", Month{}, Month{}, nil)
	b, _ := json.Marshal(r)
	s := string(b)
	for _, want := range []string{`"spent":0`, `"spent_prev_month_pct":null`, `"saved_ratio":null`, `"by_category":[]`, `"top_saving_receipts":[]`, `"recent_receipts":[]`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}

func TestService(t *testing.T) {
	ctx := context.Background()
	owner := authz.Principal{UID: "u1", Role: authz.RoleOwner}
	st := &fakeStore{months: map[string]Month{"2026-01": {SpentVND: 10}}}
	svc := New(st)

	if _, err := svc.Get(ctx, owner, "2026-01"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(st.asked, ",") != "2026-01,2025-12" {
		t.Fatalf("asked %v", st.asked)
	}
	var ae *apperr.Error
	if _, err := svc.Get(ctx, owner, "2026-1"); !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
		t.Fatalf("bad month err=%v", err)
	}
	member := authz.Principal{UID: "u2", Role: authz.RoleMember}
	if _, err := svc.Get(ctx, member, "2026-01"); err != nil {
		t.Fatalf("member err=%v", err)
	}
}

func TestSameRange(t *testing.T) {
	cur := []ReceiptRow{{PurchasedOn: "2026-11-02", TotalVND: 300}, {PurchasedOn: "2026-11-08", TotalVND: 100}}
	prev := []ReceiptRow{
		{PurchasedOn: "2026-10-01", TotalVND: 100}, {PurchasedOn: "2026-10-08", TotalVND: 100},
		{PurchasedOn: "2026-10-09", TotalVND: 5000}, // sau ngày 8: không tính
		{PurchasedOn: "2026-10-30", TotalVND: 9000},
	}
	got := SameRange(cur, prev, 8)
	if got == nil || *got != 100 {
		t.Fatalf("same range = %v, want 100", got)
	}
	if SameRange(cur, prev[2:], 8) != nil {
		t.Fatal("prev empty in range: want nil")
	}
}

func TestServiceSameRangeOnlyCurrentMonth(t *testing.T) {
	ctx := context.Background()
	owner := authz.Principal{UID: "u1", Role: authz.RoleOwner}
	st := &fakeStore{months: map[string]Month{}}
	svc := New(st)
	svc.now = func() time.Time { return time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC) } // 9/11 giờ VN
	r, err := svc.Get(ctx, owner, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if r.SameRangeDay != 9 {
		t.Fatalf("day = %d, want 9 (giờ Việt Nam)", r.SameRangeDay)
	}
	r, _ = svc.Get(ctx, owner, "2026-10")
	if r.SameRangeDay != 0 || r.SpentPrevSameRangePct != nil {
		t.Fatalf("tháng cũ không so cùng kỳ: %+v", r)
	}
}
