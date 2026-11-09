// Package config đọc cấu hình từ biến môi trường (12-factor). Mọi lỗi cấu hình được
// báo một lần, ngay lúc khởi động, thay vì nổ ra giữa chừng khi đang phục vụ request.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port      string
	ProjectID string
	Region    string
	Env       string // dev | prod

	FirestoreDB      string
	GeminiModel      string
	GeminiLocation   string
	GeminiThinking   string // MINIMAL | LOW | MEDIUM | HIGH; rỗng: để model tự chọn
	AppCheckRequired bool

	ReceiptDailyLimit int
	PriceSyncLimit    int           // số món tối đa job price-sync tra mỗi đêm (ADR-004), PRICE_SYNC_LIMIT
	AIDailyLimit      int           // số lượt "Hỏi Mimi" mỗi người mỗi ngày (AI_DAILY_LIMIT); 0: tắt Hỏi Mimi
	ReadTimeout       time.Duration // ngân sách cho POST /receipts/read, dưới giới hạn 60 s của Hosting rewrite
	DefaultTimeout    time.Duration

	TasksQueue   string // projects/.../locations/.../queues/notify
	WorkerURL    string // URL nội bộ của mimi-worker
	TasksSAEmail string // danh tính OIDC khi Cloud Tasks gọi worker
	EmailAPIKey  string // gắn từ Secret Manager, chỉ worker và job cần
	EmailFrom    string
	TaxonomyKeys []string // nạp từ file nhúng trong binary, không từ env

	// Channel3: nguồn giá đồ dùng nhà, thuốc, đồ trẻ em ở Mỹ (secret channel3-api-key). Rỗng: tắt.
	Channel3Key string

	// PayPal (Mimi 2.0 mục 15). PAYPAL_MODE: sandbox | live | fake | off.
	// fake: giả lập trong bộ nhớ để demo khi chưa có tài khoản Sandbox.
	PayPalMode     string
	PayPalClientID string
	PayPalSecret   string // gắn từ Secret Manager
	VNDPerUSD      int64  // PayPal không nhận VND: tỷ giá quy đổi cho đơn online
	// Nguồn và ngày của tỷ giá (vấn đề 44): lưu kèm mỗi đơn để sau này biết đơn được quy đổi
	// theo tỷ giá nào. PAYPAL_RATE_SOURCE, PAYPAL_RATE_AS_OF (YYYY-MM-DD).
	VNDRateSource string
	VNDRateAsOf   string

	// Braintree: lựa chọn "Trả bằng thẻ". BRAINTREE_MODE: sandbox | live | fake | off.
	BraintreeMode       string
	BraintreePublicKey  string
	BraintreePrivateKey string // gắn từ Secret Manager
	BraintreeMerchantAc string // merchant account USD; rỗng: tài khoản mặc định
}

func Load() (Config, error) {
	c := Config{
		Port:                get("PORT", "8080"),
		ProjectID:           os.Getenv("GOOGLE_CLOUD_PROJECT"),
		Region:              get("GOOGLE_CLOUD_REGION", "asia-southeast1"),
		Env:                 get("MIMI_ENV", "dev"),
		FirestoreDB:         get("FIRESTORE_DATABASE", "(default)"),
		GeminiModel:         os.Getenv("GEMINI_MODEL"),
		GeminiThinking:      get("GEMINI_THINKING", "LOW"),    // chép chữ trên bill không cần nghĩ lâu; nghĩ ít thì trả lời nhanh hơn
		GeminiLocation:      get("GEMINI_LOCATION", "global"), // checklist 48h Gate 1: Vertex AI gọi endpoint global
		AppCheckRequired:    get("APP_CHECK_REQUIRED", "true") == "true",
		TasksQueue:          os.Getenv("TASKS_QUEUE"),
		WorkerURL:           os.Getenv("WORKER_URL"),
		TasksSAEmail:        os.Getenv("TASKS_SA_EMAIL"),
		EmailAPIKey:         os.Getenv("EMAIL_API_KEY"),
		EmailFrom:           get("EMAIL_FROM", "Mimi <hello@mimi.app>"),
		ReadTimeout:         55 * time.Second,
		DefaultTimeout:      10 * time.Second,
		ReceiptDailyLimit:   20,
		PriceSyncLimit:      200,
		AIDailyLimit:        300, // một câu hỏi thường tốn 2 đến 6 lượt (gọi tool rồi trả lời)
		PayPalMode:          get("PAYPAL_MODE", "fake"),
		PayPalClientID:      strings.TrimSpace(os.Getenv("PAYPAL_CLIENT_ID")),
		PayPalSecret:        strings.TrimSpace(os.Getenv("PAYPAL_CLIENT_SECRET")), // secret cất bằng --data-file=- thường dính dấu xuống dòng
		VNDPerUSD:           25_000,
		VNDRateSource:       get("PAYPAL_RATE_SOURCE", "mac-dinh-cau-hinh"),
		VNDRateAsOf:         os.Getenv("PAYPAL_RATE_AS_OF"),
		BraintreeMode:       get("BRAINTREE_MODE", "off"),
		BraintreePublicKey:  strings.TrimSpace(os.Getenv("BRAINTREE_PUBLIC_KEY")),
		BraintreePrivateKey: strings.TrimSpace(os.Getenv("BRAINTREE_PRIVATE_KEY")),
		BraintreeMerchantAc: strings.TrimSpace(os.Getenv("BRAINTREE_MERCHANT_ACCOUNT_ID")),
		Channel3Key:         strings.TrimSpace(os.Getenv("CHANNEL3_API_KEY")),
	}
	var errs []error
	if c.ProjectID == "" {
		errs = append(errs, errors.New("GOOGLE_CLOUD_PROJECT is required"))
	}
	if v := os.Getenv("RECEIPT_DAILY_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			errs = append(errs, fmt.Errorf("RECEIPT_DAILY_LIMIT invalid: %q", v))
		}
		c.ReceiptDailyLimit = n
	}
	if v := os.Getenv("PRICE_SYNC_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 2000 {
			errs = append(errs, fmt.Errorf("PRICE_SYNC_LIMIT invalid: %q", v))
		}
		c.PriceSyncLimit = n
	}
	if v := os.Getenv("AI_DAILY_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 5000 {
			errs = append(errs, fmt.Errorf("AI_DAILY_LIMIT invalid: %q", v))
		}
		c.AIDailyLimit = n
	}
	if v := os.Getenv("PAYPAL_VND_PER_USD"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1_000 || n > 1_000_000 {
			errs = append(errs, fmt.Errorf("PAYPAL_VND_PER_USD invalid: %q", v))
		}
		c.VNDPerUSD = n
	}
	if c.VNDRateAsOf != "" {
		if _, err := time.Parse("2006-01-02", c.VNDRateAsOf); err != nil {
			errs = append(errs, fmt.Errorf("PAYPAL_RATE_AS_OF invalid: %q (YYYY-MM-DD)", c.VNDRateAsOf))
		}
	}
	// Tiền thật thì không được chạy bằng tỷ giá mặc định trong code.
	if c.PayPalMode == "live" && (os.Getenv("PAYPAL_VND_PER_USD") == "" || c.VNDRateAsOf == "") {
		errs = append(errs, errors.New("PAYPAL_VND_PER_USD and PAYPAL_RATE_AS_OF are required for PAYPAL_MODE=live"))
	}
	switch c.PayPalMode {
	case "fake", "off":
	case "sandbox", "live":
		if c.PayPalClientID == "" || c.PayPalSecret == "" {
			errs = append(errs, errors.New("PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are required for PAYPAL_MODE="+c.PayPalMode))
		}
	default:
		errs = append(errs, fmt.Errorf("PAYPAL_MODE invalid: %q (sandbox | live | fake | off)", c.PayPalMode))
	}
	switch c.BraintreeMode {
	case "fake", "off":
	case "sandbox", "live":
		if c.BraintreePublicKey == "" || c.BraintreePrivateKey == "" {
			errs = append(errs, errors.New("BRAINTREE_PUBLIC_KEY and BRAINTREE_PRIVATE_KEY are required for BRAINTREE_MODE="+c.BraintreeMode))
		}
	default:
		errs = append(errs, fmt.Errorf("BRAINTREE_MODE invalid: %q (sandbox | live | fake | off)", c.BraintreeMode))
	}
	if c.Env == "prod" && !c.AppCheckRequired {
		errs = append(errs, errors.New("APP_CHECK_REQUIRED must be true in prod"))
	}
	return c, errors.Join(errs...)
}

// RequireAPI kiểm tra các biến chỉ mimi-api cần.
func (c Config) RequireAPI() error {
	var errs []error
	if c.GeminiModel == "" {
		errs = append(errs, errors.New("GEMINI_MODEL is required"))
	}
	// Cloud Tasks nằm ngoài phạm vi demo (checklist 48h): API chưa đẩy task nào nên không
	// bắt buộc TASKS_QUEUE, WORKER_URL, TASKS_SA_EMAIL.
	return errors.Join(errs...)
}

// RequireWorker kiểm tra các biến worker và job gửi email cần.
func (c Config) RequireWorker() error {
	if c.EmailAPIKey == "" {
		return errors.New("EMAIL_API_KEY is required (mount from Secret Manager)")
	}
	return nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
