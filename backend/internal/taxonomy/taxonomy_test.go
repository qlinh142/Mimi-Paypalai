package taxonomy

import (
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// QA checklist của 0.2 mục 12.
func TestTaxonomyData(t *testing.T) {
	d := All()
	if d.Version != "1.2" || len(d.Groups) != 300 || len(d.Parents) != 20 {
		t.Fatalf("version %s, %d groups, %d parents", d.Version, len(d.Groups), len(d.Parents))
	}
	parents := map[string]bool{}
	for _, p := range d.Parents {
		parents[p.Key] = true
	}
	seen := map[string]bool{}
	for _, g := range d.Groups {
		if seen[g.Key] {
			t.Errorf("duplicate key %s", g.Key)
		}
		seen[g.Key] = true
		if g.Labels.VI == "" || g.Labels.EN == "" {
			t.Errorf("%s: missing label", g.Key)
		}
		if !parents[g.Parent] {
			t.Errorf("%s: unknown parent %s", g.Key, g.Parent)
		}
		switch g.Unit {
		case "g", "ml", "m", "item", "source":
		default:
			t.Errorf("%s: canonical unit %q is not language neutral", g.Key, g.Unit)
		}
		if (g.Unit == "item") != (g.CountUnit != "") {
			t.Errorf("%s: count unit %q with canonical unit %s", g.Key, g.CountUnit, g.Unit)
		}
	}
}

func TestLookupAndLabels(t *testing.T) {
	for _, k := range []string{"sua_tuoi", "sua-tuoi", " SUA_TUOI "} {
		if !Has(k) {
			t.Fatalf("Has(%q) = false", k)
		}
	}
	if Label("sua_bot", "vi") != "Sữa bột" || Label("sua-bot", "en") != "Milk powder" {
		t.Fatalf("labels: %q %q", Label("sua_bot", "vi"), Label("sua-bot", "en"))
	}
	if Label("khong-co", "vi") != "" || LabelsOf("khong-co") != nil {
		t.Fatal("unknown key must have no label")
	}
	if u, ok := DefaultUnit("trung_ga"); !ok || u != domain.UnitItem {
		t.Fatalf("trung_ga unit %v", u)
	}
	if _, ok := DefaultUnit("nuoc_giat"); ok {
		t.Fatal("nuoc_giat keeps the source dimension")
	}
}

// 3.1 mục 4.2: gõ "dầu g" gợi ý các nhóm dầu gội, không gọi AI.
func TestSearch(t *testing.T) {
	got := Search("dau g", 5)
	if len(got) == 0 || got[0].Key != "dau-goi" {
		t.Fatalf("search: %+v", got)
	}
	for _, g := range got {
		if g.Key == "dau-an" {
			t.Fatal("dầu ăn should not match 'dau g'")
		}
	}
	if en := Search("shampoo", 3); len(en) == 0 || en[0].Key != "dau-goi" {
		t.Fatalf("english search: %+v", en)
	}
	if len(Search("", 5)) != 0 {
		t.Fatal("empty query")
	}
}

func TestBrand(t *testing.T) {
	for in, want := range map[string]string{"COCA COLA": "Coca-Cola", "p/s": "P/S", "omo": "OMO", "th true milk": "TH true MILK"} {
		if got, ok := Brand(in); !ok || got != want {
			t.Errorf("Brand(%q) = %q, %v", in, got, ok)
		}
	}
	if _, ok := Brand("unknown brand"); ok {
		t.Fatal("unknown brand")
	}
}

func TestSpendingCategoryCoversEveryParent(t *testing.T) {
	for _, p := range All().Parents {
		if spendingByParent[p.Key] == "" {
			t.Errorf("parent %s has no spending category", p.Key)
		}
	}
	if got := SpendingCategory("sua_tuoi"); got != "sua" {
		t.Errorf("sua_tuoi → %q, want sua", got)
	}
	if got := SpendingCategory("khong-co"); got != "" {
		t.Errorf("unknown key → %q", got)
	}
}

func TestEveryGroupActiveByDefault(t *testing.T) {
	if n := len(Keys()); n != 300 {
		t.Fatalf("active keys = %d, want 300", n)
	}
}
