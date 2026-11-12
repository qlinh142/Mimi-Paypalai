package demo

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Bộ dữ liệu Mỹ đi qua service thật: đủ 30-50 hóa đơn, mọi dòng vào tồn kho và dự đoán,
// món sắp hết đúng ngày như video demo.
func TestSeedUS(t *testing.T) {
	st := &memStore{}
	svc, err := receipt.New(nil, memDrafts{}, memQuota{}, memFX{}, st,
		slog.New(slog.NewTextHandler(io.Discard, nil)), receipt.WithTaxonomy(taxonomy.SpendingKeys()))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)
	n, err := SeedUS(context.Background(), svc, "uid-judge", now, "26300")
	if err != nil {
		t.Fatal(err)
	}
	if n < 30 || n > 50 {
		t.Fatalf("có %d hóa đơn, muốn 30-50", n)
	}
	lastOlive := ""
	ids := map[string]bool{}
	var total int64
	for _, r := range st.saved {
		if ids[r.ID] || !validLike(r.ID) {
			t.Fatalf("mã hóa đơn %q trùng hoặc sai dạng", r.ID)
		}
		ids[r.ID] = true
		for _, l := range r.Lines {
			if !l.ForInventory() {
				t.Errorf("%s: %q chưa vào tồn kho (cat=%q group=%q qty=%v review=%v)", r.ID, l.Name, l.CategoryKey, l.GroupKey, l.CanonicalQty, l.NeedsReview)
			}
			if l.GroupKey == "dau-o-liu" {
				lastOlive = r.PurchasedOn.String()
			}
		}
		total += int64(r.TotalVND)
	}
	if lastOlive != "2026-10-16" { // 28 ngày trước 13/11 (giờ Los Angeles)
		t.Fatalf("olive oil mua lần cuối %s", lastOlive)
	}
	usd := float64(total) / 26300
	t.Logf("%d hóa đơn, tổng %.0f USD (%.0f USD/tháng)", n, usd, usd/3)
	if usd < 1200 || usd > 4000 {
		t.Fatalf("tổng chi %.0f USD không hợp lý", usd)
	}
	if _, err := SeedUS(context.Background(), svc, "", now, "26300"); err == nil {
		t.Fatal("thiếu uid phải lỗi")
	}
}
