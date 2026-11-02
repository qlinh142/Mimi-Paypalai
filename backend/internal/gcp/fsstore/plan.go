package fsstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/sync/errgroup"

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
	st.Rev, _ = snap.Data()["plan_rev"].(int64)
	return st, nil
}

func (s *Store) SetState(ctx context.Context, uid string, st plan.State) (int64, error) {
	var hour, minute any // nil: chưa chọn giờ
	if st.BuyHour != nil {
		hour = int64(*st.BuyHour)
		if st.BuyMinute != nil {
			minute = int64(*st.BuyMinute)
		}
	}
	ref := s.planStateDoc(ctx, uid)
	var rev int64
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if err != nil {
			return err // không có tài liệu: Update bên dưới cũng sẽ lỗi NotFound như trước
		}
		cur, _ := snap.Data()["plan_rev"].(int64)
		if st.BaseRev != nil && *st.BaseRev != cur {
			return apperr.New(apperr.CodeConflict, "Kế hoạch vừa được sửa trên máy khác. Mimi đã tải lại bản mới, mẹ xem rồi sửa lại nhé.").
				With("rev", cur)
		}
		rev = cur + 1
		return tx.Update(ref, []firestore.Update{
			{Path: "plan_state", Value: map[string]any{
				"buy_date": st.BuyDate, "buy_hour": hour, "buy_minute": minute, "trip_note": st.TripNote, "updated_at": time.Now(),
			}},
			{Path: "plan_rev", Value: rev},
		})
	})
	if err != nil {
		if ae := (*apperr.Error)(nil); errors.As(err, &ae) {
			return 0, ae
		}
		return 0, fmt.Errorf("plan set state: %w", err)
	}
	return rev, nil
}

// Prices đọc các lần mua gần nhất của từng nhóm (index purchases group_key + habit +
// purchased_on DESC có sẵn) và giữ giá mới nhất ở mỗi cửa hàng. Giá mỗi đơn vị =
// thành tiền dòng / số lượng. Dòng chưa quy đổi được sang VND hoặc không rõ cửa hàng bị bỏ qua.
// Giới hạn: chỉ xét dòng đánh dấu "dùng để dự đoán" (habit), vì index chỉ phủ trường hợp đó.
func (s *Store) Prices(ctx context.Context, uid string, groups []string) ([]domain.LastPrice, error) {
	groups = uniqueGroups(groups)
	per := make([][]domain.LastPrice, len(groups))
	err := eachGroup(ctx, groups, func(ctx context.Context, i int, g string) error {
		docs, err := s.user(uid).Collection("purchases").
			Where("group_key", "==", g).Where("habit", "==", true).
			OrderBy("purchased_on", firestore.Desc).Limit(pricesPerGroup).Documents(ctx).GetAll()
		if err != nil {
			return fmt.Errorf("plan prices %s: %w", g, err)
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
			per[i] = append(per[i], domain.LastPrice{
				GroupKey: g, Store: store, Unit: domain.VND(math.Round(float64(total) / qty)), On: on,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out []domain.LastPrice
	for _, p := range per {
		out = append(out, p...)
	}
	return out, nil
}

// groupReadConcurrency: số truy vấn giá chạy cùng lúc. Firestore không có truy vấn "N bản
// mới nhất của mỗi nhóm", nên mỗi nhóm vẫn là một truy vấn; chạy song song thì giỏ 20 món
// mất khoảng thời gian của 3 lượt thay vì 20 lượt nối nhau (A9).
const groupReadConcurrency = 8

// eachGroup chạy fn cho từng nhóm, tối đa groupReadConcurrency cùng lúc. Lỗi đầu tiên
// hủy các lượt còn lại. fn ghi kết quả vào ô riêng theo i, nên không cần khóa.
func eachGroup(ctx context.Context, groups []string, fn func(ctx context.Context, i int, g string) error) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(groupReadConcurrency)
	for i, g := range groups {
		eg.Go(func() error { return fn(ctx, i, g) })
	}
	return eg.Wait()
}

// uniqueGroups bỏ nhóm trùng và rỗng, giữ thứ tự gặp đầu tiên.
func uniqueGroups(groups []string) []string {
	seen := make(map[string]bool, len(groups))
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		if g != "" && !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}

// LastUnitPrices: đơn giá lần mua gần nhất của từng nhóm, ở bất kỳ cửa hàng nào (B7).
func (s *Store) LastUnitPrices(ctx context.Context, uid string, groups []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(groups) == 0 {
		return out, nil
	}
	lp, err := s.Prices(ctx, uid, groups)
	if err != nil {
		return nil, err
	}
	return latestUnitPrices(lp), nil
}

func latestUnitPrices(lp []domain.LastPrice) map[string]int64 {
	out := map[string]int64{}
	at := map[string]string{}
	for _, p := range lp {
		if p.Unit <= 0 {
			continue
		}
		if d := p.On.String(); at[p.GroupKey] == "" || d > at[p.GroupKey] {
			at[p.GroupKey], out[p.GroupKey] = d, int64(p.Unit)
		}
	}
	return out
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
	pm.Icon, _ = m["icon"].(string)
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
		"name": m.Name, "status": m.Status, "created_at": m.CreatedAt, "icon": m.Icon,
	})
	if err != nil {
		return fmt.Errorf("plan create: %w", err)
	}
	return nil
}

func (s *Store) UpdatePlan(ctx context.Context, uid string, m plan.Meta) error {
	_, err := s.user(uid).Collection("plans").Doc(m.ID).Update(ctx, []firestore.Update{
		{Path: "name", Value: m.Name}, {Path: "status", Value: m.Status}, {Path: "icon", Value: m.Icon}, {Path: "updated_at", Value: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("plan update: %w", err)
	}
	return nil
}

// DefaultPlanIcon: biểu tượng của Plan mặc định nằm ở trường default_plan_icon trên users/{uid}.
func (s *Store) DefaultPlanIcon(ctx context.Context, uid string) (string, error) {
	snap, err := s.user(uid).Get(ctx)
	if isNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("default plan icon: %w", err)
	}
	v, _ := snap.Data()["default_plan_icon"].(string)
	return v, nil
}

func (s *Store) SetDefaultPlanIcon(ctx context.Context, uid, icon string) error {
	if _, err := s.user(uid).Set(ctx, map[string]any{"default_plan_icon": icon}, firestore.MergeAll); err != nil {
		return fmt.Errorf("set default plan icon: %w", err)
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
	groups = uniqueGroups(groups)
	per := make([][]pricing.Observation, len(groups))
	err := eachGroup(ctx, groups, func(ctx context.Context, i int, g string) error {
		docs, err := s.user(uid).Collection("purchases").
			Where("group_key", "==", g).Where("habit", "==", true).
			OrderBy("purchased_on", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
		if err != nil {
			return fmt.Errorf("price history %s: %w", g, err)
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
			per[i] = append(per[i], o)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := map[string][]pricing.Observation{}
	for i, g := range groups {
		if len(per[i]) > 0 {
			out[g] = per[i]
		}
	}
	return out, nil
}
