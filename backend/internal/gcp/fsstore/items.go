package fsstore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/item"
)

var _ item.Store = (*Store)(nil)

func predictionDoc(p domain.Prediction) map[string]any {
	out := map[string]any{
		"interval_days": p.IntervalDays, "purchase_count": p.PurchaseCount,
		"confidence": string(p.Confidence), "source": string(p.Source),
		"forced_out": p.ForcedOut, "preliminary": p.Preliminary, "out_on": "",
	}
	if !p.OutOn.IsZero() {
		out["out_on"] = p.OutOn.String()
	}
	return out
}

func decodePrediction(v any) domain.Prediction {
	m, _ := v.(map[string]any)
	p := domain.Prediction{}
	if s, _ := m["out_on"].(string); s != "" {
		p.OutOn, _ = domain.ParseDate(s)
	}
	if n, ok := m["interval_days"].(int64); ok {
		p.IntervalDays = int(n)
	}
	if n, ok := m["purchase_count"].(int64); ok {
		p.PurchaseCount = int(n)
	}
	c, _ := m["confidence"].(string)
	src, _ := m["source"].(string)
	p.Confidence, p.Source = domain.Confidence(c), domain.Source(src)
	p.ForcedOut, _ = m["forced_out"].(bool)
	p.Preliminary, _ = m["preliminary"].(bool)
	return p
}

func overrideDoc(o domain.Override) map[string]any {
	return map[string]any{"days": o.Days, "out": o.Out, "set_on": o.SetOn.String(), "set_at": o.SetAt}
}

func decodeOverride(v any) *domain.Override {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	o := &domain.Override{}
	if n, ok := m["days"].(int64); ok {
		o.Days = int(n)
	}
	o.Out, _ = m["out"].(bool)
	if s, _ := m["set_on"].(string); s != "" {
		o.SetOn, _ = domain.ParseDate(s)
	}
	o.SetAt, _ = m["set_at"].(time.Time)
	return o
}

func baselineDoc(b domain.Baseline) map[string]any {
	return map[string]any{"qty": b.Qty, "unit": string(b.Unit), "on": b.On.String(), "source": string(b.Source)}
}

func decodeBaseline(v any) domain.Baseline {
	m, _ := v.(map[string]any)
	if m == nil {
		return domain.Baseline{}
	}
	b := domain.Baseline{Qty: asFloat(m["qty"])}
	u, _ := m["unit"].(string)
	src, _ := m["source"].(string)
	b.Unit, b.Source = domain.ParseUnit(u), domain.BaselineSource(src)
	if s, _ := m["on"].(string); s != "" {
		b.On, _ = domain.ParseDate(s)
	}
	return b
}

func decodeConsumed(v any) []domain.Consumption {
	xs, _ := v.([]any)
	out := make([]domain.Consumption, 0, len(xs))
	for _, x := range xs {
		m, _ := x.(map[string]any)
		s, _ := m["on"].(string)
		on, err := domain.ParseDate(s)
		if err != nil {
			continue
		}
		out = append(out, domain.Consumption{Qty: asFloat(m["qty"]), On: on})
	}
	return out
}

func decodeRecord(snap *firestore.DocumentSnapshot) item.Record {
	d := snap.Data()
	r := item.Record{GroupKey: snap.Ref.ID, Prediction: decodePrediction(d["prediction"]), Override: decodeOverride(d["override"])}
	r.Name, _ = d["name"].(string)
	r.NameNorm, _ = d["name_norm"].(string)
	r.CategoryKey, _ = d["category_key"].(string)
	r.Dropped, _ = d["dropped"].(bool)
	if s, _ := d["last_purchase"].(string); s != "" {
		r.LastPurchase, _ = domain.ParseDate(s)
	}
	r.Baseline = decodeBaseline(d["stock"])
	r.RatePerDay = asFloat(d["rate_per_day"])
	r.Consumed = decodeConsumed(d["consumed"])
	r.Extra = decodeExtra(d["extra_stock"])
	return r
}

// extra_stock: {unit: {qty, unit, on, source, rate_per_day}} cho các chiều đo phụ.
func decodeExtra(v any) map[domain.Unit]item.Stream {
	m, _ := v.(map[string]any)
	if len(m) == 0 {
		return nil
	}
	out := make(map[domain.Unit]item.Stream, len(m))
	for k, x := range m {
		xm, _ := x.(map[string]any)
		u := domain.ParseUnit(k)
		if u == "" || xm == nil {
			continue
		}
		out[u] = item.Stream{Baseline: decodeBaseline(xm), RatePerDay: asFloat(xm["rate_per_day"])}
	}
	return out
}

func extraDoc(m map[domain.Unit]item.Stream) map[string]any {
	out := make(map[string]any, len(m))
	for u, x := range m {
		d := baselineDoc(x.Baseline)
		d["rate_per_day"] = x.RatePerDay
		out[string(u)] = d
	}
	return out
}

// ListItems đọc toàn bộ items của một người: mỗi nhà chỉ vài trăm món, lọc và tìm
// không dấu trong bộ nhớ rẻ hơn dựng thêm chỉ mục tìm kiếm.
func (s *Store) ListItems(ctx context.Context, uid string) ([]item.Record, error) {
	docs, err := s.user(uid).Collection("items").Limit(1000).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	out := make([]item.Record, 0, len(docs))
	for _, d := range docs {
		out = append(out, decodeRecord(d))
	}
	return out, nil
}

func (s *Store) GetItem(ctx context.Context, uid, key string) (item.Record, error) {
	snap, err := s.user(uid).Collection("items").Doc(key).Get(ctx)
	if isNotFound(err) {
		return item.Record{}, apperr.ErrNotFound
	}
	if err != nil {
		return item.Record{}, err
	}
	return decodeRecord(snap), nil
}

// SetOverride: lần bấm sau thắng (so SetAt), rồi tính lại dự đoán từ lịch sử mua.
func (s *Store) SetOverride(ctx context.Context, uid, key string, ov domain.Override) (item.Record, error) {
	return s.mutateOverride(ctx, uid, key, func(cur *domain.Override) *domain.Override {
		if cur != nil && !ov.SetAt.After(cur.SetAt) {
			return cur // thao tác cũ đến muộn (hàng đợi offline): giữ bản mới hơn
		}
		return &ov
	})
}

func (s *Store) ClearOverride(ctx context.Context, uid, key string, actedAt time.Time) (item.Record, error) {
	return s.mutateOverride(ctx, uid, key, func(cur *domain.Override) *domain.Override {
		if cur != nil && cur.SetAt.After(actedAt) {
			return cur // đã có thao tác mới hơn lần Hoàn tác này
		}
		return nil
	})
}

func (s *Store) mutateOverride(ctx context.Context, uid, key string, next func(*domain.Override) *domain.Override) (item.Record, error) {
	u := s.user(uid)
	ref := u.Collection("items").Doc(key)
	var rec item.Record
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if isNotFound(err) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		hist, err := tx.Documents(u.Collection("purchases").
			Where("group_key", "==", key).Where("habit", "==", true).
			OrderBy("purchased_on", firestore.Desc).Limit(purchaseHistory)).GetAll()
		if err != nil {
			return err
		}
		var ps []domain.Purchase
		for _, h := range hist {
			if d, err := domain.ParseDate(fmt.Sprint(h.Data()["purchased_on"])); err == nil {
				ps = append(ps, domain.Purchase{On: d, Habit: true})
			}
		}
		rec = decodeRecord(snap)
		ov := next(rec.Override)
		rec.Override = ov
		rec.Prediction = domain.Predict(ps, ov)
		var ovDoc any // nil xóa bản sửa tay
		if ov != nil {
			ovDoc = overrideDoc(*ov)
		}
		return tx.Update(ref, []firestore.Update{
			{Path: "override", Value: ovDoc},
			{Path: "prediction", Value: predictionDoc(rec.Prediction)},
			{Path: "updated_at", Value: time.Now()},
		})
	})
	return rec, err
}

// SetBaseline ghi Confirmed Inventory mới; các lần tiêu thụ trước mốc này bị bỏ vì đã
// nằm trong lượng người dùng vừa đếm.
func (s *Store) SetBaseline(ctx context.Context, uid, key string, b domain.Baseline) (item.Record, error) {
	ref := s.user(uid).Collection("items").Doc(key)
	var rec item.Record
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if isNotFound(err) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		rec = decodeRecord(snap)
		rec.Baseline = b
		var kept []any
		var keptC []domain.Consumption
		for _, c := range rec.Consumed {
			if c.On.After(b.On) {
				kept = append(kept, map[string]any{"qty": c.Qty, "on": c.On.String()})
				keptC = append(keptC, c)
			}
		}
		rec.Consumed = keptC
		return tx.Update(ref, []firestore.Update{
			{Path: "stock", Value: baselineDoc(b)},
			{Path: "consumed", Value: kept},
			{Path: "updated_at", Value: time.Now()},
		})
	})
	return rec, err
}

func (s *Store) OwnerOf(ctx context.Context, householdID string) (string, error) {
	snap, err := s.c.Collection("households").Doc(householdID).Get(ctx)
	if isNotFound(err) {
		return "", apperr.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	owner, _ := snap.Data()["owner_uid"].(string)
	if owner == "" {
		return "", apperr.ErrNotFound
	}
	return owner, nil
}

var _ item.LotReader = (*Store)(nil)

// Lots: mỗi purchase đã xác nhận của nhóm là một lot (0.1 mục 11). Purchase cần xem lại
// hoặc chưa có lượng canonical không phải lot.
func (s *Store) Lots(ctx context.Context, uid, key string, limit int) ([]domain.Lot, error) {
	docs, err := s.user(uid).Collection("purchases").
		Where("group_key", "==", key).Where("habit", "==", true).
		OrderBy("purchased_on", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("lots: %w", err)
	}
	out := make([]domain.Lot, 0, len(docs))
	for _, d := range docs {
		m := d.Data()
		if rev, _ := m["needs_review"].(bool); rev {
			continue
		}
		q, unit := purchaseQty(m)
		if q <= 0 {
			continue
		}
		on, _ := domain.ParseDate(fmt.Sprint(m["purchased_on"]))
		l := domain.Lot{ID: d.Ref.ID, On: on, Initial: q, Unit: unit, Source: "receipt"}
		l.DetailName, _ = m["detail_name"].(string)
		if l.DetailName == "" {
			l.DetailName, _ = m["name"].(string)
		}
		l.Brand, _ = m["brand"].(string)
		l.Store, _ = m["store"].(string)
		if src, _ := m["source"].(string); src == "delivery" {
			l.Source = "delivery"
		}
		out = append(out, l)
	}
	return out, nil
}
