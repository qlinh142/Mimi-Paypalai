package fsstore

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/fx"
)

var _ fx.Store = (*Store)(nil)

// SaveRates ghi fx_rates/{currency}_{ngày}. Tỷ giá đã chốt của một ngày không bao giờ bị
// ghi đè (quy tắc FX1, FX4): chạy lại trong ngày thì giữ dòng cũ. Ngày không có dòng nào thì
// Rate (đọc khi lưu hóa đơn) lấy tỷ giá gần nhất trước ngày mua.
func (s *Store) SaveRates(ctx context.Context, rates []fx.Rate) error {
	bw := s.c.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0, len(rates))
	for _, r := range rates {
		ref := s.c.Collection("fx_rates").Doc(r.Currency + "_" + r.On.String())
		j, err := bw.Create(ref, map[string]any{
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
		if _, err := j.Results(); err != nil && status.Code(err) != codes.AlreadyExists {
			return fmt.Errorf("fx save %s: %w", rates[i].Currency, err)
		}
	}
	return nil
}

// RateHistory trả tỷ giá từng ngày của một ngoại tệ trong [from, to], cũ trước, kèm thêm một
// dòng gần nhất trước from (nếu có) để ngày đầu khoảng vẫn quy đổi được theo quy tắc "ngày
// chưa có dùng tỷ giá gần nhất trước đó". Dùng chung index (currency, on giảm dần) với Rate.
func (s *Store) RateHistory(ctx context.Context, currency string, from, to domain.Date) ([]fx.Point, error) {
	docs, err := s.c.Collection("fx_rates").
		Where("currency", "==", currency).
		Where("on", "<=", to.String()).
		OrderBy("on", firestore.Desc).Limit(fx.MaxHistoryDays + 1).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("fx history %s: %w", currency, err)
	}
	out := make([]fx.Point, 0, len(docs))
	for _, d := range docs {
		var v struct {
			On   string `firestore:"on"`
			Rate string `firestore:"rate"`
		}
		if err := d.DataTo(&v); err != nil {
			return nil, fmt.Errorf("fx history decode: %w", err)
		}
		out = append(out, fx.Point{On: v.On, VNDPerOne: v.Rate})
		if v.On < from.String() {
			break // dòng gần nhất trước from là đủ
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
