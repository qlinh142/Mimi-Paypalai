// Package paypal là client tối thiểu cho PayPal REST API (Orders v2, Payments v2) dùng
// trong luồng Mimi mua online (Mimi 2.0 mục 15, 18): tạo order, capture sau khi người
// dùng approve, refund một phần hoặc toàn bộ. Chỉ dùng thư viện chuẩn.
//
// Phạm vi hackathon: tài khoản Sandbox và merchant do team kiểm soát. Không có thanh
// toán off-session (vaulting) và không theo dõi vận chuyển thật.
package paypal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	SandboxBaseURL = "https://api-m.sandbox.paypal.com"
	LiveBaseURL    = "https://api-m.paypal.com"
)

// Currency: PayPal không hỗ trợ VND, đơn được tính bằng USD theo tỷ giá cấu hình.
const Currency = "USD"

var (
	// ErrDeclined: PayPal trả lỗi nghiệp vụ (order chưa approve, thẻ bị từ chối...).
	ErrDeclined = errors.New("paypal: declined")
	// ErrUnavailable: lỗi mạng hoặc 5xx, có thể thử lại.
	ErrUnavailable = errors.New("paypal: unavailable")
)

// Client gọi PayPal bằng OAuth2 client credentials; access token được cache tới gần hết hạn.
type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	http         *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

func New(baseURL, clientID, clientSecret string) (*Client, error) {
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("paypal: client id and secret are required")
	}
	if baseURL == "" {
		baseURL = SandboxBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), clientID: clientID, clientSecret: clientSecret,
		http: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

// Order là kết quả tạo order: ID dùng cho nút PayPal ở client, ApproveURL cho chuyển hướng.
type Order struct {
	ID         string
	Status     string
	ApproveURL string
}

type Capture struct {
	ID     string
	Status string // COMPLETED | PENDING | DECLINED...
	Amount string // USD, "12.34"
}

type Refund struct {
	ID     string
	Status string // COMPLETED | PENDING...
}

// CreateOrder tạo order intent CAPTURE. ref là ID đơn của Mimi, dùng làm custom_id và
// PayPal-Request-Id để gọi lại không tạo order trùng.
func (c *Client) CreateOrder(ctx context.Context, ref, amountUSD, description string) (Order, error) {
	body := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{{
			"reference_id": ref, "custom_id": ref, "description": truncate(description, 127),
			"amount": map[string]any{"currency_code": Currency, "value": amountUSD},
		}},
		"payment_source": map[string]any{"paypal": map[string]any{"experience_context": map[string]any{
			"shipping_preference": "NO_SHIPPING", "user_action": "PAY_NOW", "brand_name": "Mimi",
		}}},
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Links  []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if err := c.do(ctx, http.MethodPost, "/v2/checkout/orders", "create-"+ref, body, &out); err != nil {
		return Order{}, err
	}
	o := Order{ID: out.ID, Status: out.Status}
	for _, l := range out.Links {
		if l.Rel == "approve" || l.Rel == "payer-action" {
			o.ApproveURL = l.Href
		}
	}
	return o, nil
}

// CaptureOrder thu tiền sau khi người dùng approve trên PayPal.
func (c *Client) CaptureOrder(ctx context.Context, paypalOrderID, ref string) (Capture, error) {
	var out struct {
		Status        string `json:"status"`
		PurchaseUnits []struct {
			Payments struct {
				Captures []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Amount struct {
						Value string `json:"value"`
					} `json:"amount"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
	}
	path := "/v2/checkout/orders/" + url.PathEscape(paypalOrderID) + "/capture"
	if err := c.do(ctx, http.MethodPost, path, "capture-"+ref, map[string]any{}, &out); err != nil {
		return Capture{}, err
	}
	for _, pu := range out.PurchaseUnits {
		for _, cp := range pu.Payments.Captures {
			return Capture{ID: cp.ID, Status: cp.Status, Amount: cp.Amount.Value}, nil
		}
	}
	return Capture{}, fmt.Errorf("%w: capture missing in response (order status %s)", ErrDeclined, out.Status)
}

// RefundCapture hoàn một phần hoặc toàn bộ số tiền đã capture. ref phân biệt các lần
// refund của cùng một đơn để gọi lại không hoàn hai lần.
func (c *Client) RefundCapture(ctx context.Context, captureID, amountUSD, ref, note string) (Refund, error) {
	body := map[string]any{"amount": map[string]any{"currency_code": Currency, "value": amountUSD}}
	// PayPal từ chối note_to_payer rỗng (400 INVALID_REQUEST): chỉ gửi khi có ghi chú.
	if n := strings.TrimSpace(note); n != "" {
		body["note_to_payer"] = truncate(n, 255)
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	path := "/v2/payments/captures/" + url.PathEscape(captureID) + "/refund"
	if err := c.do(ctx, http.MethodPost, path, "refund-"+ref, body, &out); err != nil {
		return Refund{}, err
	}
	return Refund{ID: out.ID, Status: out.Status}, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/oauth2/token",
		strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token: %v", ErrUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: token status %d", classify(res.StatusCode), res.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("%w: token decode: %v", ErrUnavailable, err)
	}
	c.token = out.AccessToken
	// Làm mới sớm một phút để không dùng token vừa hết hạn giữa chừng.
	c.expires = time.Now().Add(time.Duration(out.ExpiresIn)*time.Second - time.Minute)
	return c.token, nil
}

func (c *Client) do(ctx context.Context, method, path, requestID string, body, out any) error {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	req.Header.Set("PayPal-Request-Id", requestID)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		var pe struct {
			Name    string `json:"name"`
			Details []struct {
				Issue string `json:"issue"`
			} `json:"details"`
		}
		_ = json.Unmarshal(raw, &pe)
		issue := pe.Name
		if len(pe.Details) > 0 {
			issue += "/" + pe.Details[0].Issue
		}
		return fmt.Errorf("%w: %s %s: %d %s", classify(res.StatusCode), method, path, res.StatusCode, issue)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrUnavailable, err)
	}
	return nil
}

func classify(status int) error {
	if status == http.StatusTooManyRequests || status >= 500 {
		return ErrUnavailable
	}
	return ErrDeclined
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
