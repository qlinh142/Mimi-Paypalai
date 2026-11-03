package receipt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// ---- fakes: hiện thực tối thiểu các interface, không mock thư viện ----

type fakeReader struct {
	mu    sync.Mutex
	calls int
	fn    func(call int) (Page, error)
}

func (f *fakeReader) ReadPage(ctx context.Context, img Image, lang string) (Page, error) {
	f.mu.Lock()
	f.calls++
	c := f.calls
	f.mu.Unlock()
	return f.fn(c)
}

type fakeDrafts struct{ m map[string]Draft }

func (f *fakeDrafts) Get(_ context.Context, uid, id string) (*Draft, error) {
	if d, ok := f.m[uid+id]; ok {
		return &d, nil
	}
	return nil, nil
}
func (f *fakeDrafts) Put(_ context.Context, uid string, d Draft) error {
	f.m[uid+d.ReceiptID] = d
	return nil
}

type fakeQuota struct{ used, released int }

func (f *fakeQuota) Reserve(_ context.Context, uid, day string, limit int) error {
	if f.used >= limit {
		return apperr.New(apperr.CodeQuotaExceeded, "hết lượt")
	}
	f.used++
	return nil
}
func (f *fakeQuota) Release(_ context.Context, uid, day string) error {
	f.used--
	f.released++
	return nil
}

type fakeFX struct{ rates map[string]string }

func (f fakeFX) Rate(_ context.Context, cur string, _ domain.Date) (FX, bool, error) {
	r, ok := f.rates[cur]
	return FX{Rate: r, Source: "test"}, ok, nil
}

type fakeStore struct {
	saved   map[string]Receipt
	similar bool
}

func (f *fakeStore) SaveReceipt(_ context.Context, uid string, r Receipt, _ *time.Location) (SaveOutcome, error) {
	if old, ok := f.saved[r.ID]; ok {
		if old.PayloadHash != r.PayloadHash {
			return SaveOutcome{}, apperr.New(apperr.CodeReceiptConflict, "khác nội dung")
		}
		return SaveOutcome{ReceiptID: r.ID, Replayed: true}, nil
	}
	f.saved[r.ID] = r
	return SaveOutcome{ReceiptID: r.ID}, nil
}
func (f *fakeStore) HasSimilar(context.Context, string, string, string) (bool, error) {
	return f.similar, nil
}

var jpeg = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}
var owner = authz.Principal{UID: "u1", Role: authz.RoleOwner, Timezone: "Asia/Ho_Chi_Minh"}

func newSvc(t *testing.T, r Reader, q *fakeQuota, st *fakeStore) *Service {
	t.Helper()
	fixed := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	s, err := New(r, &fakeDrafts{m: map[string]Draft{}}, q, fakeFX{rates: map[string]string{"SGD": "18450.25"}}, st,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithClock(func() time.Time { return fixed }),
		WithTaxonomy([]string{"sua", "hoa-pham"}),
		WithPageTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func fields(err error) map[string]any {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Fields
	}
	return nil
}

func code(err error) apperr.Code {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestRead(t *testing.T) {
	page := Page{Store: "Co.opmart", PurchasedOn: "2026-10-15", TotalPrinted: "102.000", Lines: []PageLine{
		{RawName: "Sữa tươi", Qty: 2, Amount: "64.000", CategoryKey: "sua"},
		{RawName: "Nước rửa chén", Qty: 1, Amount: "38,000", CategoryKey: "khong-co"},
	}}

	t.Run("parses VND, checks total, drops unknown category", func(t *testing.T) {
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return page, nil }}, &fakeQuota{}, &fakeStore{})
		d, err := s.Read(context.Background(), owner, "r-1", "vi", []Image{{jpeg, "image/jpeg"}})
		if err != nil {
			t.Fatal(err)
		}
		if d.SumVND != 102000 || d.TotalMismatch {
			t.Fatalf("sum %d mismatch %v", d.SumVND, d.TotalMismatch)
		}
		if d.Lines[1].CategoryKey != "" {
			t.Fatalf("unknown category kept: %q", d.Lines[1].CategoryKey)
		}
	})

	t.Run("category follows the product group when AI gives none", func(t *testing.T) {
		pg := Page{Store: "X", PurchasedOn: "2026-10-15", Lines: []PageLine{
			{RawName: "Nước rửa chén Sunlight 750ml", Qty: 1, Amount: "38000", GroupKey: "nuoc_rua_chen"},
		}}
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return pg, nil }}, &fakeQuota{}, &fakeStore{})
		d, err := s.Read(context.Background(), owner, "r-cat", "vi", []Image{{jpeg, "image/jpeg"}})
		if err != nil {
			t.Fatal(err)
		}
		if l := d.Lines[0]; l.CategoryKey != "hoa-pham" || l.NeedsReview {
			t.Fatalf("category %q needs_review %v", l.CategoryKey, l.NeedsReview)
		}
	})

	t.Run("translated name shown first, bill name kept as original_name", func(t *testing.T) {
		pg := Page{Store: "WinMart", PurchasedOn: "2026-10-15", Lines: []PageLine{
			{RawName: "Nước rửa chén Sunlight 750ml", Translated: "Sunlight dish soap 750ml", SourceLang: "vi", Qty: 1, Amount: "38000"},
			{RawName: "Sữa tươi", Qty: 1, Amount: "30000"},
		}}
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return pg, nil }}, &fakeQuota{}, &fakeStore{})
		d, err := s.Read(context.Background(), owner, "r-tr", "en", []Image{{jpeg, "image/jpeg"}})
		if err != nil {
			t.Fatal(err)
		}
		if l := d.Lines[0]; l.Name != "Sunlight dish soap 750ml" || l.OriginalName == "" || l.RawName != "Nước rửa chén Sunlight 750ml" {
			t.Fatalf("translated line %+v", l)
		}
		if l := d.Lines[1]; l.OriginalName != "" {
			t.Fatalf("untranslated line has original_name %q", l.OriginalName)
		}
	})

	t.Run("replay returns cached draft without calling AI or quota", func(t *testing.T) {
		r := &fakeReader{fn: func(int) (Page, error) { return page, nil }}
		q := &fakeQuota{}
		s := newSvc(t, r, q, &fakeStore{})
		for i := 0; i < 2; i++ {
			if _, err := s.Read(context.Background(), owner, "r-2", "vi", []Image{{jpeg, "image/jpeg"}}); err != nil {
				t.Fatal(err)
			}
		}
		if r.calls != 1 || q.used != 1 {
			t.Fatalf("calls=%d used=%d", r.calls, q.used)
		}
	})

	t.Run("transient failure retried once then succeeds", func(t *testing.T) {
		r := &fakeReader{fn: func(c int) (Page, error) {
			if c == 1 {
				return Page{}, fmt.Errorf("429: %w", ErrTransient)
			}
			return page, nil
		}}
		s := newSvc(t, r, &fakeQuota{}, &fakeStore{})
		if _, err := s.Read(context.Background(), owner, "r-3", "vi", []Image{{jpeg, "image/jpeg"}}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("AI down releases quota", func(t *testing.T) {
		q := &fakeQuota{}
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return Page{}, ErrTransient }}, q, &fakeStore{})
		_, err := s.Read(context.Background(), owner, "r-4", "vi", []Image{{jpeg, "image/jpeg"}})
		if code(err) != apperr.CodeReadFailed || q.released != 1 || q.used != 0 {
			t.Fatalf("err=%v released=%d used=%d", err, q.released, q.used)
		}
	})

	t.Run("unreadable image names the image and frees quota", func(t *testing.T) {
		q := &fakeQuota{}
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return Page{}, ErrUnreadable }}, q, &fakeStore{})
		_, err := s.Read(context.Background(), owner, "r-5", "vi", []Image{{jpeg, "image/jpeg"}})
		if code(err) != apperr.CodeUnreadableImage || q.used != 0 || fields(err)["image_index"] != 1 {
			t.Fatalf("err=%v used=%d", err, q.used)
		}
	})

	t.Run("AI rejecting the request is not blamed on the photo", func(t *testing.T) {
		q := &fakeQuota{}
		calls := 0
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) {
			calls++
			return Page{}, fmt.Errorf("gemini: 400 bad schema: %w", ErrAIRejected)
		}}, q, &fakeStore{})
		_, err := s.Read(context.Background(), owner, "r-5b", "vi", []Image{{jpeg, "image/jpeg"}})
		if code(err) != apperr.CodeAIUnavailable || q.used != 0 || calls != 1 {
			t.Fatalf("err=%v used=%d calls=%d", err, q.used, calls)
		}
	})

	t.Run("content type not trusted", func(t *testing.T) {
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return page, nil }}, &fakeQuota{}, &fakeStore{})
		_, err := s.Read(context.Background(), owner, "r-6", "vi", []Image{{[]byte("<svg>"), "image/jpeg"}})
		if code(err) != apperr.CodeUnsupportedMedia {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("foreign currency without rate is flagged, not guessed", func(t *testing.T) {
		pg := page
		pg.Currency = "JPY"
		pg.TotalPrinted = ""
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return pg, nil }}, &fakeQuota{}, &fakeStore{})
		d, err := s.Read(context.Background(), owner, "r-7", "vi", []Image{{jpeg, "image/jpeg"}})
		if err != nil || !d.FXMissing || d.Lines[0].AmountVND != nil || d.SumVND != 0 {
			t.Fatalf("err=%v draft=%+v", err, d)
		}
	})

	t.Run("signed out cannot read receipts", func(t *testing.T) {
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return page, nil }}, &fakeQuota{}, &fakeStore{})
		_, err := s.Read(context.Background(), authz.Principal{}, "r-8", "vi", []Image{{jpeg, "image/jpeg"}})
		if code(err) != apperr.CodeUnauthenticated {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestSave(t *testing.T) {
	req := SaveRequest{
		ReceiptID: "r-10", Store: "Co.opmart", PurchasedOn: "2026-10-15", Currency: "VND",
		UseForPrediction: true,
		Lines: []SaveLine{
			{LineID: "l1", Name: "Sữa tươi", Qty: 2, Amount: "64000", CategoryKey: "sua", Habit: true},
			{LineID: "l2", Name: "Bánh quy", Qty: 1, Amount: "74000", Habit: false},
		},
	}

	t.Run("server recomputes totals and replay is idempotent", func(t *testing.T) {
		st := &fakeStore{saved: map[string]Receipt{}}
		s := newSvc(t, nil, &fakeQuota{}, st)
		out, err := s.Save(context.Background(), owner, req)
		if err != nil || out.Replayed {
			t.Fatalf("first save: %v %+v", err, out)
		}
		if st.saved["r-10"].TotalVND != 138000 || st.saved["r-10"].Lines[1].Habit {
			t.Fatalf("saved %+v", st.saved["r-10"])
		}
		out, err = s.Save(context.Background(), owner, req)
		if err != nil || !out.Replayed {
			t.Fatalf("replay: %v %+v", err, out)
		}
	})

	t.Run("same id different content conflicts", func(t *testing.T) {
		st := &fakeStore{saved: map[string]Receipt{}}
		s := newSvc(t, nil, &fakeQuota{}, st)
		if _, err := s.Save(context.Background(), owner, req); err != nil {
			t.Fatal(err)
		}
		changed := req
		changed.Lines = append([]SaveLine(nil), req.Lines...)
		changed.Lines[0].Amount = "1"
		if _, err := s.Save(context.Background(), owner, changed); code(err) != apperr.CodeReceiptConflict {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("possible duplicate asks for confirmation", func(t *testing.T) {
		st := &fakeStore{saved: map[string]Receipt{}, similar: true}
		s := newSvc(t, nil, &fakeQuota{}, st)
		if _, err := s.Save(context.Background(), owner, req); code(err) != apperr.CodePossibleDuplicate {
			t.Fatalf("err=%v", err)
		}
		confirmed := req
		confirmed.ConfirmDuplicate = true
		if _, err := s.Save(context.Background(), owner, confirmed); err != nil {
			t.Fatal(err)
		}
	})

	// A7: ô giá trống là chưa rõ giá: vẫn lưu món, không tính 0đ, không cộng vào tổng.
	t.Run("blank amount is saved as unknown price", func(t *testing.T) {
		st := &fakeStore{saved: map[string]Receipt{}}
		s := newSvc(t, nil, &fakeQuota{}, st)
		r := req
		r.ReceiptID = "r-12"
		r.Lines = append(append([]SaveLine(nil), req.Lines...), SaveLine{LineID: "l9", Name: "Rau muống", Qty: 1, Amount: "  "})
		if _, err := s.Save(context.Background(), owner, r); err != nil {
			t.Fatal(err)
		}
		got := st.saved["r-12"]
		if got.TotalVND != 138000 || len(got.Lines) != 3 || got.Lines[2].VND != nil {
			t.Fatalf("saved %+v", got)
		}
		// B7: nguồn giá của từng dòng.
		if got.Lines[0].PriceSource != PriceReceipt || got.Lines[2].PriceSource != PriceMissing {
			t.Fatalf("price source %q %q", got.Lines[0].PriceSource, got.Lines[2].PriceSource)
		}
	})

	// B7: chỉ nhận receipt | manual | estimate; giá online để dành, chưa nhận từ app.
	t.Run("price source is validated", func(t *testing.T) {
		st := &fakeStore{saved: map[string]Receipt{}}
		s := newSvc(t, nil, &fakeQuota{}, st)
		r := req
		r.ReceiptID = "r-14"
		r.Lines = append([]SaveLine(nil), req.Lines...)
		r.Lines[0].PriceSource = PriceEstimate
		if _, err := s.Save(context.Background(), owner, r); err != nil {
			t.Fatal(err)
		}
		if got := st.saved["r-14"].Lines[0].PriceSource; got != PriceEstimate {
			t.Fatalf("got %q", got)
		}
		r.ReceiptID = "r-15"
		r.Lines[0].PriceSource = PriceOnline
		if _, err := s.Save(context.Background(), owner, r); apperr.From(err).Code != apperr.CodeValidation {
			t.Fatalf("online must be rejected for now, got %v", err)
		}
	})

	// B3: mỗi dòng mang danh sách Kế hoạch nó trừ vào.
	t.Run("per-line plans are validated and kept", func(t *testing.T) {
		st := &fakeStore{saved: map[string]Receipt{}}
		s := newSvc(t, nil, &fakeQuota{}, st)
		r := req
		r.ReceiptID = "r-13"
		ps := []PlanShare{{PlanID: "p-be", Qty: 1}, {PlanID: "", Qty: 1}}
		none := []PlanShare{}
		r.Lines = []SaveLine{req.Lines[0], req.Lines[1]}
		r.Lines[0].Plans, r.Lines[1].Plans = &ps, &none
		if _, err := s.Save(context.Background(), owner, r); err != nil {
			t.Fatal(err)
		}
		got := st.saved["r-13"].Lines
		if !got[0].PlanSet || len(got[0].Plans) != 2 || got[0].Plans[1].PlanID != "default" || !got[1].PlanSet || len(got[1].Plans) != 0 {
			t.Fatalf("plans: %+v", got)
		}
		dup := []PlanShare{{PlanID: "p-be"}, {PlanID: "p-be"}}
		bad := r
		bad.ReceiptID = "r-14"
		bad.Lines = []SaveLine{req.Lines[0]}
		bad.Lines[0].Plans = &dup
		if _, err := s.Save(context.Background(), owner, bad); code(err) != apperr.CodeValidation {
			t.Fatalf("kế hoạch lặp phải bị từ chối, got %v", err)
		}
	})

	t.Run("prediction switch off marks every line habit=false", func(t *testing.T) {
		st := &fakeStore{saved: map[string]Receipt{}}
		s := newSvc(t, nil, &fakeQuota{}, st)
		off := req
		off.ReceiptID = "r-11"
		off.UseForPrediction = false
		if _, err := s.Save(context.Background(), owner, off); err != nil {
			t.Fatal(err)
		}
		if st.saved["r-11"].Lines[0].Habit {
			t.Fatal("habit should be false")
		}
	})

	t.Run("validation", func(t *testing.T) {
		s := newSvc(t, nil, &fakeQuota{}, &fakeStore{saved: map[string]Receipt{}})
		bad := []struct {
			mut       func(*SaveRequest)
			code      apperr.Code
			field, id string
		}{
			{func(r *SaveRequest) { r.ReceiptID = "../x" }, apperr.CodeValidation, "receipt_id", ""},
			{func(r *SaveRequest) { r.PurchasedOn = "2027-01-01" }, apperr.CodeInvalidPurchaseDate, "purchased_on", ""}, // tương lai
			{func(r *SaveRequest) { r.PurchasedOn = "15/10/2026" }, apperr.CodeInvalidPurchaseDate, "purchased_on", ""},
			{func(r *SaveRequest) { r.Lines = nil }, apperr.CodeValidation, "lines", ""},
			{func(r *SaveRequest) { r.Lines = []SaveLine{{LineID: "l1", Name: "x", Qty: 1, Amount: "-5"}} }, apperr.CodeValidation, "lines", ""},
			{func(r *SaveRequest) { r.Lines = []SaveLine{{LineID: "l3", Name: "x", Qty: 0, Amount: "5"}} }, apperr.CodeInvalidQuantity, "qty", "l3"},
			{func(r *SaveRequest) {
				r.Lines = []SaveLine{{LineID: "l3", Name: "Cá", Qty: 1, Amount: "₫1,330,000abc"}}
			}, apperr.CodeInvalidLineTotal, "amount", "l3"},
		}
		for i, c := range bad {
			r := req
			r.ReceiptID = fmt.Sprintf("r-v%d", i)
			c.mut(&r)
			_, err := s.Save(context.Background(), owner, r)
			f := fields(err)
			if code(err) != c.code || f["field"] != c.field || (c.id != "" && f["line_id"] != c.id) {
				t.Errorf("case %d: err=%v fields=%v", i, err, f)
			}
		}
	})
}

func TestToVND(t *testing.T) {
	cases := []struct {
		in      string
		want    domain.VND
		neg, ok bool
	}{
		{"64.000", 64000, false, true},
		{"64,000", 64000, false, true},
		{"64.000đ", 64000, false, true},
		{"64.000 VNĐ", 64000, false, true},
		{"₫64.000", 64000, false, true},
		{"-10.000", 10000, true, true},
		{"(10.000)", 10000, true, true},
		{"10.000-", 10000, true, true},
		{"abc", 0, false, false},
		{"", 0, false, false},
	}
	for _, c := range cases {
		v, neg, ok := toVND(c.in, "VND", nil)
		if v != c.want || neg != c.neg || ok != c.ok {
			t.Errorf("toVND(%q) = %d,%v,%v want %d,%v,%v", c.in, v, neg, ok, c.want, c.neg, c.ok)
		}
	}
}

func TestReadDiscounts(t *testing.T) {
	t.Run("discount line is subtracted and total within tolerance matches", func(t *testing.T) {
		pg := Page{Store: "WinMart", PurchasedOn: "2026-10-15", TotalPrinted: "90.500đ", Lines: []PageLine{
			{RawName: "Sữa tươi", Qty: 2, Unit: "Hộp", Amount: "64.000đ", GroupKey: "sua_tuoi"},
			{RawName: "Nước rửa chén", Qty: 1, Amount: "38.000đ"},
			{RawName: "Giảm giá KM", Qty: 1, Amount: "-12.000đ"},
		}}
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return pg, nil }}, &fakeQuota{}, &fakeStore{})
		d, err := s.Read(context.Background(), owner, "r-d1", "vi", []Image{{jpeg, "image/jpeg"}})
		if err != nil {
			t.Fatal(err)
		}
		if d.SumVND != 90000 || d.TotalMismatch {
			t.Fatalf("sum %d mismatch %v", d.SumVND, d.TotalMismatch)
		}
		if !d.Lines[2].Discount || d.Lines[2].Habit || *d.Lines[2].AmountVND != -12000 {
			t.Fatalf("discount line %+v", d.Lines[2])
		}
		if d.Lines[0].Unit != "hop" || d.Lines[0].GroupKey != "sua-tuoi" {
			t.Fatalf("unit/group %+v", d.Lines[0])
		}
	})

	t.Run("gross lines with printed discount total match", func(t *testing.T) {
		pg := Page{TotalPrinted: "92.000", DiscountTotal: "-10.000", Lines: []PageLine{
			{RawName: "Sữa tươi", Qty: 2, Amount: "64.000"}, {RawName: "Bánh", Qty: 1, Amount: "38.000"},
		}}
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return pg, nil }}, &fakeQuota{}, &fakeStore{})
		d, err := s.Read(context.Background(), owner, "r-d2", "vi", []Image{{jpeg, "image/jpeg"}})
		if err != nil || d.TotalMismatch {
			t.Fatalf("err=%v mismatch=%v", err, d.TotalMismatch)
		}
	})

	t.Run("real mismatch is flagged", func(t *testing.T) {
		pg := Page{TotalPrinted: "150.000", Lines: []PageLine{{RawName: "Sữa tươi", Qty: 2, Amount: "64.000"}}}
		s := newSvc(t, &fakeReader{fn: func(int) (Page, error) { return pg, nil }}, &fakeQuota{}, &fakeStore{})
		d, _ := s.Read(context.Background(), owner, "r-d3", "vi", []Image{{jpeg, "image/jpeg"}})
		if !d.TotalMismatch {
			t.Fatal("expected mismatch")
		}
	})
}

func TestSaveDiscounts(t *testing.T) {
	req := SaveRequest{
		ReceiptID: "r-s1", Store: "WinMart", PurchasedOn: "2026-10-15", Currency: "VND", UseForPrediction: true,
		DiscountTotal: "-12.000đ",
		Lines: []SaveLine{
			{LineID: "l1", Name: "Sữa tươi", Qty: 2, Unit: "hop", Amount: "64.000đ", GroupKey: "sua_tuoi", Habit: true},
			{LineID: "l2", Name: "Nước rửa chén", Qty: 1, Amount: "38.000đ", Habit: true},
			{LineID: "l3", Name: "Giảm giá KM", Qty: 1, Amount: "-12.000đ"},
		},
	}
	st := &fakeStore{saved: map[string]Receipt{}}
	s := newSvc(t, nil, &fakeQuota{}, st)
	if _, err := s.Save(context.Background(), owner, req); err != nil {
		t.Fatal(err)
	}
	r := st.saved["r-s1"]
	if r.TotalVND != 90000 || r.DiscountVND != 12000 || len(r.Lines) != 2 {
		t.Fatalf("total %d discount %d lines %d", r.TotalVND, r.DiscountVND, len(r.Lines))
	}
	if r.Lines[0].GroupKey != "sua-tuoi" || r.Lines[0].Unit != "hop" {
		t.Fatalf("line %+v", r.Lines[0])
	}
	// Giảm giá cả hóa đơn chia về từng dòng theo tỷ lệ tiền: 64k/102k và 38k/102k của 12k.
	if r.Lines[0].DiscountVND+r.Lines[1].DiscountVND != 12000 || r.Lines[0].DiscountVND != 7529 {
		t.Fatalf("allocated %d %d", r.Lines[0].DiscountVND, r.Lines[1].DiscountVND)
	}
}
