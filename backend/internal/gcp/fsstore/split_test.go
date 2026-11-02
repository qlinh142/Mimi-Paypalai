package fsstore

import (
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

func vline(owner string, vnd int64, cat string) receipt.Line {
	v := domain.VND(vnd)
	return receipt.Line{InventoryOwner: owner, VND: &v, CategoryKey: cat}
}

// Ví dụ trong đề xuất bước 6: hóa đơn 135.000đ, 120.000đ món của chủ giỏ, 15.000đ món của Minh.
func TestSplitSpend(t *testing.T) {
	lines := []receipt.Line{vline("owner", 70000, "sua"), vline("owner", 50000, "rau"), vline("", 15000, "banh")}
	payer, others := splitSpend(lines, "minh", 135000)
	if payer != 15000 || others["owner"] != 120000 || len(others) != 1 {
		t.Fatalf("payer=%d others=%v", payer, others)
	}

	// Giảm giá cả hóa đơn trừ vào phần người trả.
	payer, others = splitSpend(lines, "minh", 130000)
	if payer != 10000 || others["owner"] != 120000 {
		t.Fatalf("discount: payer=%d others=%v", payer, others)
	}

	// Giảm giá lớn hơn phần người trả: phần người được mua hộ thu nhỏ theo tỉ lệ, tổng vẫn khớp.
	lines = []receipt.Line{vline("a", 60000, ""), vline("b", 40000, "")}
	payer, others = splitSpend(lines, "minh", 90000)
	if payer != 0 || others["a"]+others["b"] != 90000 || others["a"] != 54000 {
		t.Fatalf("shrink: payer=%d others=%v", payer, others)
	}

	// Không có món mua hộ: cả hóa đơn của người trả.
	payer, others = splitSpend([]receipt.Line{vline("", 5000, "")}, "minh", 5000)
	if payer != 5000 || len(others) != 0 {
		t.Fatalf("own only: payer=%d others=%v", payer, others)
	}

	// Món chưa có giá không cộng vào ai.
	noPrice := receipt.Line{InventoryOwner: "owner"}
	payer, others = splitSpend([]receipt.Line{noPrice, vline("", 5000, "")}, "minh", 5000)
	if payer != 5000 || others["owner"] != 0 {
		t.Fatalf("no price: payer=%d others=%v", payer, others)
	}
}

func TestSplitCategories(t *testing.T) {
	lines := []receipt.Line{vline("owner", 70000, "sua"), vline("owner", 50000, ""), vline("", 15000, "banh")}
	got := splitCategories(lines, "minh")
	if got["owner"]["sua"] != 70000 || got["owner"]["chua-phan-loai"] != 50000 || got["minh"]["banh"] != 15000 || got["minh"]["sua"] != 0 {
		t.Fatalf("%v", got)
	}
	// Người trả tiền luôn có mục để đếm hóa đơn, kể cả khi mọi món là mua hộ.
	got = splitCategories([]receipt.Line{vline("owner", 1000, "")}, "minh")
	if _, ok := got["minh"]; !ok || len(got) != 2 {
		t.Fatalf("payer entry missing: %v", got)
	}
}

func TestDelegatedRows(t *testing.T) {
	lines := []map[string]any{
		{"payer_uid": "minh", "receipt_id": "r1", "store": "BHX", "purchased_on": "2026-11-02", "line_total_vnd": int64(70000)},
		{"payer_uid": "minh", "receipt_id": "r1", "store": "BHX", "purchased_on": "2026-11-02", "line_total_vnd": int64(50000)},
		{"payer_uid": "owner", "receipt_id": "r2", "line_total_vnd": int64(9000)}, // của mình: đã có trong receipts
		{"payer_uid": "minh", "receipt_id": "r3", "line_total_vnd": nil},          // chưa có giá
	}
	rows := delegatedRows(lines, "owner")
	if len(rows) != 2 || rows[0].ID != "d-r1" || rows[0].TotalVND != 120000 || !rows[0].Delegated || rows[0].Store != "BHX" || rows[1].TotalVND != 0 {
		t.Fatalf("%+v", rows)
	}
}
