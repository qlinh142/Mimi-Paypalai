package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/idem"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/spending"
	"github.com/smartlife-lab/mimi/backend/internal/user"
)

// flakyReceipts lỗi tạm thời fail lần đầu rồi lưu bình thường.
type flakyReceipts struct {
	fakeReceipts
	fail int
}

func (f *flakyReceipts) Save(ctx context.Context, p authz.Principal, req receipt.SaveRequest) (receipt.SaveOutcome, error) {
	if f.fail > 0 {
		f.fail--
		return receipt.SaveOutcome{}, apperr.New(apperr.CodeTransactionConflict, "thử lại")
	}
	return f.fakeReceipts.Save(ctx, p, req)
}

func newIdemServer(t *testing.T, fail int) (http.Handler, *flakyReceipts, *idem.Memory) {
	t.Helper()
	rc := &flakyReceipts{fail: fail}
	st := idem.NewMemory()
	s, err := New(Deps{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tokens:   fakeTokens{"owner-tok": "u1", "member-tok": "u2"},
		AppCheck: fakeAppCheck{},
		Principals: fakePrincipals{
			"u1": {UID: "u1", Role: authz.RoleOwner},
			"u2": {UID: "u2", Role: authz.RoleOwner},
		},
		Profiles: fakeProfiles{}, Receipts: rc, Items: fakeItems{}, Ready: fakeReady{},
		Home: fakeHome{}, Spending: spending.New(fakeSpendingStore{}), Settings: user.NewSettingsService(fakeSettingsStore{}, nil),
		Plan:  plan.New(&fakePlanStore{lines: map[string]plan.Line{}}, fakePlanItems{}, nil),
		Notes: fakeNotes{},
	}, WithIdempotency(st))
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler(), rc, st
}

func send(h http.Handler, token, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/receipts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Firebase-AppCheck", "ok")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const body1 = `{"receipt_id":"r-1","store":"BHX"}`

func TestIdempotency(t *testing.T) {
	t.Run("same key and body is applied once and replayed", func(t *testing.T) {
		h, rc, _ := newIdemServer(t, 0)
		a := send(h, "owner-tok", "key-00000001", body1)
		b := send(h, "owner-tok", "key-00000001", body1)
		if a.Code != http.StatusCreated || b.Code != http.StatusCreated {
			t.Fatalf("status %d %d: %s", a.Code, b.Code, b.Body)
		}
		if rc.saved != 1 {
			t.Fatalf("saved %d times, want 1", rc.saved)
		}
		if b.Header().Get("Idempotent-Replayed") != "true" || a.Body.String() != b.Body.String() {
			t.Fatalf("replay mismatch: %q vs %q", a.Body, b.Body)
		}
	})
	t.Run("same key different body is rejected", func(t *testing.T) {
		h, rc, _ := newIdemServer(t, 0)
		send(h, "owner-tok", "key-00000002", body1)
		r := send(h, "owner-tok", "key-00000002", `{"receipt_id":"r-2","store":"BHX"}`)
		if r.Code != http.StatusUnprocessableEntity || !strings.Contains(r.Body.String(), "idempotency_mismatch") {
			t.Fatalf("got %d %s", r.Code, r.Body)
		}
		if rc.saved != 1 {
			t.Fatalf("saved %d", rc.saved)
		}
	})
	t.Run("keys are per user", func(t *testing.T) {
		h, rc, _ := newIdemServer(t, 0)
		send(h, "owner-tok", "key-00000003", body1)
		send(h, "member-tok", "key-00000003", body1)
		if rc.saved != 2 {
			t.Fatalf("saved %d, want 2", rc.saved)
		}
	})
	t.Run("no key keeps old behaviour", func(t *testing.T) {
		h, rc, _ := newIdemServer(t, 0)
		send(h, "owner-tok", "", body1)
		send(h, "owner-tok", "", body1)
		if rc.saved != 2 {
			t.Fatalf("saved %d, want 2", rc.saved)
		}
	})
	t.Run("in flight is a retryable conflict", func(t *testing.T) {
		h, rc, st := newIdemServer(t, 0)
		if _, err := st.Begin(context.Background(), "u1", "key-00000004", fingerprint("POST", "/receipts", []byte(body1)), time.Now()); err != nil {
			t.Fatal(err)
		}
		r := send(h, "owner-tok", "key-00000004", body1)
		if r.Code != http.StatusConflict || !strings.Contains(r.Body.String(), `"retryable":true`) {
			t.Fatalf("got %d %s", r.Code, r.Body)
		}
		if rc.saved != 0 {
			t.Fatalf("saved %d", rc.saved)
		}
	})
	t.Run("temporary failure is not stored", func(t *testing.T) {
		h, rc, _ := newIdemServer(t, 1)
		a := send(h, "owner-tok", "key-00000005", body1)
		b := send(h, "owner-tok", "key-00000005", body1)
		if a.Code != http.StatusConflict || b.Code != http.StatusCreated || rc.saved != 1 {
			t.Fatalf("got %d then %d, saved %d", a.Code, b.Code, rc.saved)
		}
	})
	t.Run("bad key is rejected", func(t *testing.T) {
		h, rc, _ := newIdemServer(t, 0)
		r := send(h, "owner-tok", "bad key/../x", body1)
		if r.Code != http.StatusBadRequest || rc.saved != 0 {
			t.Fatalf("got %d saved %d", r.Code, rc.saved)
		}
	})
}
