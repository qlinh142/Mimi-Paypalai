package domain

import "testing"

// AC-06: lot 01/10 2000 ml, lot 10/10 2000 ml, đã dùng 2500 ml → lot cũ còn 0, lot mới 1500.
func TestAllocateFIFO(t *testing.T) {
	lots := []Lot{
		{ID: "a", On: d("2026-10-01"), Initial: 2000, Unit: UnitML},
		{ID: "b", On: d("2026-10-10"), Initial: 2000, Unit: UnitML},
		{ID: "c", On: d("2026-10-12"), Initial: 800, Unit: UnitGram}, // khác chiều đo: stream riêng
	}
	out, extra := AllocateFIFO(lots, 4000-2500, UnitML)
	if out[0].Remaining != 0 || out[1].Remaining != 1500 || out[2].Remaining != 0 || extra != 0 {
		t.Fatalf("fifo: %+v extra %v", out, extra)
	}
	// AC-07: người dùng SET 5000 ml, nhiều hơn tổng lot: phần dư không gán cho lot nào.
	out, extra = AllocateFIFO(lots, 5000, UnitML)
	if out[0].Remaining != 2000 || out[1].Remaining != 2000 || extra != 1000 {
		t.Fatalf("set: %+v extra %v", out, extra)
	}
	if lots[1].Remaining != 0 {
		t.Fatal("input lots must not be modified")
	}
}
