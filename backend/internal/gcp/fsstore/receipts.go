package fsstore

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

var (
	_ receipt.Store      = (*Store)(nil)
	_ receipt.PageCache  = (*Store)(nil)
	_ receipt.DraftCache = (*Store)(nil)
	_ receipt.Quota      = (*Store)(nil)
	_ receipt.FXRates    = (*Store)(nil)
)

// ---------- Bản nháp (TTL 1 giờ, không có ảnh) ----------

func (s *Store) Get(ctx context.Context, uid, receiptID string) (*receipt.Draft, error) {
	snap, err := s.user(uid).Collection("drafts").Doc(receiptID).Get(ctx)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, _ := snap.Data()["json"].(string)
	var d receipt.Draft
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) Put(ctx context.Context, uid string, d receipt.Draft) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = s.user(uid).Collection("drafts").Doc(d.ReceiptID).Set(ctx, map[string]any{
		"json": string(b), "expire_at": time.Now().Add(time.Hour),
	})
	return err
}

// ---------- Kết quả đọc từng ảnh (TTL 1 giờ, theo hash nội dung, không có ảnh) ----------

func (s *Store) GetPage(ctx context.Context, uid, hash string) (*receipt.Page, error) {
	snap, err := s.user(uid).Collection("page_cache").Doc(hash).Get(ctx)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if exp, _ := snap.Data()["expire_at"].(time.Time); !exp.IsZero() && time.Now().After(exp) {
		return nil, nil // TTL policy xóa trễ: coi như không có
	}
	raw, _ := snap.Data()["json"].(string)
	var p receipt.Page
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) PutPage(ctx context.Context, uid, hash string, p receipt.Page) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.user(uid).Collection("page_cache").Doc(hash).Set(ctx, map[string]any{
		"json": string(b), "expire_at": time.Now().Add(time.Hour),
	})
	return err
}

// ---------- Tỷ giá ----------

// Rate lấy tỷ giá gần nhất không sau ngày mua, do job fx-sync ghi (có nguồn và thời điểm).
func (s *Store) Rate(ctx context.Context, currency string, on domain.Date) (receipt.FX, bool, error) {
	docs, err := s.c.Collection("fx_rates").
		Where("currency", "==", currency).
		Where("on", "<=", on.String()).
		OrderBy("on", firestore.Desc).Limit(1).
		Documents(ctx).GetAll()
	if err != nil || len(docs) == 0 {
		return receipt.FX{}, false, err
	}
	var v struct {
		Rate      string    `firestore:"rate"`
		Source    string    `firestore:"source"`
		FetchedAt time.Time `firestore:"fetched_at"`
		Stale     bool      `firestore:"stale"`
	}
	if err := docs[0].DataTo(&v); err != nil {
		return receipt.FX{}, false, err
	}
	return receipt.FX{Rate: v.Rate, Source: v.Source, At: v.FetchedAt}, true, nil
}

// ---------- Cảnh báo trùng ----------

func (s *Store) HasSimilar(ctx context.Context, uid, key, exceptID string) (bool, error) {
	docs, err := s.user(uid).Collection("receipts").Where("dedupe_key", "==", key).Limit(2).Documents(ctx).GetAll()
	if err != nil {
		return false, err
	}
	for _, d := range docs {
		if d.Ref.ID != exceptID {
			return true, nil
		}
	}
	return false, nil
}

// reconcilePlanLine trả thay đổi cho một dòng Kế hoạch khi có hàng vào. bought_qty lưu
// theo đơn vị của dòng Kế hoạch (2 hộp, 1,5 kg) để hiển thị cạnh số lượng cần mua.
func reconcilePlanLine(plan map[string]any, bought float64, boughtUnit domain.Unit, by string, now time.Time) ([]firestore.Update, bool) {
	unit, _ := plan["unit"].(string)
	qty := asFloat(plan["qty"])
	already := asFloat(plan["bought_qty"])
	factor, wantUnit := domain.ToBase(1, unit) // số đơn vị gốc trong một đơn vị của dòng
	got := already
	if wantUnit == boughtUnit && factor > 0 {
		got += bought / factor
	}
	if qty > 0 && wantUnit == boughtUnit && got < qty {
		return []firestore.Update{{Path: "bought_qty", Value: got}, {Path: "partial_by", Value: by}, {Path: "updated_at", Value: now}}, false
	}
	return []firestore.Update{
		{Path: "status", Value: "done"}, {Path: "bought_qty", Value: got}, {Path: "done_by", Value: by}, {Path: "done_at", Value: now},
	}, true
}

func lineDoc(l receipt.Line) map[string]any {
	var vnd any
	if l.VND != nil {
		vnd = int64(*l.VND)
	}
	doc := map[string]any{
		"line_id": l.ID, "name": l.Name, "name_norm": l.NameNorm, "qty": l.Qty, "unit": l.Unit, "orig": l.Orig,
		"line_total_vnd": vnd, "category_key": l.CategoryKey, "group_key": l.GroupKey, "habit": l.Habit,
		"raw_name": l.RawName, "brand": l.Brand, "product_name": l.ProductName, "detail_name": l.DetailName,
		"product_key": l.ProductKey, "source_language": l.SourceLang, "original_name": l.OriginalName,
		"package_count": l.PackageCount, "package_size": l.PackageSize, "package_unit": l.PackageUnit,
		"structure": l.Structure, "canonical_quantity": l.CanonicalQty, "canonical_unit": l.CanonicalUnit,
		"needs_review": l.NeedsReview, "inventory_owner": l.InventoryOwner, "price_source": l.PriceSource,
	}
	if l.DiscountVND > 0 {
		// Phần giảm giá của cả hóa đơn chia về dòng (mẹ đồng ý 03/11); giá sau giảm = line_total_vnd - discount_alloc_vnd.
		doc["discount_alloc_vnd"] = int64(l.DiscountVND)
	}
	if l.PlanSet {
		// B3: Kế hoạch mà dòng này đã trừ vào. Dòng cũ không có ô này: đọc theo source_plan_id.
		ps := make([]any, 0, len(l.Plans))
		for _, p := range l.Plans {
			ps = append(ps, map[string]any{"plan_id": p.PlanID, "qty": p.Qty})
		}
		doc["plans"] = ps
	}
	return doc
}

func catOrOther(c string) string {
	if c == "" {
		return taxonomy.CategoryUncategorized
	}
	return c
}

func latest(ps []domain.Purchase) domain.Date {
	var d domain.Date
	for _, p := range ps {
		if d.IsZero() || p.On.After(d) {
			d = p.On
		}
	}
	return d
}

func fxDoc(fx *receipt.FX) any {
	if fx == nil {
		return nil
	}
	return map[string]any{"rate": fx.Rate, "source": fx.Source, "at": fx.At}
}
