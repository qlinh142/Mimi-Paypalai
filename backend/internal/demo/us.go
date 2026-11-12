package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Bộ dữ liệu demo kiểu Mỹ cho tài khoản ban giám khảo (cuộc thi PayPal): một gia đình ở
// Mỹ, khoảng 3 tháng hóa đơn tiếng Anh, giá bằng USD. Tính lùi từ ngày chạy để khi mở app
// thì Olive oil, Milk, Eggs đang sắp hết như trong video demo.

type usItem struct {
	name  string
	group string
	usd   float64 // giá ở cửa hàng chuẩn
	count float64 // số món trong một gói (trứng 12 quả); 0 là 1
	size  float64 // dung tích hoặc khối lượng một gói
	unit  string  // "ml" hoặc "g"; rỗng khi đếm theo cái
	every int     // chu kỳ mua (ngày)
	last  int     // lần mua gần nhất cách hôm nay bao nhiêu ngày
}

var usItems = []usItem{
	// Sắp hết khi mở app: còn khoảng 1-3 ngày.
	{"Whole Milk, Half Gallon", "sua-tuoi", 3.49, 0, 1890, "ml", 6, 4},
	{"Large Brown Eggs, Dozen", "trung-ga", 4.29, 12, 0, "", 9, 7},
	{"Extra Virgin Olive Oil", "dau-o-liu", 12.99, 0, 1000, "ml", 30, 28},
	{"Ground Coffee, Medium Roast", "ca-phe", 9.99, 0, 340, "g", 18, 16},
	{"Paper Towels, 6 Rolls", "giay-bep", 11.99, 6, 0, "", 21, 19},
	// Mua đều, còn nhiều.
	{"Bananas", "chuoi", 1.69, 0, 1100, "g", 5, 1},
	{"Greek Yogurt, Plain", "sua-chua", 5.49, 0, 907, "g", 8, 4},
	{"Sourdough Bread", "banh-mi", 4.99, 0, 680, "g", 6, 1},
	{"Baby Spinach", "rau-cai", 3.99, 0, 312, "g", 7, 1},
	{"Boneless Chicken Breast", "thit-ga", 11.49, 0, 1130, "g", 8, 4},
	{"Atlantic Salmon Fillet", "ca-hoi", 13.99, 0, 680, "g", 14, 7},
	{"Ground Beef 85% Lean", "thit-xay", 7.99, 0, 680, "g", 12, 4},
	{"Honeycrisp Apples", "tao", 5.99, 0, 1360, "g", 10, 4},
	{"Strawberries", "dau-tay", 4.49, 0, 454, "g", 9, 1},
	{"Roma Tomatoes", "ca-chua", 2.49, 0, 680, "g", 7, 1},
	{"Hass Avocados, Bag", "qua-bo", 5.49, 0, 1130, "g", 11, 7},
	{"Russet Potatoes", "khoai-tay", 4.99, 0, 2270, "g", 20, 10},
	{"Yellow Onions", "hanh-tay", 3.49, 0, 1360, "g", 21, 10},
	{"Shredded Cheddar Cheese", "pho-mai", 4.79, 0, 454, "g", 14, 7},
	{"Salted Butter", "bo-sua", 5.29, 0, 454, "g", 20, 10},
	{"Penne Pasta", "mi-y", 1.89, 0, 454, "g", 14, 10},
	{"Jasmine Rice", "gao", 18.99, 0, 9070, "g", 45, 22},
	{"Old Fashioned Oats", "yen-mach", 5.49, 0, 1190, "g", 28, 13},
	{"Orange Juice, No Pulp", "nuoc-trai-cay", 4.99, 0, 1530, "ml", 7, 1},
	{"Sparkling Water, 12 Pack", "nuoc-khoang", 6.49, 12, 355, "ml", 10, 4},
	{"Tortilla Chips", "snack", 4.29, 0, 368, "g", 12, 4},
	{"Dark Chocolate Bar", "chocolate", 3.49, 0, 100, "g", 15, 7},
	{"Ketchup", "tuong-ca", 3.79, 0, 907, "g", 45, 19},
	{"Soy Sauce", "nuoc-tuong", 3.29, 0, 444, "ml", 50, 25},
	{"Toilet Paper, 12 Mega Rolls", "giay-ve-sinh", 17.99, 12, 0, "", 28, 13},
	{"Laundry Detergent", "nuoc-giat", 13.99, 0, 2720, "ml", 35, 16},
	{"Dish Soap", "nuoc-rua-chen", 3.99, 0, 828, "ml", 25, 10},
	{"Dishwasher Pods", "vien-rua-chen", 14.99, 48, 0, "", 40, 19},
	{"Trash Bags, 13 Gallon", "tui-rac", 12.49, 45, 0, "", 40, 13},
	{"Shampoo", "dau-goi", 8.99, 0, 750, "ml", 40, 22},
	{"Body Wash", "sua-tam", 7.49, 0, 650, "ml", 32, 16},
	{"Toothpaste", "kem-danh-rang", 4.49, 0, 170, "g", 30, 13},
	{"Multivitamin Gummies", "vitamin-tong-hop", 12.99, 90, 0, "", 45, 28},
}

// Cửa hàng và mức giá so với giá chuẩn, để màn so sánh giá có nơi rẻ hơn.
var usStores = []struct {
	name   string
	markup float64
}{
	{"Trader Joe's", 0.97},
	{"Safeway", 1.06},
	{"Target", 1.0},
	{"Whole Foods Market", 1.12},
	{"Costco", 0.88},
	{"Walmart", 0.94},
}

// USDays là số ngày lịch sử của bộ dữ liệu Mỹ.
const USDays = 90

// usTrips: những ngày đi chợ (cách hôm nay bao nhiêu ngày), 3 ngày một lần.
// Món đến hạn mua được dồn vào chuyến gần nhất; các mốc "last" ở trên đều là ngày đi chợ
// nên món sắp hết vẫn đúng số ngày còn lại.
func usTrips() []int {
	set := map[int]bool{}
	for d := 1; d <= USDays; d += 3 { // 1, 4, 7, 10, ...
		set[d] = true
	}
	for _, it := range usItems {
		set[it.last] = true
	}
	var out []int
	for d := range set {
		out = append(out, d)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out))) // cũ nhất trước
	return out
}

func nearest(trips []int, d int) int {
	best := trips[0]
	for _, t := range trips {
		if abs(t-d) < abs(best-d) || abs(t-d) == abs(best-d) && t < best {
			best = t
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// usBills trả các chuyến đi chợ có hàng, cũ nhất trước: ngày (cách hôm nay) và các món.
func usBills() (days []int, lines map[int][]usItem) {
	trips := usTrips()
	lines = map[int][]usItem{}
	for _, it := range usItems {
		for d := it.last; d <= USDays; d += it.every {
			t := it.last
			if d != it.last {
				t = nearest(trips, d)
			}
			lines[t] = append(lines[t], it)
		}
	}
	for _, t := range trips {
		if len(lines[t]) > 0 {
			days = append(days, t)
		}
	}
	return days, lines
}

// SeedUS lưu khoảng 3 tháng hóa đơn kiểu Mỹ cho uid. usdToVND là tỷ giá USD (đồng cho 1
// USD) lấy từ fx_rates: app lưu tiền bằng đồng, người dùng chọn hiển thị USD thì app quy
// đổi lại. Mã hóa đơn cố định theo uid và ngày, nên chạy lại cùng ngày không ghi trùng.
func SeedUS(ctx context.Context, s Saver, uid string, now time.Time, usdToVND string) (int, error) {
	if uid == "" {
		return 0, fmt.Errorf("demo: thiếu uid")
	}
	const tz = "America/Los_Angeles"
	today := domain.Today(now, domain.Location(tz))
	p := authz.Principal{UID: uid, Role: authz.RoleOwner, Timezone: tz}
	h := sha256.Sum256([]byte("us:" + uid))
	days, lines := usBills()
	n := 0
	for i, d := range days {
		st := usStores[(i*5+d)%len(usStores)]
		on := today.AddDays(-d)
		req := receipt.SaveRequest{
			ReceiptID:        fmt.Sprintf("demo-us-%s-%s", hex.EncodeToString(h[:4]), on.String()),
			Store:            st.name,
			PurchasedOn:      on.String(),
			Currency:         "VND",
			UseForPrediction: true,
			ConfirmDuplicate: true,
		}
		for j, it := range lines[d] {
			// Giá dao động nhẹ theo ngày để biểu đồ lịch sử giá có hình.
			wiggle := 1 + float64((d*7+j*3)%9-4)/100
			usd := fmt.Sprintf("%.2f", it.usd*st.markup*wiggle)
			vnd, err := domain.ConvertToVND(usd, usdToVND)
			if err != nil {
				return n, fmt.Errorf("demo: quy đổi %s: %w", usd, err)
			}
			count := it.count
			if count == 0 {
				count = 1
			}
			ln := receipt.SaveLine{
				LineID: fmt.Sprintf("l%d", j+1), Name: it.name, RawName: it.name, Qty: 1,
				Amount: fmt.Sprint(int64(vnd)), CategoryKey: taxonomy.SpendingCategory(it.group),
				GroupKey: it.group, Habit: true, SourceLang: "en",
				PackageCount: count, PackageSize: it.size, PackageUnit: it.unit,
			}
			req.Lines = append(req.Lines, ln)
		}
		if _, err := s.Save(ctx, p, req); err != nil {
			return n, fmt.Errorf("demo bill %s (%s): %w", req.PurchasedOn, st.name, err)
		}
		n++
	}
	return n, nil
}
