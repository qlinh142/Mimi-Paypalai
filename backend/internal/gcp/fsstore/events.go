package fsstore

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/notify"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
)

// ---------- Thông báo theo sự kiện: users/{uid}/events/{id} (B9) ----------

// WithQueue gắn hàng đợi notify. Không gắn thì sự kiện chỉ hiện trong app.
func (s *Store) WithQueue(q notify.Enqueuer) *Store {
	s.queue = q
	return s
}

// pendingEvent: sự kiện đã ghi trong transaction, chờ đẩy vào hàng đợi khi transaction xong.
type pendingEvent struct{ uid, id string }

// event là một sự kiện sắp ghi. planOwner: chủ Kế hoạch, để đọc tên Kế hoạch lúc hiển thị.
type event struct {
	notify.Event
	planOwner string
}

// writeEvent ghi sự kiện trong transaction. ID cố định theo sự kiện nên transaction chạy lại
// hay gửi lại cùng thao tác chỉ ghi đè chính nó.
func (s *Store) writeEvent(tx *firestore.Transaction, uid string, ev event) (pendingEvent, error) {
	err := tx.Set(s.user(uid).Collection("events").Doc(ev.ID), map[string]any{
		"kind": ev.Kind, "at": ev.At, "actor_name": ev.ActorName, "plan_id": ev.PlanID, "plan_owner_uid": ev.planOwner,
		"assignment_id": ev.AssignmentID, "items": ev.Items, "returned": ev.Returned, "read": false, "delivery": "pending",
		"expire_at": ev.At.Add(EventRetention), // TTL policy xóa; job cleanup dọn bản cũ chưa có trường này
	})
	return pendingEvent{uid: uid, id: ev.ID}, err
}

// enqueue đẩy sự kiện vào hàng đợi sau khi transaction đã ghi. Lỗi chỉ ghi log: sự kiện đã
// nằm trong app, job digest không phụ thuộc vào đây.
func (s *Store) enqueue(ctx context.Context, evs ...pendingEvent) {
	if s.queue == nil {
		return
	}
	for _, e := range evs {
		if e.id == "" {
			continue
		}
		if err := s.queue.Enqueue(ctx, notify.Task{UID: e.uid, EventID: e.id}); err != nil {
			slog.WarnContext(ctx, "enqueue notify", "event_id", e.id, "err", err)
		}
	}
}

func decodeEvent(d *firestore.DocumentSnapshot) (notify.Event, string) {
	m := d.Data()
	ev := notify.Event{ID: d.Ref.ID}
	ev.Kind, _ = m["kind"].(string)
	ev.At, _ = m["at"].(time.Time)
	ev.ActorName, _ = m["actor_name"].(string)
	ev.PlanID, _ = m["plan_id"].(string)
	ev.AssignmentID, _ = m["assignment_id"].(string)
	ev.Items = int(asInt64(m["items"]))
	ev.Returned = int(asInt64(m["returned"]))
	ev.Read, _ = m["read"].(bool)
	owner, _ := m["plan_owner_uid"].(string)
	return ev, owner
}

// planName đọc tên Kế hoạch lúc hiển thị (Kế hoạch có thể đã đổi tên). Kế hoạch chính: "".
func (s *Store) planName(ctx context.Context, owner, planID string) string {
	if owner == "" || planID == "" || planID == plan.DefaultPlanID {
		return ""
	}
	snap, err := s.user(owner).Collection("plans").Doc(planID).Get(ctx)
	if err != nil {
		return ""
	}
	n, _ := snap.Data()["name"].(string)
	return n
}

// Events trả các sự kiện trong 30 ngày, mới nhất trước.
func (s *Store) Events(ctx context.Context, uid string, limit int) ([]notify.Event, error) {
	docs, err := s.user(uid).Collection("events").Where("at", ">=", time.Now().AddDate(0, 0, -30)).
		OrderBy("at", firestore.Desc).Limit(limit).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	out := make([]notify.Event, 0, len(docs))
	names := map[string]string{}
	for _, d := range docs {
		ev, owner := decodeEvent(d)
		k := owner + "/" + ev.PlanID
		if _, ok := names[k]; !ok {
			names[k] = s.planName(ctx, owner, ev.PlanID)
		}
		ev.PlanName = names[k]
		out = append(out, ev)
	}
	return out, nil
}

// MarkEventRead: sự kiện không có thì trả ErrNotFound.
func (s *Store) MarkEventRead(ctx context.Context, uid, id string) error {
	if id == "" || len(id) > 120 {
		return apperr.ErrNotFound
	}
	_, err := s.user(uid).Collection("events").Doc(id).Update(ctx, []firestore.Update{{Path: "read", Value: true}})
	if isNotFound(err) {
		return apperr.ErrNotFound
	}
	return err
}

var _ notify.EventStore = (*NotifyStore)(nil)

// Event (worker): sự kiện, trạng thái gửi và email người nhận. Người nhận tắt nhắc nhở
// (settings.remind_enabled false) thì email rỗng: không gửi thư, sự kiện vẫn ở trong app.
func (n *NotifyStore) Event(ctx context.Context, uid, id string) (notify.Event, string, string, error) {
	snap, err := n.user(uid).Collection("events").Doc(id).Get(ctx)
	if isNotFound(err) {
		return notify.Event{}, "", "", notify.ErrNotFound
	}
	if err != nil {
		return notify.Event{}, "", "", err
	}
	ev, owner := decodeEvent(snap)
	ev.PlanName = n.planName(ctx, owner, ev.PlanID)
	delivery, _ := snap.Data()["delivery"].(string)
	us, err := n.user(uid).Get(ctx)
	if err != nil && !isNotFound(err) {
		return notify.Event{}, "", "", err
	}
	if us != nil && us.Exists() {
		if st, ok := us.Data()["settings"].(map[string]any); ok {
			ev.Lang, _ = st["lang"].(string) // thư theo ngôn ngữ người nhận
			if on, ok := st["remind_enabled"].(bool); ok && !on {
				return ev, delivery, "", nil
			}
		}
	}
	email, err := n.emails.Email(ctx, uid)
	if err != nil {
		return ev, delivery, "", nil // chưa có email (đăng nhập ẩn danh): bỏ qua thư
	}
	return ev, delivery, email, nil
}

func (n *NotifyStore) MarkDelivery(ctx context.Context, uid, id, status string) error {
	_, err := n.user(uid).Collection("events").Doc(id).Update(ctx, []firestore.Update{
		{Path: "delivery", Value: status}, {Path: "delivery_at", Value: time.Now()},
	})
	return err
}
