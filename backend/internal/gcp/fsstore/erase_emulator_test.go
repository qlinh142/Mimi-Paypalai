package fsstore

import (
	"context"
	"testing"
)

// B10: xóa đệ quy cả collection con nhiều tầng, gọi lại không lỗi.
func TestEraseUserEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	u := c.Collection("users").Doc(uid)
	mustSet(t, u, map[string]any{"display_name": "Mẹ"})
	mustSet(t, u.Collection("plans").Doc("p1"), map[string]any{"name": "Tết"})
	mustSet(t, u.Collection("plans").Doc("p1").Collection("items").Doc("sua"), map[string]any{"qty": 1})
	// Tài liệu "ma": chỉ có collection con, không có dữ liệu.
	mustSet(t, u.Collection("months").Doc("2026-11").Collection("x").Doc("y"), map[string]any{"v": 1})
	for i := 0; i < 2; i++ {
		if err := s.EraseUser(ctx, uid); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"plans/p1", "plans/p1/items/sua", "months/2026-11/x/y"} {
		if snap, err := c.Doc("users/" + uid + "/" + path).Get(ctx); err == nil && snap.Exists() {
			t.Fatalf("%s còn", path)
		}
	}
	if snap, err := u.Get(ctx); err == nil && snap.Exists() {
		t.Fatal("user doc còn")
	}
}
