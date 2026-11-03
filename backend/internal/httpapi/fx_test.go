package httpapi

import (
	"context"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/fx"
)

type fakeFX struct{}

func (fakeFX) RateHistory(_ context.Context, cur string, from, to domain.Date) ([]fx.Point, error) {
	return []fx.Point{{On: from.AddDays(-1).String(), VNDPerOne: "25000"}, {On: to.String(), VNDPerOne: "25100"}}, nil
}

func TestFXRates(t *testing.T) {
	h, _ := newTestServer(t)
	rec, body := do(h, "GET", "/api/fx/rates?currency=USD&from=2026-10-01&to=2026-10-31", "owner-tok", "")
	if rec.Code != 200 || body["currency"] != "USD" {
		t.Fatalf("%d %v", rec.Code, body)
	}
	rates, _ := body["rates"].([]any)
	if len(rates) != 2 || rates[0].(map[string]any)["on"] != "2026-09-30" || rates[1].(map[string]any)["rate"] != "25100" {
		t.Fatalf("rates %v", rates)
	}
	for _, q := range []string{"currency=VND", "currency=XYZ", "currency=USD&from=2020-01-01&to=2026-01-01", "currency=USD&to=bad"} {
		if rec, _ := do(h, "GET", "/api/fx/rates?"+q, "owner-tok", ""); rec.Code != 422 {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}
	if rec, _ := do(h, "GET", "/api/fx/rates?currency=USD", "", ""); rec.Code != 401 {
		t.Errorf("no token: %d", rec.Code)
	}
}
