// Package idem giữ kết quả của các thao tác ghi theo Idempotency-Key (data model 1.5
// thay đổi 5, API contract V1): client gửi lại cùng một key thì nhận lại đúng kết quả
// lần đầu, máy chủ không ghi lần hai. Dùng cho mọi route ghi, kể cả hàng đợi offline.
package idem

import (
	"context"
	"errors"
	"sync"
	"time"
)

// TTL: sau 24 giờ key hết hạn (TTL policy của Firestore tự xóa). Hàng đợi offline trên
// máy thường gửi lại trong vài phút đến vài giờ.
const TTL = 24 * time.Hour

// PendingStale: một key ở trạng thái "đang xử lý" lâu hơn mức này được coi là bỏ dở
// (instance chết giữa chừng) và được nhận lại. Lớn hơn timeout dài nhất của route (55 s).
const PendingStale = 2 * time.Minute

var (
	// ErrInFlight: cùng key đang được xử lý ở request khác.
	ErrInFlight = errors.New("idem: key in flight")
	// ErrMismatch: key đã dùng cho một request khác (khác route hoặc khác nội dung).
	ErrMismatch = errors.New("idem: key reused with different request")
)

// Record là kết quả đã lưu của một request.
type Record struct {
	Fingerprint string
	Status      int
	ContentType string
	Body        []byte
}

// Store lưu trạng thái theo từng người dùng (users/{uid}/idempotency/{key}).
type Store interface {
	// Begin giữ key cho request có fingerprint fp. Trả (nil, nil) khi key mới (hoặc đã hết
	// hạn, hoặc bị bỏ dở): người gọi xử lý rồi Finish hay Abort. Trả (rec, nil) khi đã có
	// kết quả: người gọi trả lại rec. Trả ErrInFlight hoặc ErrMismatch như trên.
	Begin(ctx context.Context, uid, key, fp string, now time.Time) (*Record, error)
	Finish(ctx context.Context, uid, key string, rec Record, now time.Time) error
	// Abort bỏ giữ key để lần gửi sau được xử lý lại (lỗi tạm thời, 5xx).
	Abort(ctx context.Context, uid, key string) error
}

// Memory là Store trong bộ nhớ, dùng cho test và chạy thử ở máy.
type Memory struct {
	mu sync.Mutex
	m  map[string]memEntry
}

type memEntry struct {
	rec     Record
	done    bool
	started time.Time
	expires time.Time
}

func NewMemory() *Memory { return &Memory{m: map[string]memEntry{}} }

func (s *Memory) Begin(_ context.Context, uid, key, fp string, now time.Time) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := uid + "/" + key
	e, ok := s.m[k]
	switch {
	case !ok, now.After(e.expires), !e.done && now.Sub(e.started) > PendingStale:
		s.m[k] = memEntry{rec: Record{Fingerprint: fp}, started: now, expires: now.Add(TTL)}
		return nil, nil
	case e.rec.Fingerprint != fp:
		return nil, ErrMismatch
	case !e.done:
		return nil, ErrInFlight
	}
	r := e.rec
	return &r, nil
}

func (s *Memory) Finish(_ context.Context, uid, key string, rec Record, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[uid+"/"+key] = memEntry{rec: rec, done: true, started: now, expires: now.Add(TTL)}
	return nil
}

func (s *Memory) Abort(_ context.Context, uid, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, uid+"/"+key)
	return nil
}
