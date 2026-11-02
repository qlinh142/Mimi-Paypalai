package idem

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemory(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	s := NewMemory()
	if r, err := s.Begin(ctx, "u", "k", "fp", now); r != nil || err != nil {
		t.Fatalf("first begin: %v %v", r, err)
	}
	if _, err := s.Begin(ctx, "u", "k", "fp", now.Add(time.Second)); !errors.Is(err, ErrInFlight) {
		t.Fatalf("want in flight, got %v", err)
	}
	// Bỏ dở quá PendingStale thì được nhận lại.
	if r, err := s.Begin(ctx, "u", "k", "fp", now.Add(PendingStale+time.Second)); r != nil || err != nil {
		t.Fatalf("stale reclaim: %v %v", r, err)
	}
	_ = s.Finish(ctx, "u", "k", Record{Fingerprint: "fp", Status: 201, Body: []byte("ok")}, now)
	r, err := s.Begin(ctx, "u", "k", "fp", now.Add(time.Hour))
	if err != nil || r == nil || r.Status != 201 || string(r.Body) != "ok" {
		t.Fatalf("replay: %+v %v", r, err)
	}
	if _, err := s.Begin(ctx, "u", "k", "other", now.Add(time.Hour)); !errors.Is(err, ErrMismatch) {
		t.Fatalf("want mismatch, got %v", err)
	}
	// Hết 24 giờ thì key dùng lại được.
	if r, err := s.Begin(ctx, "u", "k", "other", now.Add(TTL+time.Second)); r != nil || err != nil {
		t.Fatalf("expired: %v %v", r, err)
	}
	_ = s.Abort(ctx, "u", "k")
	if r, err := s.Begin(ctx, "u", "k", "fp", now); r != nil || err != nil {
		t.Fatalf("after abort: %v %v", r, err)
	}
}
