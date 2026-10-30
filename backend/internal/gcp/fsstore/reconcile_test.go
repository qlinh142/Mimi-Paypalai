package fsstore

import (
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

func TestReconcilePlanLine(t *testing.T) {
	now := time.Now()
	status := func(plan map[string]any, bought float64, unit domain.Unit) (bool, float64) {
		upd, done := reconcilePlanLine(plan, bought, unit, "r1", now)
		for _, u := range upd {
			if u.Path == "bought_qty" {
				return done, u.Value.(float64)
			}
		}
		return done, -1
	}
	// Kế hoạch 12 quả trứng, hóa đơn 10 quả: còn mở, đã mua 10.
	if done, got := status(map[string]any{"qty": 12.0, "unit": "qua"}, 10, domain.UnitItem); done || got != 10 {
		t.Fatalf("partial: done=%v got=%v", done, got)
	}
	// Mua tiếp 2 quả: xong.
	if done, got := status(map[string]any{"qty": 12.0, "unit": "qua", "bought_qty": 10.0}, 2, domain.UnitItem); !done || got != 12 {
		t.Fatalf("complete: done=%v got=%v", done, got)
	}
	// Kế hoạch 1,5 kg thịt, hóa đơn 1000 g: mua 1 kg, còn mở.
	if done, got := status(map[string]any{"qty": 1.5, "unit": "kg"}, 1000, domain.UnitGram); done || got != 1 {
		t.Fatalf("kg: done=%v got=%v", done, got)
	}
	// Khác đơn vị hoặc dòng chưa nhập số lượng: coi là đủ.
	if done, _ := status(map[string]any{"qty": 2.0, "unit": "kg"}, 3, domain.UnitItem); !done {
		t.Fatal("unit mismatch should complete")
	}
	if done, _ := status(map[string]any{}, 1, domain.UnitItem); !done {
		t.Fatal("no qty should complete")
	}
}
