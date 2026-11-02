package fsstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/order"
)

var _ order.Store = (*Store)(nil)

// Đơn Mimi Online: users/{uid}/orders/{id} {json, state, created_at}. Đơn được đọc ghi
// nguyên khối (vài chục dòng), state và created_at tách ra để truy vấn và kiểm tra.
// Household Mandate: trường mandate trên users/{uid}.
// Sổ PayPal: users/{uid}/payments/{order_id}, giữ cả capture gốc và các refund để audit.

func (s *Store) Mandate(ctx context.Context, uid string) (order.Mandate, error) {
	snap, err := s.user(uid).Get(ctx)
	if isNotFound(err) {
		return order.DefaultMandate(), nil
	}
	if err != nil {
		return order.Mandate{}, fmt.Errorf("mandate: %w", err)
	}
	raw, ok := snap.Data()["mandate"].(string)
	if !ok || raw == "" {
		return order.DefaultMandate(), nil
	}
	var m order.Mandate
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return order.Mandate{}, fmt.Errorf("mandate decode: %w", err)
	}
	return m, nil
}

func (s *Store) SetMandate(ctx context.Context, uid string, m order.Mandate) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.user(uid).Set(ctx, map[string]any{"mandate": string(b), "mandate_updated_at": time.Now()}, firestore.MergeAll)
	return err
}

func (s *Store) orderRef(uid, id string) *firestore.DocumentRef {
	return s.user(uid).Collection("orders").Doc(id)
}

func orderDoc(o order.Order) (map[string]any, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return map[string]any{"json": string(b), "state": string(o.State), "created_at": o.CreatedAt, "updated_at": time.Now()}, nil
}

func decodeOrder(snap *firestore.DocumentSnapshot) (order.Order, error) {
	raw, _ := snap.Data()["json"].(string)
	var o order.Order
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		return order.Order{}, fmt.Errorf("order decode %s: %w", snap.Ref.ID, err)
	}
	return o, nil
}

func (s *Store) CreateOrder(ctx context.Context, uid string, o order.Order) error {
	doc, err := orderDoc(o)
	if err != nil {
		return err
	}
	if _, err := s.orderRef(uid, o.ID).Create(ctx, doc); err != nil {
		return fmt.Errorf("order create: %w", err)
	}
	return nil
}

func (s *Store) GetOrder(ctx context.Context, uid, id string) (order.Order, error) {
	snap, err := s.orderRef(uid, id).Get(ctx)
	if isNotFound(err) {
		return order.Order{}, apperr.ErrNotFound
	}
	if err != nil {
		return order.Order{}, err
	}
	return decodeOrder(snap)
}

func (s *Store) ListOrders(ctx context.Context, uid string, limit int) ([]order.Order, error) {
	docs, err := s.user(uid).Collection("orders").OrderBy("created_at", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("orders: %w", err)
	}
	out := make([]order.Order, 0, len(docs))
	for _, d := range docs {
		o, err := decodeOrder(d)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

// guardState đọc đơn trong transaction và từ chối nếu trạng thái đã đổi.
func (s *Store) guardState(tx *firestore.Transaction, ref *firestore.DocumentRef, from order.State) error {
	snap, err := tx.Get(ref)
	if isNotFound(err) {
		return apperr.ErrNotFound
	}
	if err != nil {
		return err
	}
	if st, _ := snap.Data()["state"].(string); st != string(from) {
		return apperr.New(apperr.CodeInvalidState, "Đơn hàng vừa được cập nhật ở nơi khác.").With("state", st)
	}
	return nil
}

func (s *Store) UpdateOrder(ctx context.Context, uid string, o order.Order, from order.State) error {
	ref := s.orderRef(uid, o.ID)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := s.guardState(tx, ref, from); err != nil {
			return err
		}
		doc, err := orderDoc(o)
		if err != nil {
			return err
		}
		return tx.Set(ref, doc)
	})
}

// RecordPayment: PAYMENT SUCCESS chỉ đổi Financial State và Spending (Mimi 2.0 mục 15),
// không cộng tồn kho. Spending ghi theo tháng thanh toán, tách online_vnd để Sổ chi tiêu
// không đếm trùng với hóa đơn quét sau này.
func (s *Store) RecordPayment(ctx context.Context, uid string, o order.Order, tz *time.Location) error {
	u := s.user(uid)
	ref := s.orderRef(uid, o.ID)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := s.guardState(tx, ref, order.StateAwaitingPayment); err != nil {
			return err
		}
		doc, err := orderDoc(o)
		if err != nil {
			return err
		}
		if err := tx.Set(ref, doc); err != nil {
			return err
		}
		paidAt := time.Now()
		if o.PaidAt != nil {
			paidAt = *o.PaidAt
		}
		sums := map[string]int64{}
		for _, l := range o.Lines {
			sums[catOrOther(l.CategoryKey)] += l.TotalVND
		}
		byCat := map[string]any{}
		for c, v := range sums {
			byCat[c] = firestore.Increment(v)
		}
		month := domain.Today(paidAt, tz).Month()
		if err := tx.Set(u.Collection("months").Doc(month), map[string]any{
			"spent_vnd": firestore.Increment(o.TotalVND), "online_vnd": firestore.Increment(o.TotalVND),
			"online_orders": firestore.Increment(1), "by_category": byCat,
		}, firestore.MergeAll); err != nil {
			return err
		}
		return tx.Set(u.Collection("payments").Doc(o.ID), map[string]any{
			"provider": "paypal", "paypal_order_id": o.PayPalOrderID, "paypal_capture_id": o.CaptureID,
			"amount_vnd": o.TotalVND, "amount_usd": o.AmountUSD, "vnd_per_usd": o.VNDPerUSD,
			"rate_source": o.RateSource, "rate_as_of": o.RateAsOf,
			"merchant": o.Merchant, "paid_at": paidAt, "month": month, "refunds": []any{},
		})
	})
}

// ApplyDelivery: chỉ lượng đã Verify mới vào Confirmed Inventory (baseline nguồn delivery),
// rồi reconcile Kế hoạch như khi lưu hóa đơn.
func (s *Store) ApplyDelivery(ctx context.Context, uid string, o order.Order, tz *time.Location) error {
	u := s.user(uid)
	ref := s.orderRef(uid, o.ID)
	day := domain.Today(time.Now(), tz)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := s.guardState(tx, ref, order.StateDelivered); err != nil {
			return err
		}
		// ---- ĐỌC ----
		type target struct {
			line     order.Line
			itemSnap *firestore.DocumentSnapshot
			planRef  *firestore.DocumentRef
			planDoc  map[string]any
		}
		var targets []target
		for _, l := range o.Lines {
			if l.VerifiedQty == nil || *l.VerifiedQty <= 0 {
				continue
			}
			t := target{line: l}
			snap, err := tx.Get(u.Collection("items").Doc(l.Key))
			if err != nil && !isNotFound(err) {
				return err
			}
			if err == nil {
				t.itemSnap = snap
			}
			pref := u.Collection("plan_items").Doc(l.Key)
			psnap, err := tx.Get(pref)
			if err != nil && !isNotFound(err) {
				return err
			}
			if err == nil && psnap.Data()["status"] == "open" {
				t.planRef, t.planDoc = pref, psnap.Data()
			}
			targets = append(targets, t)
		}
		// ---- GHI ----
		now := time.Now()
		for _, t := range targets {
			qty, unit := domain.ToBase(*t.line.VerifiedQty*t.line.PackQty, t.line.Unit)
			var prev = decodeBaselineFrom(t.itemSnap)
			rate, consumed := 0.0, []domain.Consumption(nil)
			if t.itemSnap != nil {
				rec := decodeRecord(t.itemSnap)
				rate, consumed = rec.RatePerDay, rec.Consumed
			}
			b := domain.AfterPurchase(prev, rate, consumed, qty, unit, day, domain.BaselineDelivery)
			itemDoc := map[string]any{"stock": baselineDoc(b), "consumed": []any{}, "updated_at": now}
			if t.itemSnap == nil {
				itemDoc["name"], itemDoc["name_norm"] = t.line.Name, domain.NormalizeVi(t.line.Name)
				itemDoc["category_key"], itemDoc["dropped"] = t.line.CategoryKey, false
			}
			if err := tx.Set(u.Collection("items").Doc(t.line.Key), itemDoc, firestore.MergeAll); err != nil {
				return err
			}
			if t.planRef != nil {
				upd, _ := reconcilePlanLine(t.planDoc, qty, unit, o.ID, now)
				if err := tx.Update(t.planRef, upd); err != nil {
					return err
				}
			}
		}
		doc, err := orderDoc(o)
		if err != nil {
			return err
		}
		return tx.Set(ref, doc)
	})
}

func decodeBaselineFrom(snap *firestore.DocumentSnapshot) domain.Baseline {
	if snap == nil {
		return domain.Baseline{}
	}
	return decodeBaseline(snap.Data()["stock"])
}

// RecordRefund giữ refund trong sổ PayPal (audit) và trừ Spending của tháng thanh toán.
func (s *Store) RecordRefund(ctx context.Context, uid string, o order.Order, r order.Refund, tz *time.Location) error {
	u := s.user(uid)
	ref := s.orderRef(uid, o.ID)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := s.guardState(tx, ref, order.StateVerified); err != nil {
			return err
		}
		doc, err := orderDoc(o)
		if err != nil {
			return err
		}
		if err := tx.Set(ref, doc); err != nil {
			return err
		}
		paidAt := r.At
		if o.PaidAt != nil {
			paidAt = *o.PaidAt
		}
		month := domain.Today(paidAt, tz).Month()
		if err := tx.Set(u.Collection("months").Doc(month), map[string]any{
			"spent_vnd": firestore.Increment(-r.AmountVND), "online_vnd": firestore.Increment(-r.AmountVND),
			"refund_vnd": firestore.Increment(r.AmountVND),
		}, firestore.MergeAll); err != nil {
			return err
		}
		return tx.Update(u.Collection("payments").Doc(o.ID), []firestore.Update{{
			Path: "refunds", Value: firestore.ArrayUnion(map[string]any{
				"paypal_refund_id": r.ID, "ref": r.Ref, "amount_vnd": r.AmountVND, "amount_usd": r.AmountUSD,
				"reason": r.Reason, "status": r.Status, "at": r.At,
			}),
		}})
	})
}
