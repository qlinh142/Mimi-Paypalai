package order

import (
	"fmt"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Mandate là Household Mandate (Mimi 2.0 mục 13): phạm vi Mimi được tự mua online.
// Chỉ áp dụng cho Mimi Online Shopping, không áp dụng cho người mua offline.
type Mandate struct {
	Enabled           bool     `json:"enabled"`             // false: mọi đơn đều cần người dùng duyệt (ASK)
	Categories        []string `json:"categories"`          // danh mục được phép: grocery, household
	PriceTolerancePct int      `json:"price_tolerance_pct"` // 0..50: giá được cao hơn snapshot tốt nhất bao nhiêu %
	MaxOrderVND       int64    `json:"max_order_vnd"`       // trần mỗi đơn; 0: không đặt trần
	MaxDayVND         int64    `json:"max_day_vnd"`         // trần tổng các đơn đã thanh toán trong ngày; 0: không đặt
}

// DefaultMandate: chưa bật, chỉ cho grocery và household, dung sai giá 10%, tối đa 1 triệu/đơn.
func DefaultMandate() Mandate {
	return Mandate{Categories: []string{CategoryGrocery, CategoryHousehold}, PriceTolerancePct: 10, MaxOrderVND: 1_000_000}
}

func (m Mandate) Validate() error {
	if m.PriceTolerancePct < 0 || m.PriceTolerancePct > 50 {
		return fmt.Errorf("price_tolerance_pct must be 0..50")
	}
	if m.MaxOrderVND < 0 || m.MaxDayVND < 0 || m.MaxOrderVND > int64(domain.MaxLineVND) || m.MaxDayVND > int64(domain.MaxLineVND) {
		return fmt.Errorf("budget out of range")
	}
	for _, c := range m.Categories {
		if c != CategoryGrocery && c != CategoryHousehold {
			return fmt.Errorf("unknown category %q", c)
		}
	}
	return nil
}

// Result của Household Mandate.
type Result string

const (
	Allow Result = "ALLOW"
	Ask   Result = "ASK"
	Block Result = "BLOCK"
)

// Reason giải thích một kết quả ASK/BLOCK, kèm khóa món nếu thuộc một dòng.
type Reason struct {
	Key    string `json:"key,omitempty"`
	Code   string `json:"code"` // not_in_plan | quantity_guard | category | price_guard | budget_order | budget_day | delivery_guard | mandate_off
	Result Result `json:"result"`
	Detail string `json:"detail"`
}

// Decision là kết quả đánh giá cả giỏ online.
type Decision struct {
	Result  Result   `json:"result"`
	Reasons []Reason `json:"reasons"`
}

// MaxAuthorizedPrice = Latest Best Price Snapshot × (1 + Price Tolerance), làm tròn xuống đồng.
func MaxAuthorizedPrice(snapshot int64, tolerancePct int) int64 {
	return snapshot * int64(100+tolerancePct) / 100
}

// Evaluate áp Household Mandate lên giỏ online:
//
//	Plan Item + Quantity + Price + Budget + Merchant + Need-by → ALLOW / ASK / BLOCK
//
// BLOCK: vi phạm rule cứng (món ngoài Kế hoạch, vượt số lượng, danh mục không được phép).
// ASK: ngoại lệ cần người dùng duyệt (giá vượt dung sai, vượt ngân sách, giao không kịp,
// mandate chưa bật). Promotion không tạo nhu cầu: giỏ chỉ gồm món đã có trong Kế hoạch.
func Evaluate(m Mandate, b Basket, paidTodayVND int64) Decision {
	d := Decision{Result: Allow, Reasons: []Reason{}}
	add := func(r Reason) {
		d.Reasons = append(d.Reasons, r)
		if r.Result == Block || (r.Result == Ask && d.Result == Allow) {
			d.Result = r.Result
		}
	}
	allowedCat := map[string]bool{}
	for _, c := range m.Categories {
		allowedCat[c] = true
	}
	for _, l := range b.Lines {
		if !l.InPlan {
			add(Reason{Key: l.Key, Code: "not_in_plan", Result: Block, Detail: "Món không có trong Kế hoạch"})
			continue
		}
		if l.OverPlan {
			add(Reason{Key: l.Key, Code: "quantity_guard", Result: Block, Detail: "Vượt số lượng Kế hoạch cho phép"})
		}
		if !allowedCat[l.Category] {
			add(Reason{Key: l.Key, Code: "category", Result: Block, Detail: "Danh mục " + l.Category + " chưa được ủy quyền"})
		}
		if maxP := MaxAuthorizedPrice(l.SnapshotPriceVND, m.PriceTolerancePct); l.UnitPriceVND > maxP {
			add(Reason{Key: l.Key, Code: "price_guard", Result: Ask,
				Detail: fmt.Sprintf("Giá %dđ cao hơn mức cho phép %dđ (snapshot %dđ + %d%%)", l.UnitPriceVND, maxP, l.SnapshotPriceVND, m.PriceTolerancePct)})
		}
		if l.NeedBy != "" && l.ETA != "" {
			need, err1 := domain.ParseDate(l.NeedBy)
			eta, err2 := domain.ParseDate(l.ETA)
			if err1 == nil && err2 == nil && eta.After(need) {
				add(Reason{Key: l.Key, Code: "delivery_guard", Result: Ask, Detail: "Giao dự kiến " + l.ETA + ", sau ngày cần " + l.NeedBy})
			}
		}
	}
	if m.MaxOrderVND > 0 && b.TotalVND > m.MaxOrderVND {
		add(Reason{Code: "budget_order", Result: Ask, Detail: fmt.Sprintf("Tổng đơn %dđ vượt trần %dđ/đơn", b.TotalVND, m.MaxOrderVND)})
	}
	if m.MaxDayVND > 0 && paidTodayVND+b.TotalVND > m.MaxDayVND {
		add(Reason{Code: "budget_day", Result: Ask, Detail: fmt.Sprintf("Vượt trần %dđ/ngày (đã chi hôm nay %dđ)", m.MaxDayVND, paidTodayVND)})
	}
	if !m.Enabled && d.Result == Allow {
		add(Reason{Code: "mandate_off", Result: Ask, Detail: "Chưa bật ủy quyền mua online: cần bạn duyệt đơn này"})
	}
	return d
}
