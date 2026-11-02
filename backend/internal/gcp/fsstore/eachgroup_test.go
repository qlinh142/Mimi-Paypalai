package fsstore

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// A9: các nhóm chạy song song, không vượt giới hạn, kết quả đúng ô.
func TestEachGroupParallel(t *testing.T) {
	groups := uniqueGroups([]string{"a", "b", "", "a", "c", "d", "e", "f", "g", "h", "i", "j"})
	if len(groups) != 10 || groups[0] != "a" || groups[2] != "c" {
		t.Fatalf("unique %v", groups)
	}
	var running, peak int32
	got := make([]string, len(groups))
	start := time.Now()
	err := eachGroup(context.Background(), groups, func(_ context.Context, i int, g string) error {
		n := atomic.AddInt32(&running, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		got[i] = g
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range groups {
		if got[i] != g {
			t.Fatalf("slot %d = %q, want %q", i, got[i], g)
		}
	}
	if peak < 2 || peak > groupReadConcurrency {
		t.Fatalf("peak %d", peak)
	}
	if el := time.Since(start); el > 150*time.Millisecond {
		t.Fatalf("không chạy song song: %v", el)
	}

	boom := errors.New("boom")
	if err := eachGroup(context.Background(), groups, func(context.Context, int, string) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
}
