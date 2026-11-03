package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type evStore struct {
	ev       Event
	delivery string
	email    string
	err      error
	marks    []string
}

func (m *evStore) Event(context.Context, string, string) (Event, string, string, error) {
	return m.ev, m.delivery, m.email, m.err
}
func (m *evStore) MarkDelivery(_ context.Context, _, _, status string) error {
	m.marks = append(m.marks, status)
	m.delivery = status
	return nil
}

type mailOnce struct {
	errs []error
	sent []string
}

func (m *mailOnce) Send(_ context.Context, _, subject string, _ []string) error {
	if len(m.errs) > 0 {
		err := m.errs[0]
		m.errs = m.errs[1:]
		if err != nil {
			return err
		}
	}
	m.sent = append(m.sent, subject)
	return nil
}

func TestDeliver(t *testing.T) {
	ctx := context.Background()
	task := Task{UID: "minh", EventID: "sent-trip-1"}
	at := time.Date(2026, 11, 2, 3, 32, 0, 0, time.UTC) // 10:32 giờ Việt Nam

	// Lỗi tạm thời: báo thử lại; lần sau gửi được; Cloud Tasks gửi lại task thì không gửi lần hai.
	st := &evStore{ev: Event{Kind: EventPlanUpdated, ActorName: "Mẹ", At: at}, delivery: "pending", email: "minh@x"}
	m := &mailOnce{errs: []error{errors.New("timeout")}}
	d := NewDeliverer(st, m)
	if retry, err := d.Deliver(ctx, task); !retry || err == nil {
		t.Fatalf("temporary: retry=%v err=%v", retry, err)
	}
	if retry, err := d.Deliver(ctx, task); retry || err != nil || len(m.sent) != 1 || st.delivery != "sent" {
		t.Fatalf("second: retry=%v err=%v sent=%v", retry, err, m.sent)
	}
	if !strings.Contains(m.sent[0], "Mẹ đã cập nhật kế hoạch lúc 10:32") {
		t.Fatalf("subject %q", m.sent[0])
	}
	if retry, err := d.Deliver(ctx, task); retry || err != nil || len(m.sent) != 1 {
		t.Fatalf("duplicate task must not resend: %v %v %v", retry, err, m.sent)
	}

	// Bị từ chối hẳn: không thử lại.
	st = &evStore{ev: Event{Kind: EventPlanSent}, delivery: "pending", email: "x@x"}
	d = NewDeliverer(st, &mailOnce{errs: []error{ErrPermanent}})
	if retry, _ := d.Deliver(ctx, task); retry || st.delivery != "failed" {
		t.Fatalf("permanent: retry=%v delivery=%s", retry, st.delivery)
	}

	// Người nhận tắt thông báo hoặc chưa có email: bỏ qua thư.
	st = &evStore{ev: Event{Kind: EventPlanSent}, delivery: "pending"}
	m = &mailOnce{}
	if retry, _ := NewDeliverer(st, m).Deliver(ctx, task); retry || len(m.sent) != 0 || st.delivery != "skipped" {
		t.Fatalf("no email: %v %v", m.sent, st.delivery)
	}

	// Sự kiện đã bị xóa: không thử lại. Đọc lỗi tạm thời: thử lại.
	if retry, _ := NewDeliverer(&evStore{err: ErrNotFound}, m).Deliver(ctx, task); retry {
		t.Fatal("missing event must not retry")
	}
	if retry, _ := NewDeliverer(&evStore{err: errors.New("unavailable")}, m).Deliver(ctx, task); !retry {
		t.Fatal("read error must retry")
	}
	if retry, err := NewDeliverer(st, m).Deliver(ctx, Task{}); retry || err == nil {
		t.Fatal("bad task")
	}
}

func TestEventMail(t *testing.T) {
	cases := map[string]Event{
		"Mimi: Mẹ nhờ bạn mua 3 món":                       {Kind: EventPlanSent, ActorName: "Mẹ", Items: 3},
		"Mimi: Minh đã mua xong Mua đồ cuối tuần":          {Kind: EventRunDone, ActorName: "Minh", PlanName: "Mua đồ cuối tuần", Items: 4, Returned: 1},
		"Mimi: Minh không mua được Kế hoạch chính":         {Kind: EventRunDeclined, ActorName: "Minh"},
		"Mimi: đã chuyển 1 món chưa mua sang kế hoạch mới": {Kind: EventPlanSplit, Items: 1},
	}
	for want, ev := range cases {
		if got, _ := EventMail(ev); got != want {
			t.Errorf("%s: got %q", ev.Kind, got)
		}
	}
	if _, lines := EventMail(Event{Kind: EventRunDone, Items: 4, Returned: 1}); len(lines) != 2 {
		t.Fatalf("%v", lines)
	}
}

// Thư viết theo ngôn ngữ trong Cài đặt của người nhận (mẹ chốt 03/11).
func TestEventMailLang(t *testing.T) {
	ev := Event{Kind: EventPlanSent, ActorName: "Lan", PlanName: "Kế hoạch chính", Items: 3}
	if s, l := EventMail(ev); s != "Mimi: Lan nhờ bạn mua 3 món" || l[0] != "Kế hoạch chính" {
		t.Fatalf("vi: %q %v", s, l)
	}
	ev.Lang = "en"
	if s, l := EventMail(ev); s != "Mimi: Lan asked you to buy 3 items" || l[0] != "Main plan" {
		t.Fatalf("en: %q %v", s, l)
	}
	ev = Event{Kind: EventRunDone, ActorName: "Minh", PlanName: "Weekend", Items: 1, Returned: 2, Lang: "en"}
	if s, l := EventMail(ev); s != "Mimi: Minh finished shopping for Weekend" || l[0] != "Bought 1 item" || l[1] != "2 items not bought" {
		t.Fatalf("en run_done: %q %v", s, l)
	}
	if DigestSubject(1, "en") != "Mimi: 1 item running low" || DigestSubject(4, "vi") != "Mimi: 4 món sắp hết" || DigestSubject(2, "") != "Mimi: 2 món sắp hết" {
		t.Fatal("digest subject")
	}
}
