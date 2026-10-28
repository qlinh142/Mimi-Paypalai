package paypal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientFlow(t *testing.T) {
	var gotRequestIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/oauth2/token":
			if u, p, ok := r.BasicAuth(); !ok || u != "id" || p != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
		case r.Header.Get("Authorization") != "Bearer tok":
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/v2/checkout/orders":
			gotRequestIDs = append(gotRequestIDs, r.Header.Get("PayPal-Request-Id"))
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			pu := body["purchase_units"].([]any)[0].(map[string]any)
			if pu["amount"].(map[string]any)["value"] != "12.34" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"id":"PP1","status":"PAYER_ACTION_REQUIRED","links":[{"rel":"payer-action","href":"https://pp/approve"}]}`))
		case r.URL.Path == "/v2/checkout/orders/PP1/capture":
			_, _ = w.Write([]byte(`{"status":"COMPLETED","purchase_units":[{"payments":{"captures":[{"id":"CAP1","status":"COMPLETED","amount":{"value":"12.34"}}]}}]}`))
		case r.URL.Path == "/v2/checkout/orders/BAD/capture":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"ORDER_NOT_APPROVED"}]}`))
		case r.URL.Path == "/v2/payments/captures/CAP1/refund":
			_, _ = w.Write([]byte(`{"id":"REF1","status":"COMPLETED"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New(srv.URL, "id", "secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	o, err := c.CreateOrder(ctx, "o-1", "12.34", "Mimi order")
	if err != nil || o.ID != "PP1" || o.ApproveURL != "https://pp/approve" {
		t.Fatalf("create: %v %+v", err, o)
	}
	if len(gotRequestIDs) != 1 || gotRequestIDs[0] != "create-o-1" {
		t.Fatalf("request ids %v", gotRequestIDs)
	}
	cp, err := c.CaptureOrder(ctx, "PP1", "o-1")
	if err != nil || cp.ID != "CAP1" || cp.Amount != "12.34" {
		t.Fatalf("capture: %v %+v", err, cp)
	}
	if _, err := c.CaptureOrder(ctx, "BAD", "o-2"); !errors.Is(err, ErrDeclined) || !strings.Contains(err.Error(), "ORDER_NOT_APPROVED") {
		t.Fatalf("declined: %v", err)
	}
	rf, err := c.RefundCapture(ctx, "CAP1", "2.00", "o-1-r1", "missing eggs")
	if err != nil || rf.ID != "REF1" {
		t.Fatalf("refund: %v %+v", err, rf)
	}
}

func TestFakeIsStatelessAndIdempotent(t *testing.T) {
	ctx := context.Background()
	a, _ := NewFake().CreateOrder(ctx, "o-1", "5.00", "")
	b, _ := NewFake().CreateOrder(ctx, "o-1", "5.00", "")
	if a.ID != b.ID {
		t.Fatal("same ref must return same order")
	}
	// Một instance khác capture được order do instance này tạo.
	c1, err := NewFake().CaptureOrder(ctx, a.ID, "o-1")
	c2, _ := NewFake().CaptureOrder(ctx, a.ID, "o-1")
	if err != nil || c1.ID != c2.ID || c1.Amount != "5.00" {
		t.Fatalf("capture %v %+v %+v", err, c1, c2)
	}
	if _, err := NewFake().CaptureOrder(ctx, "PP-REAL", "o-1"); !errors.Is(err, ErrDeclined) {
		t.Fatalf("unknown order: %v", err)
	}
	r, err := NewFake().RefundCapture(ctx, c1.ID, "1.00", "o-1-r1", "")
	if err != nil || r.Status != "COMPLETED" {
		t.Fatalf("refund %v %+v", err, r)
	}
}
