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

// RenameItem sửa tên chuẩn hóa của món (0.1 mục 3.2). Lịch sử mua giữ tên in trên hóa đơn.
func (s *Store) RenameItem(ctx context.Context, uid, key, name string) (item.Record, error) {
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
		rec.Name, rec.NameNorm = name, domain.NormalizeVi(name)
		return tx.Update(ref, []firestore.Update{
			{Path: "name", Value: name}, {Path: "name_norm", Value: rec.NameNorm},
			{Path: "name_by_user", Value: true}, {Path: "updated_at", Value: time.Now()},
		})
	})
	return rec, err
}

// regroupLimit: số purchase tối đa chuyển trong một transaction (giới hạn 500 thao tác ghi).
const regroupLimit = 450

// RegroupItem chuyển món sang nhóm đúng (0.1 mục 3.2 "sửa group nếu AI phân loại sai"):
//   - mọi purchase của nhóm cũ đổi group_key sang nhóm mới, số liệu mua giữ nguyên;
//   - dự đoán của nhóm mới tính lại từ lịch sử gộp;
//   - tồn còn lại của nhóm cũ cộng vào nhóm mới nếu cùng chiều đo, khác chiều đo thì thành
//     stream riêng (không cộng g với ml);
//   - ghi classification_corrections để biết người dùng đã sửa, rồi xóa món cũ.
func (s *Store) RegroupItem(ctx context.Context, uid, from, to, category string) (item.Record, error) {
	u := s.user(uid)
	fromRef, toRef := u.Collection("items").Doc(from), u.Collection("items").Doc(to)
	today := domain.Today(time.Now(), time.UTC)
	var rec item.Record
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Đọc hết trước khi ghi.
		fromSnap, err := tx.Get(fromRef)
		if isNotFound(err) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		old := decodeRecord(fromSnap)
		moved, err := tx.Documents(u.Collection("purchases").Where("group_key", "==", from).Limit(regroupLimit + 1)).GetAll()
		if err != nil {
			return err
		}
		if len(moved) > regroupLimit {
			return apperr.New(apperr.CodeInvalidState, "Món có quá nhiều lần mua để chuyển nhóm một lần.")
		}
		dst, err := s.readGroupState(tx, uid, to)
		if err != nil {
			return err
		}

		hist := append([]domain.Purchase(nil), dst.history...)
		for _, d := range moved {
			m := d.Data()
			if habit, _ := m["habit"].(bool); !habit {
				continue
			}
			if rev, _ := m["needs_review"].(bool); rev {
				continue
			}
			if on, err := domain.ParseDate(fmt.Sprint(m["purchased_on"])); err == nil {
				hist = append(hist, domain.Purchase{On: on, Habit: true})
			}
		}

		rec = old
		if dst.exists {
			rec = dst.prev
		}
		rec.GroupKey = to
		if category != "" {
			rec.CategoryKey = category
		}
		rec.Prediction = domain.Predict(hist, rec.Override)
		if dst.exists {
			mergeStock(&rec, old, today)
		}

		for _, d := range moved {
			upd := []firestore.Update{{Path: "group_key", Value: to}}
			if category != "" {
				upd = append(upd, firestore.Update{Path: "category_key", Value: category})
			}
			if err := tx.Update(d.Ref, upd); err != nil {
				return err
			}
		}
		doc := map[string]any{
			"name": rec.Name, "name_norm": domain.NormalizeVi(rec.Name), "category_key": rec.CategoryKey,
			"prediction": predictionDoc(rec.Prediction), "dropped": false, "updated_at": time.Now(),
			"stock": baselineDoc(rec.Baseline), "rate_per_day": rec.RatePerDay, "consumed": consumedDoc(rec.Consumed),
		}
		if !latest(hist).IsZero() {
			doc["last_purchase"] = latest(hist).String()
		}
		if len(rec.Extra) > 0 {
			doc["extra_stock"] = extraDoc(rec.Extra)
		}
		if rec.Override != nil {
			doc["override"] = overrideDoc(*rec.Override)
		}
		if err := tx.Set(toRef, doc, firestore.MergeAll); err != nil {
			return err
		}
		if err := tx.Set(u.Collection("classification_corrections").Doc(from), map[string]any{
			"from_group_key": from, "to_group_key": to, "purchases_moved": len(moved), "at": time.Now(),
		}); err != nil {
			return err
		}
		return tx.Delete(fromRef)
	})
	return rec, err
}

// mergeStock cộng tồn còn lại của món cũ vào món đích tại ngày chuyển nhóm.
func mergeStock(dst *item.Record, old item.Record, today domain.Date) {
	add := func(b domain.Baseline, rate float64, used []domain.Consumption) {
		if b.On.IsZero() || b.Unit == "" {
			return
		}
		left := domain.EstimateAt(b, rate, used, today)
		switch {
		case dst.Baseline.On.IsZero():
			dst.Baseline, dst.RatePerDay, dst.Consumed = domain.Baseline{Qty: left, Unit: b.Unit, On: today, Source: b.Source}, rate, nil
		case dst.Baseline.Unit == b.Unit:
			dst.Baseline = domain.AfterPurchase(dst.Baseline, dst.RatePerDay, dst.Consumed, left, b.Unit, today, dst.Baseline.Source)
			dst.Consumed = nil
		default:
			if dst.Extra == nil {
				dst.Extra = map[domain.Unit]item.Stream{}
			}
			x := dst.Extra[b.Unit]
			dst.Extra[b.Unit] = item.Stream{Baseline: domain.AfterPurchase(x.Baseline, x.RatePerDay, nil, left, b.Unit, today, b.Source), RatePerDay: max(x.RatePerDay, rate)}
		}
	}
	add(old.Baseline, old.RatePerDay, old.Consumed)
	for _, x := range old.Extra {
		add(x.Baseline, x.RatePerDay, nil)
	}
}

func consumedDoc(cs []domain.Consumption) []any {
	out := make([]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]any{"qty": c.Qty, "on": c.On.String()})
	}
	return out
}
