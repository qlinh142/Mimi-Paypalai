package pricing

import (
	"context"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

func d(s string) domain.Date { x, _ := domain.ParseDate(s); return x }

func ob(store string, paid int64, on string, qty float64, unit domain.Unit) Observation {
	return Observation{Store: store, PaidVND: paid, On: d(on), CanonicalQty: qty, CanonicalUnit: unit}
}

func TestHistoryNormalizesPackageSize(t *testing.T) {
	h := History("sua-tuoi", []Observation{
		ob("WinMart", 35_000, "2026-10-18", 1000, domain.UnitML),
		ob("Co.opmart", 30_150, "2026-10-05", 900, domain.UnitML), // 33,5 đ/ml: rẻ hơn dù cùng tổng thấp hơn
		ob("WinMart", 34_000, "2026-09-21", 1000, domain.UnitML),
		ob("Bách Hóa Xanh", 20_000, "2026-09-01", 800, domain.UnitGram), // khác chiều đo
	})
	if h.PurchaseCount != 3 || h.OtherDimension != 1 || h.Unit != domain.UnitML {
		t.Fatalf("%+v", h)
	}
	if h.LastPurchase.Store != "WinMart" || h.LastPurchase.UnitPrice != 35 {
		t.Fatalf("last %+v", h.LastPurchase)
	}
	if h.LowestHistorical.Store != "Co.opmart" || h.LowestHistorical.UnitPrice != 33.5 {
		t.Fatalf("lowest %+v", h.LowestHistorical)
	}
	if h.AverageUnitPrice == nil || *h.AverageUnitPrice != 34.17 {
		t.Fatalf("avg %v", h.AverageUnitPrice)
	}
	one := History("x", []Observation{ob("A", 10_000, "2026-10-01", 1, domain.UnitItem)})
	if one.AverageUnitPrice != nil || one.LowestHistorical != nil || one.LastPurchase == nil {
		t.Fatalf("single purchase %+v", one)
	}
}

func TestEstimateBasketCoverage(t *testing.T) {
	today := d("2026-10-30")
	obs := map[string][]Observation{
		"sua":   {ob("WinMart", 35_000, "2026-10-18", 1000, domain.UnitML), ob("Co.opmart", 33_500, "2026-10-05", 1000, domain.UnitML)},
		"trung": {ob("WinMart", 38_000, "2026-10-18", 10, domain.UnitItem), ob("Co.opmart", 39_000, "2026-10-05", 10, domain.UnitItem)},
		"giat":  {ob("Co.opmart", 110_000, "2026-10-05", 3000, domain.UnitML)},
		"cu":    {ob("WinMart", 1_000, "2026-01-01", 1, domain.UnitItem)}, // ngoài cửa sổ 90 ngày
	}
	b := EstimateBasket([]BasketLine{{Key: "sua", Qty: 2, Unit: "l"}, {Key: "trung", Qty: 1}, {Key: "giat", Qty: 1}, {Key: "cu", Qty: 1}}, obs, today)
	if len(b.NoHistory) != 1 || b.NoHistory[0] != "cu" {
		t.Fatalf("no history %v", b.NoHistory)
	}
	// Không nơi nào đủ 4/4 món: không có tổng một nơi.
	for _, o := range b.StoreOptions {
		if o.EstimatedTotal != nil {
			t.Fatalf("partial store must not have total: %+v", o)
		}
	}
	if b.StoreOptions[0].StoreName != "Co.opmart" || b.StoreOptions[0].CoveredItems != 3 {
		t.Fatalf("ranking %+v", b.StoreOptions)
	}
	// 2 lít sữa ở Co.opmart = 2000 ml × 33,5 = 67.000; trứng WinMart 38.000; nước giặt 110.000.
	if b.Split == nil || b.Split.EstimatedTotal != 67_000+38_000+110_000 || b.Recommend != "split" {
		t.Fatalf("split %+v %s", b.Split, b.Recommend)
	}
	full := EstimateBasket([]BasketLine{{Key: "sua", Qty: 1, Unit: "l"}, {Key: "trung", Qty: 1}}, obs, today)
	if full.StoreOptions[0].EstimatedTotal == nil || full.Recommend != "single" {
		t.Fatalf("full coverage %+v %s", full.StoreOptions, full.Recommend)
	}
}

type limitStore struct{ got int }

func (s *limitStore) PriceObservations(_ context.Context, _ string, _ []string, limit int) (map[string][]Observation, error) {
	s.got = limit
	return map[string][]Observation{}, nil
}

// Lỗi V3: Chi tiết một món đọc đủ lịch sử mua, không chỉ 30 lần gần nhất.
func TestItemReadsFullHistory(t *testing.T) {
	st := &limitStore{}
	svc := New(st, nil, nil)
	p := authz.Principal{UID: "u1", Role: authz.RoleOwner}
	if _, err := svc.Item(context.Background(), p, "sua-tuoi"); err != nil {
		t.Fatal(err)
	}
	if st.got != ItemHistoryLimit {
		t.Fatalf("limit %d, want %d", st.got, ItemHistoryLimit)
	}
}
