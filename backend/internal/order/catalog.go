package order

import "time"

// Product là một sản phẩm của merchant Sandbox kèm Price Snapshot (Mimi 2.0 mục 11, 12).
// Thị trường Mỹ (mẹ chốt 03/11): tên tiếng Anh, giá USD tính bằng cent cho một gói
// (PackQty theo Unit). SnapshotCents là giá tốt nhất ở lần quét gần nhất; OfferCents là giá
// merchant đang bán lúc checkout, có thể đã khác snapshot. Giá VND chỉ quy ra lúc dựng giỏ,
// theo tỷ giá của ngày đặt hàng, để so với Mandate và ghi Sổ chi tiêu.
type Product struct {
	Key           string // group_key trong taxonomy 300 nhóm
	Name          string
	Unit          string // đơn vị của gói: kg, l, hop, goi, qua...
	PackQty       float64
	SnapshotCents int64
	OfferCents    int64
	EtaDays       int
	Category      string // grocery | household
}

// Catalog là nguồn product/price cho Mimi Online. Bản hackathon dùng merchant Sandbox
// do team kiểm soát (Mimi 2.0, "Demo architecture an toàn"); Channel3 sẽ thay phần dữ liệu.
type Catalog interface {
	Merchant() string
	Product(groupKey string) (Product, bool)
	SnapshotAt() time.Time
}

// Danh mục được phép trong Household Mandate.
const (
	CategoryGrocery   = "grocery"
	CategoryHousehold = "household"
)

// SandboxCatalog: dữ liệu mẫu cho merchant "Mimi Mart Sandbox (US)". Khóa là group_key
// của taxonomy 300 nhóm (lỗi N4: trước đây dùng 7 mã cũ không có trong taxonomy nên các
// món đó luôn báo "cửa hàng không bán"). Giá là dữ liệu mẫu, không phải giá thị trường.
// "dau-an" có giá đang bán cao hơn snapshot 20% để demo nhánh ASK của price guard.
type SandboxCatalog struct {
	products map[string]Product
	at       time.Time
}

func NewSandboxCatalog(snapshotAt time.Time) *SandboxCatalog {
	ps := []Product{
		{"sua-tuoi", "Whole Milk, 1 gal", "l", 3.78, 389, 389, 1, CategoryGrocery},
		{"sua-chua", "Greek Yogurt, 4 x 5.3 oz", "loc", 1, 549, 529, 1, CategoryGrocery},
		{"trung", "Large White Eggs, 12 ct", "qua", 12, 399, 399, 1, CategoryGrocery},
		{"gao", "Jasmine Rice, 5 lb", "kg", 2.27, 899, 849, 2, CategoryGrocery},
		{"mi-an-lien", "Instant Ramen Noodles, 12 pack", "goi", 12, 499, 499, 1, CategoryGrocery},
		{"dau-an", "Vegetable Oil, 48 fl oz", "l", 1.42, 449, 539, 2, CategoryGrocery},
		{"nuoc-mam", "Fish Sauce, 16.9 fl oz", "chai", 1, 499, 499, 2, CategoryGrocery},
		{"nuoc-giat", "Liquid Laundry Detergent, 92 fl oz", "chai", 1, 1399, 1299, 2, CategoryHousehold},
		{"nuoc-rua-chen", "Dish Soap, 28 fl oz", "chai", 1, 399, 399, 2, CategoryHousehold},
		{"giay-ve-sinh", "Toilet Paper, 12 Mega Rolls", "loc", 1, 1699, 1599, 2, CategoryHousehold},
		{"sua-tam", "Body Wash, 30 fl oz", "chai", 1, 899, 899, 2, CategoryHousehold},
		{"rau-cai", "Baby Bok Choy, 1 lb", "kg", 0.45, 299, 319, 1, CategoryGrocery},
		{"thit-bo", "Ground Beef 80/20, 1 lb", "kg", 0.45, 599, 599, 1, CategoryGrocery},
		{"nuoc-loc", "Spring Water, 24 x 16.9 fl oz", "thung", 1, 599, 599, 1, CategoryGrocery},
		{"banh-quy", "Butter Cookies, 12 oz tin", "hop", 1, 549, 499, 1, CategoryGrocery},
	}
	m := make(map[string]Product, len(ps))
	for _, p := range ps {
		m[p.Key] = p
	}
	return &SandboxCatalog{products: m, at: snapshotAt}
}

func (c *SandboxCatalog) Merchant() string { return "Mimi Mart Sandbox (US)" }

func (c *SandboxCatalog) Product(k string) (Product, bool) {
	p, ok := c.products[k]
	return p, ok
}

func (c *SandboxCatalog) SnapshotAt() time.Time { return c.at }

// keys trả mọi khóa sản phẩm (cho test đối chiếu taxonomy).
func (c *SandboxCatalog) keys() []string {
	out := make([]string, 0, len(c.products))
	for k := range c.products {
		out = append(out, k)
	}
	return out
}
