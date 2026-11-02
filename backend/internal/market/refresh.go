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
	DefaultRefreshFresh    = 12 * time.Hour // món đã tra trong 12 giờ thì giữ giá đó
	DefaultRefreshMax      = 12             // tối đa số món mỗi lần bấm
	DefaultRefreshParallel = 6              // tra song song để kịp trong giới hạn 60 s của Hosting
	DefaultRefreshDaily    = 5              // số lần bấm mỗi người mỗi ngày
)

// RefreshStore là bảng giá chung kèm thời điểm tra gần nhất của từng món (kể cả món tra
// không ra giá), để biết món nào vừa tra.
type RefreshStore interface {
	Store
	Touched(ctx context.Context, keys []string) (map[string]time.Time, error)
}

// RefreshQuota đếm số lần bấm mỗi ngày. Hết lượt: trả lỗi quota_exceeded.
type RefreshQuota interface {
	ReservePriceRefresh(ctx context.Context, uid, day string, limit int) error
}

type RefreshOptions struct {
	FreshFor time.Duration
	MaxItems int
	Parallel int
	Daily    int
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
	wanted, _ := dedupe(items)
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
	if err := r.quota.ReservePriceRefresh(ctx, uid, day, r.o.Daily); err != nil {
		return res, err
	}
	stamp := now.Format("2006-01-02")
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, r.o.Parallel)
	)
	for _, w := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func(w Wanted) {
			defer func() { <-sem; wg.Done() }()
			prices, err := lookup(ctx, r.srcs, w, stamp)
			mu.Lock()
			res.Looked++
			mu.Unlock()
			if err != nil {
				r.o.Log.WarnContext(ctx, "price refresh lookup failed", "key", w.Key, "err", err)
				mu.Lock()
				res.Errors++
				mu.Unlock()
				return
			}
			// Ghi bằng context riêng: người dùng đóng màn hình giữa chừng vẫn giữ giá đã tra.
			sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := r.st.SaveEntry(sctx, Entry{Key: w.Key, Name: w.Name, GroupKey: w.GroupKey, Prices: prices, UpdatedAt: now}); err != nil {
				r.o.Log.ErrorContext(ctx, "price refresh save failed", "key", w.Key, "err", err)
				mu.Lock()
				res.Errors++
				mu.Unlock()
				return
			}
			if len(prices) > 0 {
				mu.Lock()
				res.Found++
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	return res, nil
}
