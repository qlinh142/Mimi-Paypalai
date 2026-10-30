package recurring

import (
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

func day(s string) domain.Date { d, _ := domain.ParseDate(s); return d }

func TestNextDue(t *testing.T) {
	cases := []struct {
		due    string
		cycle  Cycle
		anchor int
		want   string
	}{
		{"2026-01-31", Monthly, 31, "2026-02-28"},
		{"2026-02-28", Monthly, 31, "2026-03-31"},
		{"2026-10-19", Weekly, 19, "2026-10-26"},
		{"2026-11-30", Quarterly, 30, "2027-02-28"},
		{"2028-02-29", Yearly, 29, "2029-02-28"},
	}
	for _, c := range cases {
		if got := NextDue(day(c.due), c.cycle, c.anchor).String(); got != c.want {
			t.Errorf("%s %s: got %s want %s", c.due, c.cycle, got, c.want)
		}
	}
}

func TestMatchesReceipt(t *testing.T) {
	o := Occurrence{Name: "Netflix", AmountVND: 104_499, DueOn: "2026-10-19"}
	if !MatchesReceipt(o, "NETFLIX.COM", nil, 104_499, nil, day("2026-10-19")) {
		t.Fatal("same merchant and amount should match")
	}
	if !MatchesReceipt(o, "Apple", []string{"Netflix Premium"}, 300_000, []int64{104_000}, day("2026-10-22")) {
		t.Fatal("line name and line amount within tolerance should match")
	}
	if MatchesReceipt(o, "Netflix", nil, 104_499, nil, day("2026-11-05")) {
		t.Fatal("outside date window must not match")
	}
	if MatchesReceipt(o, "Spotify", nil, 104_499, nil, day("2026-10-19")) {
		t.Fatal("other merchant must not match")
	}
	if MatchesReceipt(o, "Netflix", nil, 260_000, nil, day("2026-10-19")) {
		t.Fatal("other amount must not match")
	}
}
