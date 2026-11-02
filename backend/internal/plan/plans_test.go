package plan

import (
	"context"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
)

type memPlans struct {
	plans       map[string]Meta
	defaultIcon string
}

func (m *memPlans) ListPlans(context.Context, string) ([]Meta, error) {
	var out []Meta
	for _, p := range m.plans {
		out = append(out, p)
	}
	return out, nil
}
func (m *memPlans) GetPlan(_ context.Context, _, id string) (Meta, error) {
	p, ok := m.plans[id]
	if !ok {
		return Meta{}, apperr.ErrNotFound
	}
	return p, nil
}
func (m *memPlans) CreatePlan(_ context.Context, _ string, p Meta) error {
	m.plans[p.ID] = p
	return nil
}
func (m *memPlans) UpdatePlan(_ context.Context, _ string, p Meta) error {
	m.plans[p.ID] = p
	return nil
}
func (m *memPlans) DeletePlan(_ context.Context, _, id string) error { delete(m.plans, id); return nil }
func (m *memPlans) DefaultPlanIcon(context.Context, string) (string, error) {
	return m.defaultIcon, nil
}
func (m *memPlans) SetDefaultPlanIcon(_ context.Context, _, icon string) error {
	m.defaultIcon = icon
	return nil
}

// B11: biểu tượng Kế hoạch lưu trên máy chủ, đăng nhập lại vẫn còn.
func TestPlanIconStoredOnServer(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup()
	ps := &memPlans{plans: map[string]Meta{"p-old": {ID: "p-old", Name: "Cũ", Status: "active"}}}
	svc.WithPlans(ps)
	str := func(s string) *string { return &s }

	m, err := svc.CreatePlan(ctx, owner, PlanInput{Name: str("Sinh nhật"), Icon: str("gift")})
	if err != nil || m.Icon != "gift" || ps.plans[m.ID].Icon != "gift" {
		t.Fatalf("create: %+v %v", m, err)
	}
	if _, err := svc.CreatePlan(ctx, owner, PlanInput{Name: str("X"), Icon: str("rocket")}); apperr.From(err).Code != apperr.CodeValidation {
		t.Fatalf("icon lạ phải bị từ chối, got %v", err)
	}
	if m, err = svc.UpdatePlan(ctx, owner, m.ID, PlanInput{Icon: str("kid")}); err != nil || m.Icon != "kid" || m.Name != "Sinh nhật" {
		t.Fatalf("update: %+v %v", m, err)
	}
	created := m.ID
	if m, err = svc.UpdatePlan(ctx, owner, DefaultPlanID, PlanInput{Icon: str("home")}); err != nil || m.Icon != "home" || !m.Default {
		t.Fatalf("default icon: %+v %v", m, err)
	}
	if _, err := svc.UpdatePlan(ctx, owner, DefaultPlanID, PlanInput{Name: str("Đổi")}); apperr.From(err).Code != apperr.CodeValidation {
		t.Fatalf("không đổi tên Kế hoạch chính, got %v", err)
	}
	all, err := svc.Plans(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, p := range all {
		got[p.ID] = p.Icon
	}
	if got[DefaultPlanID] != "home" || got[created] != "kid" || got["p-old"] != DefaultIcon {
		t.Fatalf("icons: %v", got)
	}
}
