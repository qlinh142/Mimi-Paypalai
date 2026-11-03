// Package fx là job fx-sync hằng ngày (ADR-001): lấy tỷ giá có nguồn, ghi fx_rates để quy
// đổi hóa đơn ngoại tệ sang đồng. Tỷ giá lưu dạng chuỗi thập phân "số đồng cho 1 đơn vị
// ngoại tệ"; domain.ConvertToVND tính chính xác bằng số hữu tỉ.
package fx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Currencies: ngoại tệ hay gặp trên hóa đơn đi du lịch, mua hàng nước ngoài.
var Currencies = []string{"USD", "EUR", "JPY", "KRW", "CNY", "THB", "SGD", "GBP", "AUD", "TWD", "HKD", "MYR"}

// DisplayCurrencies: tiền tệ người dùng được chọn để xem (mẹ chốt 03/11: tạm VND và USD).
// Số tiền vẫn lưu bằng VND và tiền gốc của khoản; chỉ lúc hiển thị mới quy đổi.
var DisplayCurrencies = []string{"VND", "USD"}

// MaxHistoryDays: khoảng dài nhất GET /fx/rates trả một lần.
const MaxHistoryDays = 800

// Point là tỷ giá của một ngày, gửi cho app để quy đổi lúc hiển thị.
type Point struct {
	On        string `json:"on"`   // YYYY-MM-DD
	VNDPerOne string `json:"rate"` // số đồng cho 1 đơn vị ngoại tệ
}

// Rate là một dòng fx_rates.
type Rate struct {
	Currency  string
	On        domain.Date // ngày nguồn cập nhật, theo giờ Việt Nam
	VNDPerOne string      // số đồng cho 1 đơn vị ngoại tệ, thập phân
	Source    string
	FetchedAt time.Time
}

// Source trả bảng tỷ giá: mỗi đồng tiền bao nhiêu đơn vị trên 1 đơn vị đồng tiền gốc.
type Source interface {
	Latest(ctx context.Context) (Table, error)
}

type Table struct {
	Base      string
	Rates     map[string]json.Number
	UpdatedAt time.Time
	Name      string
}

type Store interface {
	SaveRates(ctx context.Context, rates []Rate) error
}

// Sync đọc bảng tỷ giá và ghi tỷ giá VND cho từng ngoại tệ. Thiếu VND trong bảng là lỗi;
// thiếu một ngoại tệ thì bỏ qua ngoại tệ đó.
func Sync(ctx context.Context, src Source, st Store, now time.Time) ([]Rate, error) {
	t, err := src.Latest(ctx)
	if err != nil {
		return nil, err
	}
	vnd, ok := rat(t.Rates["VND"])
	if !ok {
		return nil, errors.New("fx: bảng tỷ giá không có VND")
	}
	on := domain.Today(t.UpdatedAt, domain.Location("Asia/Ho_Chi_Minh"))
	var out []Rate
	for _, c := range Currencies {
		per := vnd
		if c != t.Base {
			r, ok := rat(t.Rates[c])
			if !ok {
				continue
			}
			per = new(big.Rat).Quo(vnd, r)
		}
		out = append(out, Rate{Currency: c, On: on, VNDPerOne: per.FloatString(4), Source: t.Name, FetchedAt: now})
	}
	if len(out) == 0 {
		return nil, errors.New("fx: không có ngoại tệ nào")
	}
	return out, st.SaveRates(ctx, out)
}

func rat(n json.Number) (*big.Rat, bool) {
	r, ok := new(big.Rat).SetString(string(n))
	return r, ok && r.Sign() > 0
}

// ERAPI đọc https://open.er-api.com (ExchangeRate-API bản miễn phí, không cần khóa, cập nhật
// mỗi ngày). Điều khoản yêu cầu ghi nguồn: Source lưu tên này cùng mỗi tỷ giá.
type ERAPI struct {
	HTTP *http.Client
	URL  string // để trống: https://open.er-api.com/v6/latest/USD
}

func (e ERAPI) Latest(ctx context.Context) (Table, error) {
	url := e.URL
	if url == "" {
		url = "https://open.er-api.com/v6/latest/USD"
	}
	c := e.HTTP
	if c == nil {
		c = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Table{}, err
	}
	res, err := c.Do(req)
	if err != nil {
		return Table{}, fmt.Errorf("fx: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Table{}, fmt.Errorf("fx: HTTP %d", res.StatusCode)
	}
	var body struct {
		Result     string                 `json:"result"`
		Base       string                 `json:"base_code"`
		UpdateUnix int64                  `json:"time_last_update_unix"`
		Rates      map[string]json.Number `json:"rates"`
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 1<<20))
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		return Table{}, fmt.Errorf("fx: %w", err)
	}
	if body.Result != "success" || body.Base == "" || body.UpdateUnix == 0 {
		return Table{}, fmt.Errorf("fx: phản hồi không hợp lệ (result=%q)", body.Result)
	}
	return Table{Base: body.Base, Rates: body.Rates, UpdatedAt: time.Unix(body.UpdateUnix, 0), Name: "open.er-api.com"}, nil
}
