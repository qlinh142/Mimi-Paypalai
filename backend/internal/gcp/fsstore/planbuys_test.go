package fsstore

import (
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// B3: mỗi dòng hóa đơn trừ vào đúng các Kế hoạch app đã chia.
func TestPlanBuys(t *testing.T) {
	lines := []receipt.Line{
		// 3 hộp sữa: 2 cho Kế hoạch "bé", 1 cho Kế hoạch chính.
		{GroupKey: "sua", Qty: 3, Product: receipt.Product{CanonicalQty: 2700, CanonicalUnit: "ml"},
			PlanSet: true, Plans: []receipt.PlanShare{{PlanID: "p-be", Qty: 2}, {PlanID: "default", Qty: 1}}},
		// Dòng cũ không có phần chia: theo source_plan_id.
		{GroupKey: "gao", Qty: 1, Product: receipt.Product{CanonicalQty: 5000, CanonicalUnit: "g"}},
		// Người dùng chọn không trừ vào kế hoạch nào.
		{GroupKey: "keo", Qty: 1, PlanSet: true},
		// Dòng mua hộ: Plan của vòng.
		{GroupKey: "mam", Qty: 1, InventoryOwner: "lan", Product: receipt.Product{CanonicalQty: 500, CanonicalUnit: "ml"}},
	}
	got := planBuys(lines, "minh", "p-tet", "p-lan")
	want := map[string]float64{
		"minh|p-be|sua": 1800, "minh|default|sua": 900,
		"minh|p-tet|gao": 5000,
		"lan|p-lan|mam":  500,
	}
	if len(got) != len(want) {
		t.Fatalf("keys: %v", got)
	}
	for k, q := range want {
		if got[k] == nil || got[k].qty != q {
			t.Errorf("%s: got %+v, want %v", k, got[k], q)
		}
	}
	if got["minh|p-be|sua"].unit != "ml" {
		t.Fatalf("unit: %+v", got["minh|p-be|sua"])
	}
}
