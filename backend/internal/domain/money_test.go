package domain

import "testing"

func TestAllocateDiscount(t *testing.T) {
	v := func(n int64) *VND { x := VND(n); return &x }
	got := AllocateDiscount([]*VND{v(64000), nil, v(36000), v(0), v(38000)}, 10000)
	var sum VND
	for _, g := range got {
		sum += g
	}
	if sum != 10000 || got[1] != 0 || got[3] != 0 || got[0] != 4638 || got[2] != 2609 || got[4] != 2753 {
		t.Fatalf("%v sum %d", got, sum)
	}
	if g := AllocateDiscount([]*VND{v(100)}, 0); g[0] != 0 {
		t.Fatalf("no discount %v", g)
	}
	if g := AllocateDiscount([]*VND{v(100)}, 200); g != nil {
		t.Fatalf("discount over total %v", g)
	}
	if g := AllocateDiscount([]*VND{v(1), v(1), v(1)}, 2); g[0]+g[1]+g[2] != 2 || g[0] != 1 || g[1] != 1 {
		t.Fatalf("ties go to earlier lines %v", g)
	}
}
