package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/spending"
	"github.com/smartlife-lab/mimi/backend/internal/user"
)

type fakeSignIn map[string]time.Time // token -> lúc đăng nhập

func (f fakeSignIn) SignedInAt(_ context.Context, tok string) (string, time.Time, error) {
	at, ok := f[tok]
	if !ok {
		return "", time.Time{}, errors.New("revoked")
	}
	return "u1", at, nil
}

type fakeAccounts struct{ deleted []string }

func (f *fakeAccounts) Delete(_ context.Context, p authz.Principal) error {
	f.deleted = append(f.deleted, p.UID)
	return nil
}

// B10: xóa tài khoản chỉ khi vừa đăng nhập lại; quá 5 phút hoặc token bị thu hồi thì 403
// reauth_required (không phải 401, để app không tự đăng xuất).
func TestDeleteMeNeedsRecentSignIn(t *testing.T) {
	acc := &fakeAccounts{}
	s, err := New(Deps{
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tokens:     fakeTokens{"fresh": "u1", "old": "u1", "revoked": "u1"},
		AppCheck:   fakeAppCheck{},
		Principals: fakePrincipals{"u1": {UID: "u1", Role: authz.RoleOwner}},
		Profiles:   fakeProfiles{}, Receipts: &fakeReceipts{}, Items: fakeItems{}, Ready: fakeReady{},
		Home: fakeHome{}, Spending: spending.New(fakeSpendingStore{}), Settings: user.NewSettingsService(fakeSettingsStore{}, nil),
		Plan:     plan.New(&fakePlanStore{lines: map[string]plan.Line{}}, fakePlanItems{}, nil),
		Notes:    fakeNotes{},
		Accounts: acc,
		SignIn:   fakeSignIn{"fresh": time.Now().Add(-time.Minute), "old": time.Now().Add(-time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()
	for _, tok := range []string{"old", "revoked"} {
		rec, body := do(h, "DELETE", "/api/me", tok, "")
		if rec.Code != http.StatusForbidden || body["code"] != "reauth_required" {
			t.Fatalf("%s: %d %v", tok, rec.Code, body)
		}
	}
	if len(acc.deleted) != 0 {
		t.Fatalf("deleted %v", acc.deleted)
	}
	rec, _ := do(h, "DELETE", "/api/me", "fresh", "")
	if rec.Code != http.StatusNoContent || len(acc.deleted) != 1 || acc.deleted[0] != "u1" {
		t.Fatalf("fresh: %d %v", rec.Code, acc.deleted)
	}
}
