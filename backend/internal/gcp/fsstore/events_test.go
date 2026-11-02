package fsstore

import (
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/household"
	"github.com/smartlife-lab/mimi/backend/internal/notify"
)

// B9: ai nhận thông báo khi một lần nhờ mua kết thúc.
func TestEndEventFor(t *testing.T) {
	a := household.Assignment{ID: "trip-1", OwnerUID: "me", OwnerName: "Mẹ", BuyerUID: "minh", BuyerName: "Minh", PlanID: "p1"}
	to, ev := endEventFor(a, "cancelled", 2)
	if to != "minh" || ev.Kind != notify.EventPlanCancelled || ev.ActorName != "Mẹ" || ev.ID != "cancel-trip-1" || ev.planOwner != "me" {
		t.Fatalf("cancel %s %+v", to, ev)
	}
	to, ev = endEventFor(a, "declined", 2)
	if to != "me" || ev.Kind != notify.EventRunDeclined || ev.ActorName != "Minh" {
		t.Fatalf("decline %s %+v", to, ev)
	}
	to, ev = endEventFor(a, "closed_partial", 2)
	if to != "me" || ev.Kind != notify.EventRunDone || ev.Returned != 2 || ev.ID != "done-trip-1" {
		t.Fatalf("closed_partial %s %+v", to, ev)
	}
}
