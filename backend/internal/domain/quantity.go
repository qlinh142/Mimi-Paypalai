package domain

import (
	"math"
	"strings"
)

// Unit là đơn vị gốc của tồn kho (Mimi 2.0 mục 4): khối lượng tính bằng gram, thể tích
// bằng ml, còn lại đếm theo cái (hộp, gói, chai, quả...). Mọi số lượng tồn đều quy về
// một trong ba đơn vị này để cộng trừ được với nhau.
type Unit string

const (
	UnitGram  Unit = "g"
	UnitML    Unit = "ml"
	UnitPiece Unit = "pc"
)

// unitAliases: đơn vị in trên hóa đơn (đã bỏ dấu, chữ thường) → đơn vị gốc và hệ số.
var unitAliases = map[string]struct {
	base   Unit
	factor float64
}{
	"g": {UnitGram, 1}, "gr": {UnitGram, 1}, "gram": {UnitGram, 1},
	"kg": {UnitGram, 1000}, "kgs": {UnitGram, 1000}, "can": {UnitGram, 1000}, // "cân" = kg
	"lang": {UnitGram, 100}, // "lạng"
	"ml":   {UnitML, 1}, "l": {UnitML, 1000}, "lit": {UnitML, 1000}, "litre": {UnitML, 1000}, "liter": {UnitML, 1000},
	"pc": {UnitPiece, 1}, "pcs": {UnitPiece, 1}, "cai": {UnitPiece, 1}, "chiec": {UnitPiece, 1},
	"hop": {UnitPiece, 1}, "goi": {UnitPiece, 1}, "chai": {UnitPiece, 1}, "lon": {UnitPiece, 1},
	"tui": {UnitPiece, 1}, "bich": {UnitPiece, 1}, "qua": {UnitPiece, 1}, "trai": {UnitPiece, 1},
	"cuon": {UnitPiece, 1}, "bo": {UnitPiece, 1}, "vi": {UnitPiece, 1}, "hu": {UnitPiece, 1},
	"thung": {UnitPiece, 1}, "loc": {UnitPiece, 1}, "khay": {UnitPiece, 1}, "bao": {UnitPiece, 1},
	"cay": {UnitPiece, 1}, "tuyp": {UnitPiece, 1}, "ea": {UnitPiece, 1}, "unit": {UnitPiece, 1},
}

// NormalizeUnit chuẩn hóa đơn vị in trên hóa đơn thành khóa trong unitAliases
// ("Kg" → "kg", "Hộp" → "hop"). Trả "" nếu không nhận ra.
func NormalizeUnit(s string) string {
	k := strings.TrimSuffix(NormalizeVi(s), ".")
	if _, ok := unitAliases[k]; ok {
		return k
	}
	return ""
}

// ToBase quy một số lượng về đơn vị gốc: (2, "kg") → (2000, g). Đơn vị không rõ coi là cái.
func ToBase(qty float64, unit string) (float64, Unit) {
	a, ok := unitAliases[NormalizeUnit(unit)]
	if !ok {
		return qty, UnitPiece
	}
	return qty * a.factor, a.base
}

// BaselineSource: nguồn của một mốc tồn có bằng chứng (Confirmed Inventory).
type BaselineSource string

const (
	BaselineReceipt  BaselineSource = "receipt"  // hóa đơn đã xác nhận
	BaselineDelivery BaselineSource = "delivery" // hàng giao đã Verify
	BaselineManual   BaselineSource = "manual"   // người dùng SET số lượng thực tế
)

// Baseline là Confirmed Inventory: lượng có bằng chứng tại một ngày. Không bao giờ bị sửa
// bởi ước tính; người dùng xác nhận lượng mới thì tạo baseline mới (spec 2.0 mục 4).
type Baseline struct {
	Qty    float64 // theo Unit
	Unit   Unit
	On     Date
	Source BaselineSource
}

// Consumption là một lần tiêu thụ đã lên kế hoạch (bữa ăn đã qua) trừ vào tồn ước tính.
type Consumption struct {
	Qty float64 // theo đơn vị của baseline
	On  Date
}

// ConsumptionRate ước tính lượng dùng mỗi ngày từ lượng mua mỗi lần và chu kỳ mua.
// Trả 0 khi chưa đủ dữ liệu (chưa có chu kỳ).
func ConsumptionRate(qtyPerPurchase []float64, intervalDays int) float64 {
	if intervalDays <= 0 || len(qtyPerPurchase) == 0 {
		return 0
	}
	var sum float64
	for _, q := range qtyPerPurchase {
		sum += q
	}
	return sum / float64(len(qtyPerPurchase)) / float64(intervalDays)
}

// EstimateAt là Estimated Inventory tại một ngày: baseline trừ lượng dùng đều theo
// rate và các lần tiêu thụ đã ghi sau baseline, không âm.
func EstimateAt(b Baseline, ratePerDay float64, used []Consumption, on Date) float64 {
	if b.On.IsZero() {
		return 0
	}
	est := b.Qty
	if days := b.On.DaysUntil(on); days > 0 {
		est -= ratePerDay * float64(days)
	}
	for _, c := range used {
		if !c.On.Before(b.On) && !c.On.After(on) {
			est -= c.Qty
		}
	}
	return math.Max(0, round3(est))
}

// AfterPurchase tạo baseline mới khi có hàng vào (hóa đơn hoặc giao hàng đã Verify):
// lượng ước tính còn lại tại ngày mua cộng lượng vừa mua. Khác đơn vị thì baseline cũ
// không cộng được, lấy lượng vừa mua.
func AfterPurchase(prev Baseline, ratePerDay float64, used []Consumption, qty float64, unit Unit, on Date, src BaselineSource) Baseline {
	left := 0.0
	if prev.Unit == unit {
		left = EstimateAt(prev, ratePerDay, used, on)
	}
	return Baseline{Qty: round3(left + qty), Unit: unit, On: on, Source: src}
}

// DaysOfStock: số ngày tồn ước tính còn dùng được với rate hiện tại; nil nếu chưa có rate.
func DaysOfStock(estimate, ratePerDay float64) *int {
	if ratePerDay <= 0 {
		return nil
	}
	d := int(math.Floor(estimate / ratePerDay))
	return &d
}

// ShoppingDelta (spec 2.0 mục 9): lượng cần mua = lượng cần - tồn dùng được, không âm.
func ShoppingDelta(required, usable float64) float64 {
	return math.Max(0, round3(required-usable))
}

func round3(x float64) float64 { return math.Round(x*1000) / 1000 }
