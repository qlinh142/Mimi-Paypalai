package fsstore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/recurring"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

var _ recurring.Store = (*Store)(nil)

// Khoản định kỳ: users/{uid}/recurring/{id} {name, amount_vnd, currency, cycle, anchor_day,
// next_due_on, active, category_key}. Kỳ: users/{uid}/recurring_occurrences/{id}_{due}
// {recurring_id, name, due_on, amount_vnd, status, paid_on, source_type, source_id}.

func decodeRecurring(d *firestore.DocumentSnapshot) recurring.Recurring {
	m := d.Data()
	r := recurring.Recurring{ID: d.Ref.ID, AmountVND: asInt64(m["amount_vnd"]), Currency: "VND"}
	r.Name, _ = m["name"].(string)
	c, _ := m["cycle"].(string)
	r.Cycle = recurring.Cycle(c)
	r.NextDueOn, _ = m["next_due_on"].(string)
	r.Active, _ = m["active"].(bool)
	r.CategoryKey, _ = m["category_key"].(string)
	return r
}

func (s *Store) ListRecurring(ctx context.Context, uid string) ([]recurring.Recurring, error) {
	docs, err := s.user(uid).Collection("recurring").Limit(recurring.MaxRecurring + 1).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("recurring list: %w", err)
	}
	out := make([]recurring.Recurring, 0, len(docs))
	for _, d := range docs {
		out = append(out, decodeRecurring(d))
	}
	return out, nil
}

func (s *Store) GetRecurring(ctx context.Context, uid, id string) (recurring.Recurring, error) {
	if id == "" || len(id) > 120 {
		return recurring.Recurring{}, apperr.ErrNotFound
	}
	snap, err := s.user(uid).Collection("recurring").Doc(id).Get(ctx)
	if isNotFound(err) {
		return recurring.Recurring{}, apperr.ErrNotFound
	}
	if err != nil {
		return recurring.Recurring{}, fmt.Errorf("recurring get: %w", err)
	}
	return decodeRecurring(snap), nil
}

func recurringDoc(r recurring.Recurring, anchorDay int, now time.Time) map[string]any {
	return map[string]any{
		"name": r.Name, "amount_vnd": r.AmountVND, "currency": "VND", "cycle": string(r.Cycle), "anchor_day": int64(anchorDay),
		"next_due_on": r.NextDueOn, "active": r.Active, "category_key": r.CategoryKey, "updated_at": now,
	}
}

func occurrenceDoc(r recurring.Recurring, due string, now time.Time) map[string]any {
	return map[string]any{
		"recurring_id": r.ID, "name": r.Name, "due_on": due, "amount_vnd": r.AmountVND,
		"status": string(recurring.Pending), "created_at": now,
	}
}

func (s *Store) CreateRecurring(ctx context.Context, uid string, r recurring.Recurring, anchorDay int) error {
	u := s.user(uid)
	due, err := domain.ParseDate(r.NextDueOn)
	if err != nil {
		return err
	}
	now := time.Now()
	b := s.c.Batch()
	b.Create(u.Collection("recurring").Doc(r.ID), recurringDoc(r, anchorDay, now))
	b.Set(u.Collection("recurring_occurrences").Doc(recurring.OccurrenceID(r.ID, due)), occurrenceDoc(r, r.NextDueOn, now))
	if _, err := b.Commit(ctx); err != nil {
		return fmt.Errorf("recurring create: %w", err)
	}
	return nil
}

func (s *Store) pendingOf(ctx context.Context, tx *firestore.Transaction, uid, id string) ([]*firestore.DocumentSnapshot, error) {
	return tx.Documents(s.user(uid).Collection("recurring_occurrences").
		Where("recurring_id", "==", id).Where("status", "==", string(recurring.Pending))).GetAll()
}

func (s *Store) UpdateRecurring(ctx context.Context, uid string, r recurring.Recurring, anchorDay int, dueChanged bool) error {
	u := s.user(uid)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		pend, err := s.pendingOf(ctx, tx, uid, r.ID)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Set(u.Collection("recurring").Doc(r.ID), recurringDoc(r, anchorDay, now), firestore.MergeAll); err != nil {
			return err
		}
		// Kỳ pending luôn mang tên / số tiền mới nhất; đổi hạn thì thay kỳ đang chờ.
		for _, p := range pend {
			if dueChanged || !r.Active {
				if err := tx.Delete(p.Ref); err != nil {
					return err
				}
				continue
			}
			if err := tx.Update(p.Ref, []firestore.Update{{Path: "name", Value: r.Name}, {Path: "amount_vnd", Value: r.AmountVND}}); err != nil {
				return err
			}
		}
		if r.Active && (dueChanged || len(pend) == 0) {
			due, err := domain.ParseDate(r.NextDueOn)
			if err != nil {
				return err
			}
			return tx.Set(u.Collection("recurring_occurrences").Doc(recurring.OccurrenceID(r.ID, due)), occurrenceDoc(r, r.NextDueOn, now))
		}
		return nil
	})
}

func (s *Store) DeleteRecurring(ctx context.Context, uid, id string) error {
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		pend, err := s.pendingOf(ctx, tx, uid, id)
		if err != nil {
			return err
		}
		for _, p := range pend {
			if err := tx.Delete(p.Ref); err != nil {
				return err
			}
		}
		return tx.Delete(s.user(uid).Collection("recurring").Doc(id))
	})
}

// Occurrences dùng index recurring_occurrences (due_on); trạng thái lọc ở bộ nhớ.
func (s *Store) Occurrences(ctx context.Context, uid string, from, to domain.Date) ([]recurring.Occurrence, error) {
	docs, err := s.user(uid).Collection("recurring_occurrences").
		Where("due_on", ">=", from.String()).Where("due_on", "<=", to.String()).
		OrderBy("due_on", firestore.Asc).Limit(500).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("recurring occurrences: %w", err)
	}
	out := make([]recurring.Occurrence, 0, len(docs))
	for _, d := range docs {
		out = append(out, decodeOccurrence(d))
	}
	return out, nil
}

func (s *Store) Settle(ctx context.Context, uid, occurrenceID string, to recurring.Status, paidOn domain.Date) (recurring.Occurrence, error) {
	u := s.user(uid)
	if occurrenceID == "" || len(occurrenceID) > 140 {
		return recurring.Occurrence{}, apperr.ErrNotFound
	}
	ref := u.Collection("recurring_occurrences").Doc(occurrenceID)
	var out recurring.Occurrence
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if isNotFound(err) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		o := decodeOccurrence(snap)
		if o.Status != recurring.Pending {
			return apperr.New(apperr.CodeInvalidState, "Kỳ này đã được xử lý.").With("status", string(o.Status))
		}
		recSnap, err := tx.Get(u.Collection("recurring").Doc(o.RecurringID))
		if err != nil && !isNotFound(err) {
			return err
		}
		now := time.Now()
		upd := []firestore.Update{{Path: "status", Value: string(to)}, {Path: "updated_at", Value: now}}
		if to == recurring.Paid {
			upd = append(upd, firestore.Update{Path: "paid_on", Value: paidOn.String()}, firestore.Update{Path: "source_type", Value: "manual"})
		}
		if err := tx.Update(ref, upd); err != nil {
			return err
		}
		if to == recurring.Paid {
			// Rule R2: tiền đã thực chi cộng vào tháng thanh toán.
			cat := taxonomy.CategoryRecurring
			if recSnap != nil && recSnap.Exists() {
				if c, _ := recSnap.Data()["category_key"].(string); c != "" {
					cat = c
				}
			}
			if err := tx.Set(u.Collection("months").Doc(paidOn.Month()), map[string]any{
				"spent_vnd":     firestore.Increment(o.AmountVND),
				"recurring_vnd": firestore.Increment(o.AmountVND),
				"by_category":   map[string]any{cat: firestore.Increment(o.AmountVND)},
			}, firestore.MergeAll); err != nil {
				return err
			}
			o.PaidOn, o.SourceType = paidOn.String(), "manual"
		}
		o.Status = to
		out = o
		if recSnap == nil || !recSnap.Exists() {
			return nil
		}
		due, _ := domain.ParseDate(o.DueOn)
		cycle, _ := recSnap.Data()["cycle"].(string)
		next := recurring.NextDue(due, recurring.Cycle(cycle), int(asInt64(recSnap.Data()["anchor_day"])))
		return scheduleNext(tx, u, recSnap, o, next, now)
	})
	if err != nil {
		return recurring.Occurrence{}, err
	}
	return out, nil
}
