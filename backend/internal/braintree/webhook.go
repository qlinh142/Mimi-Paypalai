package braintree

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Webhook Braintree: Braintree POST form bt_signature + bt_payload. bt_payload là XML mã
// base64; bt_signature là các cặp "public_key|hmac" nối bằng "&", hmac là HMAC-SHA1 của
// payload với khóa SHA1(private key). Mimi chỉ tin payload khi chữ ký khớp public key của
// mình, nên kẻ khác không giả được thông báo settle hay khiếu nại.

// ErrBadWebhook: chữ ký sai hoặc payload không đọc được.
var ErrBadWebhook = errors.New("braintree: invalid webhook")

// Các kind Mimi xử lý; kind khác vẫn trả 200 để Braintree không gửi lại.
const (
	KindCheck                     = "check"
	KindTransactionSettled        = "transaction_settled"
	KindTransactionSettlementDecl = "transaction_settlement_declined"
	KindDisputeOpened             = "dispute_opened"
	KindDisputeWon                = "dispute_won"
	KindDisputeLost               = "dispute_lost"
	KindDisputeAccepted           = "dispute_accepted"
	KindDisputeAutoAccepted       = "dispute_auto_accepted"
	KindDisputeExpired            = "dispute_expired"
	maxWebhookPayload             = 64 << 10
)

// Notification là phần Mimi cần từ một webhook.
type Notification struct {
	Kind      string
	Timestamp time.Time
	TxnID     string // giao dịch liên quan (cả với khiếu nại)
	OrderID   string // orderId Mimi ghi lúc thu tiền (ID đơn Mimi)
	DisputeID string
}

type xmlTxn struct {
	ID      string `xml:"id"`
	OrderID string `xml:"order-id"`
}

type xmlNotification struct {
	Kind      string `xml:"kind"`
	Timestamp string `xml:"timestamp"`
	Subject   struct {
		Transaction *xmlTxn `xml:"transaction"`
		Dispute     *struct {
			ID          string `xml:"id"`
			Transaction xmlTxn `xml:"transaction"`
		} `xml:"dispute"`
	} `xml:"subject"`
}

// ParseWebhook kiểm chữ ký rồi đọc thông báo.
func (c *Client) ParseWebhook(signature, payload string) (Notification, error) {
	return parseWebhook(c.publicKey, c.privateKey, signature, payload)
}

func parseWebhook(publicKey, privateKey, signature, payload string) (Notification, error) {
	if payload == "" || len(payload) > maxWebhookPayload || !verifySignature(publicKey, privateKey, signature, payload) {
		return Notification{}, ErrBadWebhook
	}
	raw, err := base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, payload))
	if err != nil {
		return Notification{}, fmt.Errorf("%w: payload: %v", ErrBadWebhook, err)
	}
	var x xmlNotification
	if err := xml.Unmarshal(raw, &x); err != nil {
		return Notification{}, fmt.Errorf("%w: xml: %v", ErrBadWebhook, err)
	}
	n := Notification{Kind: strings.TrimSpace(x.Kind)}
	if n.Kind == "" {
		return Notification{}, fmt.Errorf("%w: kind missing", ErrBadWebhook)
	}
	if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(x.Timestamp)); err == nil {
		n.Timestamp = ts
	}
	switch {
	case x.Subject.Dispute != nil:
		n.DisputeID = x.Subject.Dispute.ID
		n.TxnID, n.OrderID = x.Subject.Dispute.Transaction.ID, x.Subject.Dispute.Transaction.OrderID
	case x.Subject.Transaction != nil:
		n.TxnID, n.OrderID = x.Subject.Transaction.ID, x.Subject.Transaction.OrderID
	}
	return n, nil
}

// verifySignature: thử cả payload có thêm "\n" như SDK chính thức của Braintree.
func verifySignature(publicKey, privateKey, signature, payload string) bool {
	if publicKey == "" || privateKey == "" {
		return false
	}
	key := sha1.Sum([]byte(privateKey))
	for _, pair := range strings.Split(signature, "&") {
		pk, sig, ok := strings.Cut(pair, "|")
		if !ok || pk != publicKey {
			continue
		}
		got, err := hex.DecodeString(strings.TrimSpace(sig))
		if err != nil {
			continue
		}
		for _, p := range []string{payload, payload + "\n"} {
			m := hmac.New(sha1.New, key[:])
			m.Write([]byte(p))
			if hmac.Equal(got, m.Sum(nil)) {
				return true
			}
		}
	}
	return false
}

// SignWebhook tạo cặp (signature, payload) như Braintree, cho test và bản giả lập.
func SignWebhook(publicKey, privateKey, xmlBody string) (signature, payload string) {
	payload = base64.StdEncoding.EncodeToString([]byte(xmlBody))
	key := sha1.Sum([]byte(privateKey))
	m := hmac.New(sha1.New, key[:])
	m.Write([]byte(payload))
	return publicKey + "|" + hex.EncodeToString(m.Sum(nil)), payload
}
