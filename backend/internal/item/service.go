// Package item phục vụ Đồ dùng, Cần mua, Chi tiết món (spec mục 4, 5).
// Trạng thái và số ngày còn lại luôn được tính lúc đọc từ ngày dự kiến hết đã lưu,
// nên không bao giờ "cũ" theo thời gian.
package item

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Record là tài liệu users/{uid}/items/{group_key} đã lưu.
type Record struct {
	GroupKey      string
	Name          string
	NameNorm      string
	CategoryKey   string
	Prediction    domain.Prediction
	Override      *domain.Override
	LastPurchase  domain.Date
	Dropped       bool
	AskLaterUntil *domain.Date
	// Tồn theo số lượng (Mimi 2.0 mục 4): Baseline là Confirmed Inventory, RatePerDay là
	// tốc độ dùng ước tính, Consumed là các lần tiêu thụ đã ghi (bữa ăn đã qua).
	Baseline   domain.Baseline
	RatePerDay float64
	Consumed   []domain.Consumption
	// Extra là các consumption stream khác chiều đo của cùng nhóm (0.1 mục 18, 20): "Sữa
	// tắm" có lot 800 g và lot 900 ml thì g và ml là hai stream, không cộng thành một tổng.
	Extra map[domain.Unit]Stream
}

// Stream là tồn của một chiều đo: baseline có bằng chứng và tốc độ dùng ước tính.
type Stream struct {
	Baseline   domain.Baseline
	RatePerDay float64
}

// Store đọc ghi items. Mọi hàm nhận uid của Principal, không nhận uid từ request.
type Store interface {
	ListItems(ctx context.Context, uid string) ([]Record, error)
	GetItem(ctx context.Context, uid, groupKey string) (Record, error) // apperr.ErrNotFound nếu không có
	// SetOverride ghi bản sửa tay trong transaction: chỉ ghi nếu ov.SetAt mới hơn bản đang
	// có (lần bấm sau thắng), rồi tính lại Prediction bằng domain.Predict.
	SetOverride(ctx context.Context, uid, groupKey string, ov domain.Override) (Record, error)
	ClearOverride(ctx context.Context, uid, groupKey string, setAt time.Time) (Record, error)
	// SetBaseline ghi mốc tồn mới do người dùng xác nhận; không sửa lịch sử mua.
	SetBaseline(ctx context.Context, uid, groupKey string, b domain.Baseline) (Record, error)
	// OwnerOf trả uid chủ nhà để thành viên đọc danh sách dùng chung.
	OwnerOf(ctx context.Context, householdID string) (string, error)
	// History trả các lần mua thói quen gần nhất của một nhóm, mới nhất trước
	// (truy vấn purchases theo index group_key + habit + purchased_on).
	History(ctx context.Context, uid, groupKey string, limit int) ([]HistoryEntry, error)
}

// HistoryEntry là một lần mua trong Chi tiết món. Chỉ chủ giỏ đọc được (có giá).
type HistoryEntry struct {
	On        string `json:"on"` // YYYY-MM-DD
	Store     string `json:"store,omitempty"`
	AmountVND *int64 `json:"amount_vnd"` // nil khi dòng chưa quy đổi được sang VND
	ReceiptID string `json:"receipt_id,omitempty"`
}

// Detail là phản hồi GET /items/{key}: mọi trường của Item kèm lịch sử mua.
type Detail struct {
	Item
	History []HistoryEntry `json:"history"`
	// Lots: các lần mua (purchase lot) với phần còn lại chia theo FIFO từ tồn ước tính của
	// stream cùng chiều đo (0.1 mục 11 đến 13). Rỗng khi Store chưa đọc được lot.
	Lots []domain.Lot `json:"lots"`
}

// LotReader là phần tùy chọn của Store: đọc purchase lot của một nhóm, mới nhất trước.
type LotReader interface {
	Lots(ctx context.Context, uid, key string, limit int) ([]domain.Lot, error)
}

// lotLimit: số lot đọc để chia FIFO. Mọi lần mua đều được lưu, nên đọc đủ lịch sử để
// tồn kho không bị mất phần nằm ở lot cũ (lỗi V3). lotShow: số lot đã dùng hết vẫn hiện
// (mới nhất trước); lot còn hàng luôn hiện.
const (
	lotLimit = 500
	lotShow  = 20
)

// historyLimit: số lần mua hiển thị trong Chi tiết món.
const historyLimit = 10

// NeedsHorizonDays là khoảng ngày mặc định của "Cần mua" và số trên ô Trang chủ.
const NeedsHorizonDays = 7

// Item là dữ liệu đầy đủ cho chủ giỏ.
type Item struct {
	Key           string        `json:"key"`
	Name          string        `json:"name"`
	CategoryKey   string        `json:"category_key"`
	Status        domain.Status `json:"status"`
	DaysLeft      *int          `json:"days_left"`
	OutOn         string        `json:"predicted_out_on,omitempty"`
	IntervalDays  int           `json:"interval_days,omitempty"`
	PurchaseCount int           `json:"purchase_count"`
	Confidence    string        `json:"confidence,omitempty"`
	Source        string        `json:"source"`
	Stock         *Stock        `json:"stock,omitempty"` // nil khi chưa có số lượng nào
}

// Stock tách Confirmed Inventory (có bằng chứng) và Estimated Inventory (Mimi ước tính).
type Stock struct {
	Estimated       float64 `json:"estimated"`
	Unit            string  `json:"unit"` // g | ml | pc
	Confirmed       float64 `json:"confirmed"`
	ConfirmedOn     string  `json:"confirmed_on"`
	ConfirmedSource string  `json:"confirmed_source"` // receipt | delivery | manual
	RatePerDay      float64 `json:"rate_per_day,omitempty"`
	DaysOfStock     *int    `json:"days_of_stock,omitempty"`
	// Parts: khi nhóm có lot khác chiều đo (800 g và 900 ml), mỗi chiều một dòng; UI không
	// được cộng thành một tổng giả (0.1 mục 20). Rỗng khi chỉ có một chiều.
	Parts []StockPart `json:"parts,omitempty"`
}

// StockPart là tồn của một chiều đo phụ.
type StockPart struct {
	Estimated   float64 `json:"estimated"`
	Unit        string  `json:"unit"`
	Confirmed   float64 `json:"confirmed"`
	ConfirmedOn string  `json:"confirmed_on"`
	DaysOfStock *int    `json:"days_of_stock,omitempty"`
}

// SharedItem là kiểu riêng cho thành viên nhà: KHÔNG có trường giá hay chi tiêu nào,
// nên quên lọc cũng không thể làm lộ giá (spec mục 2, 10).
type SharedItem struct {
	Key      string        `json:"key"`
	Name     string        `json:"name"`
	Status   domain.Status `json:"status"`
	DaysLeft *int          `json:"days_left"`
}

type Query struct {
	Tab     string // "soon" | "active"
	Horizon int    // 3 | 7 | 14
	Search  string
}

func (q Query) Validate() error {
	if q.Tab != "soon" && q.Tab != "active" {
		return apperr.New(apperr.CodeValidation, "Tab không hợp lệ.").With("field", "tab")
	}
	if q.Horizon != 3 && q.Horizon != 7 && q.Horizon != 14 {
		return apperr.New(apperr.CodeValidation, "Khoảng ngày chỉ là 3, 7 hoặc 14.").With("field", "horizon")
	}
	if len(q.Search) > 80 {
		return apperr.New(apperr.CodeValidation, "Từ khóa quá dài.").With("field", "q")
	}
	return nil
}

type Service struct {
	store Store
	now   func() time.Time
}

func New(store Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: now}
}

// List trả danh sách cho chủ giỏ.
func (s *Service) List(ctx context.Context, p authz.Principal, q Query) ([]Item, error) {
	if err := authz.Can(p, authz.ActionReadItemsFull); err != nil {
		return nil, err
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	recs, err := s.store.ListItems(ctx, p.UID)
	if err != nil {
		return nil, err
	}
	today := domain.Today(s.now(), loc(p))
	needle := domain.NormalizeVi(q.Search)
	out := make([]Item, 0, len(recs))
	for _, r := range recs {
		if r.Dropped || (needle != "" && !strings.Contains(r.NameNorm, needle)) {
			continue
		}
		v := domain.ViewAt(r.Prediction, today, lead(p))
		if q.Tab == "soon" && !domain.InNeeds(v, q.Horizon) {
			continue
		}
		out = append(out, toItem(r, v, today))
	}
	sortByDays(out)
	return out, nil
}

// ListShared trả danh sách món sắp hết của nhà cho thành viên, dạng rút gọn.
func (s *Service) ListShared(ctx context.Context, p authz.Principal, horizon int) ([]SharedItem, error) {
	if err := authz.Can(p, authz.ActionReadItemsShared); err != nil {
		return nil, err
	}
	if p.HouseholdID == "" {
		return []SharedItem{}, nil
	}
	ownerUID, err := s.store.OwnerOf(ctx, p.HouseholdID)
	if err != nil {
		return nil, err
	}
	recs, err := s.store.ListItems(ctx, ownerUID)
	if err != nil {
		return nil, err
	}
	today := domain.Today(s.now(), loc(p))
	out := []SharedItem{}
	for _, r := range recs {
		v := domain.ViewAt(r.Prediction, today, lead(p))
		if r.Dropped || !domain.InNeeds(v, horizon) {
			continue
		}
		out = append(out, SharedItem{Key: r.GroupKey, Name: r.Name, Status: v.Status, DaysLeft: v.DaysLeft})
	}
	return out, nil
}

// Get trả Chi tiết món cho chủ giỏ, kèm tối đa historyLimit lần mua gần nhất.
func (s *Service) Get(ctx context.Context, p authz.Principal, key string) (Detail, error) {
	if err := authz.Can(p, authz.ActionReadItemsFull); err != nil {
		return Detail{}, err
	}
	if !ValidKey(key) {
		return Detail{}, apperr.ErrNotFound
	}
	rec, err := s.store.GetItem(ctx, p.UID, key)
	if err != nil {
		return Detail{}, err
	}
	hist, err := s.store.History(ctx, p.UID, key, historyLimit)
	if err != nil {
		return Detail{}, err
	}
	if hist == nil {
		hist = []HistoryEntry{} // JSON trả [] thay vì null
	}
	today := domain.Today(s.now(), loc(p))
	v := domain.ViewAt(rec.Prediction, today, lead(p))
	d := Detail{Item: toItem(rec, v, today), History: hist, Lots: []domain.Lot{}}
	if lr, ok := s.store.(LotReader); ok {
		lots, err := lr.Lots(ctx, p.UID, key, lotLimit)
		if err != nil {
			return Detail{}, err
		}
		d.Lots = visibleLots(allocateLots(lots, d.Stock))
	}
	return d, nil
}

// visibleLots giữ mọi lot còn hàng và các lot mới nhất cho đủ lotShow, theo thứ tự đọc
// (mới nhất trước), để Chi tiết món không dài 500 dòng.
func visibleLots(lots []domain.Lot) []domain.Lot {
	out := make([]domain.Lot, 0, min(len(lots), lotShow))
	for i, l := range lots {
		if l.Remaining > 0 || i < lotShow {
			out = append(out, l)
		}
	}
	return out
}

// allocateLots chia tồn ước tính của từng stream (chính và phụ) xuống các lot cùng chiều đo.
func allocateLots(lots []domain.Lot, st *Stock) []domain.Lot {
	if len(lots) == 0 {
		return []domain.Lot{}
	}
	if st == nil {
		out := append([]domain.Lot(nil), lots...)
		for i := range out {
			out[i].Remaining = 0
		}
		return out
	}
	est := map[domain.Unit]float64{domain.ParseUnit(st.Unit): st.Estimated}
	for _, p := range st.Parts {
		est[domain.ParseUnit(p.Unit)] = p.Estimated
	}
	remaining := map[string]float64{}
	for unit, e := range est {
		alloc, _ := domain.AllocateFIFO(lots, e, unit)
		for _, l := range alloc {
			if l.Unit == unit {
				remaining[l.ID] = l.Remaining
			}
		}
	}
	out := append([]domain.Lot(nil), lots...)
	for i := range out {
		out[i].Remaining = remaining[out[i].ID]
	}
	return out
}

// CountNeeds đếm món trong "Cần mua" với khoảng ngày horizon, cùng quy tắc với màn
// Đồ dùng (tab "soon"). Mỗi thành viên nhà đếm trên tồn kho của chính mình (4.1 mục 4).
func (s *Service) CountNeeds(ctx context.Context, p authz.Principal, horizon int) (int, error) {
	if err := authz.Can(p, authz.ActionReadItemsFull); err != nil {
		return 0, err
	}
	recs, err := s.store.ListItems(ctx, p.UID)
	if err != nil {
		return 0, err
	}
	today := domain.Today(s.now(), loc(p))
	n := 0
	for _, r := range recs {
		if !r.Dropped && domain.InNeeds(domain.ViewAt(r.Prediction, today, lead(p)), horizon) {
			n++
		}
	}
	return n, nil
}

// ValidKey kiểm tra khóa nhóm trước khi dùng làm ID tài liệu Firestore: không rỗng,
// không quá dài, không chứa "/", không phải "." hay "..", không có dạng __x__ (ID dành riêng).
func ValidKey(key string) bool {
	if key == "" || len(key) > 128 || key == "." || key == ".." || strings.Contains(key, "/") {
		return false
	}
	return !(strings.HasPrefix(key, "__") && strings.HasSuffix(key, "__"))
}

// StockChange là thao tác "Còn nhiều / Sắp hết / Đã hết" hoặc nhập số ngày.
// ActedAt là lúc người dùng bấm (client gửi), để thao tác xếp hàng offline gửi muộn
// không đè lên thao tác mới hơn.
type StockChange struct {
	Quick   string    `json:"quick,omitempty"` // plenty | soon | out
	Days    *int      `json:"days,omitempty"`  // 1..60
	ActedAt time.Time `json:"acted_at"`
}

// maxClockSkew: chấp nhận đồng hồ máy người dùng lệch tối đa chừng này.
const maxClockSkew = 5 * time.Minute

func (s *Service) ChangeStock(ctx context.Context, p authz.Principal, key string, c StockChange) (Item, error) {
	if err := authz.Can(p, authz.ActionWriteItems); err != nil {
		return Item{}, err
	}
	if !ValidKey(key) {
		return Item{}, apperr.ErrNotFound
	}
	now := s.now()
	if c.ActedAt.IsZero() || c.ActedAt.After(now.Add(maxClockSkew)) {
		return Item{}, apperr.New(apperr.CodeValidation, "Thời điểm thao tác không hợp lệ.").With("field", "acted_at")
	}
	setOn := domain.Today(c.ActedAt, loc(p))
	var ov domain.Override
	var ok bool
	switch {
	case c.Quick != "" && c.Days == nil:
		ov, ok = domain.OverrideFromQuick(c.Quick, setOn, c.ActedAt)
	case c.Quick == "" && c.Days != nil:
		ov, ok = domain.OverrideFromDays(*c.Days, setOn, c.ActedAt)
	}
	if !ok {
		return Item{}, apperr.New(apperr.CodeValidation, "Chọn Còn nhiều, Sắp hết, Đã hết hoặc nhập 1 đến 60 ngày.")
	}
	rec, err := s.store.SetOverride(ctx, p.UID, key, ov)
	if err != nil {
		return Item{}, err
	}
	today := domain.Today(now, loc(p))
	return toItem(rec, domain.ViewAt(rec.Prediction, today, lead(p)), today), nil
}

// UndoStock xóa bản sửa tay (Hoàn tác). Không nhận cả tài liệu từ client.
func (s *Service) UndoStock(ctx context.Context, p authz.Principal, key string, actedAt time.Time) (Item, error) {
	if err := authz.Can(p, authz.ActionWriteItems); err != nil {
		return Item{}, err
	}
	if !ValidKey(key) {
		return Item{}, apperr.ErrNotFound
	}
	rec, err := s.store.ClearOverride(ctx, p.UID, key, actedAt)
	if err != nil {
		return Item{}, err
	}
	today := domain.Today(s.now(), loc(p))
	return toItem(rec, domain.ViewAt(rec.Prediction, today, lead(p)), today), nil
}

// QuantitySet là thao tác "SET số lượng thực tế" (Mimi 2.0 mục 4). Unit nhận dạng in
// trên bao bì ("kg", "hộp"...), server quy về g | ml | pc.
type QuantitySet struct {
	Qty  float64 `json:"qty"`
	Unit string  `json:"unit"`
	// Action (0.1 mục 14): set (mặc định, lượng đếm được), increase / decrease (cộng / trừ
	// vào tồn ước tính hiện tại, cùng chiều đo), mark_out (hết hàng: tồn = 0).
	Action string `json:"action,omitempty"`
}

// MaxStockQty chặn số phi lý: 1 tấn, 1.000 lít hoặc 100.000 cái.
const MaxStockQty = 1_000_000

// SetQuantity tạo Confirmed Inventory baseline mới từ lượng người dùng đếm được.
// Lịch sử mua giữ nguyên; tồn ước tính tính lại từ mốc này.
func (s *Service) SetQuantity(ctx context.Context, p authz.Principal, key string, q QuantitySet) (Item, error) {
	if err := authz.Can(p, authz.ActionWriteItems); err != nil {
		return Item{}, err
	}
	if !ValidKey(key) {
		return Item{}, apperr.ErrNotFound
	}
	if q.Qty < 0 || math.IsNaN(q.Qty) || math.IsInf(q.Qty, 0) {
		return Item{}, apperr.New(apperr.CodeValidation, "Số lượng không hợp lệ.").With("field", "qty")
	}
	qty, unit := domain.ToBase(q.Qty, q.Unit)
	today := domain.Today(s.now(), loc(p))
	switch q.Action {
	case "", "set":
	case "increase", "decrease", "mark_out":
		cur, err := s.store.GetItem(ctx, p.UID, key)
		if err != nil {
			return Item{}, err
		}
		base := cur.Baseline.Unit
		if q.Action == "mark_out" {
			qty, unit = 0, base
			if unit == "" {
				unit = domain.UnitItem
			}
			break
		}
		if cur.Baseline.On.IsZero() {
			return Item{}, apperr.New(apperr.CodeInvalidState, "Chưa có tồn để cộng hoặc trừ. Hãy nhập số lượng hiện có.").With("field", "action")
		}
		if unit != base {
			return Item{}, apperr.New(apperr.CodeValidation, "Đơn vị khác chiều đo với tồn hiện tại.").With("field", "unit")
		}
		est := domain.EstimateAt(cur.Baseline, cur.RatePerDay, cur.Consumed, today)
		if q.Action == "increase" {
			qty = est + qty
		} else {
			qty = math.Max(0, est-qty)
		}
	default:
		return Item{}, apperr.New(apperr.CodeValidation, "Action là set, increase, decrease hoặc mark_out.").With("field", "action")
	}
	if qty > MaxStockQty {
		return Item{}, apperr.New(apperr.CodeValidation, "Số lượng quá lớn.").With("field", "qty")
	}
	rec, err := s.store.SetBaseline(ctx, p.UID, key, domain.Baseline{Qty: qty, Unit: unit, On: today, Source: domain.BaselineManual})
	if err != nil {
		return Item{}, err
	}
	return toItem(rec, domain.ViewAt(rec.Prediction, today, lead(p)), today), nil
}

func toItem(r Record, v domain.View, today domain.Date) Item {
	it := Item{
		Key: r.GroupKey, Name: r.Name, CategoryKey: r.CategoryKey,
		Status: v.Status, DaysLeft: v.DaysLeft, PurchaseCount: r.Prediction.PurchaseCount,
		IntervalDays: r.Prediction.IntervalDays, Confidence: string(r.Prediction.Confidence),
		Source: string(r.Prediction.Source),
	}
	if !r.Prediction.OutOn.IsZero() {
		it.OutOn = r.Prediction.OutOn.String()
	}
	if !r.Baseline.On.IsZero() {
		est := domain.EstimateAt(r.Baseline, r.RatePerDay, r.Consumed, today)
		it.Stock = &Stock{
			Estimated: est, Unit: string(r.Baseline.Unit),
			Confirmed: r.Baseline.Qty, ConfirmedOn: r.Baseline.On.String(), ConfirmedSource: string(r.Baseline.Source),
			RatePerDay: r.RatePerDay, DaysOfStock: domain.DaysOfStock(est, r.RatePerDay),
		}
		for _, u := range sortedUnits(r.Extra) {
			x := r.Extra[u]
			if x.Baseline.On.IsZero() {
				continue
			}
			e := domain.EstimateAt(x.Baseline, x.RatePerDay, nil, today)
			it.Stock.Parts = append(it.Stock.Parts, StockPart{
				Estimated: e, Unit: string(u), Confirmed: x.Baseline.Qty, ConfirmedOn: x.Baseline.On.String(),
				DaysOfStock: domain.DaysOfStock(e, x.RatePerDay),
			})
		}
	}
	return it
}

func sortedUnits(m map[domain.Unit]Stream) []domain.Unit {
	out := make([]domain.Unit, 0, len(m))
	for u := range m {
		out = append(out, u)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

func sortByDays(xs []Item) {
	val := func(i Item) int {
		if i.DaysLeft == nil {
			return 1 << 30
		}
		return *i.DaysLeft
	}
	sort.SliceStable(xs, func(a, b int) bool {
		if val(xs[a]) != val(xs[b]) {
			return val(xs[a]) < val(xs[b])
		}
		return xs[a].Name < xs[b].Name
	})
}

func lead(p authz.Principal) int {
	if p.LeadDays >= 1 && p.LeadDays <= 30 {
		return p.LeadDays
	}
	return 7
}

func loc(p authz.Principal) *time.Location { return domain.Location(p.Timezone) }

// Corrector là phần tùy chọn của Store cho màn chi tiết món (0.1 mục 3.2): sửa tên chuẩn hóa
// và chuyển món sang nhóm đúng khi AI phân loại sai. Lịch sử mua giữ nguyên số liệu, chỉ đổi
// nhóm; tồn và dự đoán của nhóm đích tính lại từ lịch sử gộp.
type Corrector interface {
	RenameItem(ctx context.Context, uid, key, name string) (Record, error)
	// RegroupItem chuyển mọi purchase của from sang to, gộp tồn vào to rồi xóa from.
	RegroupItem(ctx context.Context, uid, from, to, category string) (Record, error)
}

// Correction là thân PATCH /items/{key}: chỉ gửi trường cần sửa.
type Correction struct {
	Name     *string `json:"name,omitempty"`
	GroupKey *string `json:"group_key,omitempty"`
}

// Correct sửa tên và / hoặc nhóm của một món. Trả món sau khi sửa (key mới nếu đổi nhóm).
func (s *Service) Correct(ctx context.Context, p authz.Principal, key string, c Correction) (Item, error) {
	if err := authz.Can(p, authz.ActionWriteItems); err != nil {
		return Item{}, err
	}
	if !ValidKey(key) {
		return Item{}, apperr.ErrNotFound
	}
	cs, ok := s.store.(Corrector)
	if !ok {
		return Item{}, apperr.New(apperr.CodeNotImplemented, "Chưa hỗ trợ sửa món.")
	}
	if c.Name == nil && c.GroupKey == nil {
		return Item{}, apperr.New(apperr.CodeValidation, "Chưa có gì để sửa.")
	}
	var rec Record
	var err error
	if c.Name != nil {
		name := domain.CleanItemName(*c.Name)
		if name == "" || utf8.RuneCountInString(name) > 60 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return Item{}, apperr.New(apperr.CodeValidation, "Tên món từ 1 đến 60 ký tự.").With("field", "name")
		}
		if rec, err = cs.RenameItem(ctx, p.UID, key, name); err != nil {
			return Item{}, err
		}
	}
	if c.GroupKey != nil {
		to := taxonomy.Normalize(*c.GroupKey)
		if !taxonomy.Has(to) {
			return Item{}, apperr.New(apperr.CodeValidation, "Nhóm không có trong danh sách.").With("field", "group_key")
		}
		if to != key {
			if rec, err = cs.RegroupItem(ctx, p.UID, key, to, taxonomy.SpendingCategory(to)); err != nil {
				return Item{}, err
			}
		}
	}
	if rec.GroupKey == "" {
		if rec, err = s.store.GetItem(ctx, p.UID, key); err != nil {
			return Item{}, err
		}
	}
	today := domain.Today(s.now(), loc(p))
	return toItem(rec, domain.ViewAt(rec.Prediction, today, lead(p)), today), nil
}
