package httpapi

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/assistant"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/spending"
	"github.com/smartlife-lab/mimi/backend/internal/user"
)

type fakeAssistant struct {
	uid string
	req assistant.Request
}

func (f *fakeAssistant) Turn(_ context.Context, uid string, req assistant.Request) (assistant.Response, error) {
	f.uid, f.req = uid, req
	return assistant.Response{ID: "r1", Parts: []assistant.Part{{FunctionCall: &assistant.FunctionCall{ID: "c1", Name: "mimi_savings"}}}}, nil
}

func newAssistantServer(t *testing.T, a Assistant) *Server {
	t.Helper()
	s, err := New(Deps{
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tokens:     fakeTokens{"owner-tok": "u1"},
		AppCheck:   fakeAppCheck{},
		Principals: fakePrincipals{"u1": {UID: "u1", Role: authz.RoleOwner}},
		Profiles:   fakeProfiles{}, Receipts: &fakeReceipts{}, Items: fakeItems{}, Ready: fakeReady{},
		Home: fakeHome{}, Spending: spending.New(fakeSpendingStore{}), Settings: user.NewSettingsService(fakeSettingsStore{}, nil),
		Plan: plan.New(&fakePlanStore{}, fakePlanItems{}, nil), Notes: fakeNotes{},
		Assistant: a,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAITurn(t *testing.T) {
	fa := &fakeAssistant{}
	s := newAssistantServer(t, fa)
	body := `{"system":"You are Mimi.","contents":[{"role":"user","parts":[{"text":"Mimi giúp tiết kiệm bao nhiêu?"}]}],
		"tools":[{"name":"mimi_savings","description":"d","parameters":{"type":"object","properties":{}}}],"tool_mode":"AUTO"}`
	rec, out := do(s.Handler(), "POST", "/api/ai/turn", "owner-tok", body)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if fa.uid != "u1" || fa.req.Contents[0].Parts[0].Text != "Mimi giúp tiết kiệm bao nhiêu?" || string(fa.req.Tools[0].Parameters) == "" {
		t.Fatalf("%+v", fa)
	}
	parts := out["parts"].([]any)
	if parts[0].(map[string]any)["functionCall"].(map[string]any)["name"] != "mimi_savings" {
		t.Fatalf("%s", rec.Body)
	}
}

func TestAITurnNeedsSignIn(t *testing.T) {
	s := newAssistantServer(t, &fakeAssistant{})
	if rec, _ := do(s.Handler(), "POST", "/api/ai/turn", "", `{"contents":[]}`); rec.Code != 401 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAITurnOffWhenNoAssistant(t *testing.T) {
	s := newAssistantServer(t, nil)
	if rec, _ := do(s.Handler(), "POST", "/api/ai/turn", "owner-tok", `{"contents":[]}`); rec.Code == 200 {
		t.Fatalf("route should not be served, got %d", rec.Code)
	}
}
