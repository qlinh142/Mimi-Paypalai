package fsstore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/item"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/recurring"
)

// ---------- Lưu hóa đơn: một transaction ----------
//
// Một lần lưu ghi, trong cùng một transaction (BUG-06 mục 6, AC08):
//   - receipts/{id} của người lưu (payer) và purchases cho từng dòng;
//   - tồn kho + dự đoán của từng nhóm bị ảnh hưởng, theo người nhận hàng (inventory owner):
//     dòng mua hộ vào tồn kho của Owner, dòng còn lại vào tồn kho của người lưu (4.1 Rule 4);
//   - months/{YYYY-MM} của người lưu (Spending theo payer, 4.1 mục 12);
//   - dòng Kế hoạch của đúng Plan đang đối chiếu (3.1 mục 3A.6), hoặc Plan của Owner khi mua hộ;
//   - vòng mua hộ: món đã mua → done, món chưa mua → trả về Owner, vòng kết thúc (4.1 mục 10);
//   - kỳ thanh toán định kỳ khớp hóa đơn → matched, tạo kỳ sau (2.1 Rule R3);
//   - asks của nhà khớp nhóm đóng lại, receipt_count tăng.
// Gửi lại cùng receipt_id, cùng nội dung: không ghi gì, trả kết quả cũ.

// purchaseHistory: số lần mua gần nhất cần đọc cho mỗi nhóm (đủ cho trung vị 3 khoảng).
const purchaseHistory = 6

// groupBuy là lượng mua của một nhóm trong hóa đơn, theo canonical unit.
type groupBuy struct {
	name, cat string
	qty       map[domain.Unit]float64
	units     []domain.Unit // thứ tự gặp trong hóa đơn
}

type groupState struct {
	history []domain.Purchase
	qtys    map[domain.Unit][]float64 // lượng mỗi lần mua trước, theo canonical unit
	prev    item.Record
	exists  bool
}

// planTarget là một dòng Kế hoạch đang mở có thể được hóa đơn đóng lại.
type planTarget struct {
	ref *firestore.DocumentRef
	doc map[string]any
}

type assignmentState struct {
	ref     *firestore.DocumentRef
	ownerID string
	planID  string
	items   map[string]map[string]any // key -> item của vòng mua hộ (status assigned)
}

func (s *Store) SaveReceipt(ctx context.Context, uid string, r receipt.Receipt, tz *time.Location) (receipt.SaveOutcome, error) {
	u := s.user(uid)
	rRef := u.Collection("receipts").Doc(r.ID)

	var out receipt.SaveOutcome
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		out = receipt.SaveOutcome{ReceiptID: r.ID}
		lines := append([]receipt.Line(nil), r.Lines...)

		// ---- ĐỌC (mọi lần đọc trước mọi lần ghi) ----
		existing, err := tx.Get(rRef)
		switch {
		case err == nil:
			if h, _ := existing.Data()["payload_hash"].(string); h == r.PayloadHash {
				out.Replayed = true
				return nil
			}
			return apperr.New(apperr.CodeReceiptConflict, "Mã hóa đơn này đã được dùng cho một hóa đơn khác.")
		case !isNotFound(err):
			return err
		}
		userSnap, err := tx.Get(u)
		if err != nil {
			return err
		}
		householdID, _ := userSnap.Data()["household_id"].(string)

		// Vòng mua hộ: chỉ Buyer của vòng đó, khi vòng còn mở, mới xác nhận được (4.1 mục 15).
		var asg *assignmentState
		if r.AssignmentID != "" {
			if householdID == "" {
				return apperr.ErrForbidden
			}
			asg, err = s.readAssignment(tx, householdID, r.AssignmentID, uid)
			if err != nil {
				return err
			}
			for i := range lines {
				if _, ok := asg.items[lines[i].GroupKey]; ok && lines[i].GroupKey != "" {
					lines[i].InventoryOwner = asg.ownerID
				}
			}
		}

		// Tồn kho theo người nhận hàng.
		buys := map[string]map[string]*groupBuy{} // owner uid -> group -> lượng mua
		for _, l := range lines {
			if !l.ForInventory() {
				continue
			}
			owner := ownerOf(l, uid)
			if buys[owner] == nil {
				buys[owner] = map[string]*groupBuy{}
			}
			b := buys[owner][l.GroupKey]
			if b == nil {
				b = &groupBuy{qty: map[domain.Unit]float64{}}
				buys[owner][l.GroupKey] = b
			}
			b.name, b.cat = lineName(l), l.CategoryKey
			unit := domain.Unit(l.CanonicalUnit)
			if _, ok := b.qty[unit]; !ok {
				b.units = append(b.units, unit)
			}
			b.qty[unit] += l.CanonicalQty
		}
		states := map[string]map[string]*groupState{}
		for _, owner := range sortedKeys(buys) {
			states[owner] = map[string]*groupState{}
			for _, g := range sortedKeys(buys[owner]) {
				st, err := s.readGroupState(tx, owner, g)
				if err != nil {
					return err
				}
				states[owner][g] = st
			}
		}

		// Kế hoạch: Plan của người lưu (dòng cá nhân) và Plan của Owner (dòng mua hộ).
		planTargets := map[string]map[string]planTarget{} // owner uid -> group -> dòng
		planGroups := map[string]map[string]bool{}
		for _, l := range lines {
			if l.GroupKey == "" {
				continue
			}
			owner := ownerOf(l, uid)
			if planGroups[owner] == nil {
				planGroups[owner] = map[string]bool{}
			}
			planGroups[owner][l.GroupKey] = true
		}
		for _, owner := range sortedKeys(planGroups) {
			planID := r.SourcePlanID
			if owner != uid {
				planID = asg.planID
			}
			planTargets[owner] = map[string]planTarget{}
			for _, g := range sortedKeys(planGroups[owner]) {
				ref := s.planItems(owner, planID).Doc(g)
				snap, err := tx.Get(ref)
				if err != nil && !isNotFound(err) {
					return err
				}
				if err != nil {
					continue
				}
				st := snap.Data()["status"]
				// Dòng cá nhân chỉ đóng dòng open; dòng đang giao cho người khác mua giữ nguyên.
				if (owner == uid && st == "open") || (owner != uid && st == "assigned") {
					planTargets[owner][g] = planTarget{ref: ref, doc: snap.Data()}
				}
			}
		}
		// Dòng Kế hoạch của Owner cho các món chưa mua trong vòng (để trả về open).
		returned := map[string]*firestore.DocumentRef{}
		if asg != nil {
			for g := range asg.items {
				if planGroups[asg.ownerID][g] {
					continue
				}
				returned[g] = s.planItems(asg.ownerID, asg.planID).Doc(g)
			}
		}

		var askRefs []*firestore.DocumentRef
		allGroups := sortedKeys(planGroups[uid])
		if householdID != "" && len(allGroups) > 0 {
			// "in" giới hạn số phần tử; chia lô 30.
			for i := 0; i < len(allGroups); i += 30 {
				end := min(i+30, len(allGroups))
				asks, err := tx.Documents(s.c.Collection("households").Doc(householdID).Collection("asks").
					Where("status", "==", "open").
					Where("group_key", "in", allGroups[i:end])).GetAll()
				if err != nil {
					return err
				}
				for _, a := range asks {
					askRefs = append(askRefs, a.Ref)
				}
			}
		}

		// Kỳ thanh toán định kỳ đang chờ quanh ngày mua (2.1 mục 8).
		type occMatch struct {
			ref  *firestore.DocumentRef
			occ  recurring.Occurrence
			rec  *firestore.DocumentSnapshot
			next domain.Date
		}
		var occs []occMatch
		pend, err := tx.Documents(u.Collection("recurring_occurrences").
			Where("status", "==", string(recurring.Pending)).
			Where("due_on", ">=", r.PurchasedOn.AddDays(-recurring.MatchWindowDays).String()).
			Where("due_on", "<=", r.PurchasedOn.AddDays(recurring.MatchWindowDays).String())).GetAll()
		if err != nil {
			return err
		}
		names, amounts := lineNamesAmounts(lines)
		for _, d := range pend {
			o := decodeOccurrence(d)
			if !recurring.MatchesReceipt(o, r.Store, names, int64(r.TotalVND), amounts, r.PurchasedOn) {
				continue
			}
			recSnap, err := tx.Get(u.Collection("recurring").Doc(o.RecurringID))
			if err != nil {
				if isNotFound(err) {
					continue
				}
				return err
			}
			due, _ := domain.ParseDate(o.DueOn)
			cycle, _ := recSnap.Data()["cycle"].(string)
			anchor := int(asInt64(recSnap.Data()["anchor_day"]))
			occs = append(occs, occMatch{ref: d.Ref, occ: o, rec: recSnap, next: recurring.NextDue(due, recurring.Cycle(cycle), anchor)})
		}

		// ---- GHI ----
		now := time.Now()
		docLines := make([]map[string]any, 0, len(lines))
		for _, l := range lines {
			docLines = append(docLines, lineDoc(l))
		}
		if err := tx.Create(rRef, map[string]any{
			"store": r.Store, "store_key": r.StoreKey, "purchased_on": r.PurchasedOn.String(), "currency": r.Currency,
			"fx": fxDoc(r.FX), "lines": docLines, "total_vnd": int64(r.TotalVND), "discount_vnd": int64(r.DiscountVND),
			"dedupe_key": r.DedupeKey, "payload_hash": r.PayloadHash, "saved_at": now,
			"source_plan_id": r.SourcePlanID, "assignment_id": r.AssignmentID, "submitted_by": uid,
		}); err != nil {
			return err
		}
		for _, l := range lines {
			owner := ownerOf(l, uid)
			doc := lineDoc(l)
			doc["receipt_id"], doc["purchased_on"], doc["store"], doc["store_key"] = r.ID, r.PurchasedOn.String(), r.Store, r.StoreKey
			doc["purchased_by_uid"], doc["payer_uid"], doc["inventory_owner_uid"] = uid, uid, owner
			if owner != uid {
				doc["assignment_id"] = r.AssignmentID
			}
			if err := tx.Set(s.user(owner).Collection("purchases").Doc(r.ID+"_"+l.ID), doc); err != nil {
				return err
			}
		}
		for _, owner := range sortedKeys(buys) {
			for g, b := range buys[owner] {
				if err := writeGroup(tx, s.user(owner).Collection("items").Doc(g), states[owner][g], b, r.PurchasedOn, now); err != nil {
					return err
				}
			}
		}

		// Tổng tháng của người trả tiền. Increment để không phải đọc; job cleanup đối chiếu lại
		// hằng đêm bằng cách tính lại từ receipts (dữ liệu suy ra phải tính lại được).
		monthDoc := map[string]any{
			"spent_vnd":    firestore.Increment(int64(r.TotalVND)),
			"discount_vnd": firestore.Increment(int64(r.DiscountVND)),
			"receipts":     firestore.Increment(1),
		}
		byCat := map[string]int64{}
		for _, l := range lines {
			if l.VND != nil {
				byCat[catOrOther(l.CategoryKey)] += int64(*l.VND)
			}
		}
		cat := map[string]any{}
		for c, v := range byCat {
			cat[c] = firestore.Increment(v)
		}
		monthDoc["by_category"] = cat
		if err := tx.Set(u.Collection("months").Doc(r.PurchasedOn.Month()), monthDoc, firestore.MergeAll); err != nil {
			return err
		}

		// Reconcile (3.1 mục 15, Mimi 2.0 mục 19): mua đủ số lượng thì dòng xong; mua thiếu
		// (cùng chiều đo) thì giữ mở và ghi lượng đã mua. Dòng chưa nhập số lượng coi là đủ.
		reconciled := map[string]*firestore.DocumentRef{} // doc Plan có món vừa đối chiếu
		for _, owner := range sortedKeys(planTargets) {
			for g, t := range planTargets[owner] {
				if pd := t.ref.Parent.Parent; pd != nil {
					reconciled[pd.Path] = pd
				}
				q, unit := lineQty(lines, g, owner, uid)
				upd, done := reconcilePlanLine(t.doc, q, unit, r.ID, now)
				if owner != uid {
					// Vòng mua hộ kết thúc sau hóa đơn này: mua thiếu cũng trả lại Owner.
					upd = append(upd, firestore.Update{Path: "assigned_to_uid", Value: nil}, firestore.Update{Path: "assignment_id", Value: nil})
					if !done {
						upd = append(upd, firestore.Update{Path: "status", Value: "open"})
					}
				}
				upd = append(upd, firestore.Update{Path: "matched_receipt_id", Value: r.ID})
				if err := tx.Update(t.ref, upd); err != nil {
					return err
				}
				if done && owner == uid {
					out.PlanDone = append(out.PlanDone, g)
				}
			}
		}
		// Plan đã có hóa đơn đối chiếu → giai đoạn reconciled (3.1 mục 18).
		for _, path := range sortedKeys(reconciled) {
			if err := tx.Update(reconciled[path], []firestore.Update{{Path: "plan_reconciled_on", Value: r.PurchasedOn.String()}}); err != nil {
				return err
			}
		}
		if asg != nil {
			res := &receipt.AssignmentResult{ID: r.AssignmentID, Done: []string{}, Returned: []string{}}
			items := map[string]any{}
			for g, it := range asg.items {
				cp := map[string]any{}
				for k, v := range it {
					cp[k] = v
				}
				if planGroups[asg.ownerID][g] {
					cp["status"], cp["matched_receipt_line"] = "done", r.ID
					res.Done = append(res.Done, g)
				} else {
					cp["status"] = "returned"
					res.Returned = append(res.Returned, g)
				}
				items[g] = cp
			}
			sort.Strings(res.Done)
			sort.Strings(res.Returned)
			res.Status = "completed"
			if len(res.Returned) > 0 {
				res.Status = "closed_partial"
			}
			if err := tx.Update(asg.ref, []firestore.Update{
				{Path: "items", Value: items}, {Path: "status", Value: res.Status},
				{Path: "closed_at", Value: now}, {Path: "matched_receipt_id", Value: r.ID},
			}); err != nil {
				return err
			}
			for _, g := range sortedKeys(returned) {
				// Món chưa mua: không done, trả về Plan của Owner, không tự gửi lại (Rule 5, 6).
				if err := tx.Update(returned[g], []firestore.Update{
					{Path: "status", Value: "open"}, {Path: "assigned_to_uid", Value: nil},
					{Path: "assignment_id", Value: nil}, {Path: "updated_at", Value: now},
				}); err != nil && !isNotFound(err) {
					return err
				}
			}
			out.Assignment = res
		}
		for _, m := range occs {
			// Rule R3: hóa đơn đã nằm trong Spending, không cộng lại; kỳ hiện tại rời Forecast.
			if err := tx.Update(m.ref, []firestore.Update{
				{Path: "status", Value: string(recurring.Matched)}, {Path: "paid_on", Value: r.PurchasedOn.String()},
				{Path: "source_type", Value: "receipt"}, {Path: "source_id", Value: r.ID}, {Path: "updated_at", Value: now},
			}); err != nil {
				return err
			}
			if err := scheduleNext(tx, u, m.rec, m.occ, m.next, now); err != nil {
				return err
			}
			out.Recurring = append(out.Recurring, m.occ.ID)
		}
		for _, ref := range askRefs {
			if err := tx.Update(ref, []firestore.Update{{Path: "status", Value: "closed"}, {Path: "closed_by_receipt", Value: r.ID}}); err != nil {
				return err
			}
		}
		return tx.Update(u, []firestore.Update{
			{Path: "receipt_count", Value: firestore.Increment(1)},
			{Path: "last_receipt_at", Value: now},
		})
	}, firestore.MaxAttempts(5))
	if err != nil {
		return receipt.SaveOutcome{}, err
	}
	return out, nil
}

func ownerOf(l receipt.Line, uid string) string {
	if l.InventoryOwner != "" {
		return l.InventoryOwner
	}
	return uid
}

func lineName(l receipt.Line) string {
	if l.DetailName != "" {
		return l.DetailName
	}
	return l.Name
}

func lineNamesAmounts(lines []receipt.Line) ([]string, []int64) {
	names := make([]string, 0, len(lines))
	amounts := make([]int64, 0, len(lines))
	for _, l := range lines {
		names = append(names, l.Name, l.RawName)
		if l.VND != nil {
			amounts = append(amounts, int64(*l.VND))
		}
	}
	return names, amounts
}

func (s *Store) readAssignment(tx *firestore.Transaction, householdID, id, buyer string) (*assignmentState, error) {
	ref := s.c.Collection("households").Doc(householdID).Collection("shopping_assignments").Doc(id)
	snap, err := tx.Get(ref)
	if isNotFound(err) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d := snap.Data()
	if b, _ := d["buyer_uid"].(string); b != buyer {
		return nil, apperr.ErrForbidden
	}
	if st, _ := d["status"].(string); st != "assigned" && st != "shopping" {
		return nil, apperr.New(apperr.CodeInvalidState, "Vòng mua hộ này đã kết thúc hoặc đã bị hủy.").With("status", st)
	}
	a := &assignmentState{ref: ref, items: map[string]map[string]any{}}
	a.ownerID, _ = d["owner_uid"].(string)
	a.planID, _ = d["plan_id"].(string)
	items, _ := d["items"].(map[string]any)
	for k, v := range items {
		m, _ := v.(map[string]any)
		if m != nil && m["status"] == "assigned" {
			a.items[k] = m
		}
	}
	return a, nil
}

func (s *Store) readGroupState(tx *firestore.Transaction, owner, g string) (*groupState, error) {
	u := s.user(owner)
	st := &groupState{qtys: map[domain.Unit][]float64{}}
	hist, err := tx.Documents(u.Collection("purchases").
		Where("group_key", "==", g).
		Where("habit", "==", true).
		OrderBy("purchased_on", firestore.Desc).
		Limit(purchaseHistory)).GetAll()
	if err != nil {
		return nil, err
	}
	for _, h := range hist {
		m := h.Data()
		if rev, _ := m["needs_review"].(bool); rev {
			continue
		}
		if d, err := domain.ParseDate(fmt.Sprint(m["purchased_on"])); err == nil {
			st.history = append(st.history, domain.Purchase{On: d, Habit: true})
		}
		q, unit := purchaseQty(m)
		if q > 0 {
			st.qtys[unit] = append(st.qtys[unit], q)
		}
	}
	snap, err := tx.Get(u.Collection("items").Doc(g))
	switch {
	case err == nil:
		st.prev, st.exists = decodeRecord(snap), true
	case !isNotFound(err):
		return nil, err
	}
	return st, nil
}

// purchaseQty: lượng canonical của một purchase; purchase cũ (trước 0.1) chưa có
// canonical_quantity thì quy từ số lượng và đơn vị in cạnh số lượng.
func purchaseQty(m map[string]any) (float64, domain.Unit) {
	if q := asFloat(m["canonical_quantity"]); q > 0 {
		u, _ := m["canonical_unit"].(string)
		return q, domain.ParseUnit(u)
	}
	unit, _ := m["unit"].(string)
	return domain.ToBase(asFloat(m["qty"]), unit)
}

// writeGroup tính lại dự đoán và tồn của một nhóm sau lần mua này. Chiều đo trùng baseline
// chính (hoặc chưa có baseline) cập nhật stream chính; chiều đo khác vào extra_stock, không
// cộng g với ml (0.1 mục 18, 20).
func writeGroup(tx *firestore.Transaction, ref *firestore.DocumentRef, st *groupState, b *groupBuy, on domain.Date, now time.Time) error {
	hist := append(st.history, domain.Purchase{On: on, Habit: true})
	pred := domain.Predict(hist, st.prev.Override)
	primary := st.prev.Baseline.Unit
	if st.prev.Baseline.On.IsZero() || primary == "" {
		primary = b.units[0]
	}
	doc := map[string]any{
		"name": b.name, "name_norm": domain.NormalizeVi(b.name), "category_key": b.cat,
		"last_purchase": latest(hist).String(), "prediction": predictionDoc(pred),
		"dropped": false, "updated_at": now,
	}
	extra := map[domain.Unit]item.Stream{}
	for k, v := range st.prev.Extra {
		extra[k] = v
	}
	for _, unit := range b.units {
		qty := b.qty[unit]
		rate := domain.ConsumptionRate(append(st.qtys[unit], qty), pred.IntervalDays)
		if unit == primary {
			// Baseline mới = tồn ước tính tại ngày mua + lượng vừa mua.
			base := domain.AfterPurchase(st.prev.Baseline, st.prev.RatePerDay, st.prev.Consumed, qty, unit, on, domain.BaselineReceipt)
			doc["stock"], doc["rate_per_day"], doc["consumed"] = baselineDoc(base), rate, []any{}
			continue
		}
		x := extra[unit]
		extra[unit] = item.Stream{
			Baseline:   domain.AfterPurchase(x.Baseline, x.RatePerDay, nil, qty, unit, on, domain.BaselineReceipt),
			RatePerDay: rate,
		}
	}
	if len(extra) > 0 {
		doc["extra_stock"] = extraDoc(extra)
	}
	return tx.Set(ref, doc, firestore.MergeAll)
}

// lineQty cộng lượng canonical của một nhóm trong hóa đơn cho một người nhận hàng, theo
// chiều đo của dòng đầu tiên.
func lineQty(lines []receipt.Line, group, owner, uid string) (float64, domain.Unit) {
	var total float64
	var unit domain.Unit
	for _, l := range lines {
		if l.GroupKey != group || ownerOf(l, uid) != owner || l.CanonicalQty <= 0 {
			continue
		}
		u := domain.Unit(l.CanonicalUnit)
		if unit == "" {
			unit = u
		}
		if u == unit {
			total += l.CanonicalQty
		}
	}
	return total, unit
}

// scheduleNext tạo kỳ tiếp theo (pending) của một khoản định kỳ và dời next_due_on
// (2.1 Rule R2, AC05). ID kỳ cố định theo hạn nên gọi lại không tạo trùng.
func scheduleNext(tx *firestore.Transaction, u *firestore.DocumentRef, rec *firestore.DocumentSnapshot, o recurring.Occurrence, next domain.Date, now time.Time) error {
	if active, _ := rec.Data()["active"].(bool); !active {
		return nil
	}
	amount := asInt64(rec.Data()["amount_vnd"])
	name, _ := rec.Data()["name"].(string)
	if err := tx.Set(u.Collection("recurring_occurrences").Doc(recurring.OccurrenceID(o.RecurringID, next)), map[string]any{
		"recurring_id": o.RecurringID, "name": name, "due_on": next.String(), "amount_vnd": amount,
		"status": string(recurring.Pending), "created_at": now,
	}); err != nil {
		return err
	}
	return tx.Update(rec.Ref, []firestore.Update{{Path: "next_due_on", Value: next.String()}, {Path: "updated_at", Value: now}})
}

func decodeOccurrence(d *firestore.DocumentSnapshot) recurring.Occurrence {
	m := d.Data()
	o := recurring.Occurrence{ID: d.Ref.ID, AmountVND: asInt64(m["amount_vnd"])}
	o.RecurringID, _ = m["recurring_id"].(string)
	o.Name, _ = m["name"].(string)
	o.DueOn, _ = m["due_on"].(string)
	st, _ := m["status"].(string)
	o.Status = recurring.Status(st)
	o.PaidOn, _ = m["paid_on"].(string)
	o.SourceType, _ = m["source_type"].(string)
	o.SourceID, _ = m["source_id"].(string)
	return o
}

// planItems trả collection dòng Kế hoạch của một Plan. Plan mặc định giữ đường dẫn cũ
// users/{uid}/plan_items để dữ liệu trước khi có nhiều Plan không phải chuyển đổi.
func (s *Store) planItems(uid, planID string) *firestore.CollectionRef {
	if planID == "" || planID == plan.DefaultPlanID {
		return s.user(uid).Collection("plan_items")
	}
	return s.user(uid).Collection("plans").Doc(planID).Collection("items")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
