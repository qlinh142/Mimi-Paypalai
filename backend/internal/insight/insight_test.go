package insight

import (
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/market"
	"github.com/smartlife-lab/mimi/backend/internal/pricing"
)

const rate = 25_000 // VND / USD trong test

func day(s string) domain.Date {
	d, err := domain.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Lần mua gần nhất: sữa 1L giá $3.75.
func milkHistory() pricing.ItemHistory {
	return pricing.History("sua-tuoi", []pricing.Observation{
		{Store: "Walmart", PaidVND: 3.75 * rate, On: day("2026-11-01"), CanonicalQty: 1000, CanonicalUnit: domain.UnitML},
		{Store: "Target", PaidVND: 3.60 * rate, On: day("2026-10-15"), CanonicalQty: 1000, CanonicalUnit: domain.UnitML},
	})
}

func offer(store string, usd float64, size, on string) market.Price {
	return market.Price{Store: store, PriceVND: int64(usd * rate), Local: usd, Currency: "USD", Size: size, Source: "partner_channel3", On: on}
}

func TestPriceIncreaseMilkTwelvePercent(t *testing.T) {
	in := PriceInput{PlanID: "cuoi-tuan", Key: "sua-tuoi", Name: "Sữa tươi 1L", Qty: 2, History: milkHistory(),
		Offers: []market.Price{offer("Walmart", 4.20, "1L", "2026-11-11"), offer("Kroger", 4.49, "1 L", "2026-11-11")}, Today: day("2026-11-12")}
	got, ok := PriceIncrease(in)
	if !ok {
		t.Fatal("want insight")
	}
	p := got.Params
	if got.Type != TypePriceIncrease || got.Placement != PlacementPlanItem || got.EntityID != "sua-tuoi" || p.Pct != 12 {
		t.Fatalf("%+v", got)
	}
	if p.DisplayUnit != "L" || p.CurrentUnit != 4.20*rate || p.LastPaidUnit != 3.75*rate || p.OfferStore != "Walmart" || p.LastPaidOn != "2026-11-01" {
		t.Fatalf("%+v", p)
	}
	if p.ImpactVND != int64(0.45*2*rate) || got.Severity != "info" || got.ReportTemplate != ReportPriceHistory {
		t.Fatalf("%+v", got)
	}
	// Cùng dữ liệu thì cùng id; giá mới thì id mới (card hiện lại sau khi đã ẩn).
	again, _ := PriceIncrease(in)
	in.Offers[0] = offer("Walmart", 4.30, "1L", "2026-11-11")
	changed, _ := PriceIncrease(in)
	if again.ID != got.ID || changed.ID == got.ID {
		t.Fatalf("ids %s %s %s", got.ID, again.ID, changed.ID)
	}
}

func TestPriceIncreaseComparesPerUnitAcrossSizes(t *testing.T) {
	// Hộp 2L $7.00 là $3.50/L: rẻ hơn lần trước theo đơn vị, dù tổng tiền cao hơn. Không có card.
	in := PriceInput{PlanID: "p", Key: "sua-tuoi", Name: "Sữa", History: milkHistory(),
		Offers: []market.Price{offer("Walmart", 7.00, "2L", "2026-11-11")}, Today: day("2026-11-12")}
	if got, ok := PriceIncrease(in); ok {
		t.Fatalf("unexpected %+v", got)
	}
	// Quy cách Mỹ: 64 fl oz $8.40 ≈ 1.89 L → $4.44/L, tăng khoảng 18%.
	in.Offers = []market.Price{offer("Target", 8.40, "64 fl oz", "2026-11-11")}
	got, ok := PriceIncrease(in)
	if !ok || got.Params.Pct != 18 {
		t.Fatalf("%v %+v", ok, got)
	}
}

func TestPriceIncreaseNoClaimWithoutGoodData(t *testing.T) {
	base := PriceInput{PlanID: "p", Key: "sua-tuoi", Name: "Sữa", History: milkHistory(), Today: day("2026-11-12")}
	cases := map[string][]market.Price{
		"không có quy cách":  {offer("Walmart", 4.20, "", "2026-11-11")},
		"khác chiều đo":      {offer("Walmart", 4.20, "900g", "2026-11-11")},
		"giá cũ quá 14 ngày": {offer("Walmart", 4.20, "1L", "2026-10-20")},
		"thiếu nguồn":        {{Store: "Walmart", PriceVND: 4.20 * rate, Local: 4.20, Currency: "USD", Size: "1L", On: "2026-11-11"}},
		"thiếu ngày":         {{Store: "Walmart", PriceVND: 4.20 * rate, Local: 4.20, Currency: "USD", Size: "1L", Source: "gemini_search"}},
		"không có giá":       {},
	}
	for name, offers := range cases {
		in := base
		in.Offers = offers
		if got, ok := PriceIncrease(in); ok {
			t.Errorf("%s: unexpected %+v", name, got)
		}
	}
	// Chưa từng mua: không có giá để so.
	in := base
	in.History = pricing.History("sua-tuoi", nil)
	in.Offers = []market.Price{offer("Walmart", 4.20, "1L", "2026-11-11")}
	if _, ok := PriceIncrease(in); ok {
		t.Error("no history should give no insight")
	}
}

func TestPriceIncreaseSmallRiseNeedsImpact(t *testing.T) {
	// +4%: dưới 10% và cả kế hoạch chỉ đắt thêm $0.30 (dưới $3): không có card.
	in := PriceInput{PlanID: "p", Key: "sua-tuoi", Name: "Sữa", Qty: 2, History: milkHistory(),
		Offers: []market.Price{offer("Walmart", 3.90, "1L", "2026-11-11")}, Today: day("2026-11-12")}
	if _, ok := PriceIncrease(in); ok {
		t.Fatal("4% on 2 packs should not trigger")
	}
	// Cũng +4% nhưng mua 24 hộp: đắt thêm $3.60, vượt $3 nên có card.
	in.Qty = 24
	got, ok := PriceIncrease(in)
	if !ok || got.Params.Pct != 4 {
		t.Fatalf("%v %+v", ok, got)
	}
}

func TestPriceIncreaseVietnamThreshold(t *testing.T) {
	h := pricing.History("gao", []pricing.Observation{{Store: "Co.opmart", PaidVND: 120_000, On: day("2026-11-01"), CanonicalQty: 5000, CanonicalUnit: domain.UnitGram}})
	in := PriceInput{PlanID: "p", Key: "gao", Name: "Gạo", History: h, Today: day("2026-11-12"),
		Offers: []market.Price{{Store: "WinMart", PriceVND: 138_000, Currency: "VND", Size: "5kg", Source: "gemini_search", On: "2026-11-11"}}}
	got, ok := PriceIncrease(in)
	if !ok || got.Params.Pct != 15 || got.Params.DisplayUnit != "kg" || got.Params.CurrentUnit != 27_600 || got.Params.LastPaidUnit != 24_000 {
		t.Fatalf("%v %+v", ok, got)
	}
}

func TestBudgetOverrun(t *testing.T) {
	est, budget := int64(132.50*rate), int64(120*rate)
	got, ok := BudgetOverrun("cuoi-tuan", &est, &budget, 0, "2026-11-12")
	if !ok || got.Params.OverrunVND != int64(12.50*rate) || got.Placement != PlacementPlanSummary || got.Severity != "warning" || got.ReportTemplate != ReportBudgetBreakdown {
		t.Fatalf("%v %+v", ok, got)
	}
	// Chỉnh về đúng ngân sách hoặc ít hơn: card biến mất.
	est = budget
	if _, ok := BudgetOverrun("cuoi-tuan", &est, &budget, 0, ""); ok {
		t.Fatal("at budget should not trigger")
	}
	if _, ok := BudgetOverrun("cuoi-tuan", &est, nil, 0, ""); ok {
		t.Fatal("no budget should not trigger")
	}
	if _, ok := BudgetOverrun("cuoi-tuan", nil, &budget, 0, ""); ok {
		t.Fatal("no estimate should not trigger")
	}
}

func TestOfferSize(t *testing.T) {
	cases := map[string]struct {
		qty  float64
		unit domain.Unit
	}{
		"1L": {1000, domain.UnitML}, "900g": {900, domain.UnitGram}, "2 x 180ml": {360, domain.UnitML},
		"1 gal": {3785.41, domain.UnitML}, "5 lb": {2267.96, domain.UnitGram}, "12 ct": {12, domain.UnitItem},
		"": {0, ""}, "family size": {0, ""},
	}
	for in, want := range cases {
		q, u := OfferSize(in)
		if q != want.qty || u != want.unit {
			t.Errorf("%q: %v %q, want %v %q", in, q, u, want.qty, want.unit)
		}
	}
}
