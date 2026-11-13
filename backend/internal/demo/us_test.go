package demo

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// 50 hóa đơn anh Linh cung cấp đi qua service thật: đủ 50 hóa đơn, 280 dòng, tổng đúng
// như file Excel ($3,763.44), mọi dòng vào tồn kho và dự đoán.
func TestSeedUS(t *testing.T) {
	st := &memStore{}
	svc, err := receipt.New(nil, memDrafts{}, memQuota{}, memFX{}, st,
		slog.New(slog.NewTextHandler(io.Discard, nil)), receipt.WithTaxonomy(taxonomy.SpendingKeys()))
	if err != nil {
		t.Fatal(err)
	}
	const rate = "26300"
	n, err := SeedUS(context.Background(), svc, "uid-judge", rate)
	if err != nil {
		t.Fatal(err)
	}
	if n != 50 {
		t.Fatalf("có %d hóa đơn, muốn 50", n)
	}
	ids := map[string]bool{}
	lines := 0
	var total int64
	for _, r := range st.saved {
		if ids[r.ID] || !validLike(r.ID) {
			t.Fatalf("mã hóa đơn %q trùng hoặc sai dạng", r.ID)
		}
		ids[r.ID] = true
		for _, l := range r.Lines {
			lines++
			if !l.ForInventory() {
				t.Errorf("%s: %q chưa vào tồn kho (cat=%q group=%q qty=%v review=%v)", r.ID, l.Name, l.CategoryKey, l.GroupKey, l.CanonicalQty, l.NeedsReview)
			}
		}
		total += int64(r.TotalVND)
	}
	if lines != 280 {
		t.Fatalf("có %d dòng, muốn 280", lines)
	}
	// Mỗi dòng làm tròn tới đồng, nên tổng lệch không quá 1 đồng mỗi dòng.
	want := int64(376344) * 26300 / 100
	if d := total - want; d > 280 || d < -280 {
		t.Fatalf("tổng %d đồng, muốn khoảng %d", total, want)
	}
	if _, err := SeedUS(context.Background(), svc, "", rate); err == nil {
		t.Fatal("thiếu uid phải lỗi")
	}
}
