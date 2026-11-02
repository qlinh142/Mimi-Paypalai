package fsstore

import (
	"context"
	"testing"
	"time"
)

// Hai "instance" dùng hai RateLimiter riêng nhưng chung Firestore: tổng lượt không vượt giới hạn.
func TestRateLimiterSharedEmulator(t *testing.T) {
	s, _, uid := emulator(t)
	a, b := NewRateLimiter(s), NewRateLimiter(s)
	ctx := context.Background()
	now := time.Date(2026, 11, 2, 9, 0, 10, 0, time.UTC)
	allowed := 0
	for i := 0; i < 6; i++ {
		l := a
		if i%2 == 1 {
			l = b
		}
		ok, err := l.Allow(ctx, uid, 4, now)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			allowed++
		}
	}
	if allowed != 4 {
		t.Fatalf("allowed %d, want 4", allowed)
	}
	// Phút sau đếm lại từ đầu.
	if ok, err := a.Allow(ctx, uid, 4, now.Add(time.Minute)); err != nil || !ok {
		t.Fatalf("next minute: %v %v", ok, err)
	}
}
