// Package channel3 là nguồn giá Channel3 (danh mục sản phẩm, giá ở từng cửa hàng, link mua).
//
// Luật dùng (bản rà soát 03/11/2026, notes/channel3-ra-soat.md):
//   - Chỉ thị trường Mỹ (Channel3 không có Việt Nam, không có VND).
//   - Chỉ đồ dùng nhà, thuốc không kê đơn, đồ trẻ em; đồ ăn vẫn tra bằng Gemini (thử 16 món đồ ăn trượt gần hết).
//   - Chỉ nhận giá ở các cửa hàng lớn đã thử đúng (Walmart, Target, Costco, CVS, Walgreens...).
//   - Channel3 chỉ là danh mục: thanh toán vẫn qua PayPal hoặc thẻ (Braintree).
package channel3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/market"
)

// DefaultBaseURL: API search của Channel3 (header x-api-key).
const DefaultBaseURL = "https://api.trychannel3.com/v1"

// Categories: ngành hàng (category_key) được tra Channel3.
var Categories = map[string]bool{
	"me-be": true, "thuoc": true, "hoa-pham": true, "ve-sinh": true, "giay": true,
	"giat-say": true, "my-pham": true, "nha-cua": true, "do-dung-bep": true, "thu-cung": true,
}

// Countries: thị trường Channel3 có và Mimi đang mở.
var Countries = map[string]bool{"US": true}

// Stores: domain cửa hàng nhận giá -> tên hiện cho người dùng.
var Stores = map[string]string{
	"walmart.com": "Walmart", "target.com": "Target", "costco.com": "Costco",
	"cvs.com": "CVS", "walgreens.com": "Walgreens", "kroger.com": "Kroger",
	"samsclub.com": "Sam's Club", "riteaid.com": "Rite Aid",
}

// Client gọi API Channel3. Mỗi món một lượt search (1 credit).
type Client struct {
	Key     string
	BaseURL string
	HTTP    *http.Client
	Limit   int // số sản phẩm mỗi lượt tra, mặc định 10
}

// New tạo client; key rỗng thì trả nil (không bật nguồn này).
func New(key string) *Client {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	return &Client{Key: key, BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 15 * time.Second}, Limit: 10}
}

func (c *Client) Name() string { return "channel3" }

// Additive: giá Channel3 hiện cạnh giá các nguồn khác, không chỉ khi nguồn trước trượt.
func (c *Client) Additive() bool { return true }

// Wants: món này có được tra Channel3 không.
func Wants(country market.Country, w market.Wanted) bool {
	return Countries[country.Code] && Categories[w.Category] && strings.TrimSpace(w.Name) != ""
}

type searchResp struct {
	Products []product `json:"products"`
}

type product struct {
	Title  string  `json:"title"`
	Offers []offer `json:"offers"`
}

type offer struct {
	URL    string `json:"url"`
	Domain string `json:"domain"`
	Price  struct {
		Price    float64 `json:"price"`
		Currency string  `json:"currency"`
	} `json:"price"`
	Availability string `json:"availability"`
}

// Lookup tra từng món hợp luật, song song tối đa 3 lượt.
func (c *Client) Lookup(ctx context.Context, country market.Country, items []market.Wanted) (map[string][]market.Price, error) {
	out := map[string][]market.Price{}
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, 3)
		firstErr error
		asked    int
	)
	for _, w := range items {
		if !Wants(country, w) {
			continue
		}
		asked++
		wg.Add(1)
		sem <- struct{}{}
		go func(w market.Wanted) {
			defer func() { <-sem; wg.Done() }()
			ps, err := c.search(ctx, country, w.Name)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			if len(ps) > 0 {
				out[w.Key] = ps
			}
		}(w)
	}
	wg.Wait()
	if asked > 0 && len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func (c *Client) search(ctx context.Context, country market.Country, name string) ([]market.Price, error) {
	limit := c.Limit
	if limit <= 0 {
		limit = 10
	}
	body, _ := json.Marshal(map[string]any{"query": name, "limit": limit})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.Key)
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("channel3: search: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("channel3: search: HTTP %d", res.StatusCode)
	}
	var r searchResp
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("channel3: decode: %w", err)
	}
	return Pick(r.Products, name, country.Currency), nil
}

// Pick lọc kết quả: tên sản phẩm phải chứa từ chính của tên món, cửa hàng lớn đã thử đúng,
// đúng tiền tệ, không hết hàng. Mỗi cửa hàng giữ giá thấp nhất (market.Normalize gộp tiếp).
func Pick(ps []product, name, currency string) []market.Price {
	words := keywords(name)
	var out []market.Price
	for _, p := range ps {
		if !relevant(p.Title, words) {
			continue
		}
		for _, o := range p.Offers {
			store := Stores[domainOf(o.Domain)]
			if store == "" || o.Price.Price <= 0 || !strings.EqualFold(o.Price.Currency, currency) {
				continue
			}
			if a := strings.ToLower(o.Availability); a == "soldout" || a == "outofstock" {
				continue
			}
			out = append(out, market.Price{Store: store, Local: o.Price.Price, Currency: currency, Size: sizeOf(p.Title), URL: o.URL})
		}
	}
	return out
}

var stop = map[string]bool{"the": true, "and": true, "for": true, "with": true, "pack": true, "count": true, "size": true}

func keywords(name string) []string {
	var ws []string
	for _, f := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	}) {
		if len([]rune(f)) >= 3 && !stop[f] {
			ws = append(ws, f)
		}
	}
	return ws
}

// relevant: tên sản phẩm chứa ít nhất một nửa số từ chính (tối thiểu 1).
func relevant(title string, words []string) bool {
	if len(words) == 0 {
		return false
	}
	t := strings.ToLower(title)
	hit := 0
	for _, w := range words {
		if strings.Contains(t, w) {
			hit++
		}
	}
	return hit*2 >= len(words) && hit > 0
}

func domainOf(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	return strings.TrimPrefix(d, "www.")
}

// sizeOf: quy cách ghi trong tên ("48 oz", "12 count") để người dùng tự so.
func sizeOf(title string) string {
	f := strings.Fields(title)
	for i := 1; i < len(f); i++ {
		u := strings.ToLower(strings.Trim(f[i], ",.()"))
		switch u {
		case "oz", "fl", "lb", "lbs", "ct", "count", "pack", "pk", "ml", "l", "g", "kg":
			if n := strings.Trim(f[i-1], ",.()"); n != "" && (n[0] >= '0' && n[0] <= '9') {
				if u == "fl" && i+1 < len(f) && strings.HasPrefix(strings.ToLower(f[i+1]), "oz") {
					u = "fl oz"
				}
				return n + " " + u
			}
		}
	}
	return ""
}
