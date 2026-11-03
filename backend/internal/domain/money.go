// Package domain chứa logic nghiệp vụ thuần của Mimi: tiền, ngày, dự đoán tồn kho,
// nhãn trạng thái, nơi mua tối ưu. Package này không gọi mạng, không đọc Firestore,
// không phụ thuộc thời gian thực (mọi "hôm nay" đều được truyền vào), nên test được
// bằng dữ liệu mẫu của bản demo.
package domain

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// VND là số tiền tính bằng đồng, luôn là số nguyên. Không bao giờ dùng float cho tiền.
type VND int64

// MaxLineVND chặn giá trị phi lý (lỗi đọc hoặc dữ liệu giả mạo): 1 tỷ đồng mỗi dòng.
const MaxLineVND VND = 1_000_000_000

var (
	ErrNegativeAmount = errors.New("domain: amount must not be negative")
	ErrAmountTooLarge = errors.New("domain: amount exceeds limit")
	ErrBadDecimal     = errors.New("domain: invalid decimal string")
)

// NewVND kiểm tra một số tiền dòng hóa đơn.
func NewVND(v int64) (VND, error) {
	switch {
	case v < 0:
		return 0, ErrNegativeAmount
	case VND(v) > MaxLineVND:
		return 0, ErrAmountTooLarge
	}
	return VND(v), nil
}

// Sum cộng các khoản; trả lỗi nếu tràn giới hạn hợp lý.
func Sum(xs ...VND) (VND, error) {
	var total VND
	for _, x := range xs {
		if x < 0 {
			return 0, ErrNegativeAmount
		}
		total += x
		if total > MaxLineVND*1000 {
			return 0, ErrAmountTooLarge
		}
	}
	return total, nil
}

// ConvertToVND quy đổi một số tiền ngoại tệ dạng thập phân ("12.50") theo tỷ giá dạng
// thập phân ("18450.25") sang đồng. Tính bằng số hữu tỉ chính xác, làm tròn một lần ở
// cuối theo kiểu "nửa xa số 0". Tỷ giá phải đến từ fx_rates có nguồn; hàm này không
// bao giờ tự chọn tỷ giá.
func ConvertToVND(amount, rate string) (VND, error) {
	a, ok := new(big.Rat).SetString(strings.TrimSpace(amount))
	if !ok {
		return 0, fmt.Errorf("%w: amount %q", ErrBadDecimal, amount)
	}
	r, ok := new(big.Rat).SetString(strings.TrimSpace(rate))
	if !ok || r.Sign() <= 0 {
		return 0, fmt.Errorf("%w: rate %q", ErrBadDecimal, rate)
	}
	if a.Sign() < 0 {
		return 0, ErrNegativeAmount
	}
	product := new(big.Rat).Mul(a, r)
	// Làm tròn nửa xa số 0: floor(x + 1/2) với x >= 0.
	half := big.NewRat(1, 2)
	product.Add(product, half)
	q := new(big.Int).Quo(product.Num(), product.Denom())
	if !q.IsInt64() {
		return 0, ErrAmountTooLarge
	}
	return NewVND(q.Int64())
}

// AllocateDiscount chia tổng giảm giá của cả hóa đơn (dòng khuyến mãi in số âm) về từng dòng
// theo tỷ lệ tiền của dòng (mẹ đồng ý 03/11), để sau này giá vốn mỗi món là giá đã trừ giảm.
// Dòng chưa rõ giá (nil) hoặc 0đ không nhận phần nào. Tổng các phần đúng bằng discount
// (phần dư do làm tròn chia cho dòng có phần lẻ lớn nhất, hòa thì dòng đứng trước). Phần của
// một dòng không vượt tiền của dòng; giảm giá lớn hơn tổng các dòng thì trả nil.
func AllocateDiscount(amounts []*VND, discount VND) []VND {
	out := make([]VND, len(amounts))
	if discount <= 0 {
		return out
	}
	var total VND
	for _, a := range amounts {
		if a != nil && *a > 0 {
			total += *a
		}
	}
	if total <= 0 || discount > total {
		return nil
	}
	type rem struct {
		i int
		r int64
	}
	var given VND
	rems := make([]rem, 0, len(amounts))
	for i, a := range amounts {
		if a == nil || *a <= 0 {
			continue
		}
		num := int64(*a) * int64(discount) // tiền đồng: không tràn int64 với hóa đơn thực tế
		out[i] = VND(num / int64(total))
		given += out[i]
		rems = append(rems, rem{i, num % int64(total)})
	}
	sort.SliceStable(rems, func(a, b int) bool { return rems[a].r > rems[b].r })
	for k := 0; given < discount; k++ {
		out[rems[k%len(rems)].i]++
		given++
	}
	return out
}
