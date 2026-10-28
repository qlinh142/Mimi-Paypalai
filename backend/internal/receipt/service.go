package receipt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Service điều phối đọc và lưu hóa đơn.
type Service struct {
	reader Reader
	drafts DraftCache
	quota  Quota
	fx     FXRates
	store  Store
	log    *slog.Logger
	now    func() time.Time

	dailyLimit   int
	maxPages     int
	maxPageBytes int
	maxLines     int
	pageTimeout  time.Duration
	retryBudget  time.Duration // chỉ retry nếu còn ít nhất chừng này thời gian
	taxonomy     map[string]bool
}

type Option func(*Service) error

func WithDailyLimit(n int) Option {
	return func(s *Service) error {
		if n < 1 {
			return errors.New("receipt: daily limit must be >= 1")
		}
		s.dailyLimit = n
		return nil
	}
}

func WithPageTimeout(d time.Duration) Option {
	return func(s *Service) error {
		if d <= 0 {
			return errors.New("receipt: page timeout must be > 0")
		}
		s.pageTimeout = d
		return nil
	}
}

func WithClock(now func() time.Time) Option {
	return func(s *Service) error { s.now = now; return nil }
}

// WithTaxonomy đặt danh sách khóa danh mục hợp lệ (27 danh mục). Khóa ngoài danh sách
// bị bỏ, món rơi vào "Cần phân loại".
func WithTaxonomy(keys []string) Option {
	return func(s *Service) error {
		s.taxonomy = make(map[string]bool, len(keys))
		for _, k := range keys {
			s.taxonomy[k] = true
		}
		return nil
	}
}

func New(reader Reader, drafts DraftCache, quota Quota, fx FXRates, store Store, log *slog.Logger, opts ...Option) (*Service, error) {
	s := &Service{
		reader: reader, drafts: drafts, quota: quota, fx: fx, store: store, log: log,
		now:          time.Now,
		dailyLimit:   20,
		maxPages:     8,
		maxPageBytes: 4 << 20,
		maxLines:     150,
		pageTimeout:  20 * time.Second,
		retryBudget:  25 * time.Second,
	}
	for _, o := range opts {
		if err := o(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// ---------- Đọc hóa đơn ----------

// Read đọc các ảnh của MỘT hóa đơn và trả bản nháp. Không lưu ảnh.
func (s *Service) Read(ctx context.Context, p authz.Principal, receiptID, lang string, imgs []Image) (Draft, error) {
	if err := authz.Can(p, authz.ActionWriteReceipts); err != nil {
		return Draft{}, err
	}
	if err := s.validateImages(receiptID, imgs); err != nil {
		return Draft{}, err
	}

	// Gửi lại cùng receipt_id: trả bản nháp đã có, không tốn lượt, không gọi lại AI.
	if cached, err := s.drafts.Get(ctx, p.UID, receiptID); err != nil {
		s.log.WarnContext(ctx, "draft cache get failed", "err", err)
	} else if cached != nil {
		return *cached, nil
	}

	loc := userLocation(p)
	day := domain.Today(s.now(), loc).String()
	if err := s.quota.Reserve(ctx, p.UID, day, s.dailyLimit); err != nil {
		return Draft{}, err
	}
	pages, unreadable, err := s.readPages(ctx, imgs, lang)
	if err != nil {
		// Lỗi đọc không trừ lượt của người dùng. Dùng context riêng vì ctx có thể đã hết hạn.
		relCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if rerr := s.quota.Release(relCtx, p.UID, day); rerr != nil {
			s.log.ErrorContext(ctx, "quota release failed", "err", rerr)
		}
		return Draft{}, err
	}

	d := s.merge(receiptID, pages)
	d.UnreadablePages = unreadable
	if err := s.applyFX(ctx, &d); err != nil {
		return Draft{}, err
	}
	if err := s.drafts.Put(ctx, p.UID, d); err != nil {
		s.log.WarnContext(ctx, "draft cache put failed", "err", err) // không chặn người dùng
	}
	return d, nil
}

var allowedMIME = map[string][]byte{
	"image/jpeg": {0xFF, 0xD8, 0xFF},
	"image/png":  {0x89, 'P', 'N', 'G'},
	"image/webp": []byte("RIFF"),
}

func (s *Service) validateImages(receiptID string, imgs []Image) error {
	if !validID(receiptID) {
		return apperr.New(apperr.CodeValidation, "Mã hóa đơn không hợp lệ.").With("field", "receipt_id")
	}
	if len(imgs) == 0 {
		return apperr.New(apperr.CodeValidation, "Chưa có ảnh hóa đơn.")
	}
	if len(imgs) > s.maxPages {
		return apperr.New(apperr.CodeValidation, fmt.Sprintf("Mỗi hóa đơn tối đa %d ảnh.", s.maxPages))
	}
	for i, img := range imgs {
		if len(img.Data) > s.maxPageBytes {
			return apperr.New(apperr.CodePayloadTooLarge, "Ảnh quá lớn.").With("page", i+1)
		}
		magic, ok := allowedMIME[img.MIME]
		if !ok || !bytes.HasPrefix(img.Data, magic) { // không tin Content-Type do client gửi
			return apperr.New(apperr.CodeUnsupportedMedia, "Chỉ nhận ảnh JPEG, PNG hoặc WebP.").With("page", i+1)
		}
	}
	return nil
}

// readPages đọc song song mọi trang; mỗi trang có timeout riêng, retry một lần với lỗi
// tạm thời nếu còn đủ thời gian trong ngân sách của request.
func (s *Service) readPages(ctx context.Context, imgs []Image, lang string) ([]Page, int, error) {
	pages := make([]Page, len(imgs))
	errs := make([]error, len(imgs))
	var wg sync.WaitGroup
	for i, img := range imgs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pages[i], errs[i] = s.readOne(ctx, img, lang)
		}()
	}
	wg.Wait()

	readable := 0
	for i, err := range errs {
		switch {
		case err == nil:
			readable++
		case errors.Is(err, ErrUnreadable):
			s.log.InfoContext(ctx, "page unreadable", "page", i+1)
		default:
			return nil, 0, apperr.Wrap(apperr.CodeAIUnavailable, "Mimi đang bận, thử lại sau ít phút.", err)
		}
	}
	if readable == 0 {
		return nil, 0, apperr.New(apperr.CodeReceiptUnreadable, "Mimi chưa đọc được hóa đơn này. Chụp lại rõ hơn, đủ sáng và thẳng.")
	}
	out := pages[:0]
	for i := range pages {
		if errs[i] == nil {
			out = append(out, pages[i])
		}
	}
	return out, len(imgs) - readable, nil
}

func (s *Service) readOne(ctx context.Context, img Image, lang string) (Page, error) {
	for attempt := 0; ; attempt++ {
		pctx, cancel := context.WithTimeout(ctx, s.pageTimeout)
		page, err := s.reader.ReadPage(pctx, img, lang)
		cancel()
		if err == nil || errors.Is(err, ErrUnreadable) || attempt >= 1 {
			return page, err
		}
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) < s.retryBudget {
			return page, err
		}
		select { // backoff ngắn, tôn trọng hủy request
		case <-ctx.Done():
			return page, ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
	}
}

// merge ghép các trang bằng code: cửa hàng và ngày lấy từ trang đầu tiên có thông tin,
// tổng in lấy từ trang cuối có tổng (thường là trang cuối của hóa đơn dài).
func (s *Service) merge(receiptID string, pages []Page) Draft {
	d := Draft{ReceiptID: receiptID, Currency: "VND", Pages: len(pages)}
	n := 0
	for _, pg := range pages {
		if d.Store == "" {
			d.Store = domain.CleanItemName(pg.Store)
		}
		if d.PurchasedOn == "" {
			if _, err := domain.ParseDate(pg.PurchasedOn); err == nil {
				d.PurchasedOn = pg.PurchasedOn
			}
		}
		if c := strings.ToUpper(strings.TrimSpace(pg.Currency)); c != "" {
			d.Currency = c
		}
		if pg.TotalPrinted != "" {
			d.TotalPrinted = pg.TotalPrinted
		}
		if pg.DiscountTotal != "" {
			d.DiscountTotal = pg.DiscountTotal
		}
		for _, l := range pg.Lines {
			if len(d.Lines) >= s.maxLines {
				break
			}
			name := domain.CleanItemName(l.Name)
			if name == "" {
				continue
			}
			n++
			cat := l.CategoryKey
			if !s.taxonomy[cat] {
				cat = "" // không nằm trong taxonomy: để người dùng "Chọn nhóm"
			}
			d.Lines = append(d.Lines, DraftLine{
				LineID: fmt.Sprintf("l%d", n), Name: name, Qty: l.Qty, Unit: domain.NormalizeUnit(l.Unit), Amount: l.Amount,
				CategoryKey: cat, GroupKey: normalizeGroupKey(l.GroupKey), Habit: true,
			})
		}
	}
	return d
}

func (s *Service) applyFX(ctx context.Context, d *Draft) error {
	var fx *FX
	if d.Currency != "VND" {
		on := domain.Today(s.now(), time.UTC)
		if pd, err := domain.ParseDate(d.PurchasedOn); err == nil {
			on = pd
		}
		rate, ok, err := s.fx.Rate(ctx, d.Currency, on)
		if err != nil {
			return apperr.Wrap(apperr.CodeInternal, "Chưa lấy được tỷ giá.", err)
		}
		if ok {
			fx = &rate
			d.FX = fx
		} else {
			d.FXMissing = true
		}
	}
	var sum domain.VND
	for i := range d.Lines {
		v, neg, ok := toVND(d.Lines[i].Amount, d.Currency, fx)
		if ok {
			if neg {
				// Dòng khuyến mãi in số âm: giữ trong nháp để người dùng thấy, trừ vào tổng.
				v = -v
				d.Lines[i].Habit = false
				d.Lines[i].Discount = true
			}
			d.Lines[i].AmountVND = &v
			sum += v
		}
	}
	d.SumVND = sum
	if d.TotalPrinted != "" {
		if total, _, ok := toVND(d.TotalPrinted, d.Currency, fx); ok {
			var disc domain.VND
			if d.DiscountTotal != "" {
				disc, _, _ = toVND(d.DiscountTotal, d.Currency, fx)
			}
			// Dòng có thể là giá sau giảm (tổng = tổng dòng) hoặc giá gốc (tổng = tổng dòng
			// trừ giảm giá). Khớp một trong hai, lệch không quá ngưỡng, thì coi là khớp.
			d.TotalMismatch = !withinTolerance(total, sum) && !withinTolerance(total, sum-disc)
		}
	}
	return nil
}

// TotalTolerance: lệch tổng không quá 1.000đ (làm tròn, phí túi) không hiện cảnh báo.
const TotalTolerance domain.VND = 1_000

func withinTolerance(a, b domain.VND) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= TotalTolerance
}

// amountCleaner bỏ ký hiệu tiền tệ hay gặp trên hóa đơn: "64.000đ", "64,000 VND", "₫64.000".
var amountCleaner = strings.NewReplacer("vnđ", "", "vnd", "", "đ", "", "₫", "", "$", "", "€", "", " ", "", "\u00a0", "")

// toVND quy đổi số in trên hóa đơn sang đồng. Trả giá trị tuyệt đối và neg=true khi số
// in là số âm ("-10.000", "(10.000)", "10.000-"), thường là dòng khuyến mãi.
func toVND(amount, currency string, fx *FX) (v domain.VND, neg bool, ok bool) {
	a := amountCleaner.Replace(strings.ToLower(strings.TrimSpace(amount)))
	if strings.HasPrefix(a, "(") && strings.HasSuffix(a, ")") {
		a, neg = a[1:len(a)-1], true
	}
	for _, minus := range []string{"-", "\u2212"} {
		if strings.HasPrefix(a, minus) {
			a, neg = strings.TrimPrefix(a, minus), true
		} else if strings.HasSuffix(a, minus) {
			a, neg = strings.TrimSuffix(a, minus), true
		}
	}
	if a == "" {
		return 0, false, false
	}
	if currency == "VND" {
		// Hóa đơn Việt Nam hay in "64.000" hoặc "64,000": bỏ dấu phân cách nghìn.
		clean := strings.NewReplacer(".", "", ",", "").Replace(a)
		n, err := strconv.ParseInt(clean, 10, 64)
		if err != nil {
			return 0, false, false
		}
		v, err := domain.NewVND(n)
		return v, neg, err == nil
	}
	if fx == nil {
		return 0, false, false
	}
	// Ngoại tệ in kiểu "1,234.50": bỏ dấu phẩy phân cách nghìn.
	v, err := domain.ConvertToVND(strings.ReplaceAll(a, ",", ""), fx.Rate)
	return v, neg, err == nil
}

// ---------- Lưu hóa đơn ----------

// SaveLine là dòng người dùng đã xác nhận (có thể đã sửa tên, bỏ dòng, đổi nhóm, tắt habit).
type SaveLine struct {
	LineID      string  `json:"line_id"`
	Name        string  `json:"name"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit,omitempty"`
	Amount      string  `json:"amount"`
	CategoryKey string  `json:"category_key"`
	GroupKey    string  `json:"group_key"`
	Habit       bool    `json:"habit"`
}

type SaveRequest struct {
	ReceiptID        string     `json:"receipt_id"`
	Store            string     `json:"store"`
	PurchasedOn      string     `json:"purchased_on"`
	Currency         string     `json:"currency"`
	DiscountTotal    string     `json:"discount_total"`
	Lines            []SaveLine `json:"lines"`
	UseForPrediction bool       `json:"use_for_prediction"`
	ConfirmDuplicate bool       `json:"confirm_duplicate"`
}

// Save kiểm tra, tính lại mọi con số bằng code rồi ghi trong một transaction.
func (s *Service) Save(ctx context.Context, p authz.Principal, req SaveRequest) (SaveOutcome, error) {
	if err := authz.Can(p, authz.ActionWriteReceipts); err != nil {
		return SaveOutcome{}, err
	}
	r, err := s.build(ctx, p, req)
	if err != nil {
		return SaveOutcome{}, err
	}
	if !req.ConfirmDuplicate {
		dup, err := s.store.HasSimilar(ctx, p.UID, r.DedupeKey, r.ID)
		if err != nil {
			return SaveOutcome{}, apperr.Wrap(apperr.CodeInternal, "Chưa lưu được, thử lại.", err)
		}
		if dup {
			return SaveOutcome{}, apperr.New(apperr.CodePossibleDuplicate, "Hóa đơn này giống một hóa đơn đã lưu. Vẫn lưu?")
		}
	}
	out, err := s.store.SaveReceipt(ctx, p.UID, r, userLocation(p))
	if err != nil {
		return SaveOutcome{}, err // Store đã trả apperr (receipt_conflict) hoặc lỗi hạ tầng
	}
	return out, nil
}

func (s *Service) build(ctx context.Context, p authz.Principal, req SaveRequest) (Receipt, error) {
	verr := apperr.New(apperr.CodeValidation, "Hóa đơn chưa hợp lệ.")
	if !validID(req.ReceiptID) {
		return Receipt{}, verr.With("field", "receipt_id")
	}
	on, err := domain.ParseDate(req.PurchasedOn)
	if err != nil {
		return Receipt{}, verr.With("field", "purchased_on")
	}
	if on.After(domain.Today(s.now(), userLocation(p)).AddDays(1)) {
		return Receipt{}, verr.With("field", "purchased_on").With("reason", "future")
	}
	if len(req.Lines) == 0 || len(req.Lines) > s.maxLines {
		return Receipt{}, verr.With("field", "lines")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "VND"
	}

	var fx *FX
	if currency != "VND" {
		rate, ok, err := s.fx.Rate(ctx, currency, on)
		if err != nil {
			return Receipt{}, apperr.Wrap(apperr.CodeInternal, "Chưa lấy được tỷ giá.", err)
		}
		if ok {
			fx = &rate
		}
	}

	r := Receipt{ID: req.ReceiptID, Store: domain.CleanItemName(req.Store), PurchasedOn: on, Currency: currency, FX: fx, SavedAt: s.now()}
	var lineDiscount domain.VND
	for i, l := range req.Lines {
		v, neg, ok := toVND(l.Amount, currency, fx)
		if ok && neg {
			// Dòng khuyến mãi: không phải món, trừ vào số đã chi và cộng vào tiết kiệm.
			lineDiscount += v
			continue
		}
		name := domain.CleanItemName(l.Name)
		if name == "" || l.Qty <= 0 || l.Qty > 1000 || math.IsNaN(l.Qty) {
			return Receipt{}, verr.With("line", i)
		}
		line := Line{
			ID: l.LineID, Name: name, NameNorm: domain.NormalizeVi(name), Qty: l.Qty,
			Unit: domain.NormalizeUnit(l.Unit),
			Orig: strings.TrimSpace(l.Amount), Habit: l.Habit && req.UseForPrediction,
		}
		if s.taxonomy[l.CategoryKey] {
			line.CategoryKey = l.CategoryKey
		}
		line.GroupKey = groupKeyFor(l.GroupKey, line.NameNorm)
		if ok {
			line.VND = &v
			r.TotalVND += v
		} else if currency == "VND" {
			return Receipt{}, verr.With("line", i).With("field", "amount")
		}
		r.Lines = append(r.Lines, line)
	}
	if len(r.Lines) == 0 {
		return Receipt{}, verr.With("field", "lines")
	}
	if lineDiscount > r.TotalVND {
		return Receipt{}, verr.With("field", "lines").With("reason", "discount_exceeds_total")
	}
	r.TotalVND -= lineDiscount
	r.DiscountVND = lineDiscount
	if req.DiscountTotal != "" {
		// Tiết kiệm = discount_total in trên hóa đơn (spec mục 3), không tự tính từ giá gốc.
		// Số in có thể âm ("-10.000"): lấy giá trị tuyệt đối. Khi có cả dòng khuyến mãi
		// và tổng giảm giá, tổng in trên hóa đơn đã gồm các dòng đó nên lấy số lớn hơn.
		dv, _, ok := toVND(req.DiscountTotal, currency, fx)
		if !ok && currency == "VND" {
			return Receipt{}, verr.With("field", "discount_total")
		}
		if dv > r.DiscountVND {
			r.DiscountVND = dv
		}
	}
	r.DedupeKey = fmt.Sprintf("%s|%s|%d", domain.NormalizeVi(r.Store), on, r.TotalVND)
	r.PayloadHash = hashPayload(req)
	return r, nil
}

// groupKeyFor: nhóm do AI/taxonomy gán nếu hợp lệ, ngược lại dùng tên chuẩn hóa.
func groupKeyFor(given, nameNorm string) string {
	g := normalizeGroupKey(given)
	if validID(g) {
		return g
	}
	return strings.ReplaceAll(nameNorm, " ", "-")
}

// normalizeGroupKey đưa khóa nhóm về một dạng: chữ thường, gạch nối. Danh sách nhóm demo
// (benchmark/nhom_demo.json) viết "sua_tuoi", code và dữ liệu mẫu viết "sua-tuoi".
func normalizeGroupKey(k string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(k)), "_", "-")
}

func hashPayload(req SaveRequest) string {
	req.ConfirmDuplicate = false // không ảnh hưởng nội dung hóa đơn
	b, _ := json.Marshal(req)    // struct cố định nên Marshal không lỗi và có thứ tự ổn định
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// validID: id do client sinh (UUID) hoặc khóa nhóm; chỉ chữ thường, số, gạch; dài 1..64.
func validID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func userLocation(p authz.Principal) *time.Location {
	if p.Timezone != "" {
		if loc, err := time.LoadLocation(p.Timezone); err == nil {
			return loc
		}
	}
	return vnLocation
}

// Asia/Ho_Chi_Minh không có giờ mùa hè, nên múi cố định +7 là dự phòng an toàn khi
// máy chủ thiếu tzdata (image distroless/static có kèm tzdata nếu import time/tzdata).
var vnLocation = time.FixedZone("ICT", 7*3600)
