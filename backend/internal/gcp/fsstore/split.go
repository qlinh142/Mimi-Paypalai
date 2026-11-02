package fsstore

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/notify"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// ---------- Món chưa mua sang kế hoạch mới (B6, đề xuất bước 6 câu 3) ----------
//
// Khi một Kế hoạch đặt tên được chốt (người mua bấm "Mua hàng xong", hoặc chủ giỏ xác nhận hóa đơn
// trừ vào kế hoạch đó), món còn mở chuyển sang Kế hoạch mới "[tên cũ] · còn lại":
//   - giữ biểu tượng và nơi mua, chưa có ngày đi mua, chưa có người mua;
//   - nằm trong danh sách của người nhờ, không tự gửi lại;
//   - mua đủ hết thì không tạo gì. Kế hoạch chính (mặc định) là danh sách dùng lâu dài nên không tách.
// Chạy sau transaction lưu hóa đơn. Plan cũ ghi split_to nên gọi lại không tạo kế hoạch thứ hai.

const remainSuffix = " · còn lại"

// remainName: "Mua đồ cuối tuần" → "Mua đồ cuối tuần · còn lại", không lặp hậu tố, tối đa 40 ký tự.
func remainName(old string) string {
	base := strings.TrimSuffix(strings.TrimSpace(old), remainSuffix)
	if base == "" {
		base = "Kế hoạch"
	}
	max := 40 - utf8.RuneCountInString(remainSuffix)
	if r := []rune(base); len(r) > max {
		base = strings.TrimSpace(string(r[:max]))
	}
	return base + remainSuffix
}

// remainLine: dòng chuyển sang kế hoạch mới chỉ còn phần chưa mua, bỏ dấu đối chiếu cũ.
func remainLine(m map[string]any, now time.Time) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		switch k {
		case "bought_qty", "partial_by", "matched_receipt_id", "assigned_to_uid", "assignment_id", "done_by", "done_at":
			continue
		}
		out[k] = v
	}
	if q, b := asFloat(m["qty"]), asFloat(m["bought_qty"]); q > 0 && b > 0 && b < q {
		out["qty"] = q - b
	}
	out["status"], out["updated_at"] = "open", now
	return out
}

// SplitRemaining chuyển món còn mở của Kế hoạch planID sang Kế hoạch mới. Không có món nào còn
// mở, hoặc là Kế hoạch chính, thì trả nil. actor khác rỗng (người đi mua hộ vừa xong) thì ghi
// thêm thông báo cho chủ Kế hoạch (B9); chủ giỏ tự lưu hóa đơn thì đã thấy ngay trên màn lưu.
func (s *Store) SplitRemaining(ctx context.Context, uid, planID, actor string, now time.Time) (*receipt.SplitResult, error) {
	if planID == "" || planID == plan.DefaultPlanID {
		return nil, nil
	}
	pref := s.user(uid).Collection("plans").Doc(planID)
	var out *receipt.SplitResult
	var ev pendingEvent
	err := s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		out, ev = nil, pendingEvent{}
		snap, err := tx.Get(pref)
		if isNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		pd := snap.Data()
		if to, _ := pd["split_to"].(string); to != "" {
			return nil
		}
		open, err := tx.Documents(s.planItems(uid, planID).Where("status", "==", "open").Limit(maxPlanLines)).GetAll()
		if err != nil {
			return err
		}
		if len(open) == 0 {
			return nil
		}
		old, _ := pd["name"].(string)
		id := strings.ReplaceAll("p-"+now.UTC().Format("060102150405.000"), ".", "") + "r"
		nref := s.user(uid).Collection("plans").Doc(id)
		doc := map[string]any{
			"name": remainName(old), "status": "active", "created_at": now, "icon": pd["icon"], "split_from": planID,
		}
		if ps, ok := pd["plan_state"].(map[string]any); ok {
			if note, _ := ps["trip_note"].(string); note != "" {
				doc["plan_state"] = map[string]any{"trip_note": note}
			}
		}
		if err := tx.Create(nref, doc); err != nil {
			return err
		}
		for _, d := range open {
			if err := tx.Set(nref.Collection("items").Doc(d.Ref.ID), remainLine(d.Data(), now)); err != nil {
				return err
			}
			if err := tx.Delete(d.Ref); err != nil {
				return err
			}
		}
		if err := tx.Update(pref, []firestore.Update{{Path: "split_to", Value: id}, {Path: "split_at", Value: now}}); err != nil {
			return err
		}
		out = &receipt.SplitResult{FromPlanID: planID, PlanID: id, Name: remainName(old), Moved: len(open)}
		if actor != "" {
			if ev, err = s.writeEvent(tx, uid, event{Event: notify.Event{
				ID: "split-" + planID, Kind: notify.EventPlanSplit, At: now, ActorName: actor, PlanID: id, Items: len(open),
			}, planOwner: uid}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("split remaining: %w", err)
	}
	s.enqueue(ctx, ev)
	return out, nil
}
