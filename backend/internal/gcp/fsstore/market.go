package fsstore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/smartlife-lab/mimi/backend/internal/market"
)

var (
	_ market.Store        = (*Store)(nil)
	_ market.RefreshStore = (*Store)(nil)
	_ market.RefreshQuota = (*Store)(nil)
	_ market.Demand       = (*Store)(nil)
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
	Store    string `firestore:"store"`
	PriceVND int64  `firestore:"price_vnd"`
	Size     string `firestore:"size"`
	Source   string `firestore:"source"`
	URL      string `firestore:"url"`
	On       string `firestore:"on"`
}

func (s *Store) SaveEntry(ctx context.Context, e market.Entry) error {
	prices := make([]map[string]any, 0, len(e.Prices))
	for _, p := range e.Prices {
		prices = append(prices, map[string]any{
			"store": p.Store, "price_vnd": p.PriceVND, "size": p.Size, "source": p.Source, "url": p.URL, "on": p.On,
		})
	}
	doc := map[string]any{"name": e.Name, "group_key": e.GroupKey, "prices": prices, "updated_at": e.UpdatedAt}
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
		out[d.Ref.ID] = market.Entry{Key: d.Ref.ID, Name: raw.Name, GroupKey: raw.GroupKey, Prices: prices, UpdatedAt: raw.UpdatedAt}
	}
	return out, nil
}

// WantedItems đọc mọi dòng Kế hoạch đang mở của mọi nhà (Plan mặc định plan_items và
// plans/{id}/items), chỉ lấy tên và nhóm. Truy vấn collection group không lọc nên không
// cần index; số dòng lớn thì đổi sang lọc status với index collection group.
func (s *Store) WantedItems(ctx context.Context) ([]market.Wanted, error) {
	var out []market.Wanted
	for _, group := range []string{"plan_items", "items"} {
		it := s.c.CollectionGroup(group).Select("name", "group_key", "status").Documents(ctx)
		for {
			d, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				it.Stop()
				return nil, fmt.Errorf("market wanted %s: %w", group, err)
			}
			// "items" cũng có thể là tên collection khác: chỉ nhận users/{uid}/plans/{id}/items.
			if group == "items" && (d.Ref.Parent.Parent == nil || d.Ref.Parent.Parent.Parent.ID != "plans") {
				continue
			}
			m := d.Data()
			if st, _ := m["status"].(string); st != "open" && st != "assigned" {
				continue
			}
			name, _ := m["name"].(string)
			gk, _ := m["group_key"].(string)
			if gk == "" {
				gk = d.Ref.ID // dòng plan_items dùng group_key làm ID
			}
			if name == "" {
				continue
			}
			out = append(out, market.Wanted{Key: market.Key(name), Name: name, GroupKey: gk})
		}
		it.Stop()
	}
	return out, nil
}
