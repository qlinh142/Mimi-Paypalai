package fsstore

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/demo"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// V5: dữ liệu demo đi qua đường lưu thật; Sữa tươi được dự đoán hết sau khoảng 2 ngày.
func TestDemoSeedEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	mustSet(t, c.Collection("users").Doc(uid), map[string]any{"display_name": "Lan", "role": "owner"})
	svc, err := receipt.New(nil, s, s, s, s, slog.New(slog.NewTextHandler(io.Discard, nil)), receipt.WithTaxonomy(demo.CategoryKeys()))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	n, err := demo.Seed(ctx, svc, uid, now)
	if err != nil || n != 7 {
		t.Fatalf("%d %v", n, err)
	}
	snap, err := c.Collection("users").Doc(uid).Collection("items").Doc("sua-tuoi").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := snap.Data()["prediction"].(map[string]any)
	out, _ := p["out_on"].(string)
	got, _ := time.Parse("2006-01-02", out)
	if days := got.Sub(now.Truncate(24*time.Hour)).Hours() / 24; days < 0 || days > 4 {
		t.Fatalf("sữa hết vào %s (%v ngày nữa)", out, days)
	}
	// Chạy lại: không ghi trùng.
	if _, err := demo.Seed(ctx, svc, uid, now); err != nil {
		t.Fatal(err)
	}
	docs, _ := c.Collection("users").Doc(uid).Collection("receipts").Documents(ctx).GetAll()
	if len(docs) != 7 {
		t.Fatalf("%d hóa đơn", len(docs))
	}
}
