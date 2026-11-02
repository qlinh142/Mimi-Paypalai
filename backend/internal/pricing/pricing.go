// Package pricing là module so giá dùng chung cho Đồ dùng và Kế hoạch (8.1 mục 8, AC10).
//
//   - Giá so sánh là giá chuẩn hóa: tiền đã trả / canonical quantity (35.000đ / 1000 ml =
//     35 đ/ml). Không so tổng tiền khi quy cách khác nhau (mục 5).
//   - Chỉ so cùng group_key, cùng chiều đo, cùng tiền tệ VND, canonical quantity > 0 (6.2).
//   - Nguồn duy nhất là hóa đơn người dùng đã lưu: không tra giá online (AC09).
//   - Giỏ chỉ có tổng một nơi khi nơi đó có giá cho mọi món (mục 11); xếp hạng ưu tiên đủ
//     dữ liệu, rồi dữ liệu mới, rồi tổng (mục 18).
package pricing

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/item"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
)

// WindowDays: cửa sổ lịch sử mặc định (mục 16).
const WindowDays = 90

// Observation là một lần mua đã lưu của một nhóm.
type Observation struct {
	GroupKey      string      `json:"-"`
	Store         string      `json:"store_name"`
	StoreKey      string      `json:"store_key,omitempty"`
	PaidVND       int64       `json:"paid_price"`
	On            domain.Date `json:"purchased_at"`
	CanonicalQty  float64     `json:"canonical_quantity"`
	CanonicalUnit domain.Unit `json:"canonical_unit"`
	UnitPrice     float64     `json:"unit_price"` // đ / canonical unit
	DetailName    string      `json:"detail_name,omitempty"`
}

// Comparable: đủ điều kiện so giá (6.2).
func (o Observation) Comparable() bool {
	return o.PaidVND > 0 && o.CanonicalQty > 0 && o.CanonicalUnit != ""
}

func storeID(o Observation) string {
	if o.StoreKey != "" {
		return o.StoreKey
	}
	k, _ := domain.StoreKey(o.Store)
	return k
}

// ItemHistory là phản hồi GET /items/{key}/price-history (14.1).
type ItemHistory struct {
	GroupKey         string        `json:"group_key"`
	PurchaseCount    int           `json:"purchase_count"`
	Unit             domain.Unit   `json:"canonical_unit,omitempty"`
	LastPurchase     *Observation  `json:"last_purchase"`
	LowestHistorical *Observation  `json:"lowest_historical"`
	AverageUnitPrice *float64      `json:"average_unit_price"` // null khi chỉ có một lần mua (Case B)
	LastStore        string        `json:"last_store,omitempty"`
	LowestStore      string        `json:"lowest_price_store,omitempty"`
	OtherDimension   int           `json:"other_dimension_count"` // lần mua khác chiều đo, không so (Case D)
	History          []Observation `json:"history"`
	WindowDays       int           `json:"window_days"`
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// History tính chỉ số giá của một nhóm. Chiều đo so sánh là chiều đo của lần mua gần nhất.
func History(key string, obs []Observation) ItemHistory {
	out := ItemHistory{GroupKey: key, History: []Observation{}, WindowDays: WindowDays}
	var cmp []Observation
	for _, o := range obs {
		if o.Comparable() {
			o.UnitPrice = round2(float64(o.PaidVND) / o.CanonicalQty)
			cmp = append(cmp, o)
		}
	}
	sort.SliceStable(cmp, func(i, j int) bool { return cmp[i].On.After(cmp[j].On) })
	if len(cmp) == 0 {
		return out
	}
	out.Unit = cmp[0].CanonicalUnit
	var sum float64
	for _, o := range cmp {
		if o.CanonicalUnit != out.Unit {
			out.OtherDimension++
			continue
		}
		out.History = append(out.History, o)
		sum += o.UnitPrice
		if out.LowestHistorical == nil || o.UnitPrice < out.LowestHistorical.UnitPrice {
			x := o
			out.LowestHistorical = &x
		}
	}
	out.PurchaseCount = len(out.History)
	last := out.History[0]
	out.LastPurchase, out.LastStore = &last, last.Store
	out.LowestStore = out.LowestHistorical.Store
	if out.PurchaseCount >= 2 {
		avg := round2(sum / float64(out.PurchaseCount))
		out.AverageUnitPrice = &avg
	} else {
		out.LowestHistorical = nil // một lần mua: chỉ "giá mua gần nhất"
		out.LowestStore = ""
	}
	return out
}

// BasketLine là một món đang mở của Kế hoạch.
type BasketLine struct {
	Key  string
	Qty  float64
	Unit string // đơn vị dòng Kế hoạch (kg, hop...)
}

type StoreOption struct {
	StoreName      string   `json:"store_name"`
	StoreKey       string   `json:"store_key"`
	CoveredItems   int      `json:"covered_items"`
	TotalItems     int      `json:"total_items"`
	EstimatedTotal *int64   `json:"estimated_total"` // null khi chưa đủ dữ liệu cho cả giỏ (mục 11)
	Subtotal       int64    `json:"covered_subtotal"`
	MissingKeys    []string `json:"missing_keys"`
	LatestOn       string   `json:"latest_purchase_at"`
}

type SplitStop struct {
	StoreName string   `json:"store_name"`
	Keys      []string `json:"keys"`
	Total     int64    `json:"total"`
}

type SplitOption struct {
	EstimatedTotal int64       `json:"estimated_total"`
	Stores         []SplitStop `json:"stores"`
	CoveredItems   int         `json:"covered_items"`
}

// LineEstimate: giá ước tính của một món ở nơi rẻ nhất đã biết.
type LineEstimate struct {
	Key       string   `json:"key"`
	Estimate  *int64   `json:"estimate"`
	Store     string   `json:"store_name,omitempty"`
	UnitPrice *float64 `json:"unit_price"`
	Unit      string   `json:"canonical_unit,omitempty"`
	Basis     string   `json:"basis,omitempty"` // quantity | last_package
}

// Basket là phản hồi GET /plan/price-history (14.2).
type Basket struct {
	BasketItemCount int            `json:"basket_item_count"`
	StoreOptions    []StoreOption  `json:"store_options"`
	Split           *SplitOption   `json:"split_store_option"`
	Recommend       string         `json:"recommend"` // single | split | none
	Lines           []LineEstimate `json:"lines"`
	NoHistory       []string       `json:"no_history"`
	WindowDays      int            `json:"window_days"`
}

// cost ước tính tiền một món ở một nơi: giá chuẩn hóa gần nhất × lượng cần mua (khi đổi
// được sang cùng chiều đo), hoặc giá một lần mua gần nhất ở đó × số lượng (mua như lần trước).
func cost(l BasketLine, last Observation) (int64, string) {
	qty := l.Qty
	if qty <= 0 {
		qty = 1
	}
	if base, unit := domain.ToBase(qty, l.Unit); l.Unit != "" && unit == last.CanonicalUnit && base > 0 {
		return int64(math.Round(base * float64(last.PaidVND) / last.CanonicalQty)), "quantity"
	}
	return int64(math.Round(qty * float64(last.PaidVND))), "last_package"
}

// EstimateBasket dựng các phương án một nơi và chia nhiều nơi.
func EstimateBasket(lines []BasketLine, obs map[string][]Observation, today domain.Date) Basket {
	out := Basket{BasketItemCount: len(lines), StoreOptions: []StoreOption{}, Lines: []LineEstimate{}, NoHistory: []string{}, WindowDays: WindowDays, Recommend: "none"}
	since := today.AddDays(-WindowDays)
	type storeAgg struct {
		opt    StoreOption
		latest domain.Date
	}
	stores := map[string]*storeAgg{}
	type pick struct {
		store string
		cost  int64
	}
	best := map[string]pick{}
	for _, l := range lines {
		h := History(l.Key, obs[l.Key])
		le := LineEstimate{Key: l.Key, Unit: string(h.Unit)}
		// Giá gần nhất ở mỗi nơi trong cửa sổ, cùng chiều đo với lần mua gần nhất.
		perStore := map[string]Observation{}
		for _, o := range h.History {
			if o.On.Before(since) {
				continue
			}
			id := storeID(o)
			if id == "" {
				continue
			}
			if cur, ok := perStore[id]; !ok || o.On.After(cur.On) {
				perStore[id] = o
			}
		}
		if len(perStore) == 0 {
			out.NoHistory = append(out.NoHistory, l.Key)
			out.Lines = append(out.Lines, le)
			continue
		}
		for id, o := range perStore {
			c, basis := cost(l, o)
			a := stores[id]
			if a == nil {
				a = &storeAgg{opt: StoreOption{StoreName: o.Store, StoreKey: id, MissingKeys: []string{}}}
				stores[id] = a
			}
			a.opt.CoveredItems++
			a.opt.Subtotal += c
			if o.On.After(a.latest) {
				a.latest = o.On
			}
			if b, ok := best[l.Key]; !ok || c < b.cost || (c == b.cost && id < b.store) {
				best[l.Key] = pick{store: id, cost: c}
				v, up := c, o.UnitPrice
				le.Estimate, le.Store, le.UnitPrice, le.Basis = &v, o.Store, &up, basis
			}
		}
		out.Lines = append(out.Lines, le)
	}
	for _, a := range stores {
		a.opt.TotalItems = len(lines)
		a.opt.LatestOn = a.latest.String()
		if a.opt.CoveredItems == len(lines) {
			v := a.opt.Subtotal
			a.opt.EstimatedTotal = &v
		}
		for _, l := range lines {
			if _, ok := findStore(obs[l.Key], a.opt.StoreKey, since); !ok {
				a.opt.MissingKeys = append(a.opt.MissingKeys, l.Key)
			}
		}
		out.StoreOptions = append(out.StoreOptions, a.opt)
	}
	sort.Slice(out.StoreOptions, func(i, j int) bool {
		a, b := out.StoreOptions[i], out.StoreOptions[j]
		if a.CoveredItems != b.CoveredItems {
			return a.CoveredItems > b.CoveredItems
		}
		if (a.EstimatedTotal != nil) && (b.EstimatedTotal != nil) && *a.EstimatedTotal != *b.EstimatedTotal {
			return *a.EstimatedTotal < *b.EstimatedTotal
		}
		if a.LatestOn != b.LatestOn {
			return a.LatestOn > b.LatestOn
		}
		return a.StoreKey < b.StoreKey
	})
	if len(best) > 0 {
		split := &SplitOption{Stores: []SplitStop{}, CoveredItems: len(best)}
		byStore := map[string]*SplitStop{}
		for _, l := range lines {
			b, ok := best[l.Key]
			if !ok {
				continue
			}
			st := byStore[b.store]
			if st == nil {
				st = &SplitStop{StoreName: stores[b.store].opt.StoreName}
				byStore[b.store] = st
			}
			st.Keys = append(st.Keys, l.Key)
			st.Total += b.cost
			split.EstimatedTotal += b.cost
		}
		for _, st := range byStore {
			split.Stores = append(split.Stores, *st)
		}
		sort.Slice(split.Stores, func(i, j int) bool { return split.Stores[i].Total > split.Stores[j].Total })
		out.Split = split
	}
	switch {
	case len(out.StoreOptions) > 0 && out.StoreOptions[0].EstimatedTotal != nil &&
		(out.Split == nil || len(out.Split.Stores) <= 1 || *out.StoreOptions[0].EstimatedTotal-out.Split.EstimatedTotal < 10_000):
		// Chia nhiều nơi chỉ đáng khi tiết kiệm đáng kể so với đi một nơi đủ món.
		out.Recommend = "single"
	case out.Split != nil:
		out.Recommend = "split"
	}
	return out
}

func findStore(obs []Observation, id string, since domain.Date) (Observation, bool) {
	for _, o := range obs {
		if o.Comparable() && !o.On.Before(since) && storeID(o) == id {
			return o, true
		}
	}
	return Observation{}, false
}

// Store đọc tối đa limit lần mua đã lưu của từng nhóm, mới nhất trước.
type Store interface {
	PriceObservations(ctx context.Context, uid string, groups []string, limit int) (map[string][]Observation, error)
}

// Số lần mua đọc cho mỗi nhóm. Mọi lần mua đều được lưu (không bao giờ xóa), nên Chi tiết
// một món đọc toàn bộ lịch sử để giá thấp nhất và giá trung bình tính trên mọi lần mua
// (lỗi V3); 500 lần là khoảng 10 năm nếu mua hằng tuần. Giỏ nhiều món chỉ cần giá gần đây.
const (
	ItemHistoryLimit = 500
	BasketPriceLimit = 30
)

// Lines là các dòng đang mở của Plan trong context (plan.IDFrom).
type Lines interface {
	OpenLines(ctx context.Context, uid string) ([]plan.Line, error)
}

type Service struct {
	store Store
	lines Lines
	now   func() time.Time
}

func New(store Store, lines Lines, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, lines: lines, now: now}
}

func (s *Service) Item(ctx context.Context, p authz.Principal, key string) (ItemHistory, error) {
	if err := authz.Can(p, authz.ActionReadItemsFull); err != nil {
		return ItemHistory{}, err
	}
	if !item.ValidKey(key) {
		return ItemHistory{}, apperr.ErrNotFound
	}
	obs, err := s.store.PriceObservations(ctx, p.UID, []string{key}, ItemHistoryLimit)
	if err != nil {
		return ItemHistory{}, err
	}
	return History(key, obs[key]), nil
}

func (s *Service) Basket(ctx context.Context, p authz.Principal) (Basket, error) {
	if err := authz.Can(p, authz.ActionReadPlan); err != nil {
		return Basket{}, err
	}
	all, err := s.lines.OpenLines(ctx, p.UID)
	if err != nil {
		return Basket{}, err
	}
	var lines []BasketLine
	var keys []string
	for _, l := range all {
		if l.Assigned() {
			continue
		}
		lines = append(lines, BasketLine{Key: l.Key, Qty: l.Qty, Unit: l.Unit})
		keys = append(keys, l.Key)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Key < lines[j].Key })
	obs := map[string][]Observation{}
	if len(keys) > 0 {
		if obs, err = s.store.PriceObservations(ctx, p.UID, keys, BasketPriceLimit); err != nil {
			return Basket{}, err
		}
	}
	return EstimateBasket(lines, obs, domain.Today(s.now(), domain.Location(p.Timezone))), nil
}
