package braintree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// FakeClientToken: app thấy token này thì hiện nút thẻ thử thay cho Drop-in thật.
const FakeClientToken = "fake-client-token"

// FakeNonce là nonce thẻ hợp lệ của Braintree Sandbox; Fake chỉ nhận nonce này.
const FakeNonce = "fake-valid-nonce"

// Fake giả lập Braintree khi chưa có tài khoản Sandbox (BRAINTREE_MODE=fake). Không giữ
// trạng thái: ID suy ra từ ref và số tiền nằm trong ID giao dịch.
type Fake struct{}

func NewFake() *Fake { return &Fake{} }

func (Fake) ClientToken(context.Context) (string, error) { return FakeClientToken, nil }

func (Fake) Charge(_ context.Context, nonce, amountUSD, ref string) (Transaction, error) {
	if nonce != FakeNonce {
		return Transaction{}, ErrDeclined
	}
	return Transaction{ID: "FAKEBT-" + amountUSD + "-" + short(ref), Status: "SUBMITTED_FOR_SETTLEMENT", Amount: amountUSD}, nil
}

func (Fake) Refund(_ context.Context, txnID, _, ref string, _ bool) (Refund, error) {
	if !strings.HasPrefix(txnID, "FAKEBT-") {
		return Refund{}, ErrDeclined
	}
	return Refund{ID: "FAKEBTREF-" + short(ref), Status: "SUBMITTED_FOR_SETTLEMENT"}, nil
}

func short(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// Khóa giả lập để ký webhook cho BRAINTREE_MODE=fake và test (không phải khóa thật).
const (
	FakePublicKey  = "fake-public-key"
	FakePrivateKey = "fake-private-key"
)

func (Fake) ParseWebhook(signature, payload string) (Notification, error) {
	return parseWebhook(FakePublicKey, FakePrivateKey, signature, payload)
}
