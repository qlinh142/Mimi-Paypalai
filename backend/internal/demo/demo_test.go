package demo

import (
	"context"
	"testing"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

type rec struct{ reqs []receipt.SaveRequest }

func (r *rec) Save(_ context.Context, _ authz.Principal, req receipt.SaveRequest) (receipt.SaveOutcome, error) {
	r.reqs = append(r.reqs, req)
	return receipt.SaveOutcome{}, nil
}

// V5: ngày mua tính lùi từ hôm nay, không cố định.
func TestSeedDatesFollowToday(t *testing.T) {
	r := &rec{}
	now := time.Date(2027, 3, 20, 9, 0, 0, 0, time.UTC)
	n, err := Seed(context.Background(), r, "uid-lan", now)
	if err != nil || n != len(bills) {
		t.Fatalf("%d %v", n, err)
	}
	if r.reqs[0].PurchasedOn != "2027-02-13" || r.reqs[len(r.reqs)-1].PurchasedOn != "2027-03-15" {
		t.Fatalf("%s .. %s", r.reqs[0].PurchasedOn, r.reqs[len(r.reqs)-1].PurchasedOn)
	}
	// Sữa tươi: lần cuối cách 5 ngày, mỗi 7 ngày một lần.
	var milk []string
	for _, q := range r.reqs {
		for _, l := range q.Lines {
			if l.GroupKey == "sua-tuoi" {
				milk = append(milk, q.PurchasedOn)
			}
		}
	}
	if len(milk) != 3 || milk[2] != "2027-03-15" || milk[1] != "2027-03-08" {
		t.Fatalf("%v", milk)
	}
	// Chạy lại: cùng mã hóa đơn.
	r2 := &rec{}
	_, _ = Seed(context.Background(), r2, "uid-lan", now)
	if r2.reqs[3].ReceiptID != r.reqs[3].ReceiptID || !validLike(r.reqs[3].ReceiptID) {
		t.Fatal(r.reqs[3].ReceiptID)
	}
	if _, err := Seed(context.Background(), r, "", now); err == nil {
		t.Fatal("thiếu uid phải lỗi")
	}
}

func validLike(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return len(s) > 0 && len(s) <= 64
}
