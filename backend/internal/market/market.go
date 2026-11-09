// Package market giữ bảng giá chung (ADR-004, mẹ duyệt 02/11/2026).
//
//   - Một bảng cho mọi nhà cùng nước: market_prices/{id}, id = nước + tên món chuẩn hóa
//     (ItemID, ví dụ "vn_sua-tuoi"). Nhà ở nước nào chỉ thấy giá nước đó (mẹ chốt 02/11).
//   - Chỉ lấy từ nguồn công khai (đối tác khi có, tra Google bằng Gemini). Giá trên hóa đơn
//     của một nhà KHÔNG vào bảng này: nó chỉ dùng cho chính nhà đó (package pricing).
//   - Danh sách món cần giá chỉ có tên món, không kèm ai cần.
//   - Món cần giá (anh chốt 09/11/2026): mọi món từng nằm trong tồn kho, hóa đơn đã quét hoặc
//     kế hoạch của mọi tài khoản. Tra theo thứ tự ưu tiên: món trong kế hoạch đang mở, món sắp
//     hết, rồi món nhiều nhà có.
//   - Job price-sync chạy mỗi đêm: món nào đã có giá trong FreshFor thì bỏ qua; mỗi đêm tối
//     đa Limit món để giữ chi phí cố định.
package market

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Price là giá một món tại một nơi bán, lấy từ một nguồn.
type Price struct {
	Store    string `json:"store"`
	PriceVND int64  `json:"price_vnd"` // quy ra VND để so với giá trên hóa đơn
	// Local, Currency: số tiền gốc nơi bán ghi (ví dụ 4.5 USD). Ở Việt Nam Local = PriceVND.
	Local    float64 `json:"local,omitempty"`
	Currency string  `json:"currency,omitempty"`
	Size     string  `json:"size,omitempty"` // quy cách nguồn ghi (1L, 900g...), để người dùng tự so
	Source   string  `json:"source"`         // gemini_search | partner_...
	URL      string  `json:"url,omitempty"`
	On       string  `json:"on"` // YYYY-MM-DD ngày lấy giá
}

// Entry là một dòng bảng giá chung: các giá hiện có của một món ở một nước.
type Entry struct {
	Key       string    `json:"key"` // ItemID
	Country   string    `json:"country,omitempty"`
	Name      string    `json:"name"`
	GroupKey  string    `json:"group_key,omitempty"`
	Prices    []Price   `json:"prices"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Wanted là một món có nhà đang cần mua (chỉ tên, nước, ngành hàng; không kèm ai cần).
type Wanted struct {
	Key      string // ItemID; rỗng thì tính từ Country + Name
	Country  string
	Name     string
	GroupKey string
	Category string // category_key, để gom món cùng ngành hàng và dùng link ngành hàng trong sổ nguồn
	// Priority: 2 món trong kế hoạch đang mở, 1 món sắp hết trong tồn kho, 0 còn lại.
	// Trong giới hạn mỗi đêm, món ưu tiên cao được tra trước.
	Priority int
}

// Demand liệt kê món cần có giá của mọi nhà (tồn kho, hóa đơn, kế hoạch), có thể trùng nhau.
type Demand interface {
	WantedItems(ctx context.Context) ([]Wanted, error)
}

// Store là bảng giá chung.
type Store interface {
	// FreshKeys: các key có giá cập nhật từ since trở đi.
	FreshKeys(ctx context.Context, since time.Time) (map[string]bool, error)
	SaveEntry(ctx context.Context, e Entry) error
	Entries(ctx context.Context, keys []string) (map[string]Entry, error)
}

// Source là một nguồn giá công khai. Mỗi lần hỏi một nhóm món cùng nước (tối đa BatchSize),
// trả giá theo Wanted.Key; món không tìm thấy thì vắng mặt.
type Source interface {
	Name() string
	Lookup(ctx context.Context, c Country, items []Wanted) (map[string][]Price, error)
}

// Defaults của job (ADR-004): giá giữ 3 ngày, tối đa 200 món mỗi đêm.
const (
	DefaultLimit    = 200
	DefaultFreshFor = 72 * time.Hour
	// HistoryRetention: lịch sử giá mỗi món giữ 30 ngày gần nhất (mẹ chốt 02/11/2026).
	HistoryRetention = 30 * 24 * time.Hour
	// BatchSize: số món mỗi lần hỏi AI (mẹ chốt 02/11: gom món cùng ngành hàng, tối đa 5).
	BatchSize = 5
	maxPrices = 5
	// Lỗi liên tiếp quá ngưỡng thì dừng sớm: nguồn đang hỏng, chạy tiếp chỉ tốn tiền.
	maxConsecutiveErrors = 5
)

// Options điều chỉnh một lượt Sync.
type Options struct {
	Limit    int
	FreshFor time.Duration
	Now      time.Time
	Log      *slog.Logger
	FX       FX // nil: chỉ nhận giá VND
}

// Result tóm tắt một lượt Sync (ghi log).
type Result struct {
	Wanted  int // số món khác nhau đang được cần
	Fresh   int // bỏ qua vì đã có giá mới
	Looked  int // đã tra
	Found   int // tra ra giá
	Skipped int // còn lại vì chạm Limit, đêm sau tra tiếp
	Errors  int // số lần hỏi nguồn bị lỗi (mỗi lần một nhóm món)
}

// ErrSourceDown: nhiều lỗi liên tiếp, dừng lượt này.
var ErrSourceDown = errors.New("market: price source keeps failing")

// Key chuẩn hóa tên món (chưa kèm nước).
func Key(name string) string { return domain.Slug(name) }

// Sync tra giá cho các món đang được cần mà chưa có giá mới, rồi ghi vào bảng chung.
func Sync(ctx context.Context, d Demand, st Store, srcs []Source, o Options) (Result, error) {
	if o.Limit <= 0 {
		o.Limit = DefaultLimit
	}
	if o.FreshFor <= 0 {
		o.FreshFor = DefaultFreshFor
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	var res Result
	all, err := d.WantedItems(ctx)
	if err != nil {
		return res, fmt.Errorf("market: wanted items: %w", err)
	}
	wanted, counts, prio := dedupe(all)
	res.Wanted = len(wanted)
	fresh, err := st.FreshKeys(ctx, o.Now.Add(-o.FreshFor))
	if err != nil {
		return res, fmt.Errorf("market: fresh keys: %w", err)
	}
	todo := wanted[:0]
	for _, w := range wanted {
		if fresh[w.Key] {
			res.Fresh++
			continue
		}
		todo = append(todo, w)
	}
	// Ưu tiên cao tra trước, rồi món nhiều nhà cần; cùng số nhà thì theo key cho ổn định.
	sort.SliceStable(todo, func(i, j int) bool {
		if prio[todo[i].Key] != prio[todo[j].Key] {
			return prio[todo[i].Key] > prio[todo[j].Key]
		}
		if counts[todo[i].Key] != counts[todo[j].Key] {
			return counts[todo[i].Key] > counts[todo[j].Key]
		}
		return todo[i].Key < todo[j].Key
	})
	if len(todo) > o.Limit {
		res.Skipped = len(todo) - o.Limit
		todo = todo[:o.Limit]
	}
	day := o.Now.Format("2006-01-02")
	streak := 0
	for _, b := range batches(todo) {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		res.Looked += len(b)
		got, err := lookupBatch(ctx, srcs, b, o.FX, day)
		if err != nil {
			res.Errors++
			streak++
			o.Log.WarnContext(ctx, "price lookup failed", "country", b[0].Country, "items", len(b), "err", err)
			if streak >= maxConsecutiveErrors {
				return res, ErrSourceDown
			}
			continue
		}
		streak = 0
		for _, w := range b {
			prices := got[w.Key]
			// Không tìm ra giá vẫn ghi dòng rỗng: món đó nghỉ FreshFor rồi mới tra lại, không tốn lượt mỗi đêm.
			if len(prices) > 0 {
				res.Found++
			}
			if err := st.SaveEntry(ctx, Entry{Key: w.Key, Country: w.Country, Name: w.Name, GroupKey: w.GroupKey, Prices: prices, UpdatedAt: o.Now}); err != nil {
				return res, fmt.Errorf("market: save %s: %w", w.Key, err)
			}
		}
	}
	return res, nil
}

// batches gom món theo nước rồi theo ngành hàng, mỗi nhóm tối đa BatchSize món. Giữ thứ tự
// ưu tiên: nhóm chứa món đứng trước được hỏi trước.
func batches(items []Wanted) [][]Wanted {
	type gk struct{ country, cat string }
	var order []gk
	groups := map[gk][]Wanted{}
	for _, w := range items {
		k := gk{w.Country, w.Category}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], w)
	}
	var out [][]Wanted
	for _, k := range order {
		g := groups[k]
		for len(g) > 0 {
			n := min(BatchSize, len(g))
			out = append(out, g[:n])
			g = g[n:]
		}
	}
	return out
}

// lookupBatch chạy các nguồn theo thứ tự ưu tiên cho một nhóm món cùng nước; món nào nguồn
// trước chưa ra giá hợp lệ thì hỏi tiếp nguồn sau. Lỗi chỉ trả khi mọi nguồn đều lỗi.
func lookupBatch(ctx context.Context, srcs []Source, items []Wanted, fx FX, day string) (map[string][]Price, error) {
	c := Countries[CountryFor(items[0].Country, "")]
	out := map[string][]Price{}
	left := items
	var firstErr error
	failed := 0
	var extra []Source
	primary, primaryFailed := 0, 0
	for _, s := range srcs {
		if a, ok := s.(Additive); ok && a.Additive() {
			extra = append(extra, s)
			continue
		}
		primary++
		if len(left) == 0 {
			continue
		}
		got, err := s.Lookup(ctx, c, left)
		if err != nil {
			failed++
			primaryFailed++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", s.Name(), err)
			}
			continue
		}
		var miss []Wanted
		for _, w := range left {
			ps, err := Normalize(ctx, got[w.Key], c, fx, s.Name(), day)
			if err != nil {
				return out, err
			}
			if len(ps) > 0 {
				out[w.Key] = ps
			} else {
				miss = append(miss, w)
			}
		}
		left = miss
	}
	// Nguồn bổ sung (Channel3): hỏi mọi món, giá ghép thêm cạnh giá nguồn chính. Lỗi chỉ ghi log
	// ở nơi gọi khi mọi nguồn đều lỗi; nguồn chính vẫn dùng được.
	for _, s := range extra {
		got, err := s.Lookup(ctx, c, items)
		if err != nil {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", s.Name(), err)
			}
			continue
		}
		for _, w := range items {
			ps, err := Normalize(ctx, got[w.Key], c, fx, s.Name(), day)
			if err != nil {
				return out, err
			}
			if len(ps) > 0 {
				out[w.Key] = merge(out[w.Key], ps)
			}
		}
	}
	if failed == len(srcs) && firstErr != nil {
		return out, firstErr
	}
	// Nguồn chính lỗi hết mà nguồn bổ sung cũng không ra giá nào: coi như lỗi để thử lại sau.
	if primary > 0 && primaryFailed == primary && len(out) == 0 && firstErr != nil {
		return out, firstErr
	}
	return out, nil
}

// Additive: nguồn bổ sung, được hỏi mọi món và ghép giá thêm, không chỉ hỏi món nguồn trước trượt.
type Additive interface {
	Additive() bool
}

// maxMerged: tối đa giá mỗi món khi ghép nhiều nguồn.
const maxMerged = 8

// merge ghép giá hai nguồn, sắp rẻ trước, tối đa maxMerged. Cùng cửa hàng ở hai nguồn thì giữ cả hai
// (mỗi nguồn có link riêng), người dùng chọn ở bước So sánh giá.
func merge(a, b []Price) []Price {
	out := append(append([]Price{}, a...), b...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].PriceVND < out[j].PriceVND })
	if len(out) > maxMerged {
		out = out[:maxMerged]
	}
	return out
}

// Normalize đổi giá nguồn trả về sang dạng lưu: kiểm tiền gốc theo nước, bỏ người bán lẻ
// trên Shopee/Lazada, quy ra VND, rồi Clean. Chưa có tỷ giá thì bỏ giá ngoại tệ.
func Normalize(ctx context.Context, ps []Price, c Country, fx FX, source, day string) ([]Price, error) {
	if len(ps) == 0 {
		return nil, nil
	}
	rate := 1.0
	if c.Currency != "VND" {
		if fx == nil {
			return nil, nil
		}
		r, ok, err := fx.VNDPer(ctx, c.Currency, day)
		if err != nil {
			return nil, fmt.Errorf("market: fx %s: %w", c.Currency, err)
		}
		if !ok || r <= 0 {
			return nil, nil
		}
		rate = r
	}
	keep := make([]Price, 0, len(ps))
	for _, p := range ps {
		if p.Local == 0 && c.Currency == "VND" {
			p.Local = float64(p.PriceVND)
		}
		if p.Local < c.MinLocal || p.Local > c.MaxLocal || math.IsNaN(p.Local) {
			continue
		}
		if isMarketplace(p.URL) && !isOfficialStore(p.Store) {
			continue
		}
		p.Currency = c.Currency
		p.PriceVND = int64(math.Round(p.Local * rate))
		if c.Currency == "VND" {
			p.Local = 0 // ở Việt Nam không cần lưu hai lần
		}
		keep = append(keep, p)
	}
	return Clean(keep, source, day), nil
}

// Clean gộp trùng nơi bán (giữ giá thấp nhất), bỏ link lạ, sắp rẻ trước, tối đa 5 giá.
// Kiểm giá vô lý theo nước nằm ở Normalize.
func Clean(ps []Price, source, day string) []Price {
	best := map[string]Price{}
	for _, p := range ps {
		p.Store = strings.TrimSpace(p.Store)
		if p.Store == "" || p.PriceVND <= 0 {
			continue
		}
		if !strings.HasPrefix(p.URL, "https://") && !strings.HasPrefix(p.URL, "http://") {
			p.URL = ""
		}
		p.Size = strings.TrimSpace(p.Size)
		p.Source, p.On = source, day
		k := domain.NormalizeVi(p.Store)
		if old, ok := best[k]; !ok || p.PriceVND < old.PriceVND {
			best[k] = p
		}
	}
	out := make([]Price, 0, len(best))
	for _, p := range best {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PriceVND != out[j].PriceVND {
			return out[i].PriceVND < out[j].PriceVND
		}
		return out[i].Store < out[j].Store
	})
	if len(out) > maxPrices {
		out = out[:maxPrices]
	}
	return out
}

func dedupe(all []Wanted) ([]Wanted, map[string]int, map[string]int) {
	counts, prio := map[string]int{}, map[string]int{}
	var out []Wanted
	for _, w := range all {
		w.Name = strings.TrimSpace(w.Name)
		w.Country = CountryFor(w.Country, "")
		if w.Key == "" {
			w.Key = ItemID(w.Country, w.Name)
		}
		if w.Key == "" || w.Name == "" {
			continue
		}
		if counts[w.Key] == 0 {
			out = append(out, w)
		}
		counts[w.Key]++
		prio[w.Key] = max(prio[w.Key], w.Priority)
	}
	return out, counts, prio
}
