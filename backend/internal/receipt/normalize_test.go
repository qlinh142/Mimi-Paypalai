package receipt

import (
	"context"
	"sync"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
)

type memPages struct {
	mu sync.Mutex
	m  map[string]Page
}

func (c *memPages) GetPage(_ context.Context, uid, hash string) (*Page, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.m[uid+hash]; ok {
		return &p, nil
	}
	return nil, nil
}

func (c *memPages) PutPage(_ context.Context, uid, hash string, p Page) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[uid+hash] = p
	return nil
}

// BUG-05 AC03: chỉ ảnh 2 mờ. Lỗi chỉ đúng ảnh 2; chụp lại ảnh 2 thì ảnh 1 không gọi AI lần nữa.
func TestRetakeOnlyBadImage(t *testing.T) {
	img1 := Image{append(append([]byte{}, jpeg...), 1), "image/jpeg"}
	img2 := Image{append(append([]byte{}, jpeg...), 2), "image/jpeg"}
	img2b := Image{append(append([]byte{}, jpeg...), 3), "image/jpeg"}
	var mu sync.Mutex
	reads := map[byte]int{}
	r := readerFunc(func(img Image) (Page, error) {
		mu.Lock()
		reads[img.Data[len(img.Data)-1]]++
		mu.Unlock()
		switch img.Data[len(img.Data)-1] {
		case 2:
			return Page{}, ErrUnreadable
		case 3:
			return Page{TotalPrinted: "64.000", Lines: []PageLine{{RawName: "Bánh mì", Qty: 1, Amount: "20.000"}}}, nil
		}
		return Page{Store: "WinMart", PurchasedOn: "2026-10-15", Lines: []PageLine{{RawName: "Sữa tươi 1L", Qty: 1, Amount: "44.000"}}}, nil
	})
	q := &fakeQuota{}
	s := newSvc(t, r, q, &fakeStore{})
	s.pages = &memPages{m: map[string]Page{}}

	_, err := s.Read(context.Background(), owner, "r-p1", "vi", []Image{img1, img2})
	f := fields(err)
	if code(err) != apperr.CodeUnreadableImage || f["image_index"] != 2 || q.used != 0 {
		t.Fatalf("err=%v fields=%v used=%d", err, f, q.used)
	}
	d, err := s.Read(context.Background(), owner, "r-p1", "vi", []Image{img1, img2b})
	if err != nil {
		t.Fatal(err)
	}
	if reads[1] != 1 || reads[3] != 1 || len(d.Lines) != 2 || d.Store != "WinMart" {
		t.Fatalf("reads=%v draft=%+v", reads, d)
	}
}

type readerFunc func(Image) (Page, error)

func (f readerFunc) ReadPage(_ context.Context, img Image, _ string) (Page, error) { return f(img) }

func TestReadLimitsAndNoItems(t *testing.T) {
	ok := readerFunc(func(Image) (Page, error) { return Page{}, ErrNoItems })
	s := newSvc(t, ok, &fakeQuota{}, &fakeStore{})
	if _, err := s.Read(context.Background(), owner, "r-n1", "vi", []Image{{jpeg, "image/jpeg"}}); code(err) != apperr.CodeNoItems {
		t.Fatalf("no items: %v", err)
	}
	many := make([]Image, MaxPages+1)
	for i := range many {
		many[i] = Image{jpeg, "image/jpeg"}
	}
	if _, err := s.Read(context.Background(), owner, "r-n2", "vi", many); code(err) != apperr.CodeTooManyImages {
		t.Fatalf("too many: %v", err)
	}
	big := Image{append(append([]byte{}, jpeg...), make([]byte, MaxPageBytes)...), "image/jpeg"}
	_, err := s.Read(context.Background(), owner, "r-n3", "vi", []Image{{jpeg, "image/jpeg"}, big})
	if code(err) != apperr.CodeImageTooLarge || fields(err)["image_index"] != 2 {
		t.Fatalf("too large: %v", err)
	}
}

// BUG-05 mục 6: chỉ bỏ dòng trùng khi >= 2 dòng liên tiếp giống hệt ở vùng nối hai ảnh.
func TestMergeOverlap(t *testing.T) {
	a := PageLine{RawName: "Sữa tươi", Qty: 1, Amount: "44.000"}
	b := PageLine{RawName: "Trứng gà 10 quả", Qty: 1, Amount: "38.000"}
	c := PageLine{RawName: "Bánh mì", Qty: 1, Amount: "20.000"}
	s := newSvc(t, nil, &fakeQuota{}, &fakeStore{})
	d := s.merge("r", []Page{{Lines: []PageLine{c, a, b}}, {Lines: []PageLine{a, b, c}}})
	if len(d.Lines) != 4 {
		t.Fatalf("two-line overlap should be dropped once: %d lines", len(d.Lines))
	}
	d = s.merge("r", []Page{{Lines: []PageLine{a, b}}, {Lines: []PageLine{b, c}}})
	if len(d.Lines) != 4 {
		t.Fatalf("single identical line must be kept: %d lines", len(d.Lines))
	}
}

// Tên chuẩn hóa một lần lúc đọc (7.1, 1.1), quy cách nhân combo (0.1), dòng chưa phân loại
// cần xem lại nhưng vẫn lưu được (6.1).
func TestDraftNormalization(t *testing.T) {
	pg := Page{Store: "WINMART", PurchasedOn: "2026-10-15", Lines: []PageLine{
		{RawName: "VINAMILK SUA BOT SURE DIACERNA 2X900G", Brand: "VINAMILK", ProductName: "SURE DIACERNA", Qty: 1, Amount: "1.330.000", GroupKey: "sua_bot", CategoryKey: "sua"},
		{RawName: "TAO FUJI", Qty: 1.2, Unit: "kg", Amount: "96.000", GroupKey: "tao"},
		{RawName: "CA FISH SPECIALS", Qty: 1, Amount: "120.000"},
	}}
	s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return pg, nil }}, &fakeQuota{}, &fakeStore{})
	d, err := s.Read(context.Background(), owner, "r-c1", "en", []Image{{jpeg, "image/jpeg"}})
	if err != nil {
		t.Fatal(err)
	}
	l := d.Lines[0]
	if l.Name != "Vinamilk Sure diacerna" || l.RawName != "VINAMILK SUA BOT SURE DIACERNA 2X900G" || l.ProductKey != "vinamilk-sure-diacerna" {
		t.Fatalf("name: %+v", l)
	}
	if l.CanonicalQty != 1800 || l.CanonicalUnit != "g" || l.GroupKey != "sua-bot" || l.NeedsReview {
		t.Fatalf("quantity: %+v", l)
	}
	if apple := d.Lines[1]; apple.CanonicalQty != 1200 || apple.CanonicalUnit != "g" {
		t.Fatalf("weighed goods: %+v", apple)
	}
	if fish := d.Lines[2]; !fish.NeedsReview || len(fish.ReviewReasons) != 2 {
		t.Fatalf("unclassified line: %+v", fish)
	}
	if d.TotalMatch != nil {
		t.Fatal("no printed total: total_match must be null")
	}

	// Lưu: dòng chưa phân loại vẫn lưu, chi tiêu có, nhưng không vào tồn kho.
	st := &fakeStore{saved: map[string]Receipt{}}
	s = newSvc(t, nil, &fakeQuota{}, st)
	req := SaveRequest{ReceiptID: "r-c2", Store: d.Store, PurchasedOn: d.PurchasedOn, Currency: "VND", UseForPrediction: true}
	for _, dl := range d.Lines {
		req.Lines = append(req.Lines, SaveLine{
			LineID: dl.LineID, Name: dl.Name, Qty: dl.Qty, Unit: dl.Unit, Amount: dl.Amount, CategoryKey: dl.CategoryKey,
			GroupKey: dl.GroupKey, Habit: true, RawName: dl.RawName, Brand: dl.Brand, ProductName: dl.ProductName,
			PackageCount: dl.PackageCount, PackageSize: dl.PackageSize, PackageUnit: dl.PackageUnit,
		})
	}
	if _, err := s.Save(context.Background(), owner, req); err != nil {
		t.Fatal(err)
	}
	r := st.saved["r-c2"]
	if r.StoreKey != "winmart" || r.TotalVND != 1_546_000 {
		t.Fatalf("receipt %+v", r)
	}
	if !r.Lines[0].ForInventory() || r.Lines[0].CanonicalQty != 1800 || r.Lines[2].ForInventory() || !r.Lines[2].NeedsReview {
		t.Fatalf("lines %+v", r.Lines)
	}
	// Táo chưa có danh mục chi tiêu: vẫn lưu, cần xem lại.
	if !r.Lines[1].NeedsReview {
		t.Fatalf("missing category must need review: %+v", r.Lines[1])
	}
}
