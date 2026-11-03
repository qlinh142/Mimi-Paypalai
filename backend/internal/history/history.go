// Package history: Lịch sử mua hàng (màn 4c) và Chi tiết hóa đơn (màn 4d), Spec 1.5 mục 4.
//
// Lịch sử gồm ba loại dòng:
//   - own: hóa đơn người này tự lưu (users/{uid}/receipts);
//   - delegated: phần món người nhà mua hộ cho người này, suy từ purchases có payer_uid khác
//     uid (không có hóa đơn gốc, chỉ các món của mình);
//   - recurring: khoản định kỳ trả tay (không qua hóa đơn), để Sổ chi tiêu đủ các khoản.
//
// Chỉ đọc, không ghi gì. Tên người mua hộ trả kèm uid để app hiện tên gọi người dùng đã đặt.
package history

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
)

// Giới hạn đọc. Hóa đơn chỉ là chữ nên nhẹ; 500 hóa đơn đủ vài năm của một nhà.
const (
	ReceiptLimit  = 500
	DelegateLimit = 1000 // dòng mua hộ (mỗi dòng một món)
	PaidLimit     = 200
)

// Line là một dòng đã lưu, của hóa đơn hoặc của purchases.
type Line struct {
	ReceiptID    string
	Name         string
	OriginalName string // tên theo bill khi Name là tên đã dịch
	Structure    string
	Qty          float64
	VND          *int64 // nil: chưa rõ giá (A7), không tính vào tổng
	CategoryKey  string
	Owner        string // uid nhận hàng; "" là người lưu
	// Chỉ có ở purchases (dòng mua hộ):
	PayerUID     string
	Store        string
	PurchasedOn  string
	SavedAt      time.Time
	AssignmentID string
	// B3: Kế hoạch mà dòng đã trừ vào. PlanSet false: dòng lưu trước B3, theo SourcePlanID.
	PlanIDs []string
	PlanSet bool
	// B7: receipt | manual | estimate | missing; rỗng ở dòng lưu trước B7.
	PriceSource string
}

// Receipt là hóa đơn đã lưu của chính người gọi.
type Receipt struct {
	ID           string
	Store        string
	PurchasedOn  string // YYYY-MM-DD
	SavedAt      time.Time
	TotalVND     int64
	DiscountVND  int64
	SourcePlanID string
	Lines        []Line
}

// Paid là một kỳ định kỳ đã trả tay.
type Paid struct {
	ID, RecurringID, Name, PaidOn string
	AmountVND                     int64
}

type Store interface {
	Receipts(ctx context.Context, uid string, limit int) ([]Receipt, error)
	Receipt(ctx context.Context, uid, id string) (Receipt, error) // apperr.ErrNotFound
	DelegatedLines(ctx context.Context, uid string, limit int) ([]Line, error)
	PaidManually(ctx context.Context, uid string, limit int) ([]Paid, error)
	DisplayNames(ctx context.Context, uids []string) (map[string]string, error)
	PlanNames(ctx context.Context, uid string) (map[string]string, error) // id -> tên; Plan mặc định không có
}

// Entry là một dòng của GET /receipts/history.
type Entry struct {
	ReceiptID   string   `json:"receipt_id"`
	Store       string   `json:"store"`
	PurchasedAt string   `json:"purchased_at"` // RFC 3339
	TotalVND    int64    `json:"total_vnd"`
	ItemNames   []string `json:"item_names"`
	Kind        string   `json:"kind"` // own | delegated | recurring
	BuyerUID    string   `json:"buyer_uid,omitempty"`
	BuyerName   string   `json:"buyer_name,omitempty"`
	PlanName    string   `json:"plan_name,omitempty"`
	RecurringID string   `json:"recurring_id,omitempty"`
	// MissingPrice: số món chưa rõ giá, không nằm trong tổng.
	MissingPrice int `json:"missing_price,omitempty"`
}

type DetailLine struct {
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name,omitempty"` // tên theo bill, chữ nhỏ dưới tên dịch
	Spec         string  `json:"spec,omitempty"`
	Qty          float64 `json:"qty"`
	UnitPrice    *int64  `json:"unit_price"` // null: chưa rõ giá
	Amount       *int64  `json:"amount"`
	CategoryKey  string  `json:"category_key,omitempty"`
	Tag          string  `json:"tag,omitempty"` // for_owner | mine, khi hóa đơn có món mua hộ
	// PriceSource (B7): estimate thì app ghi "Ước tính"; missing là chưa có giá.
	PriceSource string `json:"price_source,omitempty"`
}

type PlanRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Split struct {
	ForOwner  int64  `json:"for_owner"`
	Mine      int64  `json:"mine"`
	OwnerUID  string `json:"owner_uid"`
	OwnerName string `json:"owner_name"`
}

// Detail là phản hồi GET /receipts/{id}.
type Detail struct {
	Entry
	Plans    []PlanRef    `json:"plans"`
	Lines    []DetailLine `json:"lines"`
	Subtotal int64        `json:"subtotal"`
	Discount int64        `json:"discount"`
	Split    *Split       `json:"split,omitempty"`
}

type Service struct {
	store Store
	loc   *time.Location
}

func New(store Store) *Service {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	return &Service{store: store, loc: loc}
}

// DelegatedID là mã của khoản mua hộ trong lịch sử: "d-" + receipt_id của người mua.
const delegatedPrefix = "d-"

// at: thời điểm hiển thị. Có giờ lưu thì dùng, không thì trưa ngày mua (giờ Việt Nam).
func (s *Service) at(day string, saved time.Time) string {
	if !saved.IsZero() && saved.In(s.loc).Format("2006-01-02") == day {
		return saved.UTC().Format(time.RFC3339)
	}
	t, err := time.ParseInLocation("2006-01-02", day, s.loc)
	if err != nil {
		return saved.UTC().Format(time.RFC3339)
	}
	return t.Add(12 * time.Hour).UTC().Format(time.RFC3339)
}

func (s *Service) History(ctx context.Context, p authz.Principal) ([]Entry, error) {
	if err := authz.Can(p, authz.ActionReadReceipts); err != nil {
		return nil, err
	}
	rs, err := s.store.Receipts(ctx, p.UID, ReceiptLimit)
	if err != nil {
		return nil, err
	}
	plans, err := s.store.PlanNames(ctx, p.UID)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rs))
	for _, r := range rs {
		e := Entry{ReceiptID: r.ID, Store: r.Store, PurchasedAt: s.at(r.PurchasedOn, r.SavedAt), TotalVND: r.TotalVND, Kind: "own", PlanName: firstPlan(r, plans)}
		e.ItemNames, e.MissingPrice = names(r.Lines)
		out = append(out, e)
	}
	ds, err := s.delegated(ctx, p.UID)
	if err != nil {
		return nil, err
	}
	out = append(out, ds...)
	paid, err := s.store.PaidManually(ctx, p.UID, PaidLimit)
	if err != nil {
		return nil, err
	}
	for _, x := range paid {
		out = append(out, Entry{ReceiptID: "rec-" + x.ID, Store: x.Name, PurchasedAt: s.at(x.PaidOn, time.Time{}), TotalVND: x.AmountVND,
			ItemNames: []string{x.Name}, Kind: "recurring", RecurringID: x.RecurringID})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PurchasedAt > out[j].PurchasedAt })
	return out, nil
}

func names(ls []Line) ([]string, int) {
	ns := make([]string, 0, len(ls))
	missing := 0
	for _, l := range ls {
		ns = append(ns, l.Name)
		if l.VND == nil {
			missing++
		}
	}
	return ns, missing
}

// delegated gom các dòng người khác mua hộ theo hóa đơn của người mua.
func (s *Service) delegated(ctx context.Context, uid string) ([]Entry, error) {
	ls, err := s.store.DelegatedLines(ctx, uid, DelegateLimit)
	if err != nil {
		return nil, err
	}
	groups := map[string][]Line{}
	var order []string
	payers := map[string]bool{}
	for _, l := range ls {
		if l.PayerUID == "" || l.PayerUID == uid {
			continue
		}
		if _, ok := groups[l.ReceiptID]; !ok {
			order = append(order, l.ReceiptID)
		}
		groups[l.ReceiptID] = append(groups[l.ReceiptID], l)
		payers[l.PayerUID] = true
	}
	who, err := s.store.DisplayNames(ctx, keys(payers))
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(order))
	for _, id := range order {
		g := groups[id]
		e := Entry{ReceiptID: delegatedPrefix + id, Store: g[0].Store, PurchasedAt: s.at(g[0].PurchasedOn, g[0].SavedAt), Kind: "delegated",
			BuyerUID: g[0].PayerUID, BuyerName: who[g[0].PayerUID]}
		e.ItemNames, e.MissingPrice = names(g)
		for _, l := range g {
			if l.VND != nil {
				e.TotalVND += *l.VND
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func detailLine(l Line) DetailLine {
	d := DetailLine{Name: l.Name, OriginalName: l.OriginalName, Spec: l.Structure, Qty: l.Qty, Amount: l.VND, CategoryKey: l.CategoryKey, PriceSource: l.PriceSource}
	if l.VND == nil {
		d.PriceSource = "missing"
	}
	if l.VND != nil && l.Qty > 0 {
		u := int64(float64(*l.VND)/l.Qty + 0.5)
		d.UnitPrice = &u
	}
	return d
}

func validID(id string) bool {
	return id != "" && len(id) <= 128 && !strings.ContainsAny(id, "/")
}

func (s *Service) Detail(ctx context.Context, p authz.Principal, id string) (Detail, error) {
	if err := authz.Can(p, authz.ActionReadReceipts); err != nil {
		return Detail{}, err
	}
	if !validID(id) {
		return Detail{}, apperr.ErrNotFound
	}
	if strings.HasPrefix(id, delegatedPrefix) {
		return s.delegatedDetail(ctx, p.UID, id)
	}
	r, err := s.store.Receipt(ctx, p.UID, id)
	if err != nil {
		return Detail{}, err
	}
	plans, err := s.store.PlanNames(ctx, p.UID)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Entry: Entry{ReceiptID: r.ID, Store: r.Store, PurchasedAt: s.at(r.PurchasedOn, r.SavedAt), TotalVND: r.TotalVND, Kind: "own",
		PlanName: firstPlan(r, plans)}, Plans: []PlanRef{}, Discount: r.DiscountVND}
	d.ItemNames, d.MissingPrice = names(r.Lines)
	d.Plans = receiptPlans(r, plans)
	sp := Split{}
	for _, l := range r.Lines {
		dl := detailLine(l)
		v := int64(0)
		if l.VND != nil {
			v = *l.VND
		}
		d.Subtotal += v
		if l.Owner != "" && l.Owner != p.UID {
			dl.Tag, sp.OwnerUID = "for_owner", l.Owner
			sp.ForOwner += v
		} else {
			dl.Tag = "mine"
			sp.Mine += v
		}
		d.Lines = append(d.Lines, dl)
	}
	if sp.OwnerUID == "" { // hóa đơn không có món mua hộ: không chia
		for i := range d.Lines {
			d.Lines[i].Tag = ""
		}
		return d, nil
	}
	who, err := s.store.DisplayNames(ctx, []string{sp.OwnerUID})
	if err != nil {
		return Detail{}, err
	}
	sp.OwnerName = who[sp.OwnerUID]
	d.Split = &sp
	return d, nil
}

func firstPlan(r Receipt, names map[string]string) string {
	if ps := receiptPlans(r, names); len(ps) > 0 {
		return ps[0].Name
	}
	return ""
}

// receiptPlans: các Kế hoạch đặt tên mà hóa đơn đã trừ vào, theo thứ tự gặp. Dòng lưu
// trước B3 dùng source_plan_id của hóa đơn. Kế hoạch chính và Kế hoạch đã xóa không hiện.
func receiptPlans(r Receipt, names map[string]string) []PlanRef {
	out := []PlanRef{}
	seen := map[string]bool{}
	add := func(id string) {
		if id == "" || seen[id] || names[id] == "" {
			return
		}
		seen[id] = true
		out = append(out, PlanRef{ID: id, Name: names[id]})
	}
	for _, l := range r.Lines {
		if !l.PlanSet {
			add(r.SourcePlanID)
			continue
		}
		for _, id := range l.PlanIDs {
			add(id)
		}
	}
	return out
}

func (s *Service) delegatedDetail(ctx context.Context, uid, id string) (Detail, error) {
	all, err := s.delegated(ctx, uid)
	if err != nil {
		return Detail{}, err
	}
	var e *Entry
	for i := range all {
		if all[i].ReceiptID == id {
			e = &all[i]
			break
		}
	}
	if e == nil {
		return Detail{}, apperr.ErrNotFound
	}
	ls, err := s.store.DelegatedLines(ctx, uid, DelegateLimit)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Entry: *e, Plans: []PlanRef{}}
	for _, l := range ls {
		if delegatedPrefix+l.ReceiptID != id || l.PayerUID == uid {
			continue
		}
		d.Lines = append(d.Lines, detailLine(l))
		if l.VND != nil {
			d.Subtotal += *l.VND
		}
	}
	return d, nil
}
