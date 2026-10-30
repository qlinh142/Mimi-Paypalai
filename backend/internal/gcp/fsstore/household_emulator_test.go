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
	mustSet(t, c.Collection("users").Doc(owner), map[string]any{"role": "owner"})
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
	// Gửi lại món đang giao: không còn mở.
	if _, err := svc.Assign(ctx, op, household.AssignRequest{BuyerUID: buyer, Keys: []string{"gao"}}); err == nil {
		t.Fatal("assigned line must not be sent twice")
	}
	d, err := svc.Delegated(ctx, bp)
	if err != nil || len(d.ToMe) != 1 || d.ToMe[0].ID != a.ID {
		t.Fatalf("delegated %+v %v", d, err)
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
