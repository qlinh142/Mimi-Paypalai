package fsstore

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
)

func TestRegroupItemEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	u := c.Collection("users").Doc(uid)
	mustSet(t, u, map[string]any{"name": "A"})
	today := time.Now().UTC().Format("2006-01-02")
	// AI xếp nhầm "Sữa tắm Dove" vào dầu gội; nhóm đúng (sữa tắm) đã có tồn 900 ml.
	mustSet(t, u.Collection("items").Doc("dau-goi"), map[string]any{"name": "Dove", "category_key": "ve-sinh",
		"stock": map[string]any{"qty": 500.0, "unit": "ml", "on": today, "source": "receipt"}, "rate_per_day": 0.0})
	mustSet(t, u.Collection("items").Doc("sua-tam"), map[string]any{"name": "Sữa tắm", "category_key": "ve-sinh",
		"stock": map[string]any{"qty": 900.0, "unit": "ml", "on": today, "source": "receipt"}, "rate_per_day": 0.0})
	mustSet(t, u.Collection("purchases").Doc("p1"), map[string]any{"group_key": "dau-goi", "habit": true, "purchased_on": "2026-10-01", "canonical_quantity": 500.0, "canonical_unit": "ml"})
	mustSet(t, u.Collection("purchases").Doc("p2"), map[string]any{"group_key": "sua-tam", "habit": true, "purchased_on": "2026-09-01", "canonical_quantity": 900.0, "canonical_unit": "ml"})

	rec, err := s.RegroupItem(ctx, uid, "dau-goi", "sua-tam", "ve-sinh")
	if err != nil {
		t.Fatal(err)
	}
	if rec.GroupKey != "sua-tam" || rec.Baseline.Qty != 1400 || rec.Prediction.PurchaseCount != 2 {
		t.Fatalf("record %+v", rec)
	}
	if g := get(t, u.Collection("purchases").Doc("p1"))["group_key"]; g != "sua-tam" {
		t.Fatalf("purchase group %v", g)
	}
	if _, err := u.Collection("items").Doc("dau-goi").Get(ctx); err == nil {
		t.Fatal("old item still there")
	}
	if c := get(t, u.Collection("classification_corrections").Doc("dau-goi")); c["to_group_key"] != "sua-tam" {
		t.Fatalf("correction %v", c)
	}
	if _, err := s.RegroupItem(ctx, uid, "dau-goi", "sua-tam", ""); err != apperr.ErrNotFound {
		t.Fatalf("regroup missing item: %v", err)
	}

	r2, err := s.RenameItem(ctx, uid, "sua-tam", "Dove Dưỡng ẩm")
	if err != nil || r2.Name != "Dove Dưỡng ẩm" || get(t, u.Collection("items").Doc("sua-tam"))["name"] != "Dove Dưỡng ẩm" {
		t.Fatalf("rename %+v %v", r2, err)
	}
}
