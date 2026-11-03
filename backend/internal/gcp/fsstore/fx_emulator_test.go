package fsstore

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/fx"
)

// fx-sync ghi xong thì Rate (lúc lưu hóa đơn) đọc ra đúng tỷ giá gần nhất không sau ngày mua.
func TestSaveRatesThenRateEmulator(t *testing.T) {
	s, _, _ := emulator(t)
	ctx := context.Background()
	d1, _ := domain.ParseDate("2031-01-10")
	d2, _ := domain.ParseDate("2031-01-12")
	now := time.Now()
	if err := s.SaveRates(ctx, []fx.Rate{
		{Currency: "XTS", On: d1, VNDPerOne: "100.5", Source: "test", FetchedAt: now},
		{Currency: "XTS", On: d2, VNDPerOne: "101.0", Source: "test", FetchedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	on, _ := domain.ParseDate("2031-01-11")
	r, ok, err := s.Rate(ctx, "XTS", on)
	if err != nil || !ok || r.Rate != "100.5" || r.Source != "test" {
		t.Fatalf("%+v %v %v", r, ok, err)
	}
}

// Chạy fx-sync lần hai trong cùng ngày không ghi đè tỷ giá đã chốt (FX1, FX4).
func TestSaveRatesKeepsFirstOfDayEmulator(t *testing.T) {
	s, _, _ := emulator(t)
	ctx := context.Background()
	d, _ := domain.ParseDate("2031-02-01")
	now := time.Now()
	if err := s.SaveRates(ctx, []fx.Rate{{Currency: "XTS", On: d, VNDPerOne: "200", Source: "first", FetchedAt: now}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRates(ctx, []fx.Rate{{Currency: "XTS", On: d, VNDPerOne: "999", Source: "second", FetchedAt: now}}); err != nil {
		t.Fatal(err)
	}
	r, ok, err := s.Rate(ctx, "XTS", d)
	if err != nil || !ok || r.Rate != "200" || r.Source != "first" {
		t.Fatalf("%+v %v %v", r, ok, err)
	}
}

// RateHistory trả các ngày trong khoảng, cũ trước, kèm một ngày gần nhất trước from.
func TestRateHistoryEmulator(t *testing.T) {
	s, _, _ := emulator(t)
	ctx := context.Background()
	now := time.Now()
	var rs []fx.Rate
	for i, d := range []string{"2031-03-01", "2031-03-05", "2031-03-10", "2031-03-20"} {
		on, _ := domain.ParseDate(d)
		rs = append(rs, fx.Rate{Currency: "XTH", On: on, VNDPerOne: []string{"1", "2", "3", "4"}[i], Source: "t", FetchedAt: now})
	}
	if err := s.SaveRates(ctx, rs); err != nil {
		t.Fatal(err)
	}
	from, _ := domain.ParseDate("2031-03-07")
	to, _ := domain.ParseDate("2031-03-15")
	got, err := s.RateHistory(ctx, "XTH", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].On != "2031-03-05" || got[1].On != "2031-03-10" || got[1].VNDPerOne != "3" {
		t.Fatalf("%+v", got)
	}
}
