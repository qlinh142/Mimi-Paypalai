// Package spending dựng Sổ chi tiêu theo tháng (GET /spending, chỉ chủ giỏ).
// Tổng tháng lấy từ months/{YYYY-MM} (cập nhật trong transaction lưu hóa đơn);
// danh sách hóa đơn lấy từ receipts của tháng đó, cộng phần người nhà mua hộ (B4).
package spending

import (
	"context"
	"math"
	"sort"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Month là tài liệu months/{YYYY-MM}. Giá trị rỗng nghĩa là tháng chưa có hóa đơn.
type Month struct {
	SpentVND    int64
	DiscountVND int64
	Receipts    int64
	ByCategory  map[string]int64
	// Mimi Online (PayPal): đã nằm trong SpentVND, tách ra để hiển thị; RefundVND đã trừ khỏi SpentVND.
	OnlineVND    int64
	OnlineOrders int64
	RefundVND    int64
	// DelegatedVND: phần người nhà mua hộ cho mình (B4), đã nằm trong SpentVND.
	DelegatedVND int64
}

// ReceiptRow là một hóa đơn trong danh sách của Sổ chi tiêu.
type ReceiptRow struct {
	ID          string `json:"receipt_id"`
	Store       string `json:"store,omitempty"`
	PurchasedOn string `json:"purchased_on"` // YYYY-MM-DD
	TotalVND    int64  `json:"total_vnd"`
	DiscountVND int64  `json:"discount_vnd"`
	// Delegated: dòng này là phần người nhà mua hộ (id "d-{receipt}"), TotalVND chỉ gồm món của mình.
	Delegated bool `json:"delegated,omitempty"`
}

type Store interface {
	Month(ctx context.Context, uid, month string) (Month, error) // tháng chưa có: Month{}, nil
	// MonthReceipts trả tối đa limit hóa đơn có purchased_on trong tháng, không cần thứ tự.
	MonthReceipts(ctx context.Context, uid, month string, limit int) ([]ReceiptRow, error)
}

type CategoryShare struct {
	Key    string  `json:"key"`
	Amount int64   `json:"amount"`
	Pct    float64 `json:"pct"` // phần trăm trên tổng các nhóm, 1 chữ số thập phân
}

// Report là phản hồi GET /spending.
type Report struct {
	Month             string          `json:"month"`
	Spent             int64           `json:"spent"`
	SpentPrevMonthPct *float64        `json:"spent_prev_month_pct"` // null khi tháng trước chưa có chi tiêu
	Saved             int64           `json:"saved"`
	SavedRatio        *float64        `json:"saved_ratio"` // discount / (spent + discount); null khi không có giảm giá
	ReceiptCount      int64           `json:"receipt_count"`
	OnlineSpent       int64           `json:"online_spent"`    // chi qua PayPal, net sau refund
	OnlineOrders      int64           `json:"online_orders"`   // số đơn PayPal đã thanh toán
	Refunded          int64           `json:"refunded"`        // tổng refund PayPal trong tháng
	DelegatedSpent    int64           `json:"delegated_spent"` // người nhà mua hộ, đã nằm trong spent (B4)
	ByCategory        []CategoryShare `json:"by_category"`
	TopSavingReceipts []ReceiptRow    `json:"top_saving_receipts"`
	RecentReceipts    []ReceiptRow    `json:"recent_receipts"`
}

// receiptScanLimit: số hóa đơn tối đa đọc cho một tháng. Một hộ gia đình hiếm khi vượt
// con số này; nếu vượt, hai danh sách top có thể thiếu nhưng tổng tháng vẫn đúng.
const receiptScanLimit = 500

const listSize = 3

// historySize: Lịch sử mua hàng của tháng (Mimi 1.5, góp ý 31/10 16:32) liệt kê mọi hóa đơn
// (nơi mua, ngày, số tiền) chứ không chỉ 3 hóa đơn gần nhất; màn Địa điểm mua hàng cũng gộp từ đây.
const historySize = 200

type Service struct{ store Store }

func New(store Store) *Service { return &Service{store: store} }

func (s *Service) Get(ctx context.Context, p authz.Principal, month string) (Report, error) {
	if err := authz.Can(p, authz.ActionReadSpending); err != nil {
		return Report{}, err
	}
	prev, ok := domain.ParseMonth(month)
	if !ok {
		return Report{}, apperr.New(apperr.CodeValidation, "Tháng phải có dạng YYYY-MM.").With("field", "month")
	}
	cur, err := s.store.Month(ctx, p.UID, month)
	if err != nil {
		return Report{}, err
	}
	before, err := s.store.Month(ctx, p.UID, prev)
	if err != nil {
		return Report{}, err
	}
	rows, err := s.store.MonthReceipts(ctx, p.UID, month, receiptScanLimit)
	if err != nil {
		return Report{}, err
	}
	return Build(month, cur, before, rows), nil
}

// Build là phần tính toán thuần, tách riêng để test không cần Firestore.
func Build(month string, cur, prev Month, rows []ReceiptRow) Report {
	r := Report{
		Month: month, Spent: cur.SpentVND, Saved: cur.DiscountVND, ReceiptCount: cur.Receipts,
		OnlineSpent: cur.OnlineVND, OnlineOrders: cur.OnlineOrders, Refunded: cur.RefundVND, DelegatedSpent: cur.DelegatedVND,
		ByCategory: []CategoryShare{}, TopSavingReceipts: []ReceiptRow{}, RecentReceipts: []ReceiptRow{},
	}
	if prev.SpentVND > 0 {
		v := round(float64(cur.SpentVND-prev.SpentVND)/float64(prev.SpentVND)*100, 1)
		r.SpentPrevMonthPct = &v
	}
	if cur.DiscountVND > 0 && cur.SpentVND+cur.DiscountVND > 0 {
		v := round(float64(cur.DiscountVND)/float64(cur.SpentVND+cur.DiscountVND), 3)
		r.SavedRatio = &v
	}

	var catTotal int64
	for _, v := range cur.ByCategory {
		if v > 0 {
			catTotal += v
		}
	}
	for k, v := range cur.ByCategory {
		if v <= 0 {
			continue
		}
		r.ByCategory = append(r.ByCategory, CategoryShare{Key: k, Amount: v, Pct: round(float64(v)/float64(catTotal)*100, 1)})
	}
	sort.Slice(r.ByCategory, func(a, b int) bool {
		if r.ByCategory[a].Amount != r.ByCategory[b].Amount {
			return r.ByCategory[a].Amount > r.ByCategory[b].Amount
		}
		return r.ByCategory[a].Key < r.ByCategory[b].Key
	})

	recent := append([]ReceiptRow(nil), rows...)
	sort.Slice(recent, func(a, b int) bool {
		if recent[a].PurchasedOn != recent[b].PurchasedOn {
			return recent[a].PurchasedOn > recent[b].PurchasedOn
		}
		return recent[a].ID > recent[b].ID
	})
	r.RecentReceipts = append(r.RecentReceipts, recent[:min(historySize, len(recent))]...)

	var saving []ReceiptRow
	for _, row := range rows {
		if row.DiscountVND > 0 {
			saving = append(saving, row)
		}
	}
	sort.Slice(saving, func(a, b int) bool {
		if saving[a].DiscountVND != saving[b].DiscountVND {
			return saving[a].DiscountVND > saving[b].DiscountVND
		}
		return saving[a].PurchasedOn > saving[b].PurchasedOn
	})
	r.TopSavingReceipts = append(r.TopSavingReceipts, saving[:min(listSize, len(saving))]...)
	return r
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
