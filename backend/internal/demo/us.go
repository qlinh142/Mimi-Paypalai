package demo

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Bộ dữ liệu demo kiểu Mỹ cho tài khoản ban giám khảo (cuộc thi PayPal): 50 hóa đơn giả lập
// anh Linh cung cấp (30 mua tại cửa hàng, 20 mua online qua Channel3 và PayPal sandbox),
// 01/09 đến 11/11/2026, đồ gia dụng, giá USD. Chép từ Bills_50_Combined trong file Excel;
// nhóm hàng và quy cách gói (để vào tồn kho) do mình gán theo tên món.
//
//go:embed us_bills.json
var usBillsJSON []byte

type usBill struct {
	ID     string   `json:"id"`
	On     string   `json:"on"`
	Store  string   `json:"store"`
	Online bool     `json:"online"`
	Lines  []usLine `json:"lines"`
}

type usLine struct {
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	USD   string  `json:"usd"` // tổng tiền dòng
	Group string  `json:"group"`
	Count float64 `json:"count"` // số món trong một gói (bỉm 120 miếng)
	Size  float64 `json:"size"`  // dung tích hoặc khối lượng một gói; 0 khi đếm theo cái
	Unit  string  `json:"unit"`
}

func usBills() ([]usBill, error) {
	var bills []usBill
	if err := json.Unmarshal(usBillsJSON, &bills); err != nil {
		return nil, fmt.Errorf("demo: đọc us_bills.json: %w", err)
	}
	return bills, nil
}

// SeedUS lưu 50 hóa đơn kiểu Mỹ cho uid. usdToVND là tỷ giá USD (đồng cho 1 USD) lấy từ
// fx_rates: app lưu tiền bằng đồng, người dùng chọn hiển thị USD thì app quy đổi lại. Mã
// hóa đơn cố định theo uid, nên chạy lại không ghi trùng.
func SeedUS(ctx context.Context, s Saver, uid string, usdToVND string) (int, error) {
	if uid == "" {
		return 0, fmt.Errorf("demo: thiếu uid")
	}
	bills, err := usBills()
	if err != nil {
		return 0, err
	}
	const tz = "America/Los_Angeles"
	p := authz.Principal{UID: uid, Role: authz.RoleOwner, Timezone: tz}
	h := sha256.Sum256([]byte("us:" + uid))
	n := 0
	for _, b := range bills {
		req := receipt.SaveRequest{
			ReceiptID:        fmt.Sprintf("demo-us-%s-%s", hex.EncodeToString(h[:4]), b.ID),
			Store:            b.Store,
			PurchasedOn:      b.On,
			Currency:         "VND",
			UseForPrediction: true,
			ConfirmDuplicate: true,
		}
		for j, l := range b.Lines {
			vnd, err := domain.ConvertToVND(l.USD, usdToVND)
			if err != nil {
				return n, fmt.Errorf("demo: quy đổi %s: %w", l.USD, err)
			}
			req.Lines = append(req.Lines, receipt.SaveLine{
				LineID: fmt.Sprintf("l%d", j+1), Name: l.Name, RawName: l.Name, Qty: l.Qty,
				Amount: fmt.Sprint(int64(vnd)), CategoryKey: taxonomy.SpendingCategory(l.Group),
				GroupKey: l.Group, Habit: true, SourceLang: "en",
				PackageCount: l.Count, PackageSize: l.Size, PackageUnit: l.Unit,
			})
		}
		if _, err := s.Save(ctx, p, req); err != nil {
			return n, fmt.Errorf("demo bill %s (%s): %w", b.ID, b.Store, err)
		}
		n++
	}
	return n, nil
}
