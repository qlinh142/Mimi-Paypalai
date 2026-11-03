package market

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Sổ nguồn mỗi nước (mẹ chốt 02/11/2026, xem docs/mimi15/luat-gia.md mục 4):
//   - Lưu link ngành hàng và mẫu link tìm kiếm của mỗi web, không lưu link từng sản phẩm
//     (hay đổi, nhiều bản trùng).
//   - Lần tra sau đưa các link đó cho AI đọc trước; thiếu mới tìm Google.
//   - Web mới chỉ vào sổ khi ra giá hợp lệ ít nhất MinHits lần.
//   - Trang đọc MaxMisses lần liền không ra giá thì đánh dấu, web đó quay về tìm Google.

const (
	MinHits   = 2
	MaxMisses = 2
	// QueryMark là chỗ điền tên món trong mẫu link tìm kiếm.
	QueryMark = "{q}"
	// RetryFlagged: web bị đánh dấu được thử đọc trang lại sau một tháng (web có thể đã đổi).
	RetryFlagged = 30 * 24 * time.Hour
)

// SourceRecord là một web trong sổ nguồn của một nước.
type SourceRecord struct {
	Country    string
	Domain     string
	Store      string
	Hits       int               // số lần ra giá hợp lệ
	Misses     int               // số lần liền được đưa trang mà không ra giá
	Flagged    bool              // đọc trang không ra giá: chỉ tìm Google cho web này
	FlaggedAt  time.Time         // sau RetryFlagged thì thử đọc trang lại
	SearchURL  string            // mẫu link tìm kiếm, có QueryMark
	Categories map[string]string // category_key -> link ngành hàng
	UpdatedAt  time.Time
}

// Active: web đã đủ tin để đưa link cho AI đọc.
func (r SourceRecord) Active() bool { return r.Hits >= MinHits && !r.Flagged }

// Page là một trang đưa cho AI đọc trước khi tìm Google.
type Page struct {
	Store  string
	Domain string
	URL    string
}

// SourceBook đọc và ghi sổ nguồn.
type SourceBook interface {
	Sources(ctx context.Context, country string) ([]SourceRecord, error)
	// Update đọc bản ghi của domain (chưa có thì rỗng), gọi apply rồi ghi lại, trong transaction.
	Update(ctx context.Context, country, domain string, apply func(SourceRecord) SourceRecord) error
}

// PagesFor chọn trang cho một nhóm món: web đang dùng được, ưu tiên link ngành hàng của
// ngành các món; không có thì dùng mẫu link tìm kiếm điền tên món đầu tiên chưa có ngành.
// Tối đa max trang, web nhiều lần ra giá đứng trước.
func PagesFor(recs []SourceRecord, items []Wanted, max int) []Page {
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].Hits > recs[j].Hits })
	var out []Page
	seen := map[string]bool{}
	add := func(r SourceRecord, url string) {
		if url == "" || seen[url] || len(out) >= max {
			return
		}
		seen[url] = true
		out = append(out, Page{Store: r.Store, Domain: r.Domain, URL: url})
	}
	for _, r := range recs {
		if !r.Active() {
			continue
		}
		covered := false
		for _, w := range items {
			if u := r.Categories[w.Category]; u != "" && w.Category != "" {
				add(r, u)
				covered = true
			}
		}
		if !covered && strings.Contains(r.SearchURL, QueryMark) {
			for _, w := range items {
				add(r, strings.ReplaceAll(r.SearchURL, QueryMark, urlQuery(w.Name)))
			}
		}
	}
	return out
}

func urlQuery(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r == ' ':
			b.WriteByte('+')
		case r < 0x80 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.~", r)):
			b.WriteRune(r)
		default:
			for _, c := range []byte(string(r)) {
				b.WriteString("%")
				b.WriteString(strings.ToUpper(hex2(c)))
			}
		}
	}
	return b.String()
}

func hex2(c byte) string {
	const h = "0123456789abcdef"
	return string([]byte{h[c>>4], h[c&15]})
}

// Found là một trang AI báo đã dùng (để học link ngành hàng, link tìm kiếm).
type Found struct {
	Store     string
	URL       string // trang ngành hàng hoặc trang tìm kiếm
	Kind      string // category | search
	Category  string // với Kind=category
	SearchURL string // với Kind=search: mẫu có QueryMark
}

// Learn cập nhật sổ nguồn sau một lần tra: domain ra giá thì cộng Hits, xóa Misses; trang đã
// đưa mà domain không ra giá thì cộng Misses, đủ MaxMisses thì đánh dấu; AI báo trang mới
// thì ghi link ngành hàng hoặc mẫu tìm kiếm.
func Learn(ctx context.Context, book SourceBook, country string, given []Page, found []Found, prices map[string][]Price, now time.Time) error {
	priced := map[string]string{} // domain -> tên cửa hàng
	for _, ps := range prices {
		for _, p := range ps {
			if d := Domain(p.URL); d != "" {
				priced[d] = p.Store
			}
		}
	}
	gave := map[string]bool{}
	for _, g := range given {
		gave[g.Domain] = true
	}
	missed := map[string]bool{}
	for _, g := range given {
		if _, ok := priced[g.Domain]; !ok {
			missed[g.Domain] = true
		}
	}
	learned := map[string][]Found{}
	for _, f := range found {
		if d := Domain(f.URL + f.SearchURL); d != "" {
			learned[d] = append(learned[d], f)
		}
	}
	domains := map[string]bool{}
	for d := range priced {
		domains[d] = true
	}
	for d := range missed {
		domains[d] = true
	}
	for d := range learned {
		domains[d] = true
	}
	for d := range domains {
		err := book.Update(ctx, country, d, func(r SourceRecord) SourceRecord {
			r.Country, r.Domain, r.UpdatedAt = country, d, now
			if store, ok := priced[d]; ok {
				r.Hits++
				if gave[d] {
					r.Misses = 0 // đọc trang ra giá
				}
				if r.Flagged && now.Sub(r.FlaggedAt) >= RetryFlagged {
					r.Flagged, r.Misses = false, 0
				}
				if r.Store == "" {
					r.Store = store
				}
			} else if missed[d] {
				r.Misses++
				if r.Misses >= MaxMisses && !r.Flagged {
					r.Flagged, r.FlaggedAt = true, now
				}
			}
			for _, f := range learned[d] {
				switch {
				case f.Kind == "category" && f.Category != "" && Domain(f.URL) == d:
					if r.Categories == nil {
						r.Categories = map[string]string{}
					}
					r.Categories[f.Category] = f.URL
				case f.Kind == "search" && strings.Contains(f.SearchURL, QueryMark) && Domain(f.SearchURL) == d:
					r.SearchURL = f.SearchURL
				}
				if r.Store == "" {
					r.Store = f.Store
				}
			}
			return r
		})
		if err != nil {
			return err
		}
	}
	return nil
}
