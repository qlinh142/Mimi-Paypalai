package household

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
)

// asgStore giữ các vòng mua hộ trong bộ nhớ để kiểm luật hai bản (B5).
type asgStore struct {
	Store
	list               []Assignment
	created, cancelled int
	updated            *Assignment
}

func (m *asgStore) Household(context.Context, string) (Household, error) {
	return Household{ID: "h1", Members: []Member{{UID: "me", DisplayName: "Mẹ"}, {UID: "minh", DisplayName: "Minh"}, {UID: "lan", DisplayName: "Lan"}}}, nil
}
func (m *asgStore) Assignments(context.Context, string, string, string, int) ([]Assignment, error) {
	return m.list, nil
}
func (m *asgStore) CreateAssignment(_ context.Context, _ string, a Assignment) (Assignment, error) {
	m.created++
	m.list = append(m.list, a)
	return a, nil
}
func (m *asgStore) CancelAssignment(_ context.Context, _, id, _ string) (Assignment, error) {
	m.cancelled++
	for i := range m.list {
		if m.list[i].ID == id {
			m.list[i].Status = "cancelled"
		}
	}
	return Assignment{}, nil
}
func (m *asgStore) UpdateAssignment(_ context.Context, _ string, a Assignment) (Assignment, error) {
	m.updated = &a
	return a, nil
}

type asgPlans struct{ Plans }

func (asgPlans) OpenLineInfo(_ context.Context, _, _ string, keys []string) (map[string]AssignmentItem, error) {
	out := map[string]AssignmentItem{}
	for _, k := range keys {
		if k != "da-gui" { // đang assigned nên không còn "open"
			out[k] = AssignmentItem{Key: k, Name: "Món " + k, Qty: 2}
		}
	}
	return out, nil
}

func TestAssignKeepsTwoVersions(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 32, 0, 0, time.UTC)
	p := authz.Principal{UID: "me", HouseholdID: "h1", Role: authz.RoleOwner}
	st := &asgStore{}
	svc := New(st, asgPlans{}, func() time.Time { return now })

	// Lần đầu: tạo vòng mới.
	a, err := svc.Assign(context.Background(), p, AssignRequest{BuyerUID: "minh", PlanID: "p1", Keys: []string{"sua", "trung"}})
	if err != nil || st.created != 1 || a.UpdatedAt != nil {
		t.Fatalf("first send: %v created=%d", err, st.created)
	}

	// Gửi lại cho cùng người: ghi đè bản đã gửi, không tạo vòng thứ hai.
	st.list[0].Items = []AssignmentItem{{Key: "sua", Status: "done"}, {Key: "trung", Status: "assigned"}, {Key: "da-gui", Name: "Đã gửi", Qty: 1, Status: "assigned"}}
	got, err := svc.Assign(context.Background(), p, AssignRequest{BuyerUID: "minh", PlanID: "p1", Keys: []string{"trung", "da-gui", "rau"}})
	if err != nil {
		t.Fatal(err)
	}
	if st.created != 1 || st.updated == nil || got.UpdatedAt == nil || !got.UpdatedAt.Equal(now) {
		t.Fatalf("resend must update: created=%d updated=%v", st.created, st.updated)
	}
	byKey := map[string]AssignmentItem{}
	for _, it := range got.Items {
		byKey[it.Key] = it
	}
	if len(byKey) != 4 || byKey["sua"].Status != "done" || byKey["rau"].Status != "assigned" || byKey["da-gui"].Name != "Đã gửi" {
		t.Fatalf("items %+v", got.Items)
	}

	// Kế hoạch khác: vòng riêng.
	if _, err := svc.Assign(context.Background(), p, AssignRequest{BuyerUID: "minh", PlanID: "p2", Keys: []string{"gao"}}); err != nil || st.created != 2 {
		t.Fatalf("other plan: %v created=%d", err, st.created)
	}

	// Đổi người mua: hủy bản đã gửi người cũ rồi gửi người mới.
	if _, err := svc.Assign(context.Background(), p, AssignRequest{BuyerUID: "lan", PlanID: "p2", Keys: []string{"gao"}}); err != nil || st.cancelled != 1 || st.created != 3 {
		t.Fatalf("change buyer: %v cancelled=%d created=%d", err, st.cancelled, st.created)
	}

	// Vòng đã xong thì lần gửi sau là một lần nhờ mua mới.
	st.list[0].Status = "completed"
	if _, err := svc.Assign(context.Background(), p, AssignRequest{BuyerUID: "minh", PlanID: "p1", Keys: []string{"rau"}}); err != nil || st.created != 4 {
		t.Fatalf("after complete: %v created=%d", err, st.created)
	}
}

// 2.2: vòng đã mua xong vào to_me_done (tab Được nhờ mua giữ cả kế hoạch đã hoàn thành); vòng
// đang mở vẫn ở to_me như 2.1; vòng từ chối, đã hủy không hiện.
func TestDelegatedKeepsDone(t *testing.T) {
	st := &asgStore{list: []Assignment{
		{ID: "a1", Status: "assigned"}, {ID: "a2", Status: "shopping"}, {ID: "a3", Status: "completed"},
		{ID: "a4", Status: "closed_partial"}, {ID: "a5", Status: "declined"}, {ID: "a6", Status: "cancelled"},
	}}
	svc := New(st, runPlans{}, time.Now)
	d, err := svc.Delegated(context.Background(), minh)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(l []Assignment) (s string) {
		for _, a := range l {
			s += a.ID
		}
		return
	}
	if ids(d.ToMe) != "a1a2" || ids(d.ToMeDone) != "a3a4" {
		t.Fatalf("to_me %s, done %s", ids(d.ToMe), ids(d.ToMeDone))
	}
	if d.ToMeDone[0].Trip == nil || d.ToMeDone[0].Trip.TripNote != "Bách Hóa Xanh" {
		t.Fatalf("trip %+v", d.ToMeDone[0].Trip)
	}
}
