// Package insight là luật "Mimi nhận thấy" (đặc tả Contextual Insights 12/11, P0): nhận xét
// ngắn, có căn cứ, đặt ngay dưới thông tin liên quan trong Kế hoạch.
//
//   - Chỉ dùng dữ liệu có thật: giá đã trả trên hóa đơn (pricing) và giá đang bán có nguồn,
//     có ngày (market). Không gọi AI, không đoán.
//   - Thiếu đơn vị, khác chiều đo, giá cũ quá hạn hoặc thiếu nguồn: không kết luận gì.
//   - Mọi số tiền tính bằng VND; client tự đổi sang tiền hiển thị (USD) như các màn khác.
package insight

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/market"
	"github.com/smartlife-lab/mimi/backend/internal/pricing"
)

// Loại nhận xét và vị trí đặt card.
const (
	TypePriceIncrease = "PRICE_INCREASE"
	TypeBudgetOverrun = "BUDGET_OVERRUN"

	PlacementPlanItem    = "plan.item.inline"
	PlacementPlanSummary = "plan.summary.inline"

	ReportPriceHistory    = "product_price_history"
	ReportBudgetBreakdown = "plan_budget_breakdown"
)

// Config là ngưỡng thử nghiệm (đặc tả mục C.5), chỉnh được theo thị trường.
type Config struct {
	MinPct          float64 // giá tăng từ chừng này phần trăm
	MinImpactLocal  float64 // hoặc cả kế hoạch đắt thêm từ chừng này (tiền của nơi bán)
	MaxOfferAgeDays int     // giá đang bán cũ hơn thì không dùng để kết luận
	WarnPct         float64 // từ mức này card chuyển sang màu cảnh báo
}

// ConfigFor: ngưỡng theo tiền của nơi bán. USD: $3 (đặc tả); VND: 50.000đ.
func ConfigFor(currency string) Config {
	c := Config{MinPct: 10, MinImpactLocal: 3, MaxOfferAgeDays: 14, WarnPct: 25}
	if currency == "" || currency == "VND" {
		c.MinImpactLocal = 50_000
	}
	return c
}

// Params là số liệu để client ghép câu theo i18n. Tiền là VND.
type Params struct {
	Name string `json:"name,omitempty"`
	Pct  int    `json:"pct,omitempty"`
	// Giá theo đơn vị hiển thị (L, kg, m, cái): giá đang bán và giá lần mua gần nhất.
	DisplayUnit   string  `json:"display_unit,omitempty"`
	CurrentUnit   float64 `json:"current_unit_vnd,omitempty"`
	LastPaidUnit  float64 `json:"last_paid_unit_vnd,omitempty"`
	LastPaidOn    string  `json:"last_paid_on,omitempty"`
	LastPaidStore string  `json:"last_paid_store,omitempty"`
	OfferStore    string  `json:"offer_store,omitempty"`
	OfferOn       string  `json:"offer_on,omitempty"`
	ImpactVND     int64   `json:"impact_vnd,omitempty"` // cả kế hoạch đắt thêm
	// Vượt ngân sách.
	EstimateVND  int64 `json:"estimate_vnd,omitempty"`
	BudgetVND    int64 `json:"budget_vnd,omitempty"`
	OverrunVND   int64 `json:"overrun_vnd,omitempty"`
	MissingPrice int   `json:"missing_price,omitempty"` // số món chưa có giá, chưa cộng vào dự kiến
}

// Insight là một nhận xét (DTO đặc tả mục F).
type Insight struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	EntityType     string `json:"entity_type"` // plan_item | plan
	EntityID       string `json:"entity_id"`
	PlanID         string `json:"plan_id"`
	Placement      string `json:"placement"`
	Severity       string `json:"severity"` // info | warning
	ReportTemplate string `json:"report_template"`
	Params         Params `json:"message_params"`
	SourceAsOf     string `json:"source_as_of,omitempty"` // ngày của dữ liệu mới nhất dùng để kết luận
}

// id cố định theo dữ liệu: cùng dữ liệu ra cùng id (chống trùng, ẩn theo phiên bản dữ liệu);
// dữ liệu đổi thì ra id mới và card hiện lại.
func id(prefix string, parts ...any) string {
	h := sha1.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%v|", p)
	}
	return prefix + "_" + hex.EncodeToString(h.Sum(nil))[:12]
}

// ---------- Giá tăng so với lần mua gần nhất (P0-01) ----------

// PriceInput là một dòng Kế hoạch kèm lịch sử giá đã trả và giá đang bán.
type PriceInput struct {
	PlanID  string
	Key     string
	Name    string
	Qty     float64 // số gói trong kế hoạch; 0 coi như 1
	History pricing.ItemHistory
	Offers  []market.Price
	Today   domain.Date
}

// OfferCheck là kết quả xét một giá đang bán: dùng được để so hay không, và vì sao.
type OfferCheck struct {
	market.Price
	CanonicalQty float64     `json:"canonical_quantity,omitempty"`
	Unit         domain.Unit `json:"canonical_unit,omitempty"`
	UnitPrice    float64     `json:"unit_price,omitempty"` // đ / canonical unit
	Usable       bool        `json:"usable"`
	Reason       string      `json:"reason,omitempty"` // no_size | other_unit | stale | no_source | no_price
}

// CheckOffers đọc quy cách từng giá đang bán và đánh dấu giá nào so được với giá đã trả.
func CheckOffers(offers []market.Price, unit domain.Unit, today domain.Date, cfg Config) []OfferCheck {
	out := make([]OfferCheck, 0, len(offers))
	for _, o := range offers {
		c := OfferCheck{Price: o}
		qty, u := OfferSize(o.Size)
		c.CanonicalQty, c.Unit = qty, u
		switch {
		case o.PriceVND <= 0:
			c.Reason = "no_price"
		case strings.TrimSpace(o.Source) == "" || o.On == "":
			c.Reason = "no_source"
		case stale(o.On, today, cfg.MaxOfferAgeDays):
			c.Reason = "stale"
		case u == "" || qty <= 0:
			c.Reason = "no_size"
		case unit == "" || u != unit:
			c.Reason = "other_unit"
		default:
			c.Usable = true
			c.UnitPrice = round2(float64(o.PriceVND) / qty)
		}
		out = append(out, c)
	}
	return out
}

func stale(on string, today domain.Date, maxDays int) bool {
	d, err := domain.ParseDate(on)
	if err != nil {
		return true
	}
	return d.Before(today.AddDays(-maxDays)) || d.After(today.AddDays(1))
}

// PriceIncrease: card khi giá đang bán rẻ nhất (cùng chiều đo, có nguồn, còn mới) vẫn cao hơn
// giá đã trả lần gần nhất từ MinPct trở lên, hoặc cả kế hoạch đắt thêm từ MinImpactLocal.
// So với giá rẻ nhất nên khi card hiện, mua ở đâu trong các nơi đã tra cũng đắt hơn lần trước.
func PriceIncrease(in PriceInput) (Insight, bool) {
	last := in.History.LastPurchase
	if last == nil || !last.Comparable() || last.UnitPrice <= 0 {
		return Insight{}, false
	}
	currency := "VND"
	for _, o := range in.Offers {
		if o.Currency != "" {
			currency = o.Currency
			break
		}
	}
	cfg := ConfigFor(currency)
	var best *OfferCheck
	for _, c := range CheckOffers(in.Offers, last.CanonicalUnit, in.Today, cfg) {
		if c.Usable && (best == nil || c.UnitPrice < best.UnitPrice) {
			x := c
			best = &x
		}
	}
	if best == nil {
		return Insight{}, false
	}
	delta := best.UnitPrice - last.UnitPrice
	if delta <= 0 {
		return Insight{}, false
	}
	pct := delta / last.UnitPrice * 100
	qty := in.Qty
	if qty <= 0 {
		qty = 1
	}
	// Lượng so sánh: số gói trong kế hoạch × quy cách gói của giá đang bán.
	impact := delta * qty * best.CanonicalQty
	minImpact := cfg.MinImpactLocal
	if currency != "VND" {
		if best.Local <= 0 {
			return Insight{}, false // không quy đổi được ngưỡng tiền
		}
		minImpact *= float64(best.PriceVND) / best.Local
	}
	if pct < cfg.MinPct && impact < minImpact {
		return Insight{}, false
	}
	f, du := displayFactor(last.CanonicalUnit)
	sev := "info"
	if pct >= cfg.WarnPct {
		sev = "warning"
	}
	asOf := best.On
	if last.On.String() > asOf {
		asOf = last.On.String()
	}
	return Insight{
		ID:             id("pi", in.PlanID, in.Key, last.On, last.PaidVND, best.Store, best.PriceVND, best.On, qty),
		Type:           TypePriceIncrease,
		EntityType:     "plan_item",
		EntityID:       in.Key,
		PlanID:         in.PlanID,
		Placement:      PlacementPlanItem,
		Severity:       sev,
		ReportTemplate: ReportPriceHistory,
		SourceAsOf:     asOf,
		Params: Params{
			Name: in.Name, Pct: int(math.Round(pct)), DisplayUnit: du,
			CurrentUnit: round2(best.UnitPrice * f), LastPaidUnit: round2(last.UnitPrice * f),
			LastPaidOn: last.On.String(), LastPaidStore: last.Store, OfferStore: best.Store, OfferOn: best.On,
			ImpactVND: int64(math.Round(impact)),
		},
	}, true
}

// displayFactor: đơn giá đ/ml hiển thị theo L, đ/g theo kg.
func displayFactor(u domain.Unit) (float64, string) {
	switch u {
	case domain.UnitML:
		return 1000, "L"
	case domain.UnitGram:
		return 1000, "kg"
	case domain.UnitMeter:
		return 1, "m"
	}
	return 1, "item"
}

// ---------- Vượt ngân sách cấp Kế hoạch (P0-02) ----------

// BudgetOverrun: card khi tổng dự kiến lớn hơn ngân sách đã đặt cho Kế hoạch.
func BudgetOverrun(planID string, estimate, budget *int64, missing int, asOf string) (Insight, bool) {
	if estimate == nil || budget == nil || *budget <= 0 {
		return Insight{}, false
	}
	over := *estimate - *budget
	if over <= 0 {
		return Insight{}, false
	}
	sev := "info"
	if over*10 >= *budget { // vượt từ 10% ngân sách
		sev = "warning"
	}
	return Insight{
		ID:             id("bo", planID, *estimate, *budget),
		Type:           TypeBudgetOverrun,
		EntityType:     "plan",
		EntityID:       planID,
		PlanID:         planID,
		Placement:      PlacementPlanSummary,
		Severity:       sev,
		ReportTemplate: ReportBudgetBreakdown,
		SourceAsOf:     asOf,
		Params:         Params{EstimateVND: *estimate, BudgetVND: *budget, OverrunVND: over, MissingPrice: missing},
	}, true
}

// Sort: card kế hoạch trước, rồi theo thứ tự món.
func Sort(ins []Insight, order map[string]int) {
	sort.SliceStable(ins, func(i, j int) bool {
		if (ins[i].EntityType == "plan") != (ins[j].EntityType == "plan") {
			return ins[i].EntityType == "plan"
		}
		return order[ins[i].EntityID] < order[ins[j].EntityID]
	})
}

// ---------- Quy cách của giá đang bán ----------

// Đơn vị Mỹ mà luật đọc hóa đơn Việt Nam không có (chỉ dùng ở đây, không đổi luật tồn kho).
var reUS = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(fl\.?\s*oz|oz|lbs?|gal|gallon|qt|quart|pt|pint|ct|count)\b`)

var usUnits = map[string]struct {
	unit   domain.Unit
	factor float64
}{
	"floz": {domain.UnitML, 29.5735}, "oz": {domain.UnitGram, 28.3495}, "lb": {domain.UnitGram, 453.592}, "lbs": {domain.UnitGram, 453.592},
	"gal": {domain.UnitML, 3785.41}, "gallon": {domain.UnitML, 3785.41}, "qt": {domain.UnitML, 946.353}, "quart": {domain.UnitML, 946.353},
	"pt": {domain.UnitML, 473.176}, "pint": {domain.UnitML, 473.176}, "ct": {domain.UnitItem, 1}, "count": {domain.UnitItem, 1},
}

// OfferSize đọc quy cách nguồn ghi ("1L", "900g", "2 x 180ml", "64 fl oz", "1 gal") ra lượng
// chuẩn hóa. Không đọc được thì trả 0, "".
func OfferSize(size string) (float64, domain.Unit) {
	s := strings.ToLower(strings.TrimSpace(size))
	if s == "" {
		return 0, ""
	}
	if m := reUS.FindStringSubmatch(s); m != nil {
		n, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		key := strings.NewReplacer(".", "", " ", "").Replace(m[2])
		if u, ok := usUnits[key]; ok && err == nil && n > 0 {
			return round2(n * u.factor), u.unit
		}
	}
	p := domain.ParsePackaging(s)
	if p.Known() {
		return p.Total, p.Unit
	}
	return 0, ""
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
