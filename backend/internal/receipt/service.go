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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Service điều phối đọc và lưu hóa đơn.
type Service struct {
	reader Reader
	pages  PageCache // nil: không giữ kết quả từng ảnh
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

// WithPageCache giữ kết quả đọc từng ảnh để chụp lại một ảnh không phải đọc lại cả hóa đơn.
func WithPageCache(c PageCache) Option {
	return func(s *Service) error { s.pages = c; return nil }
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
		maxPages:     MaxPages,
		maxPageBytes: MaxPageBytes,
		maxLines:     150,
		pageTimeout:  40 * time.Second, // model Gemini 3.x đọc ảnh dày chữ có lúc quá 20 s; request có 55 s
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
//
// BUG-05: lỗi được tách theo nguyên nhân (too_many_images, image_too_large, unreadable_image
// kèm số thứ tự ảnh, no_items, read_failed). Ảnh đã đọc được giữ kết quả trong PageCache theo
// hash nội dung, nên khi người dùng chỉ chụp lại một ảnh lỗi thì các ảnh còn lại không gọi AI
// lần nữa. Bước đọc không bao giờ ghi purchases, tồn kho, chi tiêu hay Kế hoạch.
func (s *Service) Read(ctx context.Context, p authz.Principal, receiptID, lang string, imgs []Image) (Draft, error) {
	if err := authz.Can(p, authz.ActionWriteReceipts); err != nil {
		return Draft{}, err
	}
	started := s.now()
	logRead := func(stage string, err error, extra ...any) {
		attrs := []any{"receipt_id", receiptID, "stage", stage, "image_count", len(imgs),
			"image_bytes", imageSizes(imgs), "duration_ms", s.now().Sub(started).Milliseconds()}
		if err != nil {
			attrs = append(attrs, "error_code", string(apperr.From(err).Code), "cause", err.Error())
		}
		s.log.InfoContext(ctx, "receipt read", append(attrs, extra...)...)
	}
	if err := s.validateImages(receiptID, imgs); err != nil {
		logRead("validate", err)
		return Draft{}, err
	}

	// Gửi lại cùng receipt_id: trả bản nháp đã có, không tốn lượt, không gọi lại AI.
	if cached, err := s.drafts.Get(ctx, p.UID, receiptID); err != nil {
		s.log.WarnContext(ctx, "draft cache get failed", "err", err)
	} else if cached != nil {
		logRead("draft_cache", nil)
		return *cached, nil
	}

	loc := userLocation(p)
	day := domain.Today(s.now(), loc).String()
	if err := s.quota.Reserve(ctx, p.UID, day, s.dailyLimit); err != nil {
		logRead("quota", err)
		return Draft{}, err
	}
	pages, aiCalls, err := s.readPages(ctx, p.UID, imgs, lang)
	if err != nil {
		// Lỗi đọc không trừ lượt của người dùng. Dùng context riêng vì ctx có thể đã hết hạn.
		relCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if rerr := s.quota.Release(relCtx, p.UID, day); rerr != nil {
			s.log.ErrorContext(ctx, "quota release failed", "err", rerr)
		}
		logRead("gemini_read", err, "ai_calls", aiCalls)
		return Draft{}, err
	}

	d := s.merge(receiptID, pages)
	if len(d.Lines) == 0 {
		err := apperr.New(apperr.CodeNoItems, "Chưa thấy món nào. Chụp lại rõ hơn.")
		logRead("merge", err, "ai_calls", aiCalls)
		return Draft{}, err
	}
	if err := s.applyFX(ctx, &d); err != nil {
		logRead("fx", err)
		return Draft{}, err
	}
	if err := s.drafts.Put(ctx, p.UID, d); err != nil {
		s.log.WarnContext(ctx, "draft cache put failed", "err", err) // không chặn người dùng
	}
	logRead("done", nil, "ai_calls", aiCalls, "lines", len(d.Lines))
	return d, nil
}

func imageSizes(imgs []Image) []int {
	out := make([]int, len(imgs))
	for i, img := range imgs {
		out[i] = len(img.Data)
	}
	return out
}

var allowedMIME = map[string][]byte{
	"image/jpeg": {0xFF, 0xD8, 0xFF},
	"image/png":  {0x89, 'P', 'N', 'G'},
	"image/webp": []byte("RIFF"),
}

// ValidImage kiểm tra MIME được phép và byte đầu file khớp MIME (không tin Content-Type).
func ValidImage(mime string, data []byte) bool {
	magic, ok := allowedMIME[mime]
	return ok && bytes.HasPrefix(data, magic)
}

// MaxPages, MaxPageBytes: giới hạn của BUG-05 mục 6 (tối đa 10 ảnh, mỗi ảnh 5 MB sau nén).
const (
	MaxPages     = 10
	MaxPageBytes = 5 << 20
)

func (s *Service) validateImages(receiptID string, imgs []Image) error {
	if !validID(receiptID) {
		return apperr.New(apperr.CodeValidation, "Mã hóa đơn không hợp lệ.").With("field", "receipt_id")
	}
	if len(imgs) == 0 {
		return apperr.New(apperr.CodeValidation, "Chưa có ảnh hóa đơn.").With("field", "images")
	}
	if len(imgs) > s.maxPages {
		return apperr.New(apperr.CodeTooManyImages, fmt.Sprintf("Một hóa đơn tối đa %d ảnh.", s.maxPages)).With("limit", s.maxPages)
	}
	for i, img := range imgs {
		if len(img.Data) > s.maxPageBytes {
			return apperr.New(apperr.CodeImageTooLarge, fmt.Sprintf("Ảnh số %d quá nặng. Chụp lại ảnh này.", i+1)).With("image_index", i+1)
		}
		magic, ok := allowedMIME[img.MIME]
		if !ok || !bytes.HasPrefix(img.Data, magic) { // không tin Content-Type do client gửi
			return apperr.New(apperr.CodeUnsupportedMedia, "Chỉ nhận ảnh JPEG, PNG hoặc WebP.").With("image_index", i+1)
		}
	}
	return nil
}

// readPages đọc song song mọi trang; mỗi trang có timeout riêng, retry một lần với lỗi
// tạm thời nếu còn đủ thời gian trong ngân sách của request. Trang đã đọc được lấy từ
// PageCache nếu có. Trả số lần thực sự gọi AI (để log).
func (s *Service) readPages(ctx context.Context, uid string, imgs []Image, lang string) ([]Page, int, error) {
	pages := make([]Page, len(imgs))
	errs := make([]error, len(imgs))
	called := make([]bool, len(imgs))
	var wg sync.WaitGroup
	for i, img := range imgs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hash := imageHash(img) + "_" + lang // tên dịch theo ngôn ngữ app nên bản đọc nhớ theo ngôn ngữ
			if s.pages != nil {
				if cached, err := s.pages.GetPage(ctx, uid, hash); err == nil && cached != nil {
					pages[i] = *cached
					return
				}
			}
			called[i] = true
			pages[i], errs[i] = s.readOne(ctx, img, lang)
			if errs[i] == nil && s.pages != nil {
				if err := s.pages.PutPage(ctx, uid, hash, pages[i]); err != nil {
					s.log.WarnContext(ctx, "page cache put failed", "err", err)
				}
			}
		}()
	}
	wg.Wait()
	calls := 0
	for _, c := range called {
		if c {
			calls++
		}
	}

	var unreadable []int
	for i, err := range errs {
		switch {
		case err == nil:
		case errors.Is(err, ErrUnreadable):
			unreadable = append(unreadable, i+1)
		case errors.Is(err, ErrAIRejected):
			return nil, calls, apperr.Wrap(apperr.CodeAIUnavailable, "Mimi tạm thời chưa đọc được hóa đơn. Mẹ thử lại sau hoặc nhập tay nhé.", err)
		default:
			return nil, calls, apperr.Wrap(apperr.CodeReadFailed, "Mimi chưa đọc được. Ảnh vẫn được giữ, hãy thử lại.", err).
				With("image_index", i+1)
		}
	}
	if len(unreadable) > 0 {
		msg := fmt.Sprintf("Ảnh số %d chưa rõ. Chụp lại ảnh này rõ hơn.", unreadable[0])
		if len(unreadable) == len(imgs) && len(imgs) == 1 {
			msg = "Mimi chưa đọc được hóa đơn này. Chụp lại rõ hơn, đủ sáng và thẳng."
		}
		return nil, calls, apperr.New(apperr.CodeUnreadableImage, msg).
			With("image_index", unreadable[0]).With("image_indexes", unreadable)
	}
	return pages, calls, nil
}

func imageHash(img Image) string {
	sum := sha256.Sum256(img.Data)
	return hex.EncodeToString(sum[:])
}

func (s *Service) readOne(ctx context.Context, img Image, lang string) (Page, error) {
	for attempt := 0; ; attempt++ {
		pctx, cancel := context.WithTimeout(ctx, s.pageTimeout)
		page, err := s.reader.ReadPage(pctx, img, lang)
		cancel()
		if errors.Is(err, ErrNoItems) {
			return Page{}, nil // ảnh chỉ có phần tổng của hóa đơn dài: hợp lệ, không có dòng
		}
		if err == nil || errors.Is(err, ErrUnreadable) || errors.Is(err, ErrAIRejected) || attempt >= 1 {
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

// merge ghép các trang bằng code (không nhờ AI ghép tiền): cửa hàng và ngày lấy từ trang
// đầu tiên có thông tin, tổng in lấy từ trang có dòng tổng (thường là trang cuối).
// Chỉ bỏ dòng trùng khi có từ 2 dòng liên tiếp giống hệt ở vùng nối hai ảnh; một dòng
// giống nhau đơn lẻ được giữ (có thể là mua hai lần cùng món).
func (s *Service) merge(receiptID string, pages []Page) Draft {
	d := Draft{ReceiptID: receiptID, Currency: "VND", Pages: len(pages)}
	n := 0
	var prev []PageLine
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
		lines := pg.Lines[overlap(prev, pg.Lines):]
		if len(pg.Lines) > 0 {
			prev = pg.Lines
		}
		for _, l := range lines {
			if len(d.Lines) >= s.maxLines {
				break
			}
			dl, ok := s.draftLine(l)
			if !ok {
				continue
			}
			n++
			dl.LineID = fmt.Sprintf("l%d", n)
			d.Lines = append(d.Lines, dl)
		}
	}
	return d
}

// overlap: số dòng đầu của trang sau trùng hệt các dòng cuối của trang trước, chỉ tính
// khi trùng từ 2 dòng liên tiếp trở lên.
func overlap(prev, next []PageLine) int {
	best := 0
	for k := 2; k <= min(len(prev), len(next)); k++ {
		same := true
		for i := 0; i < k; i++ {
			if !sameLine(prev[len(prev)-k+i], next[i]) {
				same = false
				break
			}
		}
		if same {
			best = k
		}
	}
	return best
}

func sameLine(a, b PageLine) bool {
	return domain.NormalizeVi(a.RawName) == domain.NormalizeVi(b.RawName) && a.Qty == b.Qty &&
		strings.TrimSpace(a.Amount) == strings.TrimSpace(b.Amount)
}

// draftLine chuẩn hóa một dòng AI đọc: tên theo 7.1, nhóm theo taxonomy 0.2, quy cách và
// lượng canonical theo 0.1. Dòng không có tên thì bỏ.
func (s *Service) draftLine(l PageLine) (DraftLine, bool) {
	raw := domain.CleanItemName(l.RawName)
	if raw == "" {
		return DraftLine{}, false
	}
	group := canonicalGroup(l.GroupKey)
	dl := DraftLine{
		Qty: l.Qty, Unit: domain.NormalizeUnit(l.Unit), Amount: l.Amount,
		CategoryKey: s.category(l.CategoryKey, group), GroupKey: group, Habit: true,
	}
	pn := domain.NormalizeProductName(raw, l.Brand, l.ProductName, taxonomy.Brand)
	dl.Product = Product{
		RawName: raw, Brand: pn.Brand, ProductName: pn.ProductName, DetailName: pn.DetailName, ProductKey: pn.ProductKey,
		SourceLang:   strings.ToLower(strings.TrimSpace(l.SourceLang)),
		PackageCount: l.PackageCount, PackageSize: l.PackageSize, PackageUnit: domain.NormalizeUnit(l.PackageUnit),
	}
	dl.Name = displayName(pn, raw)
	if tr := domain.CleanItemName(l.Translated); tr != "" && !strings.EqualFold(tr, dl.Name) {
		dl.OriginalName, dl.Name = dl.Name, truncateRunes(tr, maxNameRunes)
	}
	pk, reasons := measure(raw, dl.Qty, dl.Unit, l.PackageCount, l.PackageSize, l.PackageUnit, dl.GroupKey)
	dl.CanonicalQty, dl.CanonicalUnit = pk.Total, string(pk.Unit)
	dl.ReviewReasons = reviewReasons(dl.CategoryKey, dl.GroupKey, reasons)
	dl.NeedsReview = len(dl.ReviewReasons) > 0
	return dl, true
}

// displayName: tên chi tiết (brand + tên riêng) nếu có; không thì tên in trên bill đã bỏ
// quy cách và marketing, viết theo 7.1. Tên nhóm (Sữa bột) do client tra theo ngôn ngữ.
func displayName(pn domain.ProductName, raw string) string {
	if pn.DetailName != "" {
		return pn.DetailName
	}
	if n := domain.FormatField(domain.StripQuantity(domain.StripMarketing(raw))); n != "" {
		return n
	}
	return domain.FormatField(raw)
}

// canonicalGroup: chỉ nhận group_key thuộc taxonomy 0.2; "khac" hoặc khóa lạ là chưa phân
// nhóm (không tự tạo khóa mới, 0.2 mục 12).
// category chọn danh mục chi tiêu: khóa AI / người dùng gửi nếu hợp lệ, nếu không thì
// lấy theo nhóm sản phẩm (người dùng bấm "Chọn nhóm" chỉ chọn nhóm). "" = Cần phân loại.
func (s *Service) category(key, group string) string {
	if s.taxonomy[key] {
		return key
	}
	if c := taxonomy.SpendingCategory(group); s.taxonomy[c] {
		return c
	}
	return ""
}

func canonicalGroup(k string) string {
	k = taxonomy.Normalize(k)
	if taxonomy.Has(k) {
		return k
	}
	return ""
}

// measure tính tổng lượng canonical của một dòng theo thứ tự ưu tiên 0.1 mục 8:
//  1. hàng cân ký: số lượng in cạnh đơn vị đo (1,2 kg) là tổng lượng;
//  2. quy cách in trong tên (luật cố định) rồi quy cách AI đã tách, nhân số lượng mua;
//  3. số đếm tự nhiên (2 cái). Đếm cho nhóm mặc định đo theo g/ml/m thì cần xác nhận.
func measure(raw string, qty float64, unit string, aiCount, aiSize float64, aiUnit, group string) (domain.Packaging, []string) {
	if qty <= 0 || math.IsNaN(qty) {
		qty = 1
	}
	if unit != "" {
		if total, base := domain.ToBase(qty, unit); base != domain.UnitItem {
			return domain.Packaging{Structure: fmt.Sprintf("%g %s", qty, unit), Count: 1, Size: qty, SizeUnit: unit,
				Total: math.Round(total*1000) / 1000, Unit: base}, nil
		}
	}
	pk := domain.ParsePackaging(raw)
	if !pk.Known() {
		pk = domain.PackagingFrom(aiCount, aiSize, aiUnit)
	}
	if pk.Known() {
		return pk.ForLines(qty), nil
	}
	count := pk.Count
	if count <= 0 {
		count = 1
	}
	pk = domain.Packaging{Count: count, Total: math.Round(count*qty*1000) / 1000, Unit: domain.UnitItem}
	if def, ok := taxonomy.DefaultUnit(group); ok && def != domain.UnitItem {
		return pk, []string{"count_for_measured_group"}
	}
	return pk, nil
}

func reviewReasons(category, group string, extra []string) []string {
	var out []string
	if category == "" {
		out = append(out, "missing_category")
	}
	if group == "" {
		out = append(out, "missing_group")
	}
	return append(out, extra...)
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
			match := !d.TotalMismatch
			d.TotalMatch = &match
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
// Các field sản phẩm là kết quả chuẩn hóa một lần lúc đọc, client gửi lại nguyên vẹn;
// lượng canonical luôn được server tính lại từ chúng, không nhận từ client.
type SaveLine struct {
	LineID       string  `json:"line_id"`
	Name         string  `json:"name"`
	Qty          float64 `json:"qty"`
	Unit         string  `json:"unit,omitempty"`
	Amount       string  `json:"amount"`
	CategoryKey  string  `json:"category_key"`
	GroupKey     string  `json:"group_key"`
	Habit        bool    `json:"habit"`
	RawName      string  `json:"raw_name,omitempty"`
	Brand        string  `json:"brand,omitempty"`
	ProductName  string  `json:"product_name,omitempty"`
	SourceLang   string  `json:"source_language,omitempty"`
	OriginalName string  `json:"original_name,omitempty"`
	PackageCount float64 `json:"package_count,omitempty"`
	PackageSize  float64 `json:"package_size,omitempty"`
	PackageUnit  string  `json:"package_unit,omitempty"`
	// Plans (B3): Kế hoạch mà dòng này trừ vào, theo thứ tự app đã chia (kế hoạch đi mua
	// sớm nhất trước, người dùng đổi được). Vắng: dùng source_plan_id. Mảng rỗng: không trừ.
	Plans *[]PlanShare `json:"plans,omitempty"`
	// PriceSource (B7): receipt | manual | estimate. Vắng là receipt; dòng không có giá luôn là missing.
	PriceSource string `json:"price_source,omitempty"`
}

// MaxLinePlans: một dòng chia vào tối đa ngần này Kế hoạch (MaxPlans + Plan mặc định).
const MaxLinePlans = 11

type SaveRequest struct {
	ReceiptID        string     `json:"receipt_id"`
	Store            string     `json:"store"`
	PurchasedOn      string     `json:"purchased_on"`
	Currency         string     `json:"currency"`
	DiscountTotal    string     `json:"discount_total"`
	Lines            []SaveLine `json:"lines"`
	UseForPrediction bool       `json:"use_for_prediction"`
	ConfirmDuplicate bool       `json:"confirm_duplicate"`
	// SourcePlanID: quét từ trong một Plan thì chỉ đối chiếu Plan đó (3.1 mục 3A.6).
	SourcePlanID string `json:"source_plan_id,omitempty"`
	// AssignmentID: Buyer quét hóa đơn cho một vòng mua hộ (4.1 mục 9.3).
	AssignmentID string `json:"assignment_id,omitempty"`
}

// Save kiểm tra, tính lại mọi con số bằng code rồi ghi trong một transaction.
// Gửi lại cùng receipt_id và cùng nội dung (mạng chậm, bấm Lưu hai lần) trả kết quả cũ,
// không ghi gì thêm (BUG-06 mục 7).
func (s *Service) Save(ctx context.Context, p authz.Principal, req SaveRequest) (SaveOutcome, error) {
	if err := authz.Can(p, authz.ActionWriteReceipts); err != nil {
		return SaveOutcome{}, err
	}
	started := s.now()
	logSave := func(stage string, err error) {
		attrs := []any{"receipt_id", req.ReceiptID, "stage", stage, "line_count", len(req.Lines),
			"duration_ms", s.now().Sub(started).Milliseconds()}
		if err != nil {
			attrs = append(attrs, "error_code", string(apperr.From(err).Code))
		}
		s.log.InfoContext(ctx, "receipt save", attrs...)
	}
	r, err := s.build(ctx, p, req)
	if err != nil {
		logSave("validate", err)
		return SaveOutcome{}, err
	}
	if !req.ConfirmDuplicate {
		dup, err := s.store.HasSimilar(ctx, p.UID, r.DedupeKey, r.ID)
		if err != nil {
			err = apperr.Wrap(apperr.CodeInternal, "Mimi chưa lưu được hóa đơn. Dữ liệu bạn vừa sửa vẫn được giữ.", err)
			logSave("duplicate_check", err)
			return SaveOutcome{}, err
		}
		if dup {
			err := apperr.New(apperr.CodePossibleDuplicate, "Hóa đơn này giống một hóa đơn đã lưu. Vẫn lưu?")
			logSave("duplicate_check", err)
			return SaveOutcome{}, err
		}
	}
	out, err := s.store.SaveReceipt(ctx, p.UID, r, userLocation(p))
	if err != nil {
		if status.Code(err) == codes.Aborted {
			err = apperr.Wrap(apperr.CodeTransactionConflict, "Đang có thay đổi khác cùng lúc, thử lưu lại.", err)
		}
		logSave("confirm_transaction", err)
		return SaveOutcome{}, err // Store đã trả apperr (receipt_conflict...) hoặc lỗi hạ tầng
	}
	logSave("committed", nil)
	return out, nil
}

func (s *Service) build(ctx context.Context, p authz.Principal, req SaveRequest) (Receipt, error) {
	if !validID(req.ReceiptID) {
		return Receipt{}, apperr.New(apperr.CodeValidation, "Mã hóa đơn không hợp lệ.").With("field", "receipt_id")
	}
	badDate := func(msg string) *apperr.Error {
		return apperr.New(apperr.CodeInvalidPurchaseDate, msg).With("field", "purchased_on")
	}
	on, err := domain.ParseDate(req.PurchasedOn)
	if err != nil {
		return Receipt{}, badDate("Ngày mua không hợp lệ.")
	}
	if on.After(domain.Today(s.now(), userLocation(p)).AddDays(1)) {
		return Receipt{}, badDate("Ngày mua nằm trong tương lai.").With("reason", "future")
	}
	if len(req.Lines) == 0 || len(req.Lines) > s.maxLines {
		return Receipt{}, apperr.New(apperr.CodeValidation, "Hóa đơn chưa có món nào.").With("field", "lines")
	}
	for _, id := range []struct{ field, v string }{{"source_plan_id", req.SourcePlanID}, {"assignment_id", req.AssignmentID}} {
		if id.v != "" && !validID(id.v) {
			return Receipt{}, apperr.New(apperr.CodeValidation, "Mã không hợp lệ.").With("field", id.field)
		}
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

	storeKey, _ := domain.StoreKey(req.Store)
	r := Receipt{
		ID: req.ReceiptID, Store: domain.CleanItemName(req.Store), StoreKey: storeKey, PurchasedOn: on,
		Currency: currency, FX: fx, SavedAt: s.now(), SourcePlanID: req.SourcePlanID, AssignmentID: req.AssignmentID,
	}
	var lineDiscount domain.VND
	for _, l := range req.Lines {
		lineErr := func(code apperr.Code, field, msg string) error {
			return apperr.New(code, msg).With("line_id", l.LineID).With("field", field)
		}
		if !validID(l.LineID) {
			return Receipt{}, lineErr(apperr.CodeValidation, "line_id", "Dòng hóa đơn không hợp lệ.")
		}
		v, neg, ok := toVND(l.Amount, currency, fx)
		if ok && neg {
			// Dòng khuyến mãi: không phải món, trừ vào số đã chi và cộng vào tiết kiệm.
			lineDiscount += v
			continue
		}
		name := domain.CleanItemName(l.Name)
		raw := domain.CleanItemName(l.RawName)
		if raw == "" {
			raw = name // dòng người dùng tự thêm: tên nhập tay là raw_name
		}
		if name == "" && raw == "" {
			return Receipt{}, lineErr(apperr.CodeValidation, "name", "Món này chưa có tên.")
		}
		if name == "" {
			name = raw
		}
		if l.Qty <= 0 || l.Qty > 1000 || math.IsNaN(l.Qty) {
			return Receipt{}, lineErr(apperr.CodeInvalidQuantity, "qty", "Số lượng của món này chưa hợp lệ.")
		}
		// A7: ô giá để trống là "chưa rõ giá" (AI không đọc được hoặc người mua chưa nhập).
		// Món vẫn được lưu, không bịa số và không tính 0đ: VND nil, không cộng vào tổng.
		// Giá có ghi nhưng sai dạng thì vẫn báo lỗi để người dùng sửa.
		if !ok && currency == "VND" && strings.TrimSpace(l.Amount) != "" {
			return Receipt{}, lineErr(apperr.CodeInvalidLineTotal, "amount", "Giá của món này chưa hợp lệ.")
		}
		group := canonicalGroup(l.GroupKey)
		cat := s.category(l.CategoryKey, group)
		unit := domain.NormalizeUnit(l.Unit)
		pn := domain.NormalizeProductName(raw, l.Brand, l.ProductName, taxonomy.Brand)
		pk, extra := measure(raw, l.Qty, unit, l.PackageCount, l.PackageSize, l.PackageUnit, group)
		line := Line{
			ID: l.LineID, Name: name, NameNorm: domain.NormalizeVi(name), Qty: l.Qty, Unit: unit,
			Orig: strings.TrimSpace(l.Amount), Habit: l.Habit && req.UseForPrediction,
			CategoryKey: cat, GroupKey: group, Structure: pk.Structure,
			Product: Product{
				RawName: raw, Brand: pn.Brand, ProductName: pn.ProductName, DetailName: pn.DetailName, ProductKey: pn.ProductKey,
				SourceLang:   strings.ToLower(strings.TrimSpace(l.SourceLang)),
				OriginalName: truncateRunes(domain.CleanItemName(l.OriginalName), maxNameRunes),
				PackageCount: pk.Count, PackageSize: pk.Size, PackageUnit: pk.SizeUnit,
				CanonicalQty: pk.Total, CanonicalUnit: string(pk.Unit),
			},
		}
		if l.Plans != nil {
			if len(*l.Plans) > MaxLinePlans {
				return Receipt{}, lineErr(apperr.CodeValidation, "plans", "Một món chia vào quá nhiều kế hoạch.")
			}
			seen := map[string]bool{}
			for _, ps := range *l.Plans {
				id := ps.PlanID
				if id == "" {
					id = "default"
				}
				if !validID(id) || seen[id] || ps.Qty < 0 || math.IsNaN(ps.Qty) {
					return Receipt{}, lineErr(apperr.CodeValidation, "plans", "Kế hoạch của món này không hợp lệ.")
				}
				seen[id] = true
				line.Plans = append(line.Plans, PlanShare{PlanID: id, Qty: ps.Qty})
			}
			line.PlanSet = true
		}
		// Dòng chưa đủ phân loại vẫn được lưu (purchase + chi tiêu) nhưng chưa vào tồn kho
		// và dự đoán cho tới khi người dùng phân loại (BUG-06 mục 5.3, 0.1 mục 20).
		line.NeedsReview = len(reviewReasons(cat, group, extra)) > 0
		line.PriceSource = PriceMissing
		if ok {
			line.VND = &v
			r.TotalVND += v
			switch l.PriceSource {
			case "", PriceReceipt:
				line.PriceSource = PriceReceipt
			case PriceManual, PriceEstimate:
				line.PriceSource = l.PriceSource
			default:
				return Receipt{}, lineErr(apperr.CodeValidation, "price_source", "Nguồn giá không hợp lệ.")
			}
		}
		r.Lines = append(r.Lines, line)
	}
	if len(r.Lines) == 0 {
		return Receipt{}, apperr.New(apperr.CodeValidation, "Hóa đơn chưa có món nào.").With("field", "lines")
	}
	if lineDiscount > r.TotalVND {
		return Receipt{}, apperr.New(apperr.CodeValidation, "Tổng khuyến mãi lớn hơn tổng hóa đơn.").
			With("field", "lines").With("reason", "discount_exceeds_total")
	}
	r.TotalVND -= lineDiscount
	r.DiscountVND = lineDiscount
	if lineDiscount > 0 {
		amts := make([]*domain.VND, len(r.Lines))
		for i := range r.Lines {
			amts[i] = r.Lines[i].VND
		}
		for i, d := range domain.AllocateDiscount(amts, lineDiscount) {
			r.Lines[i].DiscountVND = d
		}
	}
	if req.DiscountTotal != "" {
		// Tiết kiệm = discount_total in trên hóa đơn (spec mục 3), không tự tính từ giá gốc.
		// Số in có thể âm ("-10.000"): lấy giá trị tuyệt đối. Khi có cả dòng khuyến mãi
		// và tổng giảm giá, tổng in trên hóa đơn đã gồm các dòng đó nên lấy số lớn hơn.
		dv, _, ok := toVND(req.DiscountTotal, currency, fx)
		if !ok && currency == "VND" {
			return Receipt{}, apperr.New(apperr.CodeValidation, "Tổng giảm giá không hợp lệ.").With("field", "discount_total")
		}
		if dv > r.DiscountVND {
			r.DiscountVND = dv
		}
	}
	r.DedupeKey = fmt.Sprintf("%s|%s|%d", domain.NormalizeVi(r.Store), on, r.TotalVND)
	r.PayloadHash = hashPayload(req)
	return r, nil
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

// maxNameRunes: tên món tối đa 120 ký tự như ô name của SaveLine.
const maxNameRunes = 120

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n]))
}
