package household

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

type runStore struct {
	Store // các hàm khác không dùng trong test này
	a     Assignment
	ended string
}

func (m *runStore) Assignment(context.Context, string, string) (Assignment, error) { return m.a, nil }
func (m *runStore) EndByBuyer(_ context.Context, _, _, buyer, status string) (Assignment, error) {
	if buyer != m.a.BuyerUID {
		return Assignment{}, apperr.ErrForbidden
	}
	m.ended, m.a.Status = status, status
	return m.a, nil
}

type runPlans struct{ Plans }

func (runPlans) PlanTrip(context.Context, string, string) (Trip, error) {
	return Trip{TripNote: "Bách Hóa Xanh"}, nil
}

type saver struct {
	got   *receipt.SaveRequest
	store *runStore
}

func (s *saver) Save(_ context.Context, _ authz.Principal, req receipt.SaveRequest) (receipt.SaveOutcome, error) {
	s.got = &req
	s.store.a.Status, s.store.a.MatchedReceiptID = "completed", req.ReceiptID
	return receipt.SaveOutcome{ReceiptID: req.ReceiptID}, nil
}

var minh = authz.Principal{UID: "minh", Role: authz.RoleMember, HouseholdID: "h1", Timezone: "Asia/Ho_Chi_Minh"}

func runSetup() (*Service, *runStore, *saver) {
	st := &runStore{a: Assignment{ID: "trip-ab12", OwnerUID: "lan", BuyerUID: "minh", PlanID: "default", Status: "assigned",
		Items: []AssignmentItem{{Key: "sua-tuoi", Status: "assigned"}, {Key: "nuoc-mam", Status: "assigned"}}}}
	sv := &saver{store: st}
	now := func() time.Time { return time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC) }
	return New(st, runPlans{}, now).WithReceipts(sv), st, sv
}

func price(v float64) *float64 { return &v }

func TestCompleteWritesReceipt(t *testing.T) {
	svc, st, sv := runSetup()
	a, err := svc.Complete(context.Background(), minh, "trip-ab12", CompleteRequest{Lines: []RunLine{
		{Key: "sua-tuoi", Name: "Sữa tươi", Qty: 2, Price: price(29000), Ticked: true},
		{Key: "nuoc-mam", Name: "Nước mắm", Qty: 1, Ticked: false},
		{Key: "banh-mi", Name: "Bánh mì", Qty: 3, Ticked: true, Mine: true}, // chưa rõ giá
	}})
	if err != nil || a.Status != "completed" {
		t.Fatalf("%+v %v", a, err)
	}
	r := sv.got
	if r.ReceiptID != "run-trip-ab12" || r.AssignmentID != "trip-ab12" || r.Store != "Bách Hóa Xanh" || r.PurchasedOn != "2026-11-02" || len(r.Lines) != 2 {
		t.Fatalf("receipt %+v", r)
	}
	if r.Lines[0].GroupKey != "sua-tuoi" || r.Lines[0].Amount != "58000" || r.Lines[1].Amount != "" {
		t.Fatalf("lines %+v", r.Lines)
	}

	// Gửi lại sau khi đã xong (hàng đợi offline): trả vòng hiện tại, không lỗi.
	again, err := svc.Complete(context.Background(), minh, "trip-ab12", CompleteRequest{})
	if err != nil || again.Status != "completed" {
		t.Fatalf("replay: %+v %v", again, err)
	}
	_ = st
}

func TestCompleteRules(t *testing.T) {
	svc, st, _ := runSetup()
	ctx := context.Background()
	if _, err := svc.Complete(ctx, minh, "trip-ab12", CompleteRequest{Lines: []RunLine{{Key: "gao", Name: "Gạo", Qty: 1, Ticked: true}}}); apperr.From(err).Code != apperr.CodeValidation {
		t.Fatalf("món ngoài vòng phải bị từ chối, got %v", err)
	}
	lan := authz.Principal{UID: "lan", Role: authz.RoleOwner, HouseholdID: "h1"}
	if _, err := svc.Complete(ctx, lan, "trip-ab12", CompleteRequest{}); apperr.From(err).Code != apperr.CodeForbidden {
		t.Fatalf("chỉ người được nhờ báo xong, got %v", err)
	}
	a, err := svc.Complete(ctx, minh, "trip-ab12", CompleteRequest{Lines: []RunLine{{Key: "sua-tuoi", Name: "Sữa", Qty: 1}}})
	if err != nil || st.ended != "closed_partial" || a.Status != "closed_partial" {
		t.Fatalf("không tick món nào: %+v %v", a, err)
	}
}

func TestDecline(t *testing.T) {
	svc, st, _ := runSetup()
	ctx := context.Background()
	if _, err := svc.Decline(ctx, authz.Principal{UID: "lan", Role: authz.RoleOwner, HouseholdID: "h1"}, "trip-ab12"); apperr.From(err).Code != apperr.CodeForbidden {
		t.Fatalf("người nhờ không tự từ chối, got %v", err)
	}
	a, err := svc.Decline(ctx, minh, "trip-ab12")
	if err != nil || a.Status != "declined" || st.ended != "declined" {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := svc.Decline(ctx, minh, "trip-ab12"); apperr.From(err).Code != apperr.CodeInvalidState {
		t.Fatalf("từ chối lần hai phải báo đã kết thúc, got %v", err)
	}
}
