package fsstore

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"

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
		if err := s.ReservePriceRefresh(ctx, uid, "2031-01-10", 2, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReservePriceRefresh(ctx, uid, "2031-01-10", 2, now); err == nil {
		t.Fatal("want quota error")
	}
	if used, last, err := s.PriceRefreshStatus(ctx, uid, "2031-01-10"); err != nil || used != 2 || !last.Equal(now) {
		t.Fatalf("%d %v %v", used, last, err)
	}
	// Lượt đọc hóa đơn tính riêng.
	if err := s.Reserve(ctx, uid, "2031-01-10", 1); err != nil {
		t.Fatal(err)
	}
}

func TestPriceSourcesEmulator(t *testing.T) {
	s, _, uid := emulator(t)
	ctx := context.Background()
	country := "T" + uid[len(uid)-6:] // nước giả để không lẫn với test khác
	set := func(r market.SourceRecord) market.SourceRecord {
		r.Hits++
		r.Store = "FairPrice"
		r.Categories = map[string]string{"dairy": "https://www.fairprice.com.sg/category/milk"}
		return r
	}
	for i := 0; i < 2; i++ {
		if err := s.Update(ctx, country, "fairprice.com.sg", set); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := s.Sources(ctx, country)
	if err != nil || len(recs) != 1 || recs[0].Hits != 2 || !recs[0].Active() || recs[0].Categories["dairy"] == "" {
		t.Fatalf("%+v %v", recs, err)
	}
}

// Món trong kế hoạch mang nước của chủ kế hoạch (Cài đặt), để tra giá đúng nước.
func TestWantedItemsCountryEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	if _, err := c.Collection("users").Doc(uid).Set(ctx, map[string]any{"settings": map[string]any{"country": "SG"}}); err != nil {
		t.Fatal(err)
	}
	name := "Milk " + uid
	if _, err := c.Collection("users").Doc(uid).Collection("plan_items").Doc("milk").Set(ctx,
		map[string]any{"status": "open", "name": name, "category_key": "dairy"}); err != nil {
		t.Fatal(err)
	}
	ws, err := s.WantedItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range ws {
		if w.Name == name {
			if w.Country != "SG" || w.Key != market.ItemID("SG", name) || w.Category != "dairy" || w.GroupKey != "milk" {
				t.Fatalf("%+v", w)
			}
			return
		}
	}
	t.Fatal("not found")
}

// Anh chốt 09/11: bảng giá lấy mọi món từng có trong tồn kho, hóa đơn, kế hoạch (cả kế hoạch
// đã xong). Món đã bỏ và khoản chi định kỳ không tra; ưu tiên: kế hoạch đang mở > sắp hết > còn lại.
func TestWantedItemsAllSourcesEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	u := c.Collection("users").Doc(uid)
	soon := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	later := time.Now().AddDate(0, 0, 40).Format("2006-01-02")
	set := func(ref interface {
		Set(context.Context, any, ...firestore.SetOption) (*firestore.WriteResult, error)
	}, v map[string]any) {
		if _, err := ref.Set(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	set(u.Collection("items").Doc("trung-"+uid), map[string]any{"name": "Trứng " + uid, "prediction": map[string]any{"out_on": soon}})
	set(u.Collection("items").Doc("gao-"+uid), map[string]any{"name": "Gạo " + uid, "prediction": map[string]any{"out_on": later}})
	set(u.Collection("items").Doc("bo-"+uid), map[string]any{"name": "Đã bỏ " + uid, "dropped": true})
	set(u.Collection("plans").Doc("p1").Collection("items").Doc("sua-"+uid), map[string]any{"name": "Sữa " + uid, "status": "open"})
	set(u.Collection("plans").Doc("p0").Collection("items").Doc("muoi-"+uid), map[string]any{"name": "Muối " + uid, "status": "bought"})
	set(u.Collection("plans").Doc("p1").Collection("items").Doc("custom-rec-"+uid), map[string]any{"name": "Internet " + uid, "status": "open"})

	ws, err := s.WantedItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, w := range ws {
		got[w.Name] = w.Priority + 1 // 0 = không có
	}
	want := map[string]int{"Trứng " + uid: 2, "Gạo " + uid: 1, "Sữa " + uid: 3, "Muối " + uid: 1, "Đã bỏ " + uid: 0, "Internet " + uid: 0}
	for n, p := range want {
		if got[n] != p {
			t.Errorf("%s: got %d want %d", n, got[n], p)
		}
	}
}
