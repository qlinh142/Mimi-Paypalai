package market

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Nút "Cập nhật giá" (mẹ hỏi 02/11): người dùng tự tra lại giá cho các món trong kế hoạch
// đang mở, không phải chờ job đêm. Chỉ tra món của kế hoạch đó, bỏ qua món vừa có giá, và
// giới hạn số lần bấm mỗi ngày vì mỗi món là một lượt Gemini có Google Search.
const (
	DefaultRefreshFresh    = 24 * time.Hour // món đã tra trong 24 giờ thì giữ giá đó (mẹ chốt 02/11)
	DefaultRefreshMax      = 12             // tối đa số món mỗi lần bấm
	DefaultRefreshParallel = 3              // số nhóm (mỗi nhóm tối đa 5 món) tra song song, kịp giới hạn 60 s của Hosting
	DefaultRefreshDaily    = 3              // số lần bấm mỗi người mỗi ngày (mẹ chốt 02/11)
)

// RefreshStore là bảng giá chung kèm thời điểm tra gần nhất của từng món (kể cả món tra
// không ra giá), để biết món nào vừa tra.
type RefreshStore interface {
	Store
	Touched(ctx context.Context, keys []string) (map[string]time.Time, error)
}

// RefreshQuota đếm số lần bấm mỗi ngày và nhớ lần bấm gần nhất. Hết lượt: trả lỗi quota_exceeded.
type RefreshQuota interface {
	ReservePriceRefresh(ctx context.Context, uid, day string, limit int, at time.Time) error
	PriceRefreshStatus(ctx context.Context, uid, day string) (used int, last time.Time, err error)
}

// RefreshStatus là phần app hiện dưới nút: còn mấy lần hôm nay, lần bấm gần nhất.
type RefreshStatus struct {
	Daily  int        `json:"daily"`
	Left   int        `json:"left"`
	LastAt *time.Time `json:"last_at,omitempty"`
}

// Status đọc lượt còn lại trong ngày và lần bấm gần nhất của một người.
func (r *Refresher) Status(ctx context.Context, uid, day string) (RefreshStatus, error) {
	used, last, err := r.quota.PriceRefreshStatus(ctx, uid, day)
	st := RefreshStatus{Daily: r.o.Daily, Left: max(r.o.Daily-used, 0)}
	if !last.IsZero() {
		st.LastAt = &last
	}
	return st, err
}

type RefreshOptions struct {
	FreshFor time.Duration
	MaxItems int
	Parallel int
	Daily    int
	FX       FX // nil: chỉ nhận giá VND
	Log      *slog.Logger
	Now      func() time.Time
}

type Refresher struct {
	st    RefreshStore
	srcs  []Source
	quota RefreshQuota
	o     RefreshOptions
}

func NewRefresher(st RefreshStore, srcs []Source, q RefreshQuota, o RefreshOptions) *Refresher {
	if o.FreshFor <= 0 {
		o.FreshFor = DefaultRefreshFresh
	}
	if o.MaxItems <= 0 {
		o.MaxItems = DefaultRefreshMax
	}
	if o.Parallel <= 0 {
		o.Parallel = DefaultRefreshParallel
	}
	if o.Daily <= 0 {
		o.Daily = DefaultRefreshDaily
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Refresher{st: st, srcs: srcs, quota: q, o: o}
}

// Refresh tra lại giá cho các món chưa tra trong FreshFor. day là ngày theo giờ của người
// dùng, dùng để đếm lượt. Không có món nào cần tra thì không tốn lượt.
func (r *Refresher) Refresh(ctx context.Context, uid, day string, items []Wanted) (Result, error) {
	var res Result
	wanted, _, _ := dedupe(items)
	res.Wanted = len(wanted)
	if len(wanted) == 0 {
		return res, nil
	}
	keys := make([]string, len(wanted))
	for i, w := range wanted {
		keys[i] = w.Key
	}
	touched, err := r.st.Touched(ctx, keys)
	if err != nil {
		return res, fmt.Errorf("market: touched: %w", err)
	}
	now := r.o.Now()
	var todo []Wanted
	for _, w := range wanted {
		if t, ok := touched[w.Key]; ok && now.Sub(t) < r.o.FreshFor {
			res.Fresh++
			continue
		}
		todo = append(todo, w)
	}
	if len(todo) > r.o.MaxItems {
		res.Skipped = len(todo) - r.o.MaxItems
		todo = todo[:r.o.MaxItems]
	}
	if len(todo) == 0 {
		return res, nil
	}
	if err := r.quota.ReservePriceRefresh(ctx, uid, day, r.o.Daily, now); err != nil {
		return res, err
	}
	stamp := now.Format("2006-01-02")
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, r.o.Parallel)
	)
	// Gom theo ngành hàng, mỗi nhóm tối đa BatchSize món, các nhóm chạy song song.
	for _, b := range batches(todo) {
		wg.Add(1)
		sem <- struct{}{}
		go func(b []Wanted) {
			defer func() { <-sem; wg.Done() }()
			got, err := lookupBatch(ctx, r.srcs, b, r.o.FX, stamp)
			mu.Lock()
			res.Looked += len(b)
			mu.Unlock()
			if err != nil {
				r.o.Log.WarnContext(ctx, "price refresh lookup failed", "items", len(b), "err", err)
				mu.Lock()
				res.Errors++
				mu.Unlock()
				return
			}
			// Ghi bằng context riêng: người dùng đóng màn hình giữa chừng vẫn giữ giá đã tra.
			sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			for _, w := range b {
				prices := got[w.Key]
				if err := r.st.SaveEntry(sctx, Entry{Key: w.Key, Country: w.Country, Name: w.Name, GroupKey: w.GroupKey, Prices: prices, UpdatedAt: now}); err != nil {
					r.o.Log.ErrorContext(ctx, "price refresh save failed", "key", w.Key, "err", err)
					mu.Lock()
					res.Errors++
					mu.Unlock()
					continue
				}
				if len(prices) > 0 {
					mu.Lock()
					res.Found++
					mu.Unlock()
				}
			}
		}(b)
	}
	wg.Wait()
	return res, nil
}
