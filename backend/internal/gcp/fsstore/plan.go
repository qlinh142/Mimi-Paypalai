package fsstore

import (
	"context"
	"fmt"
	"math"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/pricing"
)

var _ plan.Store = (*Store)(nil)

// Giỏ: users/{uid}/plan_items/{group_key} {status: open|done, name, category_key, added_at, src}.
// Trạng thái chuyến mua: trường plan_state trên users/{uid}.

const (
	maxPlanLines = 200
	// pricesPerGroup: số lần mua gần nhất đọc cho mỗi nhóm để lấy giá mới nhất ở từng cửa hàng.
	pricesPerGroup = 20
)

func (s *Store) OpenLines(ctx context.Context, uid string) ([]plan.Line, error) {
	docs, err := s.planItems(uid, plan.IDFrom(ctx)).Where("status", "in", []string{"open", "assigned"}).
		Limit(maxPlanLines).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("plan lines: %w", err)
	}
	out := make([]plan.Line, 0, len(docs))
	for _, d := range docs {
		m := d.Data()
		l := plan.Line{Key: d.Ref.ID}
		l.Name, _ = m["name"].(string)
		l.CategoryKey, _ = m["category_key"].(string)
		l.AddedAt, _ = m["added_at"].(time.Time)
		l.Qty, l.BoughtQty = asFloat(m["qty"]), asFloat(m["bought_qty"])
		l.Unit, _ = m["unit"].(string)
		l.NeedBy, _ = m["need_by"].(string)
		l.Priority, _ = m["priority"].(string)
		l.Source, _ = m["src"].(string)
		l.Reason, _ = m["reason"].(string)
		l.Status, _ = m["status"].(string)
		l.AssignedTo, _ = m["assigned_to_uid"].(string)
		out = append(out, l)
	}
	return out, nil
}

// AddLine ghi đè cả tài liệu (không merge) để xóa dấu "done" của lần mua trước.
func (s *Store) AddLine(ctx context.Context, uid string, l plan.Line) error {
	src := l.Source
	if src == "" {
		src = plan.SourceManual
	}
	_, err := s.planItems(uid, plan.IDFrom(ctx)).Doc(l.Key).Set(ctx, map[string]any{
		"status": "open", "name": l.Name, "category_key": l.CategoryKey, "added_at": l.AddedAt, "src": src,
		"qty": l.Qty, "unit": l.Unit, "need_by": l.NeedBy, "priority": l.Priority, "reason": l.Reason, "bought_qty": 0.0,
	})
	if err != nil {
		return fmt.Errorf("plan add: %w", err)
	}
	return nil
}

func (s *Store) UpdateLine(ctx context.Context, uid string, l plan.Line) error {
	_, err := s.planItems(uid, plan.IDFrom(ctx)).Doc(l.Key).Update(ctx, []firestore.Update{
		{Path: "qty", Value: l.Qty}, {Path: "unit", Value: l.Unit}, {Path: "need_by", Value: l.NeedBy},
		{Path: "priority", Value: l.Priority}, {Path: "reason", Value: l.Reason}, {Path: "updated_at", Value: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("plan update: %w", err)
	}
	return nil
}

// RemoveLine: xóa tài liệu không tồn tại không phải lỗi trong Firestore.
func (s *Store) RemoveLine(ctx context.Context, uid, key string) error {
	if _, err := s.planItems(uid, plan.IDFrom(ctx)).Doc(key).Delete(ctx); err != nil {
		return fmt.Errorf("plan remove: %w", err)
	}
	return nil
}

// planStateDoc: Plan mặc định giữ plan_state trên users/{uid}; Plan đặt tên giữ trên plans/{id}.
func (s *Store) planStateDoc(ctx context.Context, uid string) *firestore.DocumentRef {
	if id := plan.IDFrom(ctx); id != plan.DefaultPlanID {
		return s.user(uid).Collection("plans").Doc(id)
	}
	return s.user(uid)
}

func (s *Store) State(ctx context.Context, uid string) (plan.State, error) {
	snap, err := s.planStateDoc(ctx, uid).Get(ctx)
	if isNotFound(err) {
		return plan.State{}, nil
	}
	if err != nil {
		return plan.State{}, fmt.Errorf("plan state: %w", err)
	}
	m, _ := snap.Data()["plan_state"].(map[string]any)
	var st plan.State
	st.BuyDate, _ = m["buy_date"].(string)
	st.TripNote, _ = m["trip_note"].(string)
	if h, ok := m["buy_hour"].(int64); ok {
		hh := int(h)
		st.BuyHour = &hh
	}
	if mm, ok := m["buy_minute"].(int64); ok && st.BuyHour != nil {
		v := int(mm)
		st.BuyMinute = &v
	}
	st.ReconciledOn, _ = snap.Data()["plan_reconciled_on"].(string)
	return st, nil
}

func (s *Store) SetState(ctx context.Context, uid string, st plan.State) error {
	var hour, minute any // nil: chưa chọn giờ
	if st.BuyHour != nil {
		hour = int64(*st.BuyHour)
		if st.BuyMinute != nil {
			minute = int64(*st.BuyMinute)
		}
	}
	_, err := s.planStateDoc(ctx, uid).Update(ctx, []firestore.Update{{Path: "plan_state", Value: map[string]any{
		"buy_date": st.BuyDate, "buy_hour": hour, "buy_minute": minute, "trip_note": st.TripNote, "updated_at": time.Now(),
	}}})
	if err != nil {
		return fmt.Errorf("plan set state: %w", err)
	}
	return nil
}

// Prices đọc các lần mua gần nhất của từng nhóm (index purchases group_key + habit +
// purchased_on DESC có sẵn) và giữ giá mới nhất ở mỗi cửa hàng. Giá mỗi đơn vị =
// thành tiền dòng / số lượng. Dòng chưa quy đổi được sang VND hoặc không rõ cửa hàng bị bỏ qua.
// Giới hạn: chỉ xét dòng đánh dấu "dùng để dự đoán" (habit), vì index chỉ phủ trường hợp đó.
func (s *Store) Prices(ctx context.Context, uid string, groups []string) ([]domain.LastPrice, error) {
	var out []domain.LastPrice
	for _, g := range groups {
		docs, err := s.user(uid).Collection("purchases").
			Where("group_key", "==", g).Where("habit", "==", true).
			OrderBy("purchased_on", firestore.Desc).Limit(pricesPerGroup).Documents(ctx).GetAll()
		if err != nil {
			return nil, fmt.Errorf("plan prices %s: %w", g, err)
		}
		seen := map[string]bool{}
		for _, d := range docs {
			m := d.Data()
			store, _ := m["store"].(string)
			total, ok := m["line_total_vnd"].(int64)
			qty := asFloat(m["qty"])
			if store == "" || seen[store] || !ok || total <= 0 || qty <= 0 {
				continue
			}
			seen[store] = true // tài liệu đã xếp mới nhất trước: giá đầu tiên gặp là giá mới nhất
			on, _ := domain.ParseDate(fmt.Sprint(m["purchased_on"]))
			out = append(out, domain.LastPrice{
				GroupKey: g, Store: store, Unit: domain.VND(math.Round(float64(total) / qty)), On: on,
			})
		}
	}
	return out, nil
}

// asFloat: qty lưu dạng số thực, nhưng giá trị nguyên có thể được đọc về dạng int64.
func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	}
	return 0
}

// ---------- Nhiều Kế hoạch (3.1 mục 3A) ----------

var _ plan.PlansStore = (*Store)(nil)

func decodePlanMeta(d *firestore.DocumentSnapshot) plan.Meta {
	m := d.Data()
	pm := plan.Meta{ID: d.Ref.ID, Status: "active"}
	pm.Name, _ = m["name"].(string)
	if st, _ := m["status"].(string); st != "" {
		pm.Status = st
	}
	pm.CreatedAt, _ = m["created_at"].(time.Time)
	if ps, ok := m["plan_state"].(map[string]any); ok {
		pm.BuyDate, _ = ps["buy_date"].(string)
	}
	return pm
}

func (s *Store) ListPlans(ctx context.Context, uid string) ([]plan.Meta, error) {
	docs, err := s.user(uid).Collection("plans").Limit(50).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("plans: %w", err)
	}
	out := make([]plan.Meta, 0, len(docs))
	for _, d := range docs {
		pm := decodePlanMeta(d)
		n, err := s.planItems(uid, pm.ID).Where("status", "in", []string{"open", "assigned"}).Limit(maxPlanLines).Documents(ctx).GetAll()
		if err != nil {
			return nil, fmt.Errorf("plan count: %w", err)
		}
		pm.ItemCount = len(n)
		out = append(out, pm)
	}
	return out, nil
}

func (s *Store) GetPlan(ctx context.Context, uid, id string) (plan.Meta, error) {
	snap, err := s.user(uid).Collection("plans").Doc(id).Get(ctx)
	if isNotFound(err) {
		return plan.Meta{}, apperr.ErrNotFound
	}
	if err != nil {
		return plan.Meta{}, fmt.Errorf("plan get: %w", err)
	}
	return decodePlanMeta(snap), nil
}

func (s *Store) CreatePlan(ctx context.Context, uid string, m plan.Meta) error {
	_, err := s.user(uid).Collection("plans").Doc(m.ID).Create(ctx, map[string]any{
		"name": m.Name, "status": m.Status, "created_at": m.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("plan create: %w", err)
	}
	return nil
}

func (s *Store) UpdatePlan(ctx context.Context, uid string, m plan.Meta) error {
	_, err := s.user(uid).Collection("plans").Doc(m.ID).Update(ctx, []firestore.Update{
		{Path: "name", Value: m.Name}, {Path: "status", Value: m.Status}, {Path: "updated_at", Value: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("plan update: %w", err)
	}
	return nil
}

func (s *Store) DeletePlan(ctx context.Context, uid, id string) error {
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		items, err := tx.Documents(s.planItems(uid, id).Limit(maxPlanLines)).GetAll()
		if err != nil {
			return err
		}
		for _, d := range items {
			if d.Data()["status"] == "assigned" {
				return apperr.New(apperr.CodeInvalidState, "Kế hoạch có món đang được nhờ mua.").With("key", d.Ref.ID)
			}
		}
		for _, d := range items {
			if err := tx.Delete(d.Ref); err != nil {
				return err
			}
		}
		return tx.Delete(s.user(uid).Collection("plans").Doc(id))
	})
}

// ---------- Lịch sử giá (8.1) ----------

var _ pricing.Store = (*Store)(nil)

// PriceObservations đọc purchases của từng nhóm (index group_key + habit + purchased_on DESC).
// Purchase trước 0.1 chưa có canonical_quantity thì quy từ số lượng và đơn vị in cạnh số lượng.
// Dòng cần xem lại (needs_review) không dùng để so giá.
func (s *Store) PriceObservations(ctx context.Context, uid string, groups []string, limit int) (map[string][]pricing.Observation, error) {
	out := map[string][]pricing.Observation{}
	for _, g := range groups {
		docs, err := s.user(uid).Collection("purchases").
			Where("group_key", "==", g).Where("habit", "==", true).
			OrderBy("purchased_on", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
		if err != nil {
			return nil, fmt.Errorf("price history %s: %w", g, err)
		}
		for _, d := range docs {
			m := d.Data()
			if rev, _ := m["needs_review"].(bool); rev {
				continue
			}
			total, ok := m["line_total_vnd"].(int64)
			if !ok || total <= 0 {
				continue
			}
			q, unit := purchaseQty(m)
			on, _ := domain.ParseDate(fmt.Sprint(m["purchased_on"]))
			o := pricing.Observation{GroupKey: g, PaidVND: total, On: on, CanonicalQty: q, CanonicalUnit: unit}
			o.Store, _ = m["store"].(string)
			o.StoreKey, _ = m["store_key"].(string)
			o.DetailName, _ = m["detail_name"].(string)
			out[g] = append(out[g], o)
		}
	}
	return out, nil
}
