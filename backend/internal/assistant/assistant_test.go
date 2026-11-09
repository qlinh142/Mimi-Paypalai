package assistant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
)

type fakeModel struct {
	calls int
	err   error
}

func (f *fakeModel) Turn(context.Context, Request) (Response, error) {
	f.calls++
	if f.err != nil {
		return Response{}, f.err
	}
	return Response{ID: "r1", Parts: nil}, nil
}

func ask(text string) Request {
	return Request{Contents: []Content{{Role: "user", Parts: []Part{{Text: text}}}}}
}

func code(err error) apperr.Code {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestValidate(t *testing.T) {
	cases := map[string]Request{
		"empty":     {},
		"bad role":  {Contents: []Content{{Role: "system", Parts: []Part{{Text: "x"}}}}},
		"bad mode":  {Contents: ask("x").Contents, ToolMode: "MAYBE"},
		"tool name": {Contents: ask("x").Contents, Tools: []Tool{{Description: "no name"}}},
		"too long":  {Contents: make([]Content, MaxContents+1)},
	}
	for name, req := range cases {
		if err := req.Validate(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if err := ask("Tháng này chi bao nhiêu?").Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDailyLimitPerUser(t *testing.T) {
	m := &fakeModel{}
	s := New(m, 2)
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if _, err := s.Turn(context.Background(), "u1", ask("hi")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Turn(context.Background(), "u1", ask("hi")); code(err) != apperr.CodeQuotaExceeded {
		t.Fatalf("want quota_exceeded, got %v", err)
	}
	// Người khác vẫn hỏi được; sang ngày mới thì đếm lại.
	if _, err := s.Turn(context.Background(), "u2", ask("hi")); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24 * time.Hour)
	if _, err := s.Turn(context.Background(), "u1", ask("hi")); err != nil {
		t.Fatal(err)
	}
	if m.calls != 4 {
		t.Fatalf("model called %d times", m.calls)
	}
}

func TestInvalidRequestDoesNotUseQuotaOrModel(t *testing.T) {
	m := &fakeModel{}
	s := New(m, 1)
	if _, err := s.Turn(context.Background(), "u1", Request{}); code(err) != apperr.CodeValidation {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Turn(context.Background(), "u1", ask("hi")); err != nil {
		t.Fatal(err)
	}
	if m.calls != 1 {
		t.Fatalf("model called %d times", m.calls)
	}
}

func TestModelErrorBecomesAIUnavailable(t *testing.T) {
	s := New(&fakeModel{err: errors.New("vertex 503")}, 0)
	if _, err := s.Turn(context.Background(), "u1", ask("hi")); code(err) != apperr.CodeAIUnavailable {
		t.Fatalf("got %v", err)
	}
}

func TestPartsNeverNil(t *testing.T) {
	out, err := New(&fakeModel{}, 0).Turn(context.Background(), "u1", ask("hi"))
	if err != nil || out.Parts == nil {
		t.Fatalf("%+v %v", out, err)
	}
}
