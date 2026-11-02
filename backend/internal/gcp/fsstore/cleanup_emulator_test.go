package fsstore

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

// Job cleanup: mã nhà cố định nên không bị gỡ (kể cả mã cũ còn invite_expires_at); xóa thông báo quá 90 ngày, giữ cái mới.
func TestCleanupEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	now := time.Now()
	old, fresh := c.Collection("households").Doc(uid+"-old"), c.Collection("households").Doc(uid+"-new")
	mustSet(t, old, map[string]any{"name": "A", "invite_code": "OLD1", "invite_expires_at": now.Add(-time.Hour)})
	mustSet(t, fresh, map[string]any{"name": "B", "invite_code": "NEW1", "invite_expires_at": now.Add(time.Hour)})
	u := c.Collection("users").Doc(uid)
	mustSet(t, u, map[string]any{"display_name": "Mẹ"})
	mustSet(t, u.Collection("events").Doc("cu"), map[string]any{"at": now.Add(-100 * 24 * time.Hour)})
	mustSet(t, u.Collection("events").Doc("moi"), map[string]any{"at": now.Add(-time.Hour)})

	res, err := s.Cleanup(ctx, now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Events < 1 {
		t.Fatalf("%+v", res)
	}
	if snap, _ := old.Get(ctx); snap.Data()["invite_code"] != "OLD1" {
		t.Fatal("mã nhà cũ bị gỡ")
	}
	if snap, _ := fresh.Get(ctx); snap.Data()["invite_code"] != "NEW1" {
		t.Fatal("mã còn hạn bị gỡ")
	}
	if snap, err := u.Collection("events").Doc("cu").Get(ctx); err == nil && snap.Exists() {
		t.Fatal("thông báo cũ còn")
	}
	if _, err := u.Collection("events").Doc("moi").Get(ctx); err != nil {
		t.Fatal("thông báo mới bị xóa")
	}
}
