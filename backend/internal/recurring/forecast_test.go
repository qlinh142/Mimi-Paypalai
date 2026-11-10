package recurring

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/item"
)

type fcStore struct{ Store }

func (fcStore) Occurrences(context.Context, string, domain.Date, domain.Date) ([]Occurrence, error) {
	return nil, nil
}

type fcItems []item.Record

func (f fcItems) ListItems(context.Context, string) ([]item.Record, error) { return f, nil }

type fcPrices struct{}

func (fcPrices) Prices(_ context.Context, _ string, groups []string) ([]domain.LastPrice, error) {
	out := make([]domain.LastPrice, len(groups))
	for i, g := range groups {
		out[i] = domain.LastPrice{GroupKey: g, Unit: 10_000, On: day("2026-11-01")}
	}
	return out, nil
}

// Lỗi 10/11: món mẹ bấm "Đã hết" (chưa có ngày dự kiến) không vào Dự báo nên màn Sắp chi tính 0đ.
func TestForecastIncludesMarkedOut(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	items := fcItems{
		{GroupKey: "nuoc-mam", Name: "Nước mắm", Prediction: domain.Prediction{ForcedOut: true}},
		{GroupKey: "sua", Name: "Sữa", Prediction: domain.Prediction{OutOn: day("2026-11-12")}},
		{GroupKey: "gao", Name: "Gạo", Prediction: domain.Prediction{OutOn: day("2027-01-01")}},
		{GroupKey: "bia", Name: "Bia", Dropped: true, Prediction: domain.Prediction{ForcedOut: true}},
	}
	s := NewService(fcStore{}, items, fcPrices{}, func() time.Time { return now })
	f, err := s.Forecast(context.Background(), authz.Principal{UID: "u1", Role: authz.RoleOwner, Timezone: "Asia/Ho_Chi_Minh"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, x := range f.Shopping {
		got[x.Key] = true
	}
	if !got["nuoc-mam"] || !got["sua"] || got["gao"] || got["bia"] || len(f.Shopping) != 2 {
		t.Fatalf("shopping = %v, want nuoc-mam and sua only", got)
	}
	if f.ShoppingVND != 20_000 {
		t.Fatalf("shopping total = %d, want 20000", f.ShoppingVND)
	}
}
