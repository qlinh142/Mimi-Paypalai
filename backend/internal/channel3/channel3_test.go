package channel3

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/market"
)

const sample = `{"products":[
 {"title":"Tide Original Liquid Laundry Detergent, 92 fl oz","offers":[
   {"url":"https://buy.trychannel3.com/a","domain":"www.walmart.com","price":{"price":12.97,"currency":"USD"},"availability":"InStock"},
   {"url":"https://buy.trychannel3.com/b","domain":"target.com","price":{"price":13.49,"currency":"USD"}},
   {"url":"https://buy.trychannel3.com/c","domain":"randomshop.com","price":{"price":9.99,"currency":"USD"}},
   {"url":"https://buy.trychannel3.com/d","domain":"cvs.com","price":{"price":11.00,"currency":"USD"},"availability":"OutOfStock"}]},
 {"title":"Perfume gift set","offers":[{"url":"https://x","domain":"walmart.com","price":{"price":5,"currency":"USD"}}]}
]}`

func TestLookupFollowsRules(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-api-key") != "k" || r.URL.Path != "/search" {
			t.Errorf("bad request %s key=%q", r.URL.Path, r.Header.Get("x-api-key"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["query"] != "Tide liquid laundry detergent" {
			t.Errorf("query = %v", body["query"])
		}
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()
	c := New("k")
	c.BaseURL = srv.URL
	us := market.Countries["US"]
	items := []market.Wanted{
		{Key: "us_tide", Country: "US", Name: "Tide liquid laundry detergent", Category: "giat-say"},
		{Key: "us_milk", Country: "US", Name: "whole milk", Category: "sua"}, // đồ ăn: không tra
	}
	got, err := c.Lookup(context.Background(), us, items)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (đồ ăn không tra Channel3)", calls)
	}
	ps := got["us_tide"]
	if len(ps) != 2 || ps[0].Store != "Walmart" || ps[1].Store != "Target" {
		t.Fatalf("prices = %+v", ps)
	}
	if ps[0].Local != 12.97 || ps[0].URL == "" || ps[0].Size != "92 fl oz" {
		t.Errorf("price = %+v", ps[0])
	}
	if _, ok := got["us_milk"]; ok {
		t.Error("milk should be skipped")
	}
}

func TestVietnamNotAsked(t *testing.T) {
	c := &Client{Key: "k", BaseURL: "http://127.0.0.1:1"}
	got, err := c.Lookup(context.Background(), market.Countries["VN"],
		[]market.Wanted{{Key: "vn_x", Country: "VN", Name: "Nước giặt Omo", Category: "giat-say"}})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
}

func TestNewWithoutKey(t *testing.T) {
	if New("  ") != nil {
		t.Fatal("empty key must disable Channel3")
	}
}
