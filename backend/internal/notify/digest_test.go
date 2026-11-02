package notify

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

type memStore struct {
	users    []DueUser
	claimed  map[string]bool
	marks    map[string]string
	resched  map[string]time.Time
	attempts map[string]int
	hasItems bool
}

func (m *memStore) DueUsers(context.Context, time.Time, int) ([]DueUser, error) { return m.users, nil }
func (m *memStore) BuildDigest(context.Context, string, domain.Date) (Content, bool, error) {
	return Content{Subject: "Mimi: 2 món sắp hết", Lines: []string{"Sữa tươi", "Trứng gà"}}, m.hasItems, nil
}
func (m *memStore) ClaimDigest(_ context.Context, uid, id string, _ time.Time) (bool, error) {
	k := uid + "/" + id
	if m.claimed[k] && m.marks[k] != "retry" {
		return false, nil
	}
	if m.attempts[k] >= MaxAttempts {
		return false, nil
	}
	m.claimed[k] = true
	m.attempts[k]++
	return true, nil
}
func (m *memStore) MarkDigest(_ context.Context, uid, id, st string) error {
	m.marks[uid+"/"+id] = st
	return nil
}
func (m *memStore) Reschedule(_ context.Context, uid string, t time.Time) error {
	m.resched[uid] = t
	return nil
}

type countMailer struct {
	n    int
	fail error // khác nil: mọi lần gửi đều lỗi
}

func (c *countMailer) Send(context.Context, string, string, []string) error {
	c.n++
	return c.fail
}

func TestDigest(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	newStore := func() *memStore {
		return &memStore{users: []DueUser{{UID: "u1", Timezone: "Asia/Ho_Chi_Minh", Email: "a@b.c"}},
			claimed: map[string]bool{}, marks: map[string]string{}, resched: map[string]time.Time{}, attempts: map[string]int{}, hasItems: true}
	}

	t.Run("second run same day does not resend", func(t *testing.T) {
		st, mail := newStore(), &countMailer{}
		noon := time.Date(2026, 10, 16, 5, 0, 0, 0, time.UTC) // 12h giờ VN
		j := NewDigestJob(st, mail, log, func() time.Time { return noon })
		for i := 0; i < 2; i++ {
			if _, err := j.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		if mail.n != 1 || st.marks["u1/2026-10-16_daily"] != "sent" {
			t.Fatalf("sent %d marks %v", mail.n, st.marks)
		}
	})

	t.Run("quiet hours reschedule to 7am", func(t *testing.T) {
		st, mail := newStore(), &countMailer{}
		late := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC) // 22h giờ VN
		j := NewDigestJob(st, mail, log, func() time.Time { return late })
		res, _ := j.Run(context.Background())
		want := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC) // 7h sáng giờ VN
		if mail.n != 0 || res.Skipped != 1 || !st.resched["u1"].Equal(want) {
			t.Fatalf("sent %d res %+v next %v", mail.n, res, st.resched["u1"])
		}
	})

	t.Run("next run uses the user's digest_time", func(t *testing.T) {
		st, mail := newStore(), &countMailer{}
		st.users[0].DigestTime = "19:30"
		noon := time.Date(2026, 10, 16, 5, 0, 0, 0, time.UTC) // 12h giờ VN
		j := NewDigestJob(st, mail, log, func() time.Time { return noon })
		if _, err := j.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		want := time.Date(2026, 10, 16, 12, 30, 0, 0, time.UTC) // 19:30 giờ VN cùng ngày
		if mail.n != 1 || !st.resched["u1"].Equal(want) {
			t.Fatalf("sent %d next %v want %v", mail.n, st.resched["u1"], want)
		}
	})

	t.Run("temporary failure is retried, at most MaxAttempts times", func(t *testing.T) {
		st, mail := newStore(), &countMailer{fail: errors.New("smtp timeout")}
		now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC) // 8h giờ VN
		j := NewDigestJob(st, mail, log, func() time.Time { return now })
		if _, err := j.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if want := now.Add(RetryAfter); !st.resched["u1"].Equal(want) {
			t.Fatalf("retry at %v want %v", st.resched["u1"], want)
		}
		for i := 0; i < 5; i++ {
			_, _ = j.Run(context.Background())
		}
		if mail.n != MaxAttempts {
			t.Fatalf("sent %d times, want %d", mail.n, MaxAttempts)
		}
	})

	t.Run("permanent failure waits for tomorrow", func(t *testing.T) {
		st, mail := newStore(), &countMailer{fail: ErrPermanent}
		now := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC)
		j := NewDigestJob(st, mail, log, func() time.Time { return now })
		_, _ = j.Run(context.Background())
		_, _ = j.Run(context.Background())
		want := time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC) // 8h sáng mai giờ VN
		if mail.n != 1 || st.marks["u1/2026-10-16_daily"] != "bounced" || !st.resched["u1"].Equal(want) {
			t.Fatalf("sent %d marks %v next %v", mail.n, st.marks, st.resched["u1"])
		}
	})
}
