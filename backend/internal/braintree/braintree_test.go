package braintree

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// gql dựng server GraphQL giả: chọn câu trả lời theo tên mutation trong query.
func gql(t *testing.T, settled bool, calls *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "pub" || p != "priv" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Braintree-Version") != Version {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		in, _ := body.Variables["input"].(map[string]any)
		switch {
		case strings.Contains(body.Query, "createClientToken"):
			*calls = append(*calls, "token")
			_, _ = w.Write([]byte(`{"data":{"createClientToken":{"clientToken":"CT"}}}`))
		case strings.Contains(body.Query, "chargePaymentMethod"):
			*calls = append(*calls, "charge")
			if in["paymentMethodId"] != "nonce-ok" {
				_, _ = w.Write([]byte(`{"errors":[{"message":"Unknown or expired payment method ID.","extensions":{"errorClass":"VALIDATION"}}],"data":{"chargePaymentMethod":null}}`))
				return
			}
			txn := in["transaction"].(map[string]any)
			if txn["amount"] != "5.28" || txn["orderId"] != "o-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"chargePaymentMethod":{"transaction":{"id":"TX1","status":"SUBMITTED_FOR_SETTLEMENT","amount":{"value":"5.28"}}}}}`))
		case strings.Contains(body.Query, "refundTransaction"):
			*calls = append(*calls, "refund")
			if !settled {
				_, _ = w.Write([]byte(`{"errors":[{"message":"Cannot refund transaction unless it is settled.","extensions":{"errorClass":"VALIDATION"}}],"data":{"refundTransaction":null}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"refundTransaction":{"refund":{"id":"RF1","status":"SUBMITTED_FOR_SETTLEMENT"}}}}`))
		case strings.Contains(body.Query, "reverseTransaction"):
			*calls = append(*calls, "reverse")
			_, _ = w.Write([]byte(`{"data":{"reverseTransaction":{"reversal":{"id":"TX1","status":"VOIDED"}}}}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func TestClientFlow(t *testing.T) {
	var calls []string
	srv := gql(t, true, &calls)
	defer srv.Close()
	c, err := New(srv.URL, "pub", "priv", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if tok, err := c.ClientToken(ctx); err != nil || tok != "CT" {
		t.Fatalf("token: %v %q", err, tok)
	}
	tx, err := c.Charge(ctx, "nonce-ok", "5.28", "o-1")
	if err != nil || tx.ID != "TX1" || !Paid(tx.Status) {
		t.Fatalf("charge: %v %+v", err, tx)
	}
	if _, err := c.Charge(ctx, "nonce-bad", "5.28", "o-1"); !errors.Is(err, ErrDeclined) {
		t.Fatalf("bad nonce: %v", err)
	}
	rf, err := c.Refund(ctx, "TX1", "1.00", "o-1-r1", false)
	if err != nil || rf.ID != "RF1" {
		t.Fatalf("refund: %v %+v", err, rf)
	}
}

func TestRefundUnsettled(t *testing.T) {
	var calls []string
	srv := gql(t, false, &calls)
	defer srv.Close()
	c, _ := New(srv.URL, "pub", "priv", "")
	ctx := context.Background()
	if _, err := c.Refund(ctx, "TX1", "1.00", "o-1-r1", false); !errors.Is(err, ErrNotSettled) {
		t.Fatalf("partial unsettled: %v", err)
	}
	rf, err := c.Refund(ctx, "TX1", "5.28", "o-1-r1", true)
	if err != nil || rf.Status != "VOIDED" {
		t.Fatalf("full unsettled must reverse: %v %+v", err, rf)
	}
	if got := strings.Join(calls, ","); got != "refund,refund,reverse" {
		t.Fatalf("calls %s", got)
	}
}

func TestUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer srv.Close()
	c, _ := New(srv.URL, "pub", "priv", "")
	if _, err := c.ClientToken(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("503: %v", err)
	}
	if _, err := New("", "", "priv", ""); err == nil {
		t.Fatal("missing public key must fail")
	}
}

func TestFake(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	if tok, _ := f.ClientToken(ctx); tok != FakeClientToken {
		t.Fatal(tok)
	}
	if _, err := f.Charge(ctx, "other", "1.00", "o-1"); !errors.Is(err, ErrDeclined) {
		t.Fatalf("fake must decline unknown nonce: %v", err)
	}
	tx, err := f.Charge(ctx, FakeNonce, "1.00", "o-1")
	if err != nil || !Paid(tx.Status) {
		t.Fatalf("fake charge %v %+v", err, tx)
	}
	if _, err := f.Refund(ctx, tx.ID, "1.00", "r", true); err != nil {
		t.Fatal(err)
	}
}

func TestParseWebhook(t *testing.T) {
	c, err := New("", "pub", "priv", "")
	if err != nil {
		t.Fatal(err)
	}
	body := `<?xml version="1.0" encoding="UTF-8"?>
<notification>
  <kind>transaction_settlement_declined</kind>
  <timestamp type="datetime">2026-11-04T10:00:00Z</timestamp>
  <subject><transaction><id>txn1</id><status>settlement_declined</status><order-id>ord1</order-id></transaction></subject>
</notification>`
	sig, payload := SignWebhook("pub", "priv", body)
	n, err := c.ParseWebhook("other|00&"+sig, payload)
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != KindTransactionSettlementDecl || n.TxnID != "txn1" || n.OrderID != "ord1" || n.Timestamp.IsZero() {
		t.Fatalf("%+v", n)
	}
	// Braintree có thể xuống dòng trong base64; chữ ký tính trên payload + "\n" cũng hợp lệ.
	key := sha1.Sum([]byte("priv"))
	m := hmac.New(sha1.New, key[:])
	m.Write([]byte(payload + "\n"))
	if _, err := c.ParseWebhook("pub|"+hex.EncodeToString(m.Sum(nil)), payload); err != nil {
		t.Fatalf("newline variant: %v", err)
	}
	for _, bad := range [][2]string{
		{sig, payload + "AA"}, // payload bị sửa
		{"pub|zz", payload},   // chữ ký không phải hex
		{"", payload},         // thiếu chữ ký
		{sig, ""},             // thiếu payload
	} {
		if _, err := c.ParseWebhook(bad[0], bad[1]); !errors.Is(err, ErrBadWebhook) {
			t.Fatalf("%q: %v", bad[0], err)
		}
	}
	if _, err := NewFake().ParseWebhook(sig, payload); !errors.Is(err, ErrBadWebhook) {
		t.Fatal("fake must reject real-key signature")
	}
}
