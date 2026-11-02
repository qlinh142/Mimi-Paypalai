// Package plan phục vụ Kế hoạch mua sắm (spec mục 5): Giỏ, thời gian mua, Nơi mua tối ưu.
//
//   - Mỗi nhóm hàng có tối đa một dòng giỏ đang mở: ID dòng = group_key (plan_items/{key}).
//   - Lưu hóa đơn có món trong giỏ thì dòng đó tự chuyển "done" (trong transaction lưu hóa đơn).
//   - Giá dùng để ước tính là giá người dùng từng trả, lấy từ hóa đơn đã lưu; không tìm giá ngoài.
//   - Giỏ, Giá tốt và ô Trang chủ cùng gọi domain.PlanShopping nên con số luôn khớp nhau.
package plan

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/item"
)

// Line là một dòng giỏ đang mở (plan_items/{key} với status "open").
// Mimi 2.0 mục 6: mỗi dòng mang số lượng, hạn cần (need-by), ưu tiên, nguồn và lý do.
type Line struct {
	Key         string
	Name        string
	CategoryKey string
	AddedAt     time.Time
	Qty         float64 // số lượng cần mua theo Unit; 0: chưa nhập
	Unit        string  // đơn vị người dùng chọn (kg, hop, goi...), "" là cái
	NeedBy      string  // YYYY-MM-DD hoặc ""
	Priority    string  // high | normal | low
	Source      string  // forecast | check | meal | manual
	Reason      string
	BoughtQty   float64 // đã mua theo hóa đơn/giao hàng, cùng đơn vị gốc với Qty; > 0 và < Qty: mua thiếu
	// Status: open, hoặc assigned khi dòng đang giao cho một thành viên mua hộ (4.1 mục 8.1).
	// Dòng assigned hiện trong Giỏ để Owner thấy, nhưng không sửa được và không vào ước tính.
	Status     string
	AssignedTo string
}

// Assigned báo dòng đang nằm trong một vòng mua hộ.
func (l Line) Assigned() bool { return l.Status == "assigned" }

// Nguồn của một dòng Kế hoạch (Mimi 2.0 mục 6).
const (
	SourceForecast = "forecast" // To Buy: Mimi dự báo sắp hết
	SourceCheck    = "check"    // Check tồn: người dùng thêm từ Đồ dùng
	SourceMeal     = "meal"     // nhu cầu từ bữa ăn / sự kiện
	SourceManual   = "manual"
)

var validPriority = map[string]bool{"high": true, "normal": true, "low": true}

// LinePatch là PATCH /plan/items/{key}: chỉ trường có mặt mới đổi.
type LinePatch struct {
	Qty      *float64 `json:"qty,omitempty"`
	Unit     *string  `json:"unit,omitempty"`
	NeedBy   *string  `json:"need_by,omitempty"`
	Priority *string  `json:"priority,omitempty"`
	Reason   *string  `json:"reason,omitempty"`
}

// State là thông tin chuyến mua: ngày, giờ, ghi chú.
type State struct {
	BuyDate   string `json:"buy_date"`             // YYYY-MM-DD hoặc "" (chưa chọn)
	BuyHour   *int   `json:"buy_hour"`             // 0..23, null nếu chưa chọn
	BuyMinute *int   `json:"buy_minute,omitempty"` // 0..59, đi cùng buy_hour (3.1 mục 3.2: "09:30")
	TripNote  string `json:"trip_note"`            // tối đa 120 ký tự
	// ReconciledOn: ngày của hóa đơn gần nhất đã đối chiếu Plan này. Chỉ transaction lưu
	// hóa đơn ghi; client gửi lên bị bỏ qua.
	ReconciledOn string `json:"reconciled_on,omitempty"`
	// Rev tăng mỗi lần lưu ngày, giờ, ghi chú. Client gửi lại số đã đọc trong BaseRev; nếu
	// máy khác đã lưu trước (Rev trên máy chủ khác BaseRev), máy chủ trả 409 thay vì ghi đè.
	// Không gửi BaseRev thì ghi như cũ (bản cũ của app, thao tác dọn kế hoạch).
	Rev     int64  `json:"rev"`
	BaseRev *int64 `json:"base_rev,omitempty"`
}

// Giai đoạn của Plan (3.1 mục 18): planning trước ngày mua, shopping từ ngày mua, reconciled
// khi đã có hóa đơn đối chiếu sau lần đặt ngày mua gần nhất. Món chưa khớp vẫn open.
const (
	PhasePlanning   = "planning"
	PhaseShopping   = "shopping"
	PhaseReconciled = "reconciled"
)

func (st State) phase(today domain.Date) string {
	if st.ReconciledOn != "" && (st.BuyDate == "" || st.BuyDate <= st.ReconciledOn) {
		return PhaseReconciled
	}
	if st.BuyDate != "" && st.BuyDate <= today.String() {
		return PhaseShopping
	}
	return PhasePlanning
}

type Store interface {
	OpenLines(ctx context.Context, uid string) ([]Line, error)
	// AddLine ghi đè dòng của nhóm thành "open" (kể cả khi trước đó đã "done").
	AddLine(ctx context.Context, uid string, l Line) error
	// UpdateLine ghi số lượng, need-by, ưu tiên, lý do của một dòng đang mở.
	UpdateLine(ctx context.Context, uid string, l Line) error
	// RemoveLine xóa dòng; nhóm không có trong giỏ cũng không lỗi.
	RemoveLine(ctx context.Context, uid, key string) error
	State(ctx context.Context, uid string) (State, error)
	// SetState ghi trạng thái và trả Rev mới. s.BaseRev khác nil mà lệch Rev đang lưu thì
	// trả apperr.CodeConflict, không ghi gì.
	SetState(ctx context.Context, uid string, s State) (int64, error)
	// Prices trả giá mỗi đơn vị gần nhất của từng nhóm tại từng cửa hàng người dùng từng mua.
	Prices(ctx context.Context, uid string, groups []string) ([]domain.LastPrice, error)
}

// Items là phần đọc của item.Store mà Kế hoạch cần.
type Items interface {
	GetItem(ctx context.Context, uid, key string) (item.Record, error)
	ListItems(ctx context.Context, uid string) ([]item.Record, error)
}

// MaxCartLines: giới hạn số món trong giỏ (một chuyến chợ thực tế vài chục món).
const MaxCartLines = 100

const maxTripNote = 120

// DefaultPlanID là Plan mặc định của mỗi người, giữ ở đường dẫn cũ plan_items / plan_state.
const DefaultPlanID = "default"

// CartItem là một món trong Giỏ, kèm tồn hiện tại và giá rẻ nhất đã biết.
type CartItem struct {
	Key         string        `json:"key"`
	Name        string        `json:"name"`
	CategoryKey string        `json:"category_key"`
	Status      domain.Status `json:"status"`
	DaysLeft    *int          `json:"days_left"`
	UnitPrice   *int64        `json:"unit_price"` // null: chưa có giá
	BestStore   string        `json:"best_store,omitempty"`
	Qty         float64       `json:"qty"`
	Unit        string        `json:"unit,omitempty"`
	NeedBy      string        `json:"need_by,omitempty"`
	Priority    string        `json:"priority"`
	Source      string        `json:"source"`
	Reason      string        `json:"reason,omitempty"`
	BoughtQty   float64       `json:"bought_qty,omitempty"` // > 0: đã mua một phần
	LineStatus  string        `json:"line_status"`          // open | assigned
	AssignedTo  string        `json:"assigned_to_uid,omitempty"`
}

// Cart là phản hồi GET /plan.
type Cart struct {
	PlanID       string     `json:"plan_id"`
	Items        []CartItem `json:"items"`
	ItemCount    int        `json:"item_count"`
	Estimate     *int64     `json:"estimate"`      // tổng theo phương án khuyên dùng; null khi chưa món nào có giá
	MissingPrice int        `json:"missing_price"` // số món chưa có giá, không cộng vào ước tính
	State        State      `json:"state"`
	Phase        string     `json:"phase"` // planning | shopping | reconciled
}

type Stop struct {
	Store  string   `json:"store"`
	Groups []string `json:"groups"`
	Total  int64    `json:"total"`
}

// Prices là phản hồi GET /plan/prices (schema PlanPrices).
type Prices struct {
	SingleStore  *Stop    `json:"single_store"`
	Split        []Stop   `json:"split"`
	SplitTotal   int64    `json:"split_total"`
	Saving       int64    `json:"saving"`
	Recommend    string   `json:"recommend"` // single | split
	MissingPrice []string `json:"missing_price"`
	SingleSource []string `json:"single_source"`
	SaleOffers   []any    `json:"sale_offers"` // chưa tìm giá sale bên ngoài: luôn rỗng
}

type Service struct {
	store Store
	items Items
	plans PlansStore // nil: chỉ Plan mặc định
	now   func() time.Time
}

func New(store Store, items Items, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, items: items, now: now}
}

// Get trả Giỏ: các dòng đang mở, tồn hiện tại của từng món, giá và ước tính.
func (s *Service) Get(ctx context.Context, p authz.Principal) (Cart, error) {
	if err := authz.Can(p, authz.ActionReadPlan); err != nil {
		return Cart{}, err
	}
	if err := s.checkPlan(ctx, p.UID, false); err != nil {
		return Cart{}, err
	}
	lines, err := s.store.OpenLines(ctx, p.UID)
	if err != nil {
		return Cart{}, err
	}
	state, err := s.store.State(ctx, p.UID)
	if err != nil {
		return Cart{}, err
	}
	cart := Cart{PlanID: IDFrom(ctx), Items: []CartItem{}, State: state,
		Phase: state.phase(domain.Today(s.now(), domain.Location(p.Timezone)))}
	if len(lines) == 0 {
		return cart, nil
	}
	recs, err := s.items.ListItems(ctx, p.UID)
	if err != nil {
		return Cart{}, err
	}
	byKey := make(map[string]item.Record, len(recs))
	for _, r := range recs {
		byKey[r.GroupKey] = r
	}
	sp, prices, err := s.plan(ctx, p.UID, openOnly(lines))
	if err != nil {
		return Cart{}, err
	}
	today := domain.Today(s.now(), domain.Location(p.Timezone))
	for _, l := range lines {
		ci := CartItem{
			Key: l.Key, Name: l.Name, CategoryKey: l.CategoryKey, Status: domain.StatusWaiting,
			Qty: l.Qty, Unit: l.Unit, NeedBy: l.NeedBy, Priority: orDefault(l.Priority, "normal"),
			Source: orDefault(l.Source, SourceManual), Reason: l.Reason, BoughtQty: l.BoughtQty,
			LineStatus: orDefault(l.Status, "open"), AssignedTo: l.AssignedTo,
		}
		if r, ok := byKey[l.Key]; ok {
			v := domain.ViewAt(r.Prediction, today, leadDays(p))
			ci.Status, ci.DaysLeft = v.Status, v.DaysLeft
			if r.Name != "" {
				ci.Name = r.Name // tên mới nhất từ Đồ dùng
			}
		}
		if store, price, ok := cheapest(prices[l.Key]); ok {
			v := int64(price)
			ci.UnitPrice, ci.BestStore = &v, store
		}
		cart.Items = append(cart.Items, ci)
	}
	cart.ItemCount = len(cart.Items)
	cart.Estimate, cart.MissingPrice = estimate(sp)
	return cart, nil
}

// Summary là số phụ của ô Kế hoạch trên Trang chủ, cùng cách tính với Giỏ.
func (s *Service) Summary(ctx context.Context, p authz.Principal) (count int, est *int64, missing int, err error) {
	if err := authz.Can(p, authz.ActionReadPlan); err != nil {
		return 0, nil, 0, err
	}
	lines, err := s.store.OpenLines(ctx, p.UID)
	lines = openOnly(lines)
	if err != nil || len(lines) == 0 {
		return 0, nil, 0, err
	}
	sp, _, err := s.plan(ctx, p.UID, lines)
	if err != nil {
		return 0, nil, 0, err
	}
	est, missing = estimate(sp)
	return len(lines), est, missing, nil
}

// Add cho một món trong Đồ dùng vào giỏ. Gọi lại nhiều lần vẫn chỉ có một dòng.
func (s *Service) Add(ctx context.Context, p authz.Principal, key string) (Cart, error) {
	return s.AddWith(ctx, p, key, AddRequest{})
}

// AddWith như Add, và nhận món chưa có trong Đồ dùng: nhóm taxonomy hoặc món tự nhập.
func (s *Service) AddWith(ctx context.Context, p authz.Principal, key string, req AddRequest) (Cart, error) {
	if err := authz.Can(p, authz.ActionWritePlan); err != nil {
		return Cart{}, err
	}
	if !item.ValidKey(key) {
		return Cart{}, apperr.ErrNotFound
	}
	if err := s.checkPlan(ctx, p.UID, true); err != nil {
		return Cart{}, err
	}
	rec, err := s.items.GetItem(ctx, p.UID, key)
	var fresh *Line
	if errors.Is(err, apperr.ErrNotFound) {
		l, lerr := s.lineFor(key, req)
		if lerr != nil {
			return Cart{}, lerr // apperr.ErrNotFound khi không phải món đã biết, nhóm taxonomy hay món tự nhập
		}
		fresh, key, err = &l, l.Key, nil
	}
	if err != nil {
		return Cart{}, err
	}
	lines, err := s.store.OpenLines(ctx, p.UID)
	if err != nil {
		return Cart{}, err
	}
	already := false
	for _, l := range lines {
		if l.Key == key {
			already = true
			break
		}
	}
	if !already && len(lines) >= MaxCartLines {
		return Cart{}, apperr.New(apperr.CodeValidation, "Giỏ đã đủ 100 món.").With("limit", MaxCartLines)
	}
	if !already && fresh != nil {
		if err := s.store.AddLine(ctx, p.UID, *fresh); err != nil {
			return Cart{}, err
		}
	} else if !already {
		l := Line{Key: key, Name: rec.Name, CategoryKey: rec.CategoryKey, AddedAt: s.now(), Qty: 1, Priority: "normal", Source: SourceCheck}
		today := domain.Today(s.now(), domain.Location(p.Timezone))
		if v := domain.ViewAt(rec.Prediction, today, leadDays(p)); domain.InNeeds(v, item.NeedsHorizonDays) {
			// Món đang trong "Cần mua": nguồn là dự báo, need-by là ngày dự kiến hết.
			l.Source, l.NeedBy = SourceForecast, rec.Prediction.OutOn.String()
			l.Reason = "Mimi dự báo sắp hết"
			if rec.Prediction.OutOn.IsZero() {
				l.NeedBy = ""
			}
		}
		if err := s.store.AddLine(ctx, p.UID, l); err != nil {
			return Cart{}, err
		}
	}
	return s.Get(ctx, p)
}

// UpdateLine sửa số lượng, đơn vị, need-by, ưu tiên, lý do của một dòng đang mở.
// Chưa tạo giao dịch và không đổi tồn kho (Mimi 2.0 mục 6).
func (s *Service) UpdateLine(ctx context.Context, p authz.Principal, key string, patch LinePatch) (Cart, error) {
	if err := authz.Can(p, authz.ActionWritePlan); err != nil {
		return Cart{}, err
	}
	if !item.ValidKey(key) {
		return Cart{}, apperr.ErrNotFound
	}
	if err := s.checkPlan(ctx, p.UID, true); err != nil {
		return Cart{}, err
	}
	lines, err := s.store.OpenLines(ctx, p.UID)
	if err != nil {
		return Cart{}, err
	}
	var l *Line
	for i := range lines {
		if lines[i].Key == key {
			l = &lines[i]
			break
		}
	}
	if l == nil {
		return Cart{}, apperr.ErrNotFound
	}
	if l.Assigned() {
		return Cart{}, apperr.New(apperr.CodeInvalidState, "Món này đang được nhờ mua. Hủy vòng mua hộ trước khi sửa.").With("key", key)
	}
	bad := func(field, msg string) error {
		return apperr.New(apperr.CodeValidation, msg).With("field", field)
	}
	if patch.Qty != nil {
		if !(*patch.Qty > 0 && *patch.Qty <= 10_000) {
			return Cart{}, bad("qty", "Số lượng phải lớn hơn 0.")
		}
		l.Qty = *patch.Qty
	}
	if patch.Unit != nil {
		u := domain.NormalizeUnit(*patch.Unit)
		if u == "" && strings.TrimSpace(*patch.Unit) != "" {
			return Cart{}, bad("unit", "Đơn vị không hợp lệ.")
		}
		l.Unit = u
	}
	if patch.NeedBy != nil {
		if *patch.NeedBy != "" {
			d, err := domain.ParseDate(*patch.NeedBy)
			today := domain.Today(s.now(), domain.Location(p.Timezone))
			if err != nil || d.Before(today.AddDays(-1)) || d.After(today.AddDays(366)) {
				return Cart{}, bad("need_by", "Ngày cần không hợp lệ.")
			}
		}
		l.NeedBy = *patch.NeedBy
	}
	if patch.Priority != nil {
		if !validPriority[*patch.Priority] {
			return Cart{}, bad("priority", "Ưu tiên là high, normal hoặc low.")
		}
		l.Priority = *patch.Priority
	}
	if patch.Reason != nil {
		r := strings.TrimSpace(*patch.Reason)
		if utf8.RuneCountInString(r) > maxTripNote || strings.IndexFunc(r, unicode.IsControl) >= 0 {
			return Cart{}, bad("reason", "Lý do tối đa 120 ký tự.")
		}
		l.Reason = r
	}
	if err := s.store.UpdateLine(ctx, p.UID, *l); err != nil {
		return Cart{}, err
	}
	return s.Get(ctx, p)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Remove bỏ một món khỏi giỏ. Món không có trong giỏ vẫn coi là thành công.
func (s *Service) Remove(ctx context.Context, p authz.Principal, key string) error {
	if err := authz.Can(p, authz.ActionWritePlan); err != nil {
		return err
	}
	if !item.ValidKey(key) {
		return apperr.ErrNotFound
	}
	if err := s.checkPlan(ctx, p.UID, true); err != nil {
		return err
	}
	lines, err := s.store.OpenLines(ctx, p.UID)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if l.Key == key && l.Assigned() {
			return apperr.New(apperr.CodeInvalidState, "Món này đang được nhờ mua. Hủy vòng mua hộ trước khi xóa.").With("key", key)
		}
	}
	return s.store.RemoveLine(ctx, p.UID, key)
}

// SetState lưu ngày, giờ và ghi chú cho chuyến mua.
func (s *Service) SetState(ctx context.Context, p authz.Principal, st State) (State, error) {
	if err := authz.Can(p, authz.ActionWritePlan); err != nil {
		return State{}, err
	}
	if err := s.checkPlan(ctx, p.UID, true); err != nil {
		return State{}, err
	}
	st.TripNote = strings.TrimSpace(st.TripNote)
	st.ReconciledOn = ""
	if st.BuyHour == nil {
		st.BuyMinute = nil
	}
	if err := st.validate(domain.Today(s.now(), domain.Location(p.Timezone))); err != nil {
		return State{}, err
	}
	rev, err := s.store.SetState(ctx, p.UID, st)
	if err != nil {
		return State{}, err
	}
	st.Rev, st.BaseRev = rev, nil
	return st, nil
}

func (st State) validate(today domain.Date) error {
	bad := func(field, msg string) error {
		return apperr.New(apperr.CodeValidation, msg).With("field", field)
	}
	if st.BuyDate != "" {
		d, err := domain.ParseDate(st.BuyDate)
		if err != nil {
			return bad("buy_date", "Ngày mua không hợp lệ.")
		}
		// Cho phép chọn lùi vài ngày (ghi lại chuyến vừa đi) và tối đa một năm tới.
		if d.Before(today.AddDays(-7)) || d.After(today.AddDays(366)) {
			return bad("buy_date", "Ngày mua nằm ngoài khoảng cho phép.")
		}
	}
	if st.BuyHour != nil && (*st.BuyHour < 0 || *st.BuyHour > 23) {
		return bad("buy_hour", "Giờ mua từ 0 đến 23.")
	}
	if st.BuyMinute != nil && (*st.BuyMinute < 0 || *st.BuyMinute > 59) {
		return bad("buy_minute", "Phút từ 0 đến 59.")
	}
	if utf8.RuneCountInString(st.TripNote) > maxTripNote {
		return bad("trip_note", "Ghi chú tối đa 120 ký tự.")
	}
	if strings.IndexFunc(st.TripNote, func(r rune) bool { return unicode.IsControl(r) && r != '\n' }) >= 0 {
		return bad("trip_note", "Ghi chú có ký tự không hợp lệ.")
	}
	return nil
}

// Prices trả Nơi mua tối ưu cho các món trong giỏ.
func (s *Service) Prices(ctx context.Context, p authz.Principal) (Prices, error) {
	if err := authz.Can(p, authz.ActionReadPlan); err != nil {
		return Prices{}, err
	}
	if err := s.checkPlan(ctx, p.UID, false); err != nil {
		return Prices{}, err
	}
	lines, err := s.store.OpenLines(ctx, p.UID)
	if err != nil {
		return Prices{}, err
	}
	sp, _, err := s.plan(ctx, p.UID, openOnly(lines))
	if err != nil {
		return Prices{}, err
	}
	return toPrices(sp), nil
}

// plan đọc giá và gọi domain.PlanShopping (hàm duy nhất tính Nơi mua tối ưu).
func (s *Service) plan(ctx context.Context, uid string, lines []Line) (domain.ShoppingPlan, map[string]map[string]domain.VND, error) {
	groups := make([]string, 0, len(lines))
	for _, l := range lines {
		groups = append(groups, l.Key)
	}
	if len(groups) == 0 {
		return domain.PlanShopping(nil, nil), map[string]map[string]domain.VND{}, nil
	}
	lp, err := s.store.Prices(ctx, uid, groups)
	if err != nil {
		return domain.ShoppingPlan{}, nil, err
	}
	byGroup := map[string]map[string]domain.VND{}
	for _, x := range lp {
		if byGroup[x.GroupKey] == nil {
			byGroup[x.GroupKey] = map[string]domain.VND{}
		}
		byGroup[x.GroupKey][x.Store] = x.Unit
	}
	return domain.PlanShopping(groups, lp), byGroup, nil
}

// estimate: tổng theo phương án khuyên dùng. Món chưa có giá đếm riêng, không cộng.
func estimate(sp domain.ShoppingPlan) (*int64, int) {
	missing := len(sp.MissingPrice)
	var total domain.VND
	switch {
	case sp.Recommend == "single" && sp.SingleStore != nil:
		total = sp.SingleStore.Total
	case len(sp.Split) > 0:
		total = sp.SplitTotal
	default:
		return nil, missing
	}
	v := int64(total)
	return &v, missing
}

func toPrices(sp domain.ShoppingPlan) Prices {
	out := Prices{
		Split: []Stop{}, SplitTotal: int64(sp.SplitTotal), Saving: int64(sp.Saving), Recommend: sp.Recommend,
		MissingPrice: nonNil(sp.MissingPrice), SingleSource: nonNil(sp.SingleSource), SaleOffers: []any{},
	}
	if out.Saving < 0 {
		out.Saving = 0
	}
	if sp.SingleStore != nil {
		out.SingleStore = &Stop{Store: sp.SingleStore.Store, Groups: nonNil(sp.SingleStore.Groups), Total: int64(sp.SingleStore.Total)}
	}
	for _, st := range sp.Split {
		g := append([]string(nil), st.Groups...)
		sort.Strings(g)
		out.Split = append(out.Split, Stop{Store: st.Store, Groups: g, Total: int64(st.Total)})
	}
	return out
}

func cheapest(m map[string]domain.VND) (string, domain.VND, bool) {
	var best string
	var price domain.VND = -1
	for st, p := range m {
		if price < 0 || p < price || (p == price && st < best) {
			best, price = st, p
		}
	}
	return best, price, price >= 0
}

// openOnly bỏ các dòng đang giao cho người khác mua: không vào ước tính và Nơi mua.
func openOnly(lines []Line) []Line {
	out := make([]Line, 0, len(lines))
	for _, l := range lines {
		if !l.Assigned() {
			out = append(out, l)
		}
	}
	return out
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

func leadDays(p authz.Principal) int {
	if p.LeadDays >= 1 && p.LeadDays <= 30 {
		return p.LeadDays
	}
	return 7
}
