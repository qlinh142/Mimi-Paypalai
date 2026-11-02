package fsstore

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

func TestRemainName(t *testing.T) {
	if got := remainName("Mua đồ cuối tuần"); got != "Mua đồ cuối tuần · còn lại" {
		t.Fatal(got)
	}
	// Tách lần nữa không lặp hậu tố.
	if got := remainName("Mua đồ cuối tuần · còn lại"); got != "Mua đồ cuối tuần · còn lại" {
		t.Fatal(got)
	}
	long := strings.Repeat("a", 40)
	if got := remainName(long); utf8.RuneCountInString(got) > 40 || !strings.HasSuffix(got, " · còn lại") {
		t.Fatalf("%q", got)
	}
}

func TestRemainLine(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	got := remainLine(map[string]any{
		"name": "Dầu ăn", "qty": 3.0, "unit": "chai", "status": "open",
		"bought_qty": 1.0, "partial_by": "r1", "matched_receipt_id": "r1",
	}, now)
	if got["qty"] != 2.0 || got["status"] != "open" || got["name"] != "Dầu ăn" || got["updated_at"] != now {
		t.Fatalf("%v", got)
	}
	for _, k := range []string{"bought_qty", "partial_by", "matched_receipt_id"} {
		if _, ok := got[k]; ok {
			t.Fatalf("%s must be dropped", k)
		}
	}
	// Chưa mua gì: giữ nguyên số lượng.
	if got := remainLine(map[string]any{"qty": 2.0}, now); got["qty"] != 2.0 {
		t.Fatalf("%v", got)
	}
}

// Ví dụ đề xuất bước 6: "Mua đồ cuối tuần" mua được sữa, còn Dầu ăn → kế hoạch "· còn lại".
func TestSplitRemainingEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	u := c.Collection("users").Doc(uid)
	p := u.Collection("plans").Doc("p-cuoi-tuan")
	mustSet(t, p, map[string]any{"name": "Mua đồ cuối tuần", "status": "active", "icon": "family",
		"plan_state": map[string]any{"trip_note": "Bách Hóa Xanh", "buy_date": "2026-11-01"}})
	mustSet(t, p.Collection("items").Doc("sua-tuoi"), map[string]any{"name": "Sữa", "status": "done"})
	mustSet(t, p.Collection("items").Doc("dau-an"), map[string]any{"name": "Dầu ăn", "status": "open", "qty": 1.0})

	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	sp, err := s.SplitRemaining(ctx, uid, "p-cuoi-tuan", "", now)
	if err != nil || sp == nil || sp.Moved != 1 || sp.Name != "Mua đồ cuối tuần · còn lại" {
		t.Fatalf("%+v %v", sp, err)
	}
	np := get(t, u.Collection("plans").Doc(sp.PlanID))
	if np["icon"] != "family" || np["plan_state"].(map[string]any)["trip_note"] != "Bách Hóa Xanh" || np["plan_state"].(map[string]any)["buy_date"] != nil {
		t.Fatalf("new plan %v", np)
	}
	if get(t, u.Collection("plans").Doc(sp.PlanID).Collection("items").Doc("dau-an"))["status"] != "open" {
		t.Fatal("moved line")
	}
	if _, err := p.Collection("items").Doc("dau-an").Get(ctx); !isNotFound(err) {
		t.Fatal("old line must be gone")
	}
	// Gọi lại không tạo thêm.
	if again, err := s.SplitRemaining(ctx, uid, "p-cuoi-tuan", "", now); err != nil || again != nil {
		t.Fatalf("again %+v %v", again, err)
	}
	// Kế hoạch chính không tách.
	if sp, _ := s.SplitRemaining(ctx, uid, "default", "", now); sp != nil {
		t.Fatal("default plan must not split")
	}
}

func TestLatestUnitPrices(t *testing.T) {
	d1, _ := domain.ParseDate("2026-10-01")
	d2, _ := domain.ParseDate("2026-10-20")
	got := latestUnitPrices([]domain.LastPrice{
		{GroupKey: "sua", Store: "A", Unit: 30000, On: d1},
		{GroupKey: "sua", Store: "B", Unit: 32000, On: d2},
		{GroupKey: "gao", Store: "A", Unit: 0, On: d2},
	})
	if got["sua"] != 32000 || len(got) != 1 {
		t.Fatalf("%v", got)
	}
}
