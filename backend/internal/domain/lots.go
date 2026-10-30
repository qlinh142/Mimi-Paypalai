package domain

import "sort"

// Lot là một purchase lot (0.1 mục 11): mỗi dòng hóa đơn đã xác nhận (hoặc hàng giao đã
// Verify) là một lot độc lập. Lot không bao giờ bị sửa; Remaining chỉ là số suy ra.
type Lot struct {
	ID         string  `json:"lot_id"`
	DetailName string  `json:"detail_name,omitempty"`
	Brand      string  `json:"brand,omitempty"`
	Store      string  `json:"store,omitempty"`
	On         Date    `json:"purchased_at"`
	Initial    float64 `json:"initial_quantity"`
	Unit       Unit    `json:"canonical_unit"`
	Remaining  float64 `json:"remaining_quantity"`
	Source     string  `json:"inventory_source"` // receipt | delivery
}

// AllocateFIFO chia tồn ước tính của một consumption stream xuống từng lot theo FIFO
// (0.1 mục 13): lượng đã dùng trừ từ lot cũ nhất trước, nên phần còn lại nằm ở các lot
// mới nhất. Chỉ lot cùng Unit với stream được chia; lot khác chiều đo giữ Remaining = 0
// ở stream này (chúng thuộc stream của đơn vị khác, không cộng g với ml).
//
// Manual SET (open point 5 của 0.1) được chia cùng cách: lượng người dùng đếm được nằm ở
// các lot mới nhất. Phần vượt tổng các lot (người dùng SET nhiều hơn đã mua) trả về extra.
func AllocateFIFO(lots []Lot, estimate float64, unit Unit) (out []Lot, extra float64) {
	out = append([]Lot(nil), lots...)
	idx := make([]int, 0, len(out))
	for i := range out {
		out[i].Remaining = 0
		if out[i].Unit == unit {
			idx = append(idx, i)
		}
	}
	// Mới nhất trước; cùng ngày thì theo ID để kết quả ổn định.
	sort.SliceStable(idx, func(a, b int) bool {
		la, lb := out[idx[a]], out[idx[b]]
		if !la.On.Equal(lb.On) {
			return la.On.After(lb.On)
		}
		return la.ID > lb.ID
	})
	left := estimate
	for _, i := range idx {
		if left <= 0 {
			break
		}
		take := min(out[i].Initial, left)
		out[i].Remaining = round3(take)
		left -= take
	}
	return out, max(0, round3(left))
}
