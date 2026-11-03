package domain

import "testing"

func TestASCII(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"Mimi · Mimi Mart · 5 món", 0, "Mimi - Mimi Mart - 5 mon"},
		{"Đi mua hộ – Lẩu 6 người", 0, "Di mua ho - Lau 6 nguoi"},
		{"Sữa tươi  tiệt\ttrùng", 0, "Sua tuoi tiet trung"},
		{"Dầu ăn 1L", 6, "Dau an"},
		{"  ", 10, ""},
		{"Cà phê ☕ ngon", 0, "Ca phe ngon"},
		// "ầ" ghép rời (dán từ macOS) cũng bỏ dấu được.
		{"ầu", 0, "au"},
	}
	for _, c := range cases {
		if got := ASCII(c.in, c.max); got != c.want {
			t.Errorf("ASCII(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}
