package fsstore

import (
	"context"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
)

// Hai máy cùng đọc rev 0, máy A lưu trước; máy B gửi base_rev 0 phải nhận 409.
func TestSetStateRevEmulator(t *testing.T) {
	s, c, uid := emulator(t)
	ctx := context.Background()
	mustSet(t, c.Collection("users").Doc(uid), map[string]any{"display_name": "Mẹ"})
	var zero int64
	rev, err := s.SetState(ctx, uid, plan.State{TripNote: "máy A", BaseRev: &zero})
	if err != nil || rev != 1 {
		t.Fatalf("rev %d err %v", rev, err)
	}
	if _, err := s.SetState(ctx, uid, plan.State{TripNote: "máy B", BaseRev: &zero}); err == nil || apperr.From(err).Code != apperr.CodeConflict {
		t.Fatalf("want conflict, got %v", err)
	}
	st, err := s.State(ctx, uid)
	if err != nil || st.Rev != 1 || st.TripNote != "máy A" {
		t.Fatalf("%+v %v", st, err)
	}
	// Không gửi base_rev (bản app cũ): vẫn ghi, rev tăng.
	if rev, err := s.SetState(ctx, uid, plan.State{TripNote: "cũ"}); err != nil || rev != 2 {
		t.Fatalf("rev %d err %v", rev, err)
	}
}
