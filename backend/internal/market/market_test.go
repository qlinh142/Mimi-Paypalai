package market

import (
	"context"
	"errors"
	"fmt"
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
func (f *fakeSource) Lookup(_ context.Context, _ Country, items []Wanted) (map[string][]Price, error) {
	out := map[string][]Price{}
	for _, w := range items {
		f.calls = append(f.calls, w.Key)
		if f.err == nil && f.prices[w.Key] != nil {
			out[w.Key] = f.prices[w.Key]
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return out, nil
}

var now = time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)

func TestSyncDedupesSkipsFreshAndRespectsLimit(t *testing.T) {
	d := fakeDemand{
		{Name: "Sữa tươi Vinamilk 1L"}, {Name: "sữa tươi vinamilk 1l"}, // 2 nhà, 1 món
		{Name: "Trứng gà 10 quả"},
		{Name: "Nước mắm Nam Ngư"}, // đã có giá mới
		{Name: "Gạo ST25 5kg"},
	}
	st := &fakeStore{fresh: map[string]bool{ItemID("VN", "Nước mắm Nam Ngư"): true}}
	src := &fakeSource{name: "gemini_search", prices: map[string][]Price{
		ItemID("VN", "Sữa tươi Vinamilk 1L"): {{Store: "WinMart", PriceVND: 32_000}},
	}}
	res, err := Sync(context.Background(), d, st, []Source{src}, Options{Limit: 2, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Wanted != 4 || res.Fresh != 1 || res.Looked != 2 || res.Skipped != 1 || res.Found != 1 {
		t.Fatalf("%+v", res)
	}
	// Món nhiều nhà cần được tra trước.
	if src.calls[0] != ItemID("VN", "Sữa tươi Vinamilk 1L") {
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
	for i := 0; i < 40; i++ { // 8 nhóm 5 món, mỗi nhóm một ngành hàng riêng không cần
		n := fmt.Sprintf("m%02d", i)
		many = append(many, Wanted{Key: n, Name: n})
	}
	down := &fakeSource{name: "gemini_search", err: errors.New("quota")}
	res, err := Sync(context.Background(), many, &fakeStore{}, []Source{down}, Options{Now: now})
	if !errors.Is(err, ErrSourceDown) || res.Errors != maxConsecutiveErrors || res.Looked != maxConsecutiveErrors*BatchSize {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestNormalizeVNDropsBadPricesAndKeepsCheapestPerStore(t *testing.T) {
	got, _ := Normalize(context.Background(), []Price{
		{Store: "WinMart", PriceVND: 33_000, URL: "javascript:alert(1)"},
		{Store: "winmart ", PriceVND: 31_000},
		{Store: "Bách Hóa Xanh", PriceVND: 29_900, URL: "https://bhx.vn/sua"},
		{Store: "", PriceVND: 20_000},
		{Store: "Lạ", PriceVND: 5},
		{Store: "Đắt", PriceVND: 90_000_000},
		{Store: "Shop lẻ", PriceVND: 10_000, URL: "https://shopee.vn/sua-123"},          // người bán lẻ trên Shopee: bỏ
		{Store: "Vinamilk Official Mall", PriceVND: 30_000, URL: "https://shopee.vn/v"}, // gian hàng chính hãng: nhận
	}, Countries["VN"], nil, "gemini_search", "2026-11-03")
	if len(got) != 3 || got[0].Store != "Bách Hóa Xanh" || got[1].Store != "Vinamilk Official Mall" || got[2].PriceVND != 31_000 || got[2].URL != "" || got[0].Currency != "VND" {
		t.Fatalf("%+v", got)
	}
}

type fakeFX map[string]float64

func (f fakeFX) VNDPer(_ context.Context, cur, _ string) (float64, bool, error) {
	r, ok := f[cur]
	return r, ok, nil
}

func TestNormalizeSGConvertsToVND(t *testing.T) {
	ps := []Price{{Store: "FairPrice", Local: 4.5}, {Store: "Lạ", Local: 0.01}}
	got, err := Normalize(context.Background(), ps, Countries["SG"], fakeFX{"SGD": 19_500}, "gemini_search", "2026-11-03")
	if err != nil || len(got) != 1 || got[0].PriceVND != 87_750 || got[0].Local != 4.5 || got[0].Currency != "SGD" {
		t.Fatalf("%+v %v", got, err)
	}
	// Chưa có tỷ giá: không đoán, bỏ giá.
	if got, _ := Normalize(context.Background(), ps, Countries["SG"], fakeFX{}, "gemini_search", "2026-11-03"); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestCountryForAndItemID(t *testing.T) {
	if CountryFor("sg", "") != "SG" || CountryFor("", "Asia/Singapore") != "SG" || CountryFor("XX", "Asia/Ho_Chi_Minh") != "VN" ||
		CountryFor("us", "") != "US" || CountryFor("", "America/Los_Angeles") != "US" {
		t.Fatal("country")
	}
	if ItemID("SG", "Sữa tươi 1L") != "sg_sua-tuoi-1l" || ItemID("", "Sữa tươi 1L") != "vn_sua-tuoi-1l" {
		t.Fatal(ItemID("SG", "Sữa tươi 1L"))
	}
}

func TestBatchesGroupByCountryAndCategory(t *testing.T) {
	var items []Wanted
	for i := 0; i < 7; i++ {
		items = append(items, Wanted{Key: fmt.Sprint(i), Country: "VN", Category: "sua"})
	}
	items = append(items, Wanted{Key: "x", Country: "VN", Category: "rau"}, Wanted{Key: "y", Country: "SG", Category: "sua"})
	b := batches(items)
	if len(b) != 4 || len(b[0]) != 5 || len(b[1]) != 2 || b[2][0].Key != "x" || b[3][0].Key != "y" {
		t.Fatalf("%+v", b)
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

type fakeQuota struct {
	used int
	last time.Time
}

func (q *fakeQuota) ReservePriceRefresh(_ context.Context, _, _ string, limit int, at time.Time) error {
	if q.used >= limit {
		return errors.New("quota")
	}
	q.used++
	q.last = at
	return nil
}
func (q *fakeQuota) PriceRefreshStatus(context.Context, string, string) (int, time.Time, error) {
	return q.used, q.last, nil
}

type safeSource struct {
	mu     sync.Mutex
	prices map[string][]Price
	calls  []string
}

func (s *safeSource) Name() string { return "gemini_search" }
func (s *safeSource) Lookup(_ context.Context, _ Country, items []Wanted) (map[string][]Price, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string][]Price{}
	for _, w := range items {
		s.calls = append(s.calls, w.Key)
		out[w.Key] = s.prices[w.Key]
	}
	return out, nil
}

func TestRefreshSkipsRecentAndCountsQuota(t *testing.T) {
	st := &fakeRefreshStore{touched: map[string]time.Time{
		"sua-hat": now.Add(-20 * time.Hour), // trong 24 giờ: giữ
		"trung":   now.Add(-30 * time.Hour), // quá 24 giờ: tra lại
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
	if st, err := r.Status(context.Background(), "u1", "2026-11-03"); err != nil || st.Left != 0 || st.Daily != 1 || st.LastAt == nil || !st.LastAt.Equal(now) {
		t.Fatalf("%+v %v", st, err)
	}
}

type memBook map[string]SourceRecord

func (b memBook) Sources(_ context.Context, country string) ([]SourceRecord, error) {
	var out []SourceRecord
	for _, r := range b {
		if r.Country == country {
			out = append(out, r)
		}
	}
	return out, nil
}
func (b memBook) Update(_ context.Context, country, domain string, apply func(SourceRecord) SourceRecord) error {
	b[country+"_"+domain] = apply(b[country+"_"+domain])
	return nil
}

func TestSourceBookLearnsPagesAndFlagsMisses(t *testing.T) {
	ctx := context.Background()
	book := memBook{}
	sua := []Wanted{{Key: "vn_sua", Name: "Sữa tươi", Category: "dairy"}}
	priced := map[string][]Price{"vn_sua": {{Store: "Bách Hóa Xanh", URL: "https://www.bachhoaxanh.com/sua-tuoi/abc"}}}
	found := []Found{{Store: "Bách Hóa Xanh", Kind: "category", Category: "dairy", URL: "https://www.bachhoaxanh.com/sua-tuoi"}}
	// Lần 1: web mới, ra giá 1 lần, chưa đủ tin.
	if err := Learn(ctx, book, "VN", nil, found, priced, now); err != nil {
		t.Fatal(err)
	}
	recs, _ := book.Sources(ctx, "VN")
	if len(recs) != 1 || recs[0].Active() || len(PagesFor(recs, sua, 5)) != 0 {
		t.Fatalf("%+v", recs)
	}
	// Lần 2: đủ 2 lần ra giá, lần sau đưa link ngành hàng cho AI.
	_ = Learn(ctx, book, "VN", nil, nil, priced, now)
	recs, _ = book.Sources(ctx, "VN")
	pages := PagesFor(recs, sua, 5)
	if len(pages) != 1 || pages[0].URL != "https://www.bachhoaxanh.com/sua-tuoi" {
		t.Fatalf("%+v", pages)
	}
	// Đọc trang 2 lần liền không ra giá: đánh dấu, không đưa trang nữa.
	_ = Learn(ctx, book, "VN", pages, nil, nil, now)
	_ = Learn(ctx, book, "VN", pages, nil, nil, now)
	recs, _ = book.Sources(ctx, "VN")
	if !recs[0].Flagged || len(PagesFor(recs, sua, 5)) != 0 {
		t.Fatalf("%+v", recs)
	}
	// Một tháng sau, ra giá lại thì bỏ đánh dấu.
	_ = Learn(ctx, book, "VN", nil, nil, priced, now.Add(RetryFlagged))
	recs, _ = book.Sources(ctx, "VN")
	if recs[0].Flagged {
		t.Fatalf("%+v", recs)
	}
}

func TestPagesForUsesSearchTemplate(t *testing.T) {
	recs := []SourceRecord{{Domain: "fairprice.com.sg", Store: "FairPrice", Hits: 3, SearchURL: "https://www.fairprice.com.sg/search?query={q}"}}
	pages := PagesFor(recs, []Wanted{{Name: "Fresh milk 1L"}}, 5)
	if len(pages) != 1 || pages[0].URL != "https://www.fairprice.com.sg/search?query=Fresh+milk+1L" {
		t.Fatalf("%+v", pages)
	}
	if u := PagesFor([]SourceRecord{{Hits: 2, SearchURL: "https://x.vn/s?k={q}"}}, []Wanted{{Name: "Sữa"}}, 5)[0].URL; u != "https://x.vn/s?k=S%E1%BB%AFa" {
		t.Fatal(u)
	}
}

// Nước cho chọn trong Cài đặt đều có trong bảng nước (mẹ chốt 03/11: Việt Nam, Mỹ).
func TestSelectableCountries(t *testing.T) {
	for _, c := range Selectable {
		if Countries[c].Code != c || Countries[c].Currency == "" || len(Countries[c].Shops) == 0 {
			t.Fatalf("thiếu nước %s", c)
		}
	}
	if Countries["US"].Currency != "USD" || Countries["SG"].Code != "SG" {
		t.Fatal(Countries["US"], Countries["SG"])
	}
}

// Anh chốt 09/11: món trong kế hoạch đang mở tra trước, rồi món sắp hết, dù món khác nhiều nhà có hơn.
func TestSyncPriority(t *testing.T) {
	now := time.Date(2026, 11, 9, 2, 0, 0, 0, time.UTC)
	d := fakeDemand{
		{Country: "VN", Name: "Gạo ST25"}, {Country: "VN", Name: "Gạo ST25"}, {Country: "VN", Name: "Gạo ST25"},
		{Country: "VN", Name: "Trứng gà", Priority: 1},
		{Country: "VN", Name: "Sữa tươi", Priority: 2},
		{Country: "VN", Name: "Trứng gà"}, // cùng món ở nhà khác, vẫn giữ ưu tiên cao nhất
	}
	src := &fakeSource{name: "gemini_search", prices: map[string][]Price{}}
	res, err := Sync(context.Background(), d, &fakeStore{}, []Source{src}, Options{Limit: 2, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Wanted != 3 || res.Looked != 2 || res.Skipped != 1 {
		t.Fatalf("%+v", res)
	}
	want := []string{ItemID("VN", "Sữa tươi"), ItemID("VN", "Trứng gà")}
	if fmt.Sprint(src.calls) != fmt.Sprint(want) {
		t.Fatalf("order %v, want %v", src.calls, want)
	}
}

type fakeAdditive struct{ fakeNamed }

func (fakeAdditive) Additive() bool { return true }

type fakeNamed struct {
	name string
	ps   map[string][]Price
	err  error
}

func (f fakeNamed) Name() string { return f.name }
func (f fakeNamed) Lookup(_ context.Context, _ Country, items []Wanted) (map[string][]Price, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[string][]Price{}
	for _, w := range items {
		if p, ok := f.ps[w.Key]; ok {
			out[w.Key] = p
		}
	}
	return out, nil
}

func TestLookupBatchMergesAdditiveSource(t *testing.T) {
	vn := []Wanted{{Key: "vn_omo", Country: "VN", Name: "Omo"}}
	gem := fakeNamed{name: "gemini_search", ps: map[string][]Price{"vn_omo": {{Store: "WinMart", PriceVND: 150000}}}}
	c3 := fakeAdditive{fakeNamed{name: "channel3", ps: map[string][]Price{"vn_omo": {{Store: "Walmart", Local: 120000}}}}}
	got, err := lookupBatch(context.Background(), []Source{gem, c3}, vn, nil, "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	ps := got["vn_omo"]
	if len(ps) != 2 || ps[0].Source != "channel3" || ps[1].Source != "gemini_search" {
		t.Fatalf("merged = %+v", ps)
	}
	// Nguồn bổ sung lỗi: vẫn giữ giá nguồn chính, không báo lỗi.
	c3.err = errors.New("down")
	got, err = lookupBatch(context.Background(), []Source{gem, c3}, vn, nil, "2026-11-09")
	if err != nil || len(got["vn_omo"]) != 1 {
		t.Fatalf("got %v err %v", got, err)
	}
	// Nguồn chính lỗi, bổ sung không ra giá: báo lỗi để thử lại sau.
	gem.err = errors.New("down")
	c3.err = nil
	c3.ps = nil
	if _, err = lookupBatch(context.Background(), []Source{gem, c3}, vn, nil, "2026-11-09"); err == nil {
		t.Fatal("want error when primary fails and additive finds nothing")
	}
}
