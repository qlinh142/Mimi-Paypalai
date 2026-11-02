package fx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

type memStore struct{ got []Rate }

func (m *memStore) SaveRates(_ context.Context, r []Rate) error { m.got = r; return nil }

func TestSyncFromERAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 1 USD = 26 250 VND = 0.92 EUR; JPY vắng mặt thì bỏ qua.
		_, _ = w.Write([]byte(`{"result":"success","base_code":"USD","time_last_update_unix":1793577600,
			"rates":{"USD":1,"VND":26250,"EUR":0.92,"KRW":1400.5}}`))
	}))
	defer srv.Close()
	st := &memStore{}
	rates, err := Sync(context.Background(), ERAPI{URL: srv.URL}, st, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]string{}
	for _, r := range rates {
		by[r.Currency] = r.VNDPerOne
		if r.Source != "open.er-api.com" || r.On.String() != "2026-11-02" {
			t.Fatalf("%+v", r)
		}
	}
	if by["USD"] != "26250.0000" || by["EUR"] != "28532.6087" || by["KRW"] != "18.7433" {
		t.Fatalf("%v", by)
	}
	if _, ok := by["JPY"]; ok || len(st.got) != 3 {
		t.Fatalf("%v", by)
	}
	// Quy đổi thật bằng tỷ giá vừa lưu: 12.50 EUR.
	if v, err := domain.ConvertToVND("12.50", by["EUR"]); err != nil || v != 356658 {
		t.Fatalf("%v %v", v, err)
	}
}

func TestSyncRejectsBadTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","base_code":"USD","time_last_update_unix":1793577600,"rates":{"USD":1}}`))
	}))
	defer srv.Close()
	if _, err := Sync(context.Background(), ERAPI{URL: srv.URL}, &memStore{}, time.Now()); err == nil {
		t.Fatal("thiếu VND phải lỗi")
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer bad.Close()
	if _, err := Sync(context.Background(), ERAPI{URL: bad.URL}, &memStore{}, time.Now()); err == nil {
		t.Fatal("HTTP 500 phải lỗi")
	}
}
