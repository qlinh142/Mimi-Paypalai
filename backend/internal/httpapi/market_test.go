package httpapi

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/market"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/spending"
	"github.com/smartlife-lab/mimi/backend/internal/user"
)

type fakeMarket map[string]market.Entry

func (f fakeMarket) Entries(_ context.Context, keys []string) (map[string]market.Entry, error) {
	out := map[string]market.Entry{}
	for _, k := range keys {
		if e, ok := f[k]; ok {
			out[k] = e
		}
	}
	return out, nil
}

func TestPlanMarketPrices(t *testing.T) {
	ps := &fakePlanStore{lines: map[string]plan.Line{
		"sua-tuoi": {Key: "sua-tuoi", Name: "Sữa tươi", Status: "open"},
		"trung":    {Key: "trung", Name: "Trứng gà", Status: "open"}, // chưa có giá
	}}
	s, err := New(Deps{
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tokens:     fakeTokens{"owner-tok": "u1"},
		AppCheck:   fakeAppCheck{},
		Principals: fakePrincipals{"u1": {UID: "u1", Role: authz.RoleOwner}},
		Profiles:   fakeProfiles{}, Receipts: &fakeReceipts{}, Items: fakeItems{}, Ready: fakeReady{},
		Home: fakeHome{}, Spending: spending.New(fakeSpendingStore{}), Settings: user.NewSettingsService(fakeSettingsStore{}, nil),
		Plan:  plan.New(ps, fakePlanItems{}, nil),
		Notes: fakeNotes{},
		Market: fakeMarket{market.ItemID("VN", "Sữa tươi"): {
			Prices:    []market.Price{{Store: "WinMart", PriceVND: 32_000, Source: "gemini_search", On: "2026-11-03"}},
			UpdatedAt: time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, body := do(s.Handler(), "GET", "/api/plan/market-prices", "owner-tok", "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("%s", rec.Body)
	}
	it := items[0].(map[string]any)
	if it["key"] != "sua-tuoi" || it["prices"].([]any)[0].(map[string]any)["price_vnd"] != float64(32_000) {
		t.Fatalf("%s", rec.Body)
	}
}

type fakePriceRefresh struct {
	day   string
	items []market.Wanted
}

func (f *fakePriceRefresh) Refresh(_ context.Context, _, day string, items []market.Wanted) (market.Result, error) {
	f.day, f.items = day, items
	return market.Result{Looked: 2, Found: 1, Fresh: 0}, nil
}

func (f *fakePriceRefresh) Status(context.Context, string, string) (market.RefreshStatus, error) {
	at := time.Date(2026, 11, 2, 15, 45, 0, 0, time.UTC)
	return market.RefreshStatus{Daily: 3, Left: 2, LastAt: &at}, nil
}

func TestPlanMarketPricesRefresh(t *testing.T) {
	ps := &fakePlanStore{lines: map[string]plan.Line{
		"sua-tuoi": {Key: "sua-tuoi", Name: "Sữa tươi", Status: "open"},
		"trung":    {Key: "trung", Name: "Trứng gà", Status: "open"},
	}}
	fr := &fakePriceRefresh{}
	s, err := New(Deps{
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tokens:     fakeTokens{"owner-tok": "u1"},
		AppCheck:   fakeAppCheck{},
		Principals: fakePrincipals{"u1": {UID: "u1", Role: authz.RoleOwner}},
		Profiles:   fakeProfiles{}, Receipts: &fakeReceipts{}, Items: fakeItems{}, Ready: fakeReady{},
		Home: fakeHome{}, Spending: spending.New(fakeSpendingStore{}), Settings: user.NewSettingsService(fakeSettingsStore{}, nil),
		Plan:  plan.New(ps, fakePlanItems{}, nil),
		Notes: fakeNotes{},
		Market: fakeMarket{market.ItemID("VN", "Trứng gà"): {
			Prices: []market.Price{{Store: "WinMart", PriceVND: 29_000, Source: "gemini_search", On: "2026-11-03"}},
		}},
		PriceRefresh: fr,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, body := do(s.Handler(), "POST", "/api/plan/market-prices/refresh", "owner-tok", "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(fr.items) != 2 || fr.day == "" || body["looked"] != float64(2) || body["found"] != float64(1) {
		t.Fatalf("%+v %s", fr, rec.Body)
	}
	if items, _ := body["items"].([]any); len(items) != 1 {
		t.Fatalf("%s", rec.Body)
	}
	if st, _ := body["refresh"].(map[string]any); st["left"] != float64(2) || st["last_at"] == nil {
		t.Fatalf("%s", rec.Body)
	}
	// GET cũng trả trạng thái nút để app hiện "Lần cập nhật gần nhất".
	rec, body = do(s.Handler(), "GET", "/api/plan/market-prices", "owner-tok", "")
	if st, _ := body["refresh"].(map[string]any); rec.Code != 200 || st["daily"] != float64(3) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
