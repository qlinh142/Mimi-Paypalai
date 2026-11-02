package fsstore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/household"
	"github.com/smartlife-lab/mimi/backend/internal/notify"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
)

var (
	_ household.Store = (*Store)(nil)
	_ household.Plans = (*Store)(nil)
)

// Nhà: households/{hid} {name, owner_uid, members: {uid: {display_name, role, joined_at}},
// invite_code}. Mã nhà cố định, không hết hạn (mẹ chốt 03/11); invite_expires_at là trường cũ, bỏ qua. users/{uid}.household_id và .role trỏ ngược lại.
// Vòng mua hộ: households/{hid}/shopping_assignments/{id}
// {owner_uid, buyer_uid, plan_id, status, note, items: {group_key: {...}}, created_at, closed_at}.

func (s *Store) household(hid string) *firestore.DocumentRef {
	return s.c.Collection("households").Doc(hid)
}

func decodeHousehold(snap *firestore.DocumentSnapshot) household.Household {
	m := snap.Data()
	h := household.Household{ID: snap.Ref.ID, Members: []household.Member{}}
	h.Name, _ = m["name"].(string)
	h.OwnerUID, _ = m["owner_uid"].(string)
	members, _ := m["members"].(map[string]any)
	for uid, v := range members {
		mm, _ := v.(map[string]any)
		x := household.Member{UID: uid}
		x.DisplayName, _ = mm["display_name"].(string)
		x.Role, _ = mm["role"].(string)
		x.JoinedAt, _ = mm["joined_at"].(time.Time)
		h.Members = append(h.Members, x)
	}
	sort.Slice(h.Members, func(i, j int) bool {
		if (h.Members[i].Role == "owner") != (h.Members[j].Role == "owner") {
			return h.Members[i].Role == "owner"
		}
		return h.Members[i].JoinedAt.Before(h.Members[j].JoinedAt)
	})
	if code, _ := m["invite_code"].(string); code != "" {
		h.Invite = &household.Invite{Code: code}
	}
	return h
}

func (s *Store) Household(ctx context.Context, hid string) (household.Household, error) {
	if hid == "" {
		return household.Household{}, apperr.ErrNotFound
	}
	snap, err := s.household(hid).Get(ctx)
	if isNotFound(err) {
		return household.Household{}, apperr.ErrNotFound
	}
	if err != nil {
		return household.Household{}, fmt.Errorf("household get: %w", err)
	}
	return decodeHousehold(snap), nil
}

func memberDoc(m household.Member) map[string]any {
	return map[string]any{"display_name": m.DisplayName, "role": m.Role, "joined_at": m.JoinedAt}
}

// EnsureHousehold: ID nhà cố định theo chủ ("h-" + uid) nên gọi lại không tạo nhà thứ hai.
func (s *Store) EnsureHousehold(ctx context.Context, owner household.Member, name string) (string, error) {
	hid := "h-" + owner.UID
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		ref := s.household(hid)
		snap, err := tx.Get(ref)
		if err != nil && !isNotFound(err) {
			return err
		}
		if err == nil && snap.Exists() {
			if name != "" {
				return tx.Update(ref, []firestore.Update{{Path: "name", Value: name}})
			}
			return nil
		}
		if err := tx.Create(ref, map[string]any{
			"name": name, "owner_uid": owner.UID, "members": map[string]any{owner.UID: memberDoc(owner)}, "created_at": time.Now(),
		}); err != nil {
			return err
		}
		return tx.Set(s.user(owner.UID), map[string]any{"household_id": hid, "role": "owner"}, firestore.MergeAll)
	})
	if err != nil {
		return "", fmt.Errorf("household ensure: %w", err)
	}
	return hid, nil
}

func (s *Store) SetInvite(ctx context.Context, hid string, inv household.Invite) error {
	_, err := s.household(hid).Update(ctx, []firestore.Update{
		{Path: "invite_code", Value: inv.Code}, {Path: "invite_expires_at", Value: firestore.Delete},
	})
	if err != nil {
		return fmt.Errorf("household invite: %w", err)
	}
	return nil
}

func (s *Store) Join(ctx context.Context, m household.Member, code string, now time.Time, maxMembers int) (string, error) {
	var hid string
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		docs, err := tx.Documents(s.c.Collection("households").Where("invite_code", "==", code).Limit(1)).GetAll()
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return apperr.New(apperr.CodeNotFound, "Mã mời không đúng.").With("field", "code")
		}
		h := decodeHousehold(docs[0])
		if h.Invite == nil {
			return apperr.New(apperr.CodeNotFound, "Mã mời không đúng.").With("field", "code")
		}
		hid = h.ID
		for _, x := range h.Members {
			if x.UID == m.UID {
				return nil // đã ở trong nhà: gọi lại không lỗi
			}
		}
		if len(h.Members) >= maxMembers {
			return apperr.New(apperr.CodeValidation, "Nhà đã đủ người.").With("limit", maxMembers)
		}
		if err := tx.Update(docs[0].Ref, []firestore.Update{{FieldPath: firestore.FieldPath{"members", m.UID}, Value: memberDoc(m)}}); err != nil {
			return err
		}
		return tx.Set(s.user(m.UID), map[string]any{"household_id": hid, "role": "member"}, firestore.MergeAll)
	})
	if err != nil {
		return "", err
	}
	return hid, nil
}

func (s *Store) RemoveMember(ctx context.Context, hid, uid string) error {
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		asg := s.household(hid).Collection("shopping_assignments")
		var open []*firestore.DocumentSnapshot
		for _, field := range []string{"buyer_uid", "owner_uid"} {
			docs, err := tx.Documents(asg.Where(field, "==", uid).Where("status", "in", []string{"assigned", "shopping"})).GetAll()
			if err != nil {
				return err
			}
			open = append(open, docs...)
		}
		returns, err := s.readReturns(tx, open)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := s.writeCancel(tx, open, returns, now); err != nil {
			return err
		}
		if err := tx.Update(s.household(hid), []firestore.Update{{FieldPath: firestore.FieldPath{"members", uid}, Value: firestore.Delete}}); err != nil {
			return err
		}
		return tx.Set(s.user(uid), map[string]any{"household_id": "", "role": "owner"}, firestore.MergeAll)
	})
}

func (s *Store) Rename(ctx context.Context, hid, name string) error {
	if _, err := s.household(hid).Update(ctx, []firestore.Update{{Path: "name", Value: name}}); err != nil {
		return fmt.Errorf("household rename: %w", err)
	}
	return nil
}

// OpenLineInfo đọc các dòng đang mở của một Plan để chụp lại vào vòng mua hộ.
func (s *Store) OpenLineInfo(ctx context.Context, uid, planID string, keys []string) (map[string]household.AssignmentItem, error) {
	col := s.planItems(uid, planID)
	refs := make([]*firestore.DocumentRef, 0, len(keys))
	for _, k := range keys {
		refs = append(refs, col.Doc(k))
	}
	snaps, err := s.c.GetAll(ctx, refs)
	if err != nil {
		return nil, fmt.Errorf("plan lines: %w", err)
	}
	out := map[string]household.AssignmentItem{}
	for _, sn := range snaps {
		if !sn.Exists() || sn.Data()["status"] != "open" {
			continue
		}
		m := sn.Data()
		it := household.AssignmentItem{Key: sn.Ref.ID, Qty: asFloat(m["qty"])}
		it.Name, _ = m["name"].(string)
		it.Unit, _ = m["unit"].(string)
		out[sn.Ref.ID] = it
	}
	return out, nil
}

func assignmentItemsDoc(items []household.AssignmentItem) map[string]any {
	out := map[string]any{}
	for _, it := range items {
		out[it.Key] = map[string]any{"name_display": it.Name, "qty": it.Qty, "unit": it.Unit, "status": it.Status}
	}
	return out
}

func decodeAssignment(d *firestore.DocumentSnapshot) household.Assignment {
	m := d.Data()
	a := household.Assignment{ID: d.Ref.ID, Items: []household.AssignmentItem{}}
	a.OwnerUID, _ = m["owner_uid"].(string)
	a.OwnerName, _ = m["owner_display_name"].(string)
	a.BuyerUID, _ = m["buyer_uid"].(string)
	a.BuyerName, _ = m["buyer_display_name"].(string)
	a.PlanID, _ = m["plan_id"].(string)
	a.Status, _ = m["status"].(string)
	a.Note, _ = m["note"].(string)
	a.CreatedAt, _ = m["created_at"].(time.Time)
	if t, ok := m["updated_at"].(time.Time); ok {
		a.UpdatedAt = &t
	}
	if t, ok := m["closed_at"].(time.Time); ok {
		a.ClosedAt = &t
	}
	a.MatchedReceiptID, _ = m["matched_receipt_id"].(string)
	items, _ := m["items"].(map[string]any)
	for k, v := range items {
		mm, _ := v.(map[string]any)
		it := household.AssignmentItem{Key: k, Qty: asFloat(mm["qty"])}
		it.Name, _ = mm["name_display"].(string)
		it.Unit, _ = mm["unit"].(string)
		it.Status, _ = mm["status"].(string)
		a.Items = append(a.Items, it)
	}
	sort.Slice(a.Items, func(i, j int) bool { return a.Items[i].Key < a.Items[j].Key })
	return a
}

func (s *Store) CreateAssignment(ctx context.Context, hid string, a household.Assignment) (household.Assignment, error) {
	ref := s.household(hid).Collection("shopping_assignments").Doc(a.ID)
	col := s.planItems(a.OwnerUID, a.PlanID)
	var ev pendingEvent
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		refs := make([]*firestore.DocumentRef, 0, len(a.Items))
		for _, it := range a.Items {
			refs = append(refs, col.Doc(it.Key))
		}
		snaps, err := tx.GetAll(refs)
		if err != nil {
			return err
		}
		for _, sn := range snaps {
			if !sn.Exists() || sn.Data()["status"] != "open" {
				return apperr.New(apperr.CodeInvalidState, "Có món không còn mở trong Kế hoạch.").With("key", sn.Ref.ID)
			}
		}
		now := time.Now()
		for _, r := range refs {
			if err := tx.Update(r, []firestore.Update{
				{Path: "status", Value: "assigned"}, {Path: "assigned_to_uid", Value: a.BuyerUID},
				{Path: "assignment_id", Value: a.ID}, {Path: "updated_at", Value: now},
			}); err != nil {
				return err
			}
		}
		if ev, err = s.writeEvent(tx, a.BuyerUID, event{Event: notify.Event{
			ID: "sent-" + a.ID, Kind: notify.EventPlanSent, At: now, ActorName: a.OwnerName,
			PlanID: a.PlanID, AssignmentID: a.ID, Items: len(a.Items),
		}, planOwner: a.OwnerUID}); err != nil {
			return err
		}
		return tx.Create(ref, map[string]any{
			"owner_uid": a.OwnerUID, "owner_display_name": a.OwnerName, "buyer_uid": a.BuyerUID, "buyer_display_name": a.BuyerName,
			"plan_id": a.PlanID, "status": a.Status, "note": a.Note, "items": assignmentItemsDoc(a.Items),
			"created_at": a.CreatedAt, "closed_at": nil, "matched_receipt_id": nil,
		})
	})
	if err != nil {
		return household.Assignment{}, err
	}
	s.enqueue(ctx, ev)
	return a, nil
}

// UpdateAssignment ghi đè bản đã gửi của một vòng còn mở (B5). Trong cùng transaction: món mới
// chuyển assigned, món bị bỏ trả về open, tên và số lượng lấy lại từ dòng Kế hoạch hiện tại.
func (s *Store) UpdateAssignment(ctx context.Context, hid string, a household.Assignment) (household.Assignment, error) {
	if hid == "" || a.ID == "" || len(a.ID) > 80 {
		return household.Assignment{}, apperr.ErrNotFound
	}
	ref := s.household(hid).Collection("shopping_assignments").Doc(a.ID)
	col := s.planItems(a.OwnerUID, a.PlanID)
	var out household.Assignment
	var ev pendingEvent
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if isNotFound(err) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		cur := decodeAssignment(snap)
		if cur.OwnerUID != a.OwnerUID {
			return apperr.ErrForbidden
		}
		if !cur.Active() {
			return apperr.New(apperr.CodeInvalidState, "Vòng mua hộ này đã kết thúc.").With("status", cur.Status)
		}
		want := map[string]bool{}
		var keys []string
		for _, it := range a.Items {
			if it.Status == "assigned" {
				want[it.Key] = true
				keys = append(keys, it.Key)
			}
		}
		var dropped []string
		for _, it := range cur.Items {
			if it.Status == "assigned" && !want[it.Key] {
				dropped = append(dropped, it.Key)
			}
		}
		refs := make([]*firestore.DocumentRef, 0, len(keys)+len(dropped))
		for _, k := range append(append([]string{}, keys...), dropped...) {
			refs = append(refs, col.Doc(k))
		}
		snaps, err := tx.GetAll(refs)
		if err != nil {
			return err
		}
		lines := map[string]map[string]any{}
		for _, sn := range snaps {
			if sn.Exists() {
				lines[sn.Ref.ID] = sn.Data()
			}
		}
		mine := func(m map[string]any) bool { return m["status"] == "assigned" && m["assignment_id"] == a.ID }
		now := time.Now()
		for i, it := range a.Items {
			if it.Status != "assigned" {
				continue
			}
			m := lines[it.Key]
			if m == nil || (m["status"] != "open" && !mine(m)) {
				return apperr.New(apperr.CodeInvalidState, "Có món không còn mở trong Kế hoạch.").With("key", it.Key)
			}
			a.Items[i].Qty = asFloat(m["qty"])
			if n, _ := m["name"].(string); n != "" {
				a.Items[i].Name = n
			}
			a.Items[i].Unit, _ = m["unit"].(string)
			if m["status"] == "open" {
				if err := tx.Update(col.Doc(it.Key), []firestore.Update{
					{Path: "status", Value: "assigned"}, {Path: "assigned_to_uid", Value: cur.BuyerUID},
					{Path: "assignment_id", Value: a.ID}, {Path: "updated_at", Value: now},
				}); err != nil {
					return err
				}
			}
		}
		for _, k := range dropped {
			if m := lines[k]; m != nil && mine(m) {
				if err := tx.Update(col.Doc(k), []firestore.Update{
					{Path: "status", Value: "open"}, {Path: "assigned_to_uid", Value: nil},
					{Path: "assignment_id", Value: nil}, {Path: "updated_at", Value: now},
				}); err != nil {
					return err
				}
			}
		}
		at := now
		if a.UpdatedAt != nil {
			at = *a.UpdatedAt
		}
		if err := tx.Update(ref, []firestore.Update{
			{Path: "items", Value: assignmentItemsDoc(a.Items)}, {Path: "note", Value: a.Note}, {Path: "updated_at", Value: at},
		}); err != nil {
			return err
		}
		assigned := 0
		for _, it := range a.Items {
			if it.Status == "assigned" {
				assigned++
			}
		}
		if ev, err = s.writeEvent(tx, cur.BuyerUID, event{Event: notify.Event{
			ID: fmt.Sprintf("upd-%s-%d", a.ID, at.UnixMilli()), Kind: notify.EventPlanUpdated, At: at, ActorName: cur.OwnerName,
			PlanID: cur.PlanID, AssignmentID: a.ID, Items: assigned,
		}, planOwner: cur.OwnerUID}); err != nil {
			return err
		}
		out = cur
		out.Items, out.Note, out.UpdatedAt = a.Items, a.Note, &at
		sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Key < out.Items[j].Key })
		return nil
	})
	if err != nil {
		return household.Assignment{}, err
	}
	s.enqueue(ctx, ev)
	return out, nil
}

// readReturns đọc dòng Kế hoạch của các món còn assigned trong những vòng sắp hủy.
func (s *Store) readReturns(tx *firestore.Transaction, open []*firestore.DocumentSnapshot) (map[string][]*firestore.DocumentRef, error) {
	out := map[string][]*firestore.DocumentRef{}
	for _, d := range open {
		a := decodeAssignment(d)
		for _, it := range a.Items {
			if it.Status != "assigned" {
				continue
			}
			ref := s.planItems(a.OwnerUID, a.PlanID).Doc(it.Key)
			snap, err := tx.Get(ref)
			if err != nil && !isNotFound(err) {
				return nil, err
			}
			// Chỉ trả về dòng vẫn đang giao cho vòng này (Owner có thể đã xóa dòng).
			if err == nil && snap.Data()["assignment_id"] == a.ID && snap.Data()["status"] == "assigned" {
				out[d.Ref.ID] = append(out[d.Ref.ID], ref)
			}
		}
	}
	return out, nil
}

func (s *Store) writeCancel(tx *firestore.Transaction, open []*firestore.DocumentSnapshot, returns map[string][]*firestore.DocumentRef, now time.Time) error {
	return s.writeEnd(tx, open, returns, now, "cancelled")
}

// writeEnd kết thúc các vòng còn mở với status cho trước; món chưa mua trả về Plan của Owner.
func (s *Store) writeEnd(tx *firestore.Transaction, open []*firestore.DocumentSnapshot, returns map[string][]*firestore.DocumentRef, now time.Time, status string) error {
	seen := map[string]bool{}
	for _, d := range open {
		if seen[d.Ref.Path] {
			continue
		}
		seen[d.Ref.Path] = true
		a := decodeAssignment(d)
		for i := range a.Items {
			if a.Items[i].Status == "assigned" {
				a.Items[i].Status = "returned"
			}
		}
		if err := tx.Update(d.Ref, []firestore.Update{
			{Path: "status", Value: status}, {Path: "closed_at", Value: now}, {Path: "items", Value: assignmentItemsDoc(a.Items)},
		}); err != nil {
			return err
		}
		for _, r := range returns[d.Ref.ID] {
			if err := tx.Update(r, []firestore.Update{
				{Path: "status", Value: "open"}, {Path: "assigned_to_uid", Value: nil},
				{Path: "assignment_id", Value: nil}, {Path: "updated_at", Value: now},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) CancelAssignment(ctx context.Context, hid, id, ownerUID string) (household.Assignment, error) {
	return s.endAssignment(ctx, hid, id, "cancelled", func(a household.Assignment) bool { return a.OwnerUID == ownerUID })
}

// EndByBuyer: người được nhờ từ chối (declined) hoặc báo xong mà không mua được món nào
// (closed_partial). Mọi món trả về Plan của Owner.
func (s *Store) EndByBuyer(ctx context.Context, hid, id, buyerUID, status string) (household.Assignment, error) {
	return s.endAssignment(ctx, hid, id, status, func(a household.Assignment) bool { return a.BuyerUID == buyerUID })
}

func (s *Store) Assignment(ctx context.Context, hid, id string) (household.Assignment, error) {
	if hid == "" || id == "" || len(id) > 80 {
		return household.Assignment{}, apperr.ErrNotFound
	}
	snap, err := s.household(hid).Collection("shopping_assignments").Doc(id).Get(ctx)
	if isNotFound(err) {
		return household.Assignment{}, apperr.ErrNotFound
	}
	if err != nil {
		return household.Assignment{}, fmt.Errorf("assignment: %w", err)
	}
	return decodeAssignment(snap), nil
}

func (s *Store) endAssignment(ctx context.Context, hid, id, status string, allowed func(household.Assignment) bool) (household.Assignment, error) {
	if hid == "" || id == "" || len(id) > 80 {
		return household.Assignment{}, apperr.ErrNotFound
	}
	ref := s.household(hid).Collection("shopping_assignments").Doc(id)
	var out household.Assignment
	var ev pendingEvent
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		if isNotFound(err) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		a := decodeAssignment(snap)
		if !allowed(a) {
			return apperr.ErrForbidden
		}
		if !a.Active() {
			return apperr.New(apperr.CodeInvalidState, "Vòng mua hộ này đã kết thúc.").With("status", a.Status)
		}
		open := []*firestore.DocumentSnapshot{snap}
		returns, err := s.readReturns(tx, open)
		if err != nil {
			return err
		}
		if err := s.writeEnd(tx, open, returns, time.Now(), status); err != nil {
			return err
		}
		a.Status = status
		returned := 0
		for i := range a.Items {
			if a.Items[i].Status == "assigned" {
				a.Items[i].Status = "returned"
				returned++
			}
		}
		to, e := endEventFor(a, status, returned)
		if ev, err = s.writeEvent(tx, to, e); err != nil {
			return err
		}
		out = a
		return nil
	})
	if err != nil {
		return household.Assignment{}, err
	}
	s.enqueue(ctx, ev)
	return out, nil
}

// endEventFor: người nhờ hủy → báo người mua; người mua từ chối hoặc báo xong → báo người nhờ.
func endEventFor(a household.Assignment, status string, returned int) (string, event) {
	now := time.Now()
	base := notify.Event{At: now, PlanID: a.PlanID, AssignmentID: a.ID}
	switch status {
	case "cancelled":
		base.ID, base.Kind, base.ActorName, base.Items = "cancel-"+a.ID, notify.EventPlanCancelled, a.OwnerName, returned
		return a.BuyerUID, event{Event: base, planOwner: a.OwnerUID}
	case "declined":
		base.ID, base.Kind, base.ActorName, base.Items = "declined-"+a.ID, notify.EventRunDeclined, a.BuyerName, returned
	default: // closed_partial: bấm "Mua hàng xong" mà không tick món nào
		base.ID, base.Kind, base.ActorName, base.Returned = "done-"+a.ID, notify.EventRunDone, a.BuyerName, returned
	}
	return a.OwnerUID, event{Event: base, planOwner: a.OwnerUID}
}

func (s *Store) Assignments(ctx context.Context, hid, field, uid string, limit int) ([]household.Assignment, error) {
	docs, err := s.household(hid).Collection("shopping_assignments").
		Where(field, "==", uid).OrderBy("created_at", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("assignments: %w", err)
	}
	out := make([]household.Assignment, 0, len(docs))
	for _, d := range docs {
		out = append(out, decodeAssignment(d))
	}
	return out, nil
}

// PlanTrip: Plan mặc định giữ plan_state trên users/{uid}, Plan đặt tên trên plans/{id}.
func (s *Store) PlanTrip(ctx context.Context, uid, planID string) (household.Trip, error) {
	ref := s.user(uid)
	named := planID != "" && planID != plan.DefaultPlanID
	if named {
		ref = s.user(uid).Collection("plans").Doc(planID)
	}
	snap, err := ref.Get(ctx)
	if isNotFound(err) {
		return household.Trip{}, nil
	}
	if err != nil {
		return household.Trip{}, fmt.Errorf("plan trip: %w", err)
	}
	var t household.Trip
	if named {
		t.PlanName, _ = snap.Data()["name"].(string)
	}
	m, _ := snap.Data()["plan_state"].(map[string]any)
	t.BuyDate, _ = m["buy_date"].(string)
	t.TripNote, _ = m["trip_note"].(string)
	if h, ok := m["buy_hour"].(int64); ok {
		hh := int(h)
		t.BuyHour = &hh
		if mm, ok := m["buy_minute"].(int64); ok {
			v := int(mm)
			t.BuyMinute = &v
		}
	}
	return t, nil
}
