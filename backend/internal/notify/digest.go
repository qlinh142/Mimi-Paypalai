// Package notify chứa job digest (Cloud Scheduler gọi mỗi 15 phút) và gửi thông báo.
// Bảo đảm: không gửi trong giờ yên lặng, mỗi ngày mỗi loại gộp tối đa một thư, chạy
// lại hay chạy chồng không gửi trùng (trừ trường hợp tiến trình chết đúng giữa lúc gửi
// và lúc đánh dấu: chấp nhận gửi ít nhất một lần cho thông báo).
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// DueUser là người dùng có next_notify_at <= now (truy vấn có index, không quét toàn bộ).
type DueUser struct {
	UID      string
	Timezone string
	Email    string // lấy từ Firebase Auth lúc chạy, không lưu ở Firestore
	Name     string
	// DigestTime "HH:MM" theo giờ người dùng (settings.digest_time); rỗng: giờ mặc định.
	DigestTime string
	Lang       string // settings.lang: thư viết theo ngôn ngữ người nhận; rỗng là vi
}

// RetryAfter: gửi lỗi tạm thời thì thử lại sau chừng này; MaxAttempts: số lần gửi tối đa
// mỗi bản tin, sau đó bỏ và chờ ngày hôm sau (vấn đề 58: job bị bỏ qua phải có catch-up,
// vấn đề 8: retry phải có giới hạn).
const (
	RetryAfter  = 15 * time.Minute
	MaxAttempts = 3
)

type Content struct {
	Subject string
	Lines   []string // tên món sắp hết, khoản sắp đến hạn; không có giá
}

type Store interface {
	DueUsers(ctx context.Context, now time.Time, limit int) ([]DueUser, error)
	// BuildDigest trả nội dung; ok=false nếu không có gì đáng gửi.
	BuildDigest(ctx context.Context, uid string, today domain.Date) (Content, bool, error)
	// ClaimDigest giành quyền gửi digests/{date}_{kind}: được khi chưa có, hoặc lần trước
	// lỗi tạm thời ("retry") và chưa quá MaxAttempts, hoặc lần trước kẹt ở "sending" quá lâu.
	// false: đã gửi, đang gửi, bị từ chối hẳn hoặc hết lượt thử.
	ClaimDigest(ctx context.Context, uid, id string, now time.Time) (bool, error)
	MarkDigest(ctx context.Context, uid, id, status string) error
	Reschedule(ctx context.Context, uid string, next time.Time) error
}

type Mailer interface {
	Send(ctx context.Context, to, subject string, lines []string) error
}

// ErrPermanent: nhà cung cấp email từ chối hẳn (địa chỉ hỏng). Không retry.
var ErrPermanent = errors.New("notify: permanent send failure")

type DigestJob struct {
	store       Store
	mail        Mailer
	log         *slog.Logger
	now         func() time.Time
	batch       int
	concurrency int
	sendTimeout time.Duration
}

func NewDigestJob(store Store, mail Mailer, log *slog.Logger, now func() time.Time) *DigestJob {
	if now == nil {
		now = time.Now
	}
	return &DigestJob{store: store, mail: mail, log: log, now: now, batch: 200, concurrency: 8, sendTimeout: 10 * time.Second}
}

type Result struct {
	Processed, Sent, Skipped, Failed int
}

// Run xử lý một lượt. Trả lỗi nếu không đọc được danh sách; lỗi từng người dùng chỉ
// được đếm và log để một hộp thư hỏng không chặn cả lượt.
func (j *DigestJob) Run(ctx context.Context) (Result, error) {
	now := j.now()
	users, err := j.store.DueUsers(ctx, now, j.batch)
	if err != nil {
		return Result{}, fmt.Errorf("digest: list due users: %w", err)
	}
	var (
		mu  sync.Mutex
		res Result
		wg  sync.WaitGroup
		sem = make(chan struct{}, j.concurrency) // giới hạn số thư gửi song song
	)
	for _, u := range users {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			outcome := j.one(ctx, u, now)
			mu.Lock()
			defer mu.Unlock()
			res.Processed++
			switch outcome {
			case "sent":
				res.Sent++
			case "skipped":
				res.Skipped++
			default:
				res.Failed++
			}
		}()
	}
	wg.Wait()
	j.log.InfoContext(ctx, "digest run", "processed", res.Processed, "sent", res.Sent, "skipped", res.Skipped, "failed", res.Failed)
	return res, nil
}

func (j *DigestJob) one(ctx context.Context, u DueUser, now time.Time) string {
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	if domain.InQuietHours(now, loc) {
		if err := j.store.Reschedule(ctx, u.UID, domain.NextAllowed(now, loc)); err != nil {
			j.log.ErrorContext(ctx, "reschedule failed", "err", err)
			return "failed"
		}
		return "skipped"
	}
	today := domain.Today(now, loc)
	// Lượt tiếp theo: đúng giờ người dùng chọn trong Cài đặt (mặc định 8:00), theo giờ của họ.
	h, m := domain.DefaultDigestHour, domain.DefaultDigestMinute
	if hh, mm, ok := domain.ParseClock(u.DigestTime); ok {
		h, m = hh, mm
	}
	next := domain.NextDigestAt(now, loc, h, m)

	content, ok, err := j.store.BuildDigest(ctx, u.UID, today)
	if err != nil {
		j.log.ErrorContext(ctx, "build digest failed", "err", err)
		return "failed"
	}
	if !ok || u.Email == "" {
		_ = j.store.Reschedule(ctx, u.UID, next)
		return "skipped"
	}
	content.Subject = DigestSubject(len(content.Lines), u.Lang)
	id := today.String() + "_daily"
	claimed, err := j.store.ClaimDigest(ctx, u.UID, id, now)
	if err != nil || !claimed {
		_ = j.store.Reschedule(ctx, u.UID, next)
		return "skipped"
	}
	sctx, cancel := context.WithTimeout(ctx, j.sendTimeout)
	defer cancel()
	if err := j.mail.Send(sctx, u.Email, content.Subject, content.Lines); err != nil {
		status := "retry"
		if errors.Is(err, ErrPermanent) {
			status = "bounced"
		}
		_ = j.store.MarkDigest(ctx, u.UID, id, status)
		// Lỗi tạm thời: hẹn thử lại sớm (ClaimDigest tự dừng sau MaxAttempts lần).
		// Bị từ chối hẳn: chờ lượt ngày mai.
		retryAt := next
		if status == "retry" {
			retryAt = domain.NextAllowed(now.Add(RetryAfter), loc)
		}
		if err := j.store.Reschedule(ctx, u.UID, retryAt); err != nil {
			j.log.ErrorContext(ctx, "reschedule failed", "err", err)
		}
		j.log.WarnContext(ctx, "digest send failed", "status", status, "err", err)
		return "failed"
	}
	if err := j.store.MarkDigest(ctx, u.UID, id, "sent"); err != nil {
		j.log.ErrorContext(ctx, "mark digest failed", "err", err)
	}
	if err := j.store.Reschedule(ctx, u.UID, next); err != nil {
		j.log.ErrorContext(ctx, "reschedule failed", "err", err)
	}
	return "sent"
}
