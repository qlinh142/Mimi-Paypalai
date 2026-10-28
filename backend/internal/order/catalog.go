package order

import "time"

// Product là một sản phẩm của merchant Sandbox kèm Price Snapshot (Mimi 2.0 mục 11, 12).
// Giá tính theo một gói (PackQty theo Unit). SnapshotVND là giá tốt nhất ở lần quét gần
// nhất; OfferVND là giá merchant đang bán lúc checkout, có thể đã khác snapshot.
type Product struct {
	Key         string // group_key của nhóm tiêu dùng
	Name        string
	Unit        string // đơn vị của gói: kg, l, hop, goi, qua...
	PackQty     float64
	SnapshotVND int64
	OfferVND    int64
	EtaDays     int
	Category    string // grocery | household
}

// Catalog là nguồn product/price cho Mimi Online. Bản hackathon dùng merchant Sandbox
// do team kiểm soát (Mimi 2.0, "Demo architecture an toàn").
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

// SandboxCatalog: dữ liệu mẫu cho merchant "Mimi Mart Sandbox", khóa trùng 16 nhóm demo.
// Giá là dữ liệu mẫu, không phải giá thị trường. "dau-an" có giá đang bán cao hơn
// snapshot 20% để demo nhánh ASK của price guard.
type SandboxCatalog struct {
	products map[string]Product
	at       time.Time
}

func NewSandboxCatalog(snapshotAt time.Time) *SandboxCatalog {
	ps := []Product{
		{"sua-tuoi", "Sữa tươi tiệt trùng 1L", "hop", 1, 32_000, 32_000, 1, CategoryGrocery},
		{"sua-chua", "Sữa chua ăn (lốc 4)", "loc", 1, 26_000, 25_000, 1, CategoryGrocery},
		{"trung", "Trứng gà (vỉ 10)", "qua", 10, 34_000, 34_000, 1, CategoryGrocery},
		{"gao", "Gạo ST25 túi 5kg", "kg", 5, 165_000, 160_000, 2, CategoryGrocery},
		{"mi-goi", "Mì gói (gói)", "goi", 1, 4_500, 4_500, 1, CategoryGrocery},
		{"dau-an", "Dầu ăn 1L", "l", 1, 55_000, 66_000, 2, CategoryGrocery},
		{"nuoc-mam-gia-vi", "Nước mắm 500ml", "chai", 1, 42_000, 42_000, 2, CategoryGrocery},
		{"nuoc-giat", "Nước giặt 3,5kg", "chai", 1, 189_000, 179_000, 2, CategoryHousehold},
		{"nuoc-rua-chen", "Nước rửa chén 750g", "chai", 1, 38_000, 38_000, 2, CategoryHousehold},
		{"giay-ve-sinh", "Giấy vệ sinh (lốc 10 cuộn)", "loc", 1, 98_000, 95_000, 2, CategoryHousehold},
		{"dau-goi-sua-tam", "Sữa tắm 900g", "chai", 1, 145_000, 145_000, 2, CategoryHousehold},
		{"rau-cu-qua", "Rau củ tươi (kg)", "kg", 1, 40_000, 42_000, 1, CategoryGrocery},
		{"thit-ca", "Thịt bò (kg)", "kg", 1, 280_000, 280_000, 1, CategoryGrocery},
		{"do-uong", "Nước suối 1,5L", "chai", 1, 9_000, 9_000, 1, CategoryGrocery},
		{"banh-keo", "Bánh quy hộp", "hop", 1, 55_000, 49_000, 1, CategoryGrocery},
	}
	m := make(map[string]Product, len(ps))
	for _, p := range ps {
		m[p.Key] = p
	}
	return &SandboxCatalog{products: m, at: snapshotAt}
}

func (c *SandboxCatalog) Merchant() string { return "Mimi Mart Sandbox" }

func (c *SandboxCatalog) Product(k string) (Product, bool) {
	p, ok := c.products[k]
	return p, ok
}

func (c *SandboxCatalog) SnapshotAt() time.Time { return c.at }
