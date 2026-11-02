// Package demo ghi dữ liệu mẫu cho kịch bản demo (V5): nhà chị Lan có khoảng 5 tuần lịch
// sử mua sắm, tính lùi từ ngày chạy, để khi mở app vào ngày demo thì Sữa tươi còn khoảng
// 2 ngày, Trứng gà sắp hết, Dầu gội còn khoảng 5 ngày (kịch bản demo, cảnh 3a), thay vì
// cố định "hôm nay" là 16/10 như bản cũ.
//
// Hóa đơn đi qua đúng đường lưu thật (receipt.Service.Save) nên tồn kho, dự đoán, sổ chi
// tiêu và lịch sử giá đều được tính như khi mẹ quét bill.
package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

type Saver interface {
	Save(ctx context.Context, p authz.Principal, req receipt.SaveRequest) (receipt.SaveOutcome, error)
}

type item struct {
	name, group, category string
	qty                   float64
	unit                  string
	vnd                   int64
}

var (
	sua     = item{"Sữa tươi không đường 1L", "sua-tuoi", "sua", 1, "hộp", 36_000}
	trung   = item{"Trứng gà 10 quả", "trung-ga", "tuoi-song", 1, "vỉ", 35_000}
	thit    = item{"Thịt ba chỉ heo", "thit-heo", "tuoi-song", 0.5, "kg", 75_000}
	rau     = item{"Rau cải ngọt", "rau-cai", "rau-cu", 0.5, "kg", 15_000}
	gao     = item{"Gạo ST25 5kg", "gao", "do-kho", 1, "túi", 185_000}
	mam     = item{"Nước mắm Nam Ngư 750ml", "nuoc-mam", "gia-vi", 1, "chai", 42_000}
	ruaChen = item{"Nước rửa chén Sunlight 750g", "nuoc-rua-chen", "ve-sinh", 1, "chai", 38_000}
	dauGoi  = item{"Dầu gội Clear 630g", "dau-goi", "my-pham", 1, "chai", 159_000}
	banhQuy = item{"Bánh quy Cosy 336g", "banh-quy", "banh-keo", 1, "hộp", 28_000}
	giay    = item{"Giấy vệ sinh Pulppy 10 cuộn", "giay-ve-sinh", "giay", 1, "lốc", 89_000}
	kemRang = item{"Kem đánh răng P/S 230g", "kem-danh-rang", "ve-sinh", 1, "tuýp", 34_000}
	caChua  = item{"Cà chua", "ca-chua", "rau-cu", 0.5, "kg", 18_000}
	chuoi   = item{"Chuối già", "chuoi", "trai-cay", 1, "kg", 25_000}
)

type bill struct {
	daysAgo int
	store   string
	items   []item
	// giá chênh theo cửa hàng, để màn so sánh giá có nơi rẻ hơn
	markup float64
}

// Lịch mua: Sữa tươi mỗi 7 ngày, lần cuối cách 5 ngày, nên còn khoảng 2 ngày. Trứng mỗi
// 8 ngày, lần cuối cách 6 ngày. Dầu gội mỗi 20 ngày, lần cuối cách 15 ngày.
var bills = []bill{
	{35, "Co.opmart", []item{dauGoi, gao, mam, ruaChen}, 1.04},
	{19, "Bách Hóa Xanh", []item{sua, thit, rau, banhQuy}, 1.0},
	{15, "Co.opmart", []item{dauGoi, giay, kemRang}, 1.04},
	{14, "WinMart", []item{trung, thit, rau}, 1.02},
	{12, "Bách Hóa Xanh", []item{sua, banhQuy}, 1.0},
	{6, "WinMart", []item{trung, rau, caChua}, 1.02},
	{5, "Bách Hóa Xanh", []item{sua, thit, chuoi}, 1.0},
}

// Seed lưu các hóa đơn mẫu cho uid. Mã hóa đơn cố định theo uid và thứ tự, nên chạy lại
// cùng ngày không ghi trùng (Save trả kết quả cũ).
func Seed(ctx context.Context, s Saver, uid string, now time.Time) (int, error) {
	if uid == "" {
		return 0, fmt.Errorf("demo: thiếu uid")
	}
	loc := domain.Location("Asia/Ho_Chi_Minh")
	today := domain.Today(now, loc)
	p := authz.Principal{UID: uid, Role: authz.RoleOwner, Timezone: "Asia/Ho_Chi_Minh"}
	h := sha256.Sum256([]byte(uid))
	n := 0
	for i, b := range bills {
		req := receipt.SaveRequest{
			ReceiptID:        fmt.Sprintf("demo-%s-%d", hex.EncodeToString(h[:4]), i+1),
			Store:            b.store,
			PurchasedOn:      today.AddDays(-b.daysAgo).String(),
			Currency:         "VND",
			UseForPrediction: true,
			ConfirmDuplicate: true,
		}
		for j, it := range b.items {
			amount := int64(float64(it.vnd)*b.markup/1000+0.5) * 1000
			req.Lines = append(req.Lines, receipt.SaveLine{
				LineID: fmt.Sprintf("l%d", j+1), Name: it.name, Qty: it.qty, Unit: it.unit,
				Amount: fmt.Sprint(amount), CategoryKey: it.category, GroupKey: it.group, Habit: true,
			})
		}
		if _, err := s.Save(ctx, p, req); err != nil {
			return n, fmt.Errorf("demo bill %d (%s): %w", i+1, b.store, err)
		}
		n++
	}
	return n, nil
}

// CategoryKeys trả các danh mục chi tiêu mà dữ liệu demo dùng (cho test).
func CategoryKeys() []string {
	seen := map[string]bool{}
	var out []string
	for _, b := range bills {
		for _, it := range b.items {
			if !seen[it.category] {
				seen[it.category] = true
				out = append(out, it.category)
			}
		}
	}
	return out
}
