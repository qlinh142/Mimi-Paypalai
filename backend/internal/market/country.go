package market

import (
	"context"
	"strings"
)

// Country là một thị trường có bảng giá chung riêng (mẹ chốt 02/11: quét giá theo nước).
// Giá luôn lưu kèm số tiền gốc (Local, Currency) và quy ra VND (PriceVND) để so được với giá
// trên hóa đơn, vốn tính bằng VND.
type Country struct {
	Code     string // VN, US, SG
	Name     string // tên tiếng Anh, đưa vào câu hỏi AI
	Currency string // VND, USD, SGD
	// Giá vô lý theo tiền gốc: bỏ.
	MinLocal, MaxLocal float64
	// Shops: sổ nguồn ban đầu (tên cửa hàng, web). Web mới vào sổ khi ra giá hợp lệ 2 lần.
	Shops []Shop
}

// Shop là một nơi bán trong sổ nguồn ban đầu. Link ngành hàng, link tìm kiếm do AI tìm ra
// và được ghi vào sổ nguồn (SourceBook) theo thời gian, không viết tay ở đây.
type Shop struct {
	Name   string
	Domain string // để trống khi chưa chắc web chính thức; AI tự tìm và ghi vào sổ
}

var Countries = map[string]Country{
	"VN": {Code: "VN", Name: "Vietnam", Currency: "VND", MinLocal: 1_000, MaxLocal: 50_000_000, Shops: []Shop{
		{"WinMart", "winmart.vn"}, {"Co.opmart", "cooponline.vn"}, {"Bách Hóa Xanh", "bachhoaxanh.com"},
		{"GO!", ""}, {"AEON", "aeoneshop.com"}, {"Lotte Mart", ""}, {"MM Mega Market", ""},
	}},
	// Mẹ chốt 03/11: Cài đặt cho chọn Việt Nam và Mỹ (Selectable). Singapore vẫn giữ để người
	// đã chọn trước đó dùng tiếp và mở lại khi cần; thêm nước mới chỉ cần thêm một dòng ở đây.
	"US": {Code: "US", Name: "United States", Currency: "USD", MinLocal: 0.1, MaxLocal: 20_000, Shops: []Shop{
		{"Walmart", "walmart.com"}, {"Target", "target.com"}, {"Costco", "costco.com"},
		{"Kroger", "kroger.com"}, {"Whole Foods", "wholefoodsmarket.com"},
	}},
	"SG": {Code: "SG", Name: "Singapore", Currency: "SGD", MinLocal: 0.1, MaxLocal: 20_000, Shops: []Shop{
		{"FairPrice", "fairprice.com.sg"}, {"Cold Storage", "coldstorage.com.sg"}, {"Sheng Siong", "shengsiong.com.sg"},
		{"RedMart", "redmart.lazada.sg"}, {"Giant", "giant.sg"},
	}},
}

// Selectable: các nước hiện ra trong Cài đặt, theo thứ tự (mẹ chốt 03/11: Việt Nam, Mỹ).
var Selectable = []string{"VN", "US"}

// DefaultCountry khi chưa chọn và không đoán được.
const DefaultCountry = "VN"

// zoneCountry: múi giờ của nước khác Việt Nam, để đoán nước khi chưa chọn trong Cài đặt.
var zoneCountry = map[string]string{
	"America/New_York": "US", "America/Chicago": "US", "America/Denver": "US", "America/Phoenix": "US",
	"America/Los_Angeles": "US", "America/Anchorage": "US", "Pacific/Honolulu": "US",
	"Asia/Singapore": "SG",
}

// CountryFor: nước trong Cài đặt nếu hợp lệ; chưa chọn thì đoán theo múi giờ.
func CountryFor(setting, timezone string) string {
	if c := strings.ToUpper(strings.TrimSpace(setting)); Countries[c].Code != "" {
		return c
	}
	if c := zoneCountry[timezone]; c != "" {
		return c
	}
	return DefaultCountry
}

// ItemID là khóa một dòng bảng giá chung: nước + tên món chuẩn hóa ("vn_sua-tuoi").
func ItemID(country, name string) string {
	k := Key(name)
	if k == "" {
		return ""
	}
	return strings.ToLower(CountryFor(country, "")) + "_" + k
}

// FX quy tiền gốc ra VND. ok=false: chưa có tỷ giá, bỏ giá đó (không đoán tỷ giá).
type FX interface {
	VNDPer(ctx context.Context, currency, day string) (rate float64, ok bool, err error)
}

// Shopee, Lazada có nhiều người bán cùng món: chỉ nhận gian hàng chính hãng (mẹ chốt 02/11).
var marketplaces = []string{"shopee.", "lazada."}

func isMarketplace(url string) bool {
	u := strings.ToLower(url)
	for _, m := range marketplaces {
		if strings.Contains(u, m) {
			return true
		}
	}
	return false
}

func isOfficialStore(name string) bool {
	n := strings.ToLower(name)
	for _, w := range []string{"mall", "official", "chính hãng", "redmart"} {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

// Domain lấy tên miền từ link (bỏ www.), để ghi sổ nguồn.
func Domain(url string) string {
	u := strings.ToLower(strings.TrimSpace(url))
	for _, p := range []string{"https://", "http://"} {
		u = strings.TrimPrefix(u, p)
	}
	if i := strings.IndexAny(u, "/?#"); i >= 0 {
		u = u[:i]
	}
	return strings.TrimPrefix(u, "www.")
}
