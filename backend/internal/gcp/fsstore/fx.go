package fsstore

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"

	"github.com/smartlife-lab/mimi/backend/internal/fx"
)

var _ fx.Store = (*Store)(nil)

// SaveRates ghi fx_rates/{currency}_{ngày}. Chạy lại trong ngày thì ghi đè chính nó; Rate
// (đọc khi lưu hóa đơn) lấy dòng mới nhất không sau ngày mua.
func (s *Store) SaveRates(ctx context.Context, rates []fx.Rate) error {
	bw := s.c.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0, len(rates))
	for _, r := range rates {
		ref := s.c.Collection("fx_rates").Doc(r.Currency + "_" + r.On.String())
		j, err := bw.Set(ref, map[string]any{
			"currency": r.Currency, "on": r.On.String(), "rate": r.VNDPerOne, "source": r.Source, "fetched_at": r.FetchedAt,
		})
		if err != nil {
			bw.End()
			return fmt.Errorf("fx save %s: %w", r.Currency, err)
		}
		jobs = append(jobs, j)
	}
	bw.End()
	for i, j := range jobs {
		if _, err := j.Results(); err != nil {
			return fmt.Errorf("fx save %s: %w", rates[i].Currency, err)
		}
	}
	return nil
}
