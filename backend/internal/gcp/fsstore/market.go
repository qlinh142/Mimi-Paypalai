package fsstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/market"
)

var (
	_ market.Store        = (*Store)(nil)
	_ market.RefreshStore = (*Store)(nil)
	_ market.RefreshQuota = (*Store)(nil)
	_ market.Demand       = (*Store)(nil)
	_ market.FX           = (*Store)(nil)
)

// Bảng giá chung (ADR-004): market_prices/{key} {name, group_key, prices[], updated_at}.
// Lịch sử theo ngày: market_prices/{key}/price_history/{YYYY-MM-DD}, giữ 30 ngày (mẹ chốt 02/11):
// expire_at để TTL policy của Firestore tự xóa (firestore.indexes.json).
// Không có trường nào trỏ tới người dùng.
func (s *Store) marketPrices() *firestore.CollectionRef { return s.c.Collection("market_prices") }

func (s *Store) FreshKeys(ctx context.Context, since time.Time) (map[string]bool, error) {
	it := s.marketPrices().Where("updated_at", ">=", since).Select().Documents(ctx)
	defer it.Stop()
	out := map[string]bool{}
	for {
		d, err := it.Next()
		if err == iterator.Done {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("market fresh: %w", err)
		}
		out[d.Ref.ID] = true
	}
}

// priceDoc là một giá như lưu trong Firestore. Phải có tag firestore: market.Price chỉ có
// tag json, và Firestore khớp tên trường không phân biệt hoa thường nên "store" vẫn đọc
// được nhưng "price_vnd" không khớp PriceVND, giá ra 0đ (lỗi mẹ thấy 02/11).
type priceDoc struct {
	Store    string  `firestore:"store"`
	PriceVND int64   `firestore:"price_vnd"`
	Local    float64 `firestore:"local"`
	Currency string  `firestore:"currency"`
	Size     string  `firestore:"size"`
	Source   string  `firestore:"source"`
	URL      string  `firestore:"url"`
	On       string  `firestore:"on"`
}

func (s *Store) SaveEntry(ctx context.Context, e market.Entry) error {
	prices := make([]map[string]any, 0, len(e.Prices))
	for _, p := range e.Prices {
		prices = append(prices, map[string]any{
			"store": p.Store, "price_vnd": p.PriceVND, "local": p.Local, "currency": p.Currency,
			"size": p.Size, "source": p.Source, "url": p.URL, "on": p.On,
		})
	}
	doc := map[string]any{"name": e.Name, "country": e.Country, "group_key": e.GroupKey, "prices": prices, "updated_at": e.UpdatedAt}
	ref := s.marketPrices().Doc(e.Key)
	b := s.c.Batch()
	b.Set(ref, doc)
	if len(prices) > 0 {
		b.Set(ref.Collection("price_history").Doc(e.UpdatedAt.Format("2006-01-02")), map[string]any{
			"prices": prices, "at": e.UpdatedAt, "expire_at": e.UpdatedAt.Add(market.HistoryRetention),
		})
	}
	if _, err := b.Commit(ctx); err != nil {
		return fmt.Errorf("market save %s: %w", e.Key, err)
	}
	return nil
}

// Touched trả lần tra gần nhất của từng món, kể cả món tra không ra giá.
func (s *Store) Touched(ctx context.Context, keys []string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	refs := s.marketRefs(keys)
	if len(refs) == 0 {
		return out, nil
	}
	snaps, err := s.c.GetAll(ctx, refs)
	if err != nil {
		return nil, fmt.Errorf("market touched: %w", err)
	}
	for _, d := range snaps {
		if !d.Exists() {
			continue
		}
		if t, ok := d.Data()["updated_at"].(time.Time); ok {
			out[d.Ref.ID] = t
		}
	}
	return out, nil
}

func (s *Store) marketRefs(keys []string) []*firestore.DocumentRef {
	refs := make([]*firestore.DocumentRef, 0, len(keys))
	seen := map[string]bool{}
	for _, k := range keys {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		refs = append(refs, s.marketPrices().Doc(k))
	}
	return refs
}

func (s *Store) Entries(ctx context.Context, keys []string) (map[string]market.Entry, error) {
	out := map[string]market.Entry{}
	refs := s.marketRefs(keys)
	if len(refs) == 0 {
		return out, nil
	}
	snaps, err := s.c.GetAll(ctx, refs)
	if err != nil {
		return nil, fmt.Errorf("market entries: %w", err)
	}
	for _, d := range snaps {
		if !d.Exists() {
			continue
		}
		var raw struct {
			Name      string     `firestore:"name"`
			Country   string     `firestore:"country"`
			GroupKey  string     `firestore:"group_key"`
			Prices    []priceDoc `firestore:"prices"`
			UpdatedAt time.Time  `firestore:"updated_at"`
		}
		if err := d.DataTo(&raw); err != nil {
			continue // dòng hỏng: bỏ qua, không làm hỏng cả Giỏ
		}
		if len(raw.Prices) == 0 {
			continue
		}
		prices := make([]market.Price, 0, len(raw.Prices))
		for _, p := range raw.Prices {
			prices = append(prices, market.Price(p))
		}
		out[d.Ref.ID] = market.Entry{Key: d.Ref.ID, Country: raw.Country, Name: raw.Name, GroupKey: raw.GroupKey, Prices: prices, UpdatedAt: raw.UpdatedAt}
	}
	return out, nil
}

// WantedItems đọc mọi dòng Kế hoạch đang mở của mọi nhà (Plan mặc định plan_items và
// plans/{id}/items), chỉ lấy tên, nhóm, ngành hàng và nước của chủ kế hoạch (để tra giá đúng
// nước, mẹ chốt 02/11). Truy vấn collection group không lọc nên không cần index; số dòng lớn
// thì đổi sang lọc status với index collection group.
func (s *Store) WantedItems(ctx context.Context) ([]market.Wanted, error) {
	var out []market.Wanted
	var uids []string // uids[i] là chủ dòng out[i]
	for _, group := range []string{"plan_items", "items"} {
		it := s.c.CollectionGroup(group).Select("name", "group_key", "status", "category_key").Documents(ctx)
		for {
			d, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				it.Stop()
				return nil, fmt.Errorf("market wanted %s: %w", group, err)
			}
			uid := planLineOwner(d.Ref, group)
			if uid == "" {
				continue // "items" cũng có thể là tên collection khác: chỉ nhận users/{uid}/plans/{id}/items
			}
			m := d.Data()
			if st, _ := m["status"].(string); st != "open" && st != "assigned" {
				continue
			}
			name, _ := m["name"].(string)
			gk, _ := m["group_key"].(string)
			cat, _ := m["category_key"].(string)
			if gk == "" {
				gk = d.Ref.ID // dòng plan_items dùng group_key làm ID
			}
			if name == "" {
				continue
			}
			uids = append(uids, uid)
			out = append(out, market.Wanted{Name: name, GroupKey: gk, Category: cat})
		}
		it.Stop()
	}
	countries, err := s.userCountries(ctx, uids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Country = countries[uids[i]]
		out[i].Key = market.ItemID(out[i].Country, out[i].Name)
	}
	return out, nil
}

// planLineOwner trả uid chủ dòng kế hoạch: users/{uid}/plan_items/{id} hoặc users/{uid}/plans/{p}/items/{id}.
func planLineOwner(ref *firestore.DocumentRef, group string) string {
	coll := ref.Parent
	if group == "items" {
		if coll.Parent == nil || coll.Parent.Parent == nil || coll.Parent.Parent.ID != "plans" {
			return ""
		}
		coll = coll.Parent.Parent
	}
	if coll.Parent == nil || coll.Parent.Parent == nil || coll.Parent.Parent.ID != "users" {
		return ""
	}
	return coll.Parent.ID
}

// userCountries đọc nước của từng chủ kế hoạch (Cài đặt, chưa chọn thì đoán theo múi giờ).
func (s *Store) userCountries(ctx context.Context, uids []string) (map[string]string, error) {
	out := map[string]string{}
	var refs []*firestore.DocumentRef
	for _, uid := range uids {
		if _, ok := out[uid]; !ok {
			out[uid] = market.DefaultCountry
			refs = append(refs, s.user(uid))
		}
	}
	for len(refs) > 0 {
		n := min(len(refs), 300)
		snaps, err := s.c.GetAll(ctx, refs[:n])
		if err != nil {
			return nil, fmt.Errorf("market wanted countries: %w", err)
		}
		for _, d := range snaps {
			if !d.Exists() {
				continue
			}
			st, _ := d.Data()["settings"].(map[string]any)
			c, _ := st["country"].(string)
			tz, _ := st["timezone"].(string)
			out[d.Ref.ID] = market.CountryFor(c, tz)
		}
		refs = refs[n:]
	}
	return out, nil
}

// ---------- Sổ nguồn: price_sources/{country}_{domain} ----------

var _ market.SourceBook = (*Store)(nil)

func (s *Store) priceSources() *firestore.CollectionRef { return s.c.Collection("price_sources") }

type sourceDoc struct {
	Country    string            `firestore:"country"`
	Domain     string            `firestore:"domain"`
	Store      string            `firestore:"store"`
	Hits       int64             `firestore:"hits"`
	Misses     int64             `firestore:"misses"`
	Flagged    bool              `firestore:"flagged"`
	FlaggedAt  time.Time         `firestore:"flagged_at"`
	SearchURL  string            `firestore:"search_url"`
	Categories map[string]string `firestore:"categories"`
	UpdatedAt  time.Time         `firestore:"updated_at"`
}

func (d sourceDoc) record() market.SourceRecord {
	return market.SourceRecord{Country: d.Country, Domain: d.Domain, Store: d.Store, Hits: int(d.Hits), Misses: int(d.Misses),
		Flagged: d.Flagged, FlaggedAt: d.FlaggedAt, SearchURL: d.SearchURL, Categories: d.Categories, UpdatedAt: d.UpdatedAt}
}

func (s *Store) Sources(ctx context.Context, country string) ([]market.SourceRecord, error) {
	it := s.priceSources().Where("country", "==", country).Documents(ctx)
	defer it.Stop()
	var out []market.SourceRecord
	for {
		d, err := it.Next()
		if err == iterator.Done {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("price sources: %w", err)
		}
		var sd sourceDoc
		if err := d.DataTo(&sd); err != nil {
			continue
		}
		out = append(out, sd.record())
	}
}

func (s *Store) Update(ctx context.Context, country, domain string, apply func(market.SourceRecord) market.SourceRecord) error {
	if domain == "" || len(domain) > 200 || strings.ContainsAny(domain, "/") {
		return nil
	}
	ref := s.priceSources().Doc(strings.ToLower(country) + "_" + domain)
	return s.c.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var cur sourceDoc
		snap, err := tx.Get(ref)
		switch {
		case err == nil:
			if err := snap.DataTo(&cur); err != nil {
				return err
			}
		case !isNotFound(err):
			return err
		}
		r := apply(cur.record())
		return tx.Set(ref, sourceDoc{Country: country, Domain: domain, Store: r.Store, Hits: int64(r.Hits), Misses: int64(r.Misses),
			Flagged: r.Flagged, FlaggedAt: r.FlaggedAt, SearchURL: r.SearchURL, Categories: r.Categories, UpdatedAt: r.UpdatedAt})
	})
}

// VNDPer: tỷ giá gần nhất không sau ngày day (job fx-sync ghi mỗi sáng), để quy giá ngoài Việt Nam ra VND.
func (s *Store) VNDPer(ctx context.Context, currency, day string) (float64, bool, error) {
	on, err := domain.ParseDate(day)
	if err != nil {
		return 0, false, err
	}
	r, ok, err := s.Rate(ctx, currency, on)
	if err != nil || !ok {
		return 0, ok, err
	}
	v, err := strconv.ParseFloat(r.Rate, 64)
	if err != nil {
		return 0, false, fmt.Errorf("fx %s: %w", currency, err)
	}
	return v, true, nil
}
