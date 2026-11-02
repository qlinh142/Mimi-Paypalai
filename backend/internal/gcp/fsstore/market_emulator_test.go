package fsstore

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/market"
)

// Ghi rồi đọc lại phải giữ nguyên giá. Lỗi 02/11: price_vnd đọc ra 0 vì thiếu tag firestore.
func TestMarketSaveEntriesEmulator(t *testing.T) {
	s, _, _ := emulator(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	key := "sua-hat-test-" + now.Format("150405")
	in := market.Entry{Key: key, Name: "Sữa hạt", GroupKey: "sua-hat", UpdatedAt: now, Prices: []market.Price{
		{Store: "Co.op Online", PriceVND: 32500, Size: "4x180ml", Source: "gemini_search", URL: "https://example.com/a", On: "2031-01-10"},
	}}
	if err := s.SaveEntry(ctx, in); err != nil {
		t.Fatal(err)
	}
	got, err := s.Entries(ctx, []string{key})
	if err != nil {
		t.Fatal(err)
	}
	e, ok := got[key]
	if !ok || len(e.Prices) != 1 || e.Prices[0] != in.Prices[0] || e.Name != in.Name || !e.UpdatedAt.Equal(now) {
		t.Fatalf("%+v %v", e, ok)
	}
}

func TestMarketTouchedAndRefreshQuotaEmulator(t *testing.T) {
	s, _, uid := emulator(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	key := "rong-" + uid
	if err := s.SaveEntry(ctx, market.Entry{Key: key, Name: "Không ra giá", UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Touched(ctx, []string{key, "khong-co-" + uid})
	if err != nil || len(got) != 1 || !got[key].Equal(now) {
		t.Fatalf("%v %v", got, err)
	}
	for i := 0; i < 2; i++ {
		if err := s.ReservePriceRefresh(ctx, uid, "2031-01-10", 2); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReservePriceRefresh(ctx, uid, "2031-01-10", 2); err == nil {
		t.Fatal("want quota error")
	}
	// Lượt đọc hóa đơn tính riêng.
	if err := s.Reserve(ctx, uid, "2031-01-10", 1); err != nil {
		t.Fatal(err)
	}
}
