package fsstore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/history"
)

var _ history.Store = (*Store)(nil)

// Lịch sử mua hàng chỉ đọc lại receipts, purchases và recurring_occurrences đã có.

func histLine(m map[string]any) history.Line {
	l := history.Line{}
	l.ReceiptID, _ = m["receipt_id"].(string)
	l.Name, _ = m["name"].(string)
	l.OriginalName, _ = m["original_name"].(string)
	l.Structure, _ = m["structure"].(string)
	l.Qty = asFloat(m["qty"])
	if v, ok := m["line_total_vnd"].(int64); ok {
		l.VND = &v
	}
	l.CategoryKey, _ = m["category_key"].(string)
	l.Owner, _ = m["inventory_owner"].(string)
	l.PayerUID, _ = m["payer_uid"].(string)
	l.Store, _ = m["store"].(string)
	l.PurchasedOn, _ = m["purchased_on"].(string)
	l.AssignmentID, _ = m["assignment_id"].(string)
	l.PriceSource, _ = m["price_source"].(string)
	if ps, ok := m["plans"].([]any); ok {
		l.PlanSet = true
		for _, x := range ps {
			if pm, ok := x.(map[string]any); ok {
				if id, _ := pm["plan_id"].(string); id != "" {
					l.PlanIDs = append(l.PlanIDs, id)
				}
			}
		}
	}
	return l
}

func histReceipt(d *firestore.DocumentSnapshot) history.Receipt {
	m := d.Data()
	r := history.Receipt{ID: d.Ref.ID, TotalVND: asInt64(m["total_vnd"]), DiscountVND: asInt64(m["discount_vnd"])}
	r.Store, _ = m["store"].(string)
	r.PurchasedOn, _ = m["purchased_on"].(string)
	r.SavedAt, _ = m["saved_at"].(time.Time)
	r.SourcePlanID, _ = m["source_plan_id"].(string)
	if ls, ok := m["lines"].([]any); ok {
		for _, x := range ls {
			if lm, ok := x.(map[string]any); ok {
				r.Lines = append(r.Lines, histLine(lm))
			}
		}
	}
	return r
}

func (s *Store) Receipts(ctx context.Context, uid string, limit int) ([]history.Receipt, error) {
	docs, err := s.user(uid).Collection("receipts").OrderBy("purchased_on", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("history receipts: %w", err)
	}
	out := make([]history.Receipt, 0, len(docs))
	for _, d := range docs {
		out = append(out, histReceipt(d))
	}
	return out, nil
}

func (s *Store) Receipt(ctx context.Context, uid, id string) (history.Receipt, error) {
	snap, err := s.user(uid).Collection("receipts").Doc(id).Get(ctx)
	if isNotFound(err) {
		return history.Receipt{}, apperr.ErrNotFound
	}
	if err != nil {
		return history.Receipt{}, fmt.Errorf("history receipt: %w", err)
	}
	return histReceipt(snap), nil
}

// DelegatedLines: purchases của uid do người khác trả tiền (dòng mua hộ, 4.1 Rule 4).
func (s *Store) DelegatedLines(ctx context.Context, uid string, limit int) ([]history.Line, error) {
	docs, err := s.user(uid).Collection("purchases").Where("payer_uid", "!=", uid).Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("history delegated: %w", err)
	}
	out := make([]history.Line, 0, len(docs))
	for _, d := range docs {
		out = append(out, histLine(d.Data()))
	}
	return out, nil
}

// PaidManually: kỳ định kỳ đã trả tay; kỳ khớp hóa đơn đã có trong hóa đơn nên bỏ qua.
func (s *Store) PaidManually(ctx context.Context, uid string, limit int) ([]history.Paid, error) {
	docs, err := s.user(uid).Collection("recurring_occurrences").Where("status", "==", "paid").Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("history paid: %w", err)
	}
	var out []history.Paid
	for _, d := range docs {
		m := d.Data()
		if src, _ := m["source_type"].(string); src == "receipt" {
			continue
		}
		p := history.Paid{ID: d.Ref.ID, AmountVND: asInt64(m["amount_vnd"])}
		p.RecurringID, _ = m["recurring_id"].(string)
		p.Name, _ = m["name"].(string)
		if p.PaidOn, _ = m["paid_on"].(string); p.PaidOn == "" {
			p.PaidOn, _ = m["due_on"].(string)
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) DisplayNames(ctx context.Context, uids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(uids) == 0 {
		return out, nil
	}
	refs := make([]*firestore.DocumentRef, 0, len(uids))
	for _, u := range uids {
		refs = append(refs, s.user(u))
	}
	snaps, err := s.c.GetAll(ctx, refs)
	if err != nil {
		return nil, fmt.Errorf("history names: %w", err)
	}
	for _, sn := range snaps {
		if sn.Exists() {
			out[sn.Ref.ID], _ = sn.Data()["display_name"].(string)
		}
	}
	return out, nil
}

func (s *Store) PlanNames(ctx context.Context, uid string) (map[string]string, error) {
	docs, err := s.user(uid).Collection("plans").Select("name").Limit(50).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("history plans: %w", err)
	}
	out := map[string]string{}
	for _, d := range docs {
		out[d.Ref.ID], _ = d.Data()["name"].(string)
	}
	return out, nil
}
