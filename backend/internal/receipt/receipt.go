// Package receipt là luồng lõi của Mimi: đọc hóa đơn (Gemini) và lưu hóa đơn
// (transaction Firestore). "Chỉ hóa đơn mới đổi dữ liệu": đây là đường ghi duy nhất
// làm thay đổi tồn kho và chi tiêu.
//
// Các interface ở file này được định nghĩa ở phía dùng (package receipt), adapter
// Firestore và Gemini trong internal/gcp hiện thực chúng.
package receipt

import (
	"context"
	"errors"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Image là một ảnh hóa đơn đã nén ở máy người dùng. Chỉ nằm trong bộ nhớ request.
type Image struct {
	Data []byte
	MIME string // image/jpeg | image/png | image/webp
}

// PageLine là một dòng do AI đọc. Amount giữ nguyên dạng thập phân in trên hóa đơn
// ("64000", "12.50"); AI không tính tiền, code quy đổi.
//
// AI chuẩn hóa tên đúng một lần ở bước này (tài liệu 1.1): RawName chép nguyên chữ in
// trên bill, Brand / ProductName / Package* là các field đã tách. Code áp quy tắc viết
// hoa (7.1) và tính quy cách (0.1); đổi ngôn ngữ giao diện không bao giờ gọi lại AI.
type PageLine struct {
	RawName      string
	Brand        string
	ProductName  string  // tên riêng / variant, AI đã bỏ marketing text
	PackageCount float64 // số gói theo cấu trúc combo, 0 nếu AI không thấy
	PackageSize  float64 // dung tích / khối lượng một gói, 0 nếu không in
	PackageUnit  string
	SourceLang   string // ngôn ngữ in trên bill: vi | en | ...
	Translated   string // tên dịch sang ngôn ngữ app của người quét; "" khi bill cùng ngôn ngữ
	Qty          float64
	Unit         string // đơn vị in cạnh số lượng ("kg", "hộp", "gói"...), "" nếu không có
	Amount       string
	Discount     string // "" nếu không có
	CategoryKey  string // khóa trong taxonomy 27 danh mục chi tiêu, "" nếu chưa chọn được
	GroupKey     string // group_key của taxonomy 0.2, "" hoặc "khac" nếu không nhóm nào hợp
}

// Page là kết quả đọc một ảnh.
type Page struct {
	Store         string
	PurchasedOn   string // YYYY-MM-DD hoặc ""
	Currency      string // ISO 4217, mặc định VND
	TotalPrinted  string // tổng in trên hóa đơn, "" nếu không có
	DiscountTotal string
	Lines         []PageLine
}

// ErrUnreadable: ảnh không phải hóa đơn, quá mờ, hoặc AI trả sai schema. Không retry.
var ErrUnreadable = errors.New("receipt: page unreadable")

// ErrNoItems: ảnh đọc được nhưng không thấy dòng món nào (BUG-05 mã no_items).
var ErrNoItems = errors.New("receipt: no items on page")

// ErrAIRejected: dịch vụ AI từ chối yêu cầu (sai cấu hình, thiếu quyền, model không có).
// Không phải lỗi của ảnh nên không bảo người dùng chụp lại, và không retry.
var ErrAIRejected = errors.New("receipt: AI request rejected")

// ErrTransient: AI quá tải hoặc lỗi tạm thời. Có thể retry trong ngân sách thời gian.
var ErrTransient = errors.New("receipt: transient AI failure")

// Reader đọc một trang. Adapter Gemini phải dùng structured output và trả ErrUnreadable,
// ErrNoItems hoặc ErrTransient (bọc bằng %w) cho các loại lỗi trên. lang chỉ là ngôn ngữ
// của câu trả lời phụ (không dùng cho tên món): tên món luôn giữ theo bill.
type Reader interface {
	ReadPage(ctx context.Context, img Image, lang string) (Page, error)
}

// PageCache giữ kết quả đọc từng ảnh theo hash nội dung trong thời gian ngắn, để khi
// một ảnh lỗi người dùng chỉ chụp lại ảnh đó: các ảnh đã đọc được không gọi AI lần nữa
// (BUG-05 mục 4, 7). Không lưu ảnh, chỉ lưu kết quả đọc.
type PageCache interface {
	GetPage(ctx context.Context, uid, hash string) (*Page, error) // (nil, nil) nếu không có
	PutPage(ctx context.Context, uid, hash string, p Page) error
}

// DraftLine là một dòng trong bản nháp gửi về màn Xác nhận.
type DraftLine struct {
	LineID      string      `json:"line_id"`
	Name        string      `json:"name"` // tên hiển thị đã chuẩn hóa (brand + tên riêng), người dùng sửa được
	Qty         float64     `json:"qty"`
	Unit        string      `json:"unit,omitempty"` // khóa đơn vị in cạnh số lượng: kg, hop, goi...
	Amount      string      `json:"amount"`
	AmountVND   *domain.VND `json:"amount_vnd"` // nil khi chưa quy đổi được; âm với dòng khuyến mãi
	Discount    bool        `json:"discount,omitempty"`
	CategoryKey string      `json:"category_key,omitempty"`
	GroupKey    string      `json:"group_key,omitempty"`
	Habit       bool        `json:"habit"`
	Product
	// NeedsReview: dòng vẫn lưu được nhưng chưa dùng cho tồn kho / dự đoán cho tới khi đủ
	// phân loại (6.1 mục 5.3, 0.1 mục 20). ReviewReasons: missing_category | missing_group |
	// unknown_quantity | count_for_measured_group.
	NeedsReview   bool     `json:"needs_review,omitempty"`
	ReviewReasons []string `json:"review_reasons,omitempty"`
}

// Product là phần chuẩn hóa sản phẩm của một dòng (0.1 mục 5.1, 1.1, 7.1). Lưu canonical,
// không lưu nhãn đã dịch: nhãn nhóm tra từ taxonomy theo ngôn ngữ lúc hiển thị.
type Product struct {
	RawName     string `json:"raw_name,omitempty"`
	Brand       string `json:"brand,omitempty"`
	ProductName string `json:"product_name,omitempty"`
	DetailName  string `json:"detail_name,omitempty"`
	ProductKey  string `json:"product_key,omitempty"`
	SourceLang  string `json:"source_language,omitempty"`
	// OriginalName: tên theo bill (đã chuẩn hóa) khi Name là tên đã dịch sang ngôn ngữ app;
	// rỗng khi bill cùng ngôn ngữ. App hiện Name to, OriginalName nhỏ bên dưới để đối chiếu.
	OriginalName string  `json:"original_name,omitempty"`
	PackageCount float64 `json:"package_count,omitempty"`
	PackageSize  float64 `json:"package_size,omitempty"`
	PackageUnit  string  `json:"package_unit,omitempty"`
	// Canonical là tổng lượng sau khi nhân combo và số lượng mua; 0 khi chưa đọc được.
	CanonicalQty  float64 `json:"canonical_quantity,omitempty"`
	CanonicalUnit string  `json:"canonical_unit,omitempty"` // g | ml | m | item
}

type FX struct {
	Rate   string    `json:"rate"`
	Source string    `json:"source"`
	At     time.Time `json:"at"`
}

// Draft không chứa ảnh. Được lưu tạm (TTL 1 giờ) theo receipt_id để gửi lại không gọi Gemini lần nữa.
type Draft struct {
	ReceiptID     string      `json:"receipt_id"`
	Store         string      `json:"store,omitempty"`
	PurchasedOn   string      `json:"purchased_on,omitempty"`
	Currency      string      `json:"currency"`
	TotalPrinted  string      `json:"total_printed,omitempty"`
	DiscountTotal string      `json:"discount_total,omitempty"`
	Lines         []DraftLine `json:"lines"`
	SumVND        domain.VND  `json:"sum_vnd"`
	TotalMismatch bool        `json:"total_mismatch"`
	// TotalMatch theo BUG-06 mục 5.2: null khi hóa đơn không in tổng, false khi lệch (chỉ là
	// cảnh báo, không chặn Lưu).
	TotalMatch      *bool `json:"total_match"`
	FX              *FX   `json:"fx,omitempty"`
	FXMissing       bool  `json:"fx_missing"`
	Pages           int   `json:"pages"`
	UnreadablePages int   `json:"unreadable_pages"` // luôn 0 khi trả về: ảnh lỗi trả mã unreadable_image
}

type DraftCache interface {
	Get(ctx context.Context, uid, receiptID string) (*Draft, error) // (nil, nil) nếu không có
	Put(ctx context.Context, uid string, d Draft) error
}

// Quota: tối đa N lần đọc mỗi ngày (theo giờ người dùng). Reserve trước khi gọi AI,
// Release nếu đọc thất bại để lỗi của hệ thống không trừ lượt của người dùng.
type Quota interface {
	Reserve(ctx context.Context, uid, day string, limit int) error
	Release(ctx context.Context, uid, day string) error
}

// FXRates trả tỷ giá có nguồn cho một ngày; ok=false nếu chưa có. Không bao giờ tự đoán.
type FXRates interface {
	Rate(ctx context.Context, currency string, on domain.Date) (fx FX, ok bool, err error)
}

// Line là dòng hóa đơn đã lưu.
type Line struct {
	ID       string
	Name     string
	NameNorm string
	Qty      float64
	Unit     string // khóa đơn vị in cạnh số lượng đã chuẩn hóa, "" nếu hóa đơn không in
	Orig     string // số gốc in trên hóa đơn
	VND      *domain.VND
	// DiscountVND: phần giảm giá của cả hóa đơn chia về dòng này theo tỷ lệ tiền (0 khi hóa
	// đơn không có dòng khuyến mãi). Giá sau giảm của dòng = VND - DiscountVND.
	DiscountVND domain.VND
	CategoryKey string
	GroupKey    string
	Habit       bool
	Product
	Structure   string // quy cách đóng gói đã đọc ("2 x 900g")
	NeedsReview bool
	// InventoryOwner: uid nhận hàng vào tồn kho. Rỗng là người lưu hóa đơn; khác người lưu
	// khi dòng khớp một vòng mua hộ (4.1 mục 7): payer vẫn là người lưu hóa đơn.
	InventoryOwner string
	// B3: dòng trừ vào những Kế hoạch nào của người lưu, theo thứ tự, mỗi Kế hoạch bao
	// nhiêu (theo Qty của dòng). PlanSet false: dùng SourcePlanID của hóa đơn như trước.
	// PlanSet true với Plans rỗng: người dùng chọn "không trừ vào kế hoạch nào".
	Plans   []PlanShare
	PlanSet bool
	// PriceSource (B7): giá của dòng lấy từ đâu. Xem các hằng Price*.
	PriceSource string
}

// Nguồn giá của một dòng hóa đơn (B7). PriceOnline để dành cho giá quét online sau này
// (bước 13): kiểu dữ liệu đã có chỗ, máy chủ chưa nhận giá trị này từ app.
const (
	PriceReceipt  = "receipt"  // in trên hóa đơn, hoặc người dùng sửa trên màn Xác nhận
	PriceManual   = "manual"   // người đi mua tự nhập khi không có hóa đơn
	PriceEstimate = "estimate" // không nhập, lấy giá mua gần nhất; app ghi "Ước tính"
	PriceMissing  = "missing"  // chưa có giá: không tính vào tổng, nhắc nhập sau
	PriceOnline   = "online"   // để dành
)

// PlanShare là phần của một dòng hóa đơn trừ vào một Kế hoạch.
type PlanShare struct {
	PlanID string  `json:"plan_id"`
	Qty    float64 `json:"qty"`
}

// ForInventory báo dòng được cộng vào tồn kho và dự đoán.
func (l Line) ForInventory() bool {
	return l.Habit && l.GroupKey != "" && !l.NeedsReview && l.CanonicalQty > 0
}

// Receipt là hóa đơn đã xác nhận, sẵn sàng ghi trong một transaction.
type Receipt struct {
	ID          string
	Store       string
	PurchasedOn domain.Date
	Currency    string
	FX          *FX
	Lines       []Line
	TotalVND    domain.VND // chỉ cộng các dòng đã quy đổi
	DiscountVND domain.VND
	DedupeKey   string // cửa hàng + ngày + tổng, để cảnh báo trùng
	PayloadHash string // để phân biệt "gửi lại" với "receipt_id trùng nhưng nội dung khác"
	SavedAt     time.Time
	StoreKey    string // khóa cửa hàng đã chuẩn hóa để so giá (8.1 mục 15)
	// SourcePlanID: Plan đang đối chiếu (3.1 mục 3A.6); rỗng là Plan mặc định.
	SourcePlanID string
	// AssignmentID: vòng mua hộ mà người lưu là Buyer (4.1). Rỗng: hóa đơn cá nhân.
	AssignmentID string
}

type SaveOutcome struct {
	ReceiptID string
	Replayed  bool // cùng receipt_id, cùng nội dung: trả kết quả cũ
	// PlanDone: các dòng Kế hoạch (của Plan đang đối chiếu) chuyển done nhờ hóa đơn này.
	PlanDone []string
	// Assignment: kết quả vòng mua hộ khi người lưu là Buyer (4.1 mục 10).
	Assignment *AssignmentResult
	// Recurring: các kỳ thanh toán định kỳ đã khớp hóa đơn này (2.1 Rule R3), không cộng
	// chi tiêu lần hai.
	Recurring []string
	// Split: món chưa mua của Kế hoạch vừa chốt đã chuyển sang Kế hoạch mới "· còn lại" (B6).
	Split *SplitResult
}

// SplitResult: Kế hoạch mới tạo từ món chưa mua.
type SplitResult struct {
	FromPlanID string `json:"from_plan_id"`
	PlanID     string `json:"plan_id"`
	Name       string `json:"name"`
	Moved      int    `json:"moved"` // số món đã chuyển
}

// AssignmentResult tóm tắt một vòng mua hộ sau khi Buyer xác nhận hóa đơn.
type AssignmentResult struct {
	ID       string   `json:"assignment_id"`
	Status   string   `json:"status"`   // completed | closed_partial
	Done     []string `json:"done"`     // group_key mua được, vào tồn kho của Owner
	Returned []string `json:"returned"` // group_key chưa mua, trả về Plan của Owner
}

// Store ghi hóa đơn và mọi dữ liệu suy ra trong MỘT transaction: receipt, purchases,
// items của các nhóm bị ảnh hưởng (tính lại bằng domain.Predict), months, plan_items
// chuyển done, asks khớp nhóm đóng lại, receipt_count tăng.
type Store interface {
	SaveReceipt(ctx context.Context, uid string, r Receipt, tz *time.Location) (SaveOutcome, error)
	HasSimilar(ctx context.Context, uid, dedupeKey, exceptID string) (bool, error)
}
