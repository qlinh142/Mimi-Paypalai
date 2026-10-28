package paypal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Fake giả lập PayPal cho chạy ở máy và demo khi chưa có tài khoản Sandbox
// (PAYPAL_MODE=fake). Không giữ trạng thái: mọi ID suy ra từ ref nên nhiều instance
// Cloud Run và gọi lại đều cho cùng kết quả. Order coi như đã được approve; không có tiền thật.
type Fake struct{}

func NewFake() *Fake { return &Fake{} }

func (Fake) CreateOrder(_ context.Context, ref, amountUSD, _ string) (Order, error) {
	// Số tiền nằm trong ID để Capture trả lại đúng số mà không cần lưu.
	return Order{ID: "FAKE-" + amountUSD + "-" + short(ref), Status: "CREATED"}, nil
}

func (Fake) CaptureOrder(_ context.Context, paypalOrderID, _ string) (Capture, error) {
	rest, ok := strings.CutPrefix(paypalOrderID, "FAKE-")
	amount, sum, ok2 := strings.Cut(rest, "-")
	if !ok || !ok2 || amount == "" {
		return Capture{}, ErrDeclined
	}
	return Capture{ID: "FAKECAP-" + sum, Status: "COMPLETED", Amount: amount}, nil
}

func (Fake) RefundCapture(_ context.Context, captureID, _, ref, _ string) (Refund, error) {
	if !strings.HasPrefix(captureID, "FAKECAP-") {
		return Refund{}, ErrDeclined
	}
	return Refund{ID: "FAKEREF-" + short(ref), Status: "COMPLETED"}, nil
}

func short(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}
