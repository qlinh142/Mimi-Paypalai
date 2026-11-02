package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// sharedFake giả Firestore: một bộ đếm chung cho mọi Server.
type sharedFake struct {
	counts map[string]int
	err    error
}

func (f *sharedFake) Allow(_ context.Context, key string, limit int, _ time.Time) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.counts[key] >= limit {
		return false, nil
	}
	f.counts[key]++
	return true, nil
}

// A5: hai instance chung bộ đếm thì tổng lượt ghi của một người không vượt giới hạn.
func TestAllowWriteSharedAcrossInstances(t *testing.T) {
	shared := &sharedFake{counts: map[string]int{}}
	mk := func() *Server {
		return &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil)), limiter: newWindowLimiter(3), shared: shared}
	}
	a, b := mk(), mk()
	now := time.Now()
	allowed := 0
	for i := 0; i < 6; i++ {
		s := a
		if i%2 == 1 {
			s = b
		}
		if s.allowWrite(context.Background(), "u1", now) {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("allowed %d, want 3", allowed)
	}

	// Firestore lỗi: không chặn, bộ đếm tại chỗ vẫn giữ giới hạn từng instance.
	shared.err = errors.New("unavailable")
	c := mk()
	got := 0
	for i := 0; i < 5; i++ {
		if c.allowWrite(context.Background(), "u2", now) {
			got++
		}
	}
	if got != 3 {
		t.Fatalf("fallback allowed %d, want 3", got)
	}
}
