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
