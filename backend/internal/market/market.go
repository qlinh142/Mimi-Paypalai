// Package market giữ bảng giá chung (ADR-004, mẹ duyệt 02/11/2026).
//
//   - Một bảng cho mọi nhà: market_prices/{key}, key là tên món chuẩn hóa (domain.Slug).
//   - Chỉ lấy từ nguồn công khai (đối tác khi có, tra Google bằng Gemini). Giá trên hóa đơn
//     của một nhà KHÔNG vào bảng này: nó chỉ dùng cho chính nhà đó (package pricing).
//   - Danh sách món cần giá chỉ có tên món, không kèm ai cần.
//   - Job price-sync chạy mỗi đêm: món nào đã có giá trong FreshFor thì bỏ qua; mỗi đêm tối
//     đa Limit món để giữ chi phí cố định.
package market

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Price là giá một món tại một nơi bán, lấy từ một nguồn.
type Price struct {
	Store    string `json:"store"`
	PriceVND int64  `json:"price_vnd"`
	Size     string `json:"size,omitempty"` // quy cách nguồn ghi (1L, 900g...), để người dùng tự so
	Source   string `json:"source"`         // gemini_search | partner_...
	URL      string `json:"url,omitempty"`
	On       string `json:"on"` // YYYY-MM-DD ngày lấy giá
}

// Entry là một dòng bảng giá chung: các giá hiện có của một món.
type Entry struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	GroupKey  string    `json:"group_key,omitempty"`
	Prices    []Price   `json:"prices"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Wanted là một món có nhà đang cần mua (chỉ tên, không kèm ai cần).
type Wanted struct {
	Key      string
	Name     string
	GroupKey string
}

// Demand liệt kê món đang nằm trong kế hoạch (dòng mở) của mọi nhà, có thể trùng nhau.
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

// Source là một nguồn giá công khai. Không tìm thấy: trả nil, nil.
type Source interface {
	Name() string
	Lookup(ctx context.Context, w Wanted) ([]Price, error)
}

// Defaults của job (ADR-004): giá giữ 3 ngày, tối đa 200 món mỗi đêm.
const (
	DefaultLimit    = 200
	DefaultFreshFor = 72 * time.Hour
	// HistoryRetention: lịch sử giá mỗi món giữ 30 ngày gần nhất (mẹ chốt 02/11/2026).
	HistoryRetention = 30 * 24 * time.Hour
	maxPrices        = 5
	minVND, maxVND   = 1_000, 50_000_000
	// Lỗi liên tiếp quá ngưỡng thì dừng sớm: nguồn đang hỏng, chạy tiếp chỉ tốn tiền.
	maxConsecutiveErrors = 5
)

// Options điều chỉnh một lượt Sync.
type Options struct {
	Limit    int
	FreshFor time.Duration
	Now      time.Time
	Log      *slog.Logger
}

// Result tóm tắt một lượt Sync (ghi log).
type Result struct {
	Wanted  int // số món khác nhau đang được cần
	Fresh   int // bỏ qua vì đã có giá mới
	Looked  int // đã tra
	Found   int // tra ra giá
	Skipped int // còn lại vì chạm Limit, đêm sau tra tiếp
	Errors  int
}

// ErrSourceDown: nhiều lỗi liên tiếp, dừng lượt này.
var ErrSourceDown = errors.New("market: price source keeps failing")

// Key chuẩn hóa tên món thành khóa bảng giá chung.
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
	wanted, counts := dedupe(all)
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
	// Món nhiều nhà cần được tra trước; cùng số nhà thì theo key cho ổn định.
	sort.SliceStable(todo, func(i, j int) bool {
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
	for _, w := range todo {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		res.Looked++
		prices, err := lookup(ctx, srcs, w, day)
		if err != nil {
			res.Errors++
			streak++
			o.Log.WarnContext(ctx, "price lookup failed", "key", w.Key, "err", err)
			if streak >= maxConsecutiveErrors {
				return res, ErrSourceDown
			}
			continue
		}
		streak = 0
		// Không tìm ra giá vẫn ghi dòng rỗng: món đó nghỉ FreshFor rồi mới tra lại, không tốn lượt mỗi đêm.
		if len(prices) > 0 {
			res.Found++
		}
		if err := st.SaveEntry(ctx, Entry{Key: w.Key, Name: w.Name, GroupKey: w.GroupKey, Prices: prices, UpdatedAt: o.Now}); err != nil {
			return res, fmt.Errorf("market: save %s: %w", w.Key, err)
		}
	}
	return res, nil
}

// lookup chạy các nguồn theo thứ tự ưu tiên; nguồn đầu tiên có giá hợp lệ thắng.
func lookup(ctx context.Context, srcs []Source, w Wanted, day string) ([]Price, error) {
	var firstErr error
	for _, s := range srcs {
		ps, err := s.Lookup(ctx, w)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", s.Name(), err)
			}
			continue
		}
		if ps = Clean(ps, s.Name(), day); len(ps) > 0 {
			return ps, nil
		}
	}
	return nil, firstErr
}

// Clean bỏ giá vô lý, gộp trùng nơi bán (giữ giá thấp nhất), sắp rẻ trước, tối đa 5 giá.
func Clean(ps []Price, source, day string) []Price {
	best := map[string]Price{}
	for _, p := range ps {
		p.Store = strings.TrimSpace(p.Store)
		if p.Store == "" || p.PriceVND < minVND || p.PriceVND > maxVND {
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

func dedupe(all []Wanted) ([]Wanted, map[string]int) {
	counts := map[string]int{}
	var out []Wanted
	for _, w := range all {
		w.Name = strings.TrimSpace(w.Name)
		if w.Key == "" {
			w.Key = Key(w.Name)
		}
		if w.Key == "" || w.Name == "" {
			continue
		}
		if counts[w.Key] == 0 {
			out = append(out, w)
		}
		counts[w.Key]++
	}
	return out, counts
}
