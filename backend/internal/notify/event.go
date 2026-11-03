package notify

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ---------- Thông báo theo sự kiện (B9) ----------
//
// Sự kiện giữa hai người trong nhà (gửi kế hoạch, cập nhật, hủy, mua xong, từ chối, món chưa mua
// sang kế hoạch mới) được ghi vào users/{uid}/events/{id} CÙNG transaction với thay đổi dữ liệu,
// nên không bao giờ mất (outbox). Sau khi transaction xong, máy chủ đẩy một task vào hàng đợi
// Cloud Tasks "notify"; worker gửi thư và đánh dấu đã gửi. Lỗi tạm thời trả 5xx để Cloud Tasks
// thử lại có giãn cách; lỗi vĩnh viễn trả 2xx và ghi failed. Mất task thì sự kiện vẫn hiện
// trong màn Thông báo của app.

// Các loại sự kiện. Client dựng câu chữ theo Kind và ngôn ngữ giao diện.
const (
	EventPlanSent      = "plan_sent"      // người nhận: Buyer
	EventPlanUpdated   = "plan_updated"   // người nhận: Buyer (B5)
	EventPlanCancelled = "plan_cancelled" // người nhận: Buyer
	EventRunDone       = "run_done"       // người nhận: Owner
	EventRunDeclined   = "run_declined"   // người nhận: Owner
	EventPlanSplit     = "plan_split"     // người nhận: Owner (B6)
)

// Event là một thông báo đã lưu. Không có giá tiền (thư có thể bị người khác đọc).
type Event struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	At           time.Time `json:"at"`
	ActorName    string    `json:"actor_name,omitempty"` // người gây ra sự kiện
	PlanID       string    `json:"plan_id,omitempty"`
	PlanName     string    `json:"plan_name,omitempty"`
	AssignmentID string    `json:"assignment_id,omitempty"`
	Items        int       `json:"items,omitempty"`    // số món gửi / mua được / chuyển sang
	Returned     int       `json:"returned,omitempty"` // run_done: số món chưa mua
	Read         bool      `json:"read"`
	// Lang: ngôn ngữ trong Cài đặt của người nhận (vi | en), chỉ dùng để viết thư; vắng là vi.
	Lang string `json:"-"`
}

// Task là nội dung một task trong hàng đợi notify.
type Task struct {
	UID     string `json:"uid"`
	EventID string `json:"event_id"`
}

// Enqueuer đẩy task vào hàng đợi. Tên task suy từ sự kiện nên đẩy hai lần không gửi hai thư.
type Enqueuer interface {
	Enqueue(ctx context.Context, t Task) error
}

// NoQueue: chưa cấu hình hàng đợi (dev, demo). Sự kiện vẫn hiện trong app.
type NoQueue struct{}

func (NoQueue) Enqueue(context.Context, Task) error { return nil }

// EventStore là phần lưu trữ worker cần.
type EventStore interface {
	// Event trả sự kiện, trạng thái gửi (pending | sent | failed | skipped) và email người nhận
	// (rỗng khi người nhận tắt thông báo hoặc chưa có email).
	Event(ctx context.Context, uid, id string) (ev Event, delivery, email string, err error)
	MarkDelivery(ctx context.Context, uid, id, status string) error
}

// ErrNotFound: task trỏ tới sự kiện không còn. Không thử lại.
var ErrNotFound = errors.New("notify: event not found")

// Subject và Lines: nội dung thư cho từng loại sự kiện, theo ngôn ngữ của người nhận (mẹ chốt
// 03/11: lời nhắc, link mua hộ, email theo ngôn ngữ trong Cài đặt của người nhận).
func EventMail(ev Event) (string, []string) {
	if ev.Lang == "en" {
		return eventMailEn(ev)
	}
	who := ev.ActorName
	if who == "" {
		who = "Người nhà"
	}
	plan := ev.PlanName
	if plan == "" {
		plan = "Kế hoạch chính"
	}
	at := ev.At.In(vnZone).Format("15:04")
	switch ev.Kind {
	case EventPlanSent:
		return fmt.Sprintf("Mimi: %s nhờ bạn mua %d món", who, ev.Items), []string{plan}
	case EventPlanUpdated:
		return fmt.Sprintf("Mimi: %s đã cập nhật kế hoạch lúc %s", who, at), []string{plan}
	case EventPlanCancelled:
		return fmt.Sprintf("Mimi: %s đã hủy kế hoạch lúc %s", who, at), []string{plan}
	case EventRunDone:
		lines := []string{fmt.Sprintf("Mua được %d món", ev.Items)}
		if ev.Returned > 0 {
			lines = append(lines, fmt.Sprintf("%d món chưa mua", ev.Returned))
		}
		return fmt.Sprintf("Mimi: %s đã mua xong %s", who, plan), lines
	case EventRunDeclined:
		return fmt.Sprintf("Mimi: %s không mua được %s", who, plan), []string{"Các món đã về lại kế hoạch của bạn"}
	case EventPlanSplit:
		return fmt.Sprintf("Mimi: đã chuyển %d món chưa mua sang kế hoạch mới", ev.Items), []string{plan}
	}
	return "Mimi: có thông báo mới", nil
}

func eventMailEn(ev Event) (string, []string) {
	who := ev.ActorName
	if who == "" {
		who = "A family member"
	}
	plan := ev.PlanName
	if plan == "" || plan == "Kế hoạch chính" {
		plan = "Main plan"
	}
	at := ev.At.In(vnZone).Format("15:04")
	switch ev.Kind {
	case EventPlanSent:
		return fmt.Sprintf("Mimi: %s asked you to buy %s", who, itemsEn(ev.Items)), []string{plan}
	case EventPlanUpdated:
		return fmt.Sprintf("Mimi: %s updated the plan at %s", who, at), []string{plan}
	case EventPlanCancelled:
		return fmt.Sprintf("Mimi: %s cancelled the plan at %s", who, at), []string{plan}
	case EventRunDone:
		lines := []string{fmt.Sprintf("Bought %s", itemsEn(ev.Items))}
		if ev.Returned > 0 {
			lines = append(lines, fmt.Sprintf("%s not bought", itemsEn(ev.Returned)))
		}
		return fmt.Sprintf("Mimi: %s finished shopping for %s", who, plan), lines
	case EventRunDeclined:
		return fmt.Sprintf("Mimi: %s couldn't shop for %s", who, plan), []string{"The items are back in your plan"}
	case EventPlanSplit:
		return fmt.Sprintf("Mimi: moved %s not bought to a new plan", itemsEn(ev.Items)), []string{plan}
	}
	return "Mimi: you have a new notification", nil
}

func itemsEn(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

// DigestSubject: tiêu đề bản tin hằng ngày theo ngôn ngữ người nhận.
func DigestSubject(n int, lang string) string {
	if lang == "en" {
		if n == 1 {
			return "Mimi: 1 item running low"
		}
		return fmt.Sprintf("Mimi: %d items running low", n)
	}
	return fmt.Sprintf("Mimi: %d món sắp hết", n)
}

var vnZone = func() *time.Location {
	if l, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
		return l
	}
	return time.FixedZone("ICT", 7*3600)
}()

// Deliverer xử lý một task notify (worker POST /tasks/notify).
type Deliverer struct {
	store   EventStore
	mail    Mailer
	timeout time.Duration
}

func NewDeliverer(store EventStore, mail Mailer) *Deliverer {
	return &Deliverer{store: store, mail: mail, timeout: 10 * time.Second}
}

// Deliver trả retry=true khi lỗi tạm thời (worker trả 5xx để Cloud Tasks thử lại).
// Gửi rồi, không còn sự kiện, hoặc bị từ chối hẳn thì retry=false.
func (d *Deliverer) Deliver(ctx context.Context, t Task) (retry bool, err error) {
	if t.UID == "" || t.EventID == "" {
		return false, fmt.Errorf("notify: bad task")
	}
	ev, delivery, email, err := d.store.Event(ctx, t.UID, t.EventID)
	if errors.Is(err, ErrNotFound) {
		return false, err
	}
	if err != nil {
		return true, err
	}
	if delivery != "" && delivery != "pending" && delivery != "retry" {
		return false, nil // đã xử lý: Cloud Tasks gửi lại task thì không gửi thư lần hai
	}
	if email == "" {
		return false, d.store.MarkDelivery(ctx, t.UID, t.EventID, "skipped")
	}
	subject, lines := EventMail(ev)
	sctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	if err := d.mail.Send(sctx, email, subject, lines); err != nil {
		if errors.Is(err, ErrPermanent) {
			return false, d.store.MarkDelivery(ctx, t.UID, t.EventID, "failed")
		}
		_ = d.store.MarkDelivery(ctx, t.UID, t.EventID, "retry")
		return true, err
	}
	return false, d.store.MarkDelivery(ctx, t.UID, t.EventID, "sent")
}
