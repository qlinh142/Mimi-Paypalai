// Package braintree là client tối thiểu cho Braintree GraphQL API, dùng cho lựa chọn
// "Trả bằng thẻ" của Mimi mua online: tạo client token cho Drop-in UI ở app, thu tiền
// bằng nonce Drop-in trả về, hoàn một phần hoặc toàn bộ. Chỉ dùng thư viện chuẩn.
//
// Số thẻ không đi qua server Mimi: Drop-in gửi thẳng cho Braintree và chỉ trả về nonce
// dùng một lần.
package braintree

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	SandboxURL = "https://payments.sandbox.braintree-api.com/graphql"
	LiveURL    = "https://payments.braintree-api.com/graphql"
	// Version là phiên bản schema GraphQL Braintree yêu cầu ở mỗi request.
	Version = "2019-01-01"
)

var (
	// ErrDeclined: Braintree từ chối (thẻ bị từ chối, nonce sai, giao dịch không hoàn được).
	ErrDeclined = errors.New("braintree: declined")
	// ErrNotSettled: giao dịch chưa quyết toán nên chưa hoàn một phần được.
	ErrNotSettled = errors.New("braintree: transaction not settled")
	// ErrUnavailable: lỗi mạng, 5xx hoặc lỗi nội bộ của Braintree, thử lại được.
	ErrUnavailable = errors.New("braintree: unavailable")
)

// Transaction là kết quả thu tiền.
type Transaction struct {
	ID     string
	Status string // SUBMITTED_FOR_SETTLEMENT | SETTLING | SETTLED | AUTHORIZED | ...
	Amount string // USD, "12.34"
}

type Refund struct {
	ID     string
	Status string
}

// Paid: các trạng thái coi là đã thu tiền thành công.
func Paid(status string) bool {
	switch status {
	case "SUBMITTED_FOR_SETTLEMENT", "SETTLING", "SETTLED", "SETTLEMENT_PENDING":
		return true
	}
	return false
}

type Client struct {
	url        string
	publicKey  string
	privateKey string
	merchantAc string // merchant account USD; rỗng: dùng tài khoản mặc định
	http       *http.Client
}

func New(url, publicKey, privateKey, merchantAccountID string) (*Client, error) {
	if publicKey == "" || privateKey == "" {
		return nil, errors.New("braintree: public and private keys are required")
	}
	if url == "" {
		url = SandboxURL
	}
	return &Client{url: url, publicKey: publicKey, privateKey: privateKey, merchantAc: merchantAccountID,
		http: &http.Client{Timeout: 20 * time.Second}}, nil
}

// ClientToken tạo token cho Drop-in UI ở app.
func (c *Client) ClientToken(ctx context.Context) (string, error) {
	input := map[string]any{}
	if c.merchantAc != "" {
		input["clientToken"] = map[string]any{"merchantAccountId": c.merchantAc}
	}
	var out struct {
		CreateClientToken struct {
			ClientToken string `json:"clientToken"`
		} `json:"createClientToken"`
	}
	q := `mutation($input: CreateClientTokenInput) { createClientToken(input: $input) { clientToken } }`
	if err := c.do(ctx, q, map[string]any{"input": input}, &out); err != nil {
		return "", err
	}
	if out.CreateClientToken.ClientToken == "" {
		return "", fmt.Errorf("%w: empty client token", ErrUnavailable)
	}
	return out.CreateClientToken.ClientToken, nil
}

// Charge thu tiền bằng nonce (authorize và submit for settlement trong một bước).
// ref là ID đơn của Mimi, ghi vào orderId để đối soát trên Braintree.
func (c *Client) Charge(ctx context.Context, nonce, amountUSD, ref string) (Transaction, error) {
	txn := map[string]any{"amount": amountUSD, "orderId": ref}
	if c.merchantAc != "" {
		txn["merchantAccountId"] = c.merchantAc
	}
	var out struct {
		ChargePaymentMethod struct {
			Transaction *struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Amount struct {
					Value string `json:"value"`
				} `json:"amount"`
			} `json:"transaction"`
		} `json:"chargePaymentMethod"`
	}
	q := `mutation($input: ChargePaymentMethodInput!) { chargePaymentMethod(input: $input) { transaction { id status amount { value } } } }`
	if err := c.do(ctx, q, map[string]any{"input": map[string]any{"paymentMethodId": nonce, "transaction": txn}}, &out); err != nil {
		return Transaction{}, err
	}
	t := out.ChargePaymentMethod.Transaction
	if t == nil {
		return Transaction{}, fmt.Errorf("%w: transaction missing", ErrDeclined)
	}
	return Transaction{ID: t.ID, Status: t.Status, Amount: t.Amount.Value}, nil
}

// Refund hoàn amountUSD của giao dịch. Giao dịch chưa quyết toán chỉ hủy được toàn bộ:
// full=true thì Mimi hủy (reverse), còn hoàn một phần trả ErrNotSettled.
func (c *Client) Refund(ctx context.Context, txnID, amountUSD, ref string, full bool) (Refund, error) {
	var out struct {
		RefundTransaction struct {
			Refund struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"refund"`
		} `json:"refundTransaction"`
	}
	q := `mutation($input: RefundTransactionInput!) { refundTransaction(input: $input) { refund { id status } } }`
	in := map[string]any{"transactionId": txnID, "refund": map[string]any{"amount": amountUSD, "orderId": ref}}
	err := c.do(ctx, q, map[string]any{"input": in}, &out)
	if err == nil {
		return Refund{ID: out.RefundTransaction.Refund.ID, Status: out.RefundTransaction.Refund.Status}, nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "settled") {
		return Refund{}, err
	}
	if !full {
		return Refund{}, fmt.Errorf("%w: %v", ErrNotSettled, err)
	}
	var rev struct {
		ReverseTransaction struct {
			Reversal struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"reversal"`
		} `json:"reverseTransaction"`
	}
	rq := `mutation($input: ReverseTransactionInput!) { reverseTransaction(input: $input) { reversal { ... on Transaction { id status } ... on Refund { id status } } } }`
	if err := c.do(ctx, rq, map[string]any{"input": map[string]any{"transactionId": txnID}}, &rev); err != nil {
		return Refund{}, err
	}
	return Refund{ID: rev.ReverseTransaction.Reversal.ID, Status: rev.ReverseTransaction.Reversal.Status}, nil
}

func (c *Client) do(ctx context.Context, query string, vars map[string]any, out any) error {
	b, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.publicKey, c.privateKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Braintree-Version", Version)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return fmt.Errorf("%w: status %d", ErrUnavailable, res.StatusCode)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrDeclined, res.StatusCode)
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				ErrorClass string `json:"errorClass"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrUnavailable, err)
	}
	if len(env.Errors) > 0 {
		e := env.Errors[0]
		kind := ErrDeclined
		switch e.Extensions.ErrorClass {
		case "INTERNAL", "SERVICE_AVAILABILITY", "RESOURCE_LIMIT", "UNKNOWN":
			kind = ErrUnavailable
		}
		return fmt.Errorf("%w: %s: %s", kind, e.Extensions.ErrorClass, e.Message)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("%w: decode data: %v", ErrUnavailable, err)
	}
	return nil
}
