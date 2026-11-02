package fsstore

import (
	"context"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/household"
)

func TestHouseholdFlowEmulator(t *testing.T) {
	s, c, id := emulator(t)
	ctx := context.Background()
	owner, buyer := "o"+id, "b"+id
	mustSet(t, c.Collection("users").Doc(owner), map[string]any{"role": "owner",
		"plan_state": map[string]any{"buy_date": "2026-10-24", "buy_hour": int64(9), "buy_minute": int64(30), "trip_note": "WinMart gần nhà"}})
	mustSet(t, c.Collection("users").Doc(buyer), map[string]any{"role": "owner"})
	ou := c.Collection("users").Doc(owner)
	for _, g := range []string{"sua-tuoi", "gao"} {
		mustSet(t, ou.Collection("plan_items").Doc(g), map[string]any{"status": "open", "name": g, "qty": 1.0})
	}
	svc := household.New(s, s, nil)
	op := authz.Principal{UID: owner, Role: authz.RoleOwner}
	inv, err := svc.Invite(ctx, op, household.Member{UID: owner, DisplayName: "Mẹ"})
	if err != nil || len(inv.Code) != 8 {
		t.Fatalf("invite %+v %v", inv, err)
	}
	bp := authz.Principal{UID: buyer, Role: authz.RoleOwner}
	h, err := svc.Join(ctx, bp, household.Member{UID: buyer, DisplayName: "Bố"}, household.JoinRequest{Code: inv.Code})
	if err != nil || len(h.Members) != 2 || h.Members[0].UID != owner {
		t.Fatalf("join %+v %v", h, err)
	}
	op.HouseholdID, bp.HouseholdID, bp.Role = h.ID, h.ID, authz.RoleMember

	a, err := svc.Assign(ctx, op, household.AssignRequest{BuyerUID: buyer, Keys: []string{"sua-tuoi", "gao"}})
	if err != nil || len(a.Items) != 2 || a.BuyerName != "Bố" {
		t.Fatalf("assign %+v %v", a, err)
	}
	if st := get(t, ou.Collection("plan_items").Doc("gao"))["status"]; st != "assigned" {
		t.Fatalf("plan line %v", st)
	}
	// B5: gửi lại cho cùng người là cập nhật vòng đang mở (không tạo vòng thứ hai). Món bỏ
	// khỏi danh sách mới trả về kế hoạch, món còn lại vẫn đang giao.
	a2, err := svc.Assign(ctx, op, household.AssignRequest{BuyerUID: buyer, Keys: []string{"gao"}})
	if err != nil || a2.ID != a.ID || len(a2.Items) != 1 {
		t.Fatalf("resend %+v %v", a2, err)
	}
	if st := get(t, ou.Collection("plan_items").Doc("sua-tuoi"))["status"]; st != "open" {
		t.Fatalf("dropped line %v", st)
	}
	if st := get(t, ou.Collection("plan_items").Doc("gao"))["status"]; st != "assigned" {
		t.Fatalf("kept line %v", st)
	}
	d, err := svc.Delegated(ctx, bp)
	if err != nil || len(d.ToMe) != 1 || d.ToMe[0].ID != a.ID {
		t.Fatalf("delegated %+v %v", d, err)
	}
	// 3B.4: người nhận thấy thời gian và ghi chú của Plan gốc.
	if tr := d.ToMe[0].Trip; tr == nil || tr.BuyDate != "2026-10-24" || tr.BuyMinute == nil || *tr.BuyMinute != 30 || tr.TripNote != "WinMart gần nhà" {
		t.Fatalf("trip %+v", tr)
	}
	if _, err := svc.Cancel(ctx, bp, a.ID); err == nil {
		t.Fatal("buyer must not cancel owner's assignment")
	}
	if got, err := svc.Cancel(ctx, op, a.ID); err != nil || got.Status != "cancelled" {
		t.Fatalf("cancel %+v %v", got, err)
	}
	if st := get(t, ou.Collection("plan_items").Doc("gao"))["status"]; st != "open" {
		t.Fatalf("returned line %v", st)
	}
	// Bố rời nhà: về nhà một người.
	if err := svc.Leave(ctx, bp); err != nil {
		t.Fatal(err)
	}
	if u := get(t, c.Collection("users").Doc(buyer)); u["household_id"] != "" || u["role"] != "owner" {
		t.Fatalf("after leave %v", u)
	}
	if h, _ := s.Household(ctx, op.HouseholdID); len(h.Members) != 1 {
		t.Fatalf("members after leave %+v", h.Members)
	}
}
