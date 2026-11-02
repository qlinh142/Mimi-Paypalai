package market

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeDemand []Wanted

func (f fakeDemand) WantedItems(context.Context) ([]Wanted, error) { return f, nil }

type fakeStore struct {
	fresh map[string]bool
	saved []Entry
}

func (f *fakeStore) FreshKeys(context.Context, time.Time) (map[string]bool, error) {
	return f.fresh, nil
}
func (f *fakeStore) SaveEntry(_ context.Context, e Entry) error {
	f.saved = append(f.saved, e)
	return nil
}
func (f *fakeStore) Entries(context.Context, []string) (map[string]Entry, error) { return nil, nil }

type fakeSource struct {
	name   string
	prices map[string][]Price
	err    error
	calls  []string
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Lookup(_ context.Context, w Wanted) ([]Price, error) {
	f.calls = append(f.calls, w.Key)
	if f.err != nil {
		return nil, f.err
	}
	return f.prices[w.Key], nil
}

var now = time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)

func TestSyncDedupesSkipsFreshAndRespectsLimit(t *testing.T) {
	d := fakeDemand{
		{Name: "Sữa tươi Vinamilk 1L"}, {Name: "sữa tươi vinamilk 1l"}, // 2 nhà, 1 món
		{Name: "Trứng gà 10 quả"},
		{Name: "Nước mắm Nam Ngư"}, // đã có giá mới
		{Name: "Gạo ST25 5kg"},
	}
	st := &fakeStore{fresh: map[string]bool{Key("Nước mắm Nam Ngư"): true}}
	src := &fakeSource{name: "gemini_search", prices: map[string][]Price{
		Key("Sữa tươi Vinamilk 1L"): {{Store: "WinMart", PriceVND: 32_000}},
	}}
	res, err := Sync(context.Background(), d, st, []Source{src}, Options{Limit: 2, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Wanted != 4 || res.Fresh != 1 || res.Looked != 2 || res.Skipped != 1 || res.Found != 1 {
		t.Fatalf("%+v", res)
	}
	// Món nhiều nhà cần được tra trước.
	if src.calls[0] != Key("Sữa tươi Vinamilk 1L") {
		t.Fatalf("order %v", src.calls)
	}
	if len(st.saved) != 2 || st.saved[0].Prices[0].Source != "gemini_search" || st.saved[0].Prices[0].On != "2026-11-03" {
		t.Fatalf("saved %+v", st.saved)
	}
	if len(st.saved[1].Prices) != 0 { // không tìm ra giá: ghi dòng rỗng để nghỉ FreshFor
		t.Fatalf("empty entry %+v", st.saved[1])
	}
}

func TestSyncFallsBackToNextSourceAndStopsWhenDown(t *testing.T) {
	partner := &fakeSource{name: "partner", err: errors.New("503")}
	g := &fakeSource{name: "gemini_search", prices: map[string][]Price{"a": {{Store: "X", PriceVND: 10_000}}}}
	st := &fakeStore{}
	if _, err := Sync(context.Background(), fakeDemand{{Key: "a", Name: "A"}}, st, []Source{partner, g}, Options{Now: now}); err != nil {
		t.Fatal(err)
	}
	if len(st.saved) != 1 || st.saved[0].Prices[0].Source != "gemini_search" {
		t.Fatalf("%+v", st.saved)
	}

	var many fakeDemand
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		many = append(many, Wanted{Key: n, Name: n})
	}
	down := &fakeSource{name: "gemini_search", err: errors.New("quota")}
	res, err := Sync(context.Background(), many, &fakeStore{}, []Source{down}, Options{Now: now})
	if !errors.Is(err, ErrSourceDown) || res.Looked != maxConsecutiveErrors {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestCleanDropsBadPricesAndKeepsCheapestPerStore(t *testing.T) {
	got := Clean([]Price{
		{Store: "WinMart", PriceVND: 33_000, URL: "javascript:alert(1)"},
		{Store: "winmart ", PriceVND: 31_000},
		{Store: "Bách Hóa Xanh", PriceVND: 29_900, URL: "https://bhx.vn/sua"},
		{Store: "", PriceVND: 20_000},
		{Store: "Lạ", PriceVND: 5},
		{Store: "Đắt", PriceVND: 90_000_000},
	}, "gemini_search", "2026-11-03")
	if len(got) != 2 || got[0].Store != "Bách Hóa Xanh" || got[1].PriceVND != 31_000 || got[1].URL != "" {
		t.Fatalf("%+v", got)
	}
}

type fakeRefreshStore struct {
	fakeStore
	mu      sync.Mutex
	touched map[string]time.Time
}

func (f *fakeRefreshStore) Touched(context.Context, []string) (map[string]time.Time, error) {
	return f.touched, nil
}
func (f *fakeRefreshStore) SaveEntry(ctx context.Context, e Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fakeStore.SaveEntry(ctx, e)
}

type fakeQuota struct{ used, limit int }

func (q *fakeQuota) ReservePriceRefresh(_ context.Context, _, _ string, limit int) error {
	if q.used >= limit {
		return errors.New("quota")
	}
	q.used++
	return nil
}

type safeSource struct {
	mu     sync.Mutex
	prices map[string][]Price
	calls  []string
}

func (s *safeSource) Name() string { return "gemini_search" }
func (s *safeSource) Lookup(_ context.Context, w Wanted) ([]Price, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, w.Key)
	return s.prices[w.Key], nil
}

func TestRefreshSkipsRecentAndCountsQuota(t *testing.T) {
	st := &fakeRefreshStore{touched: map[string]time.Time{
		"sua-hat": now.Add(-2 * time.Hour),  // vừa tra: giữ
		"trung":   now.Add(-20 * time.Hour), // cũ: tra lại
	}}
	src := &safeSource{prices: map[string][]Price{"trung": {{Store: "WinMart", PriceVND: 29_000}}}}
	q := &fakeQuota{}
	r := NewRefresher(st, []Source{src}, q, RefreshOptions{Daily: 1, Now: func() time.Time { return now }})
	items := []Wanted{{Key: "sua-hat", Name: "Sữa hạt"}, {Key: "trung", Name: "Trứng"}, {Key: "gao", Name: "Gạo"}, {Key: "trung", Name: "Trứng"}}
	res, err := r.Refresh(context.Background(), "u1", "2026-11-03", items)
	if err != nil {
		t.Fatal(err)
	}
	if res.Wanted != 3 || res.Fresh != 1 || res.Looked != 2 || res.Found != 1 || q.used != 1 || len(st.saved) != 2 {
		t.Fatalf("%+v used=%d saved=%d", res, q.used, len(st.saved))
	}
	// Hết lượt trong ngày thì báo lỗi, không gọi nguồn.
	st.touched = nil
	src.calls = nil
	if _, err := r.Refresh(context.Background(), "u1", "2026-11-03", items); err == nil || len(src.calls) != 0 {
		t.Fatalf("want quota error, calls=%v", src.calls)
	}
	// Mọi món đều vừa tra: không tốn lượt.
	st.touched = map[string]time.Time{"sua-hat": now, "trung": now, "gao": now}
	if res, err := r.Refresh(context.Background(), "u1", "2026-11-03", items); err != nil || res.Fresh != 3 {
		t.Fatalf("%+v %v", res, err)
	}
}
