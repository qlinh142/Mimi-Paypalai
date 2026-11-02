package taxonomy

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSpendingKeys(t *testing.T) {
	keys := SpendingKeys()
	if len(keys) != 27 || keys[0] != "sua" || keys[len(keys)-1] != "khac" {
		t.Fatalf("keys %v", keys)
	}
	for _, k := range keys {
		if k == CategoryRecurring || k == CategoryUncategorized {
			t.Fatalf("system category %q must not be offered to AI", k)
		}
		c, _ := CategoryOf(k)
		if c.Labels.VI == "" || c.Labels.EN == "" {
			t.Fatalf("missing label %+v", c)
		}
	}
	for _, k := range []string{CategoryRecurring, CategoryUncategorized} {
		if c, ok := CategoryOf(k); !ok || c.Labels.Get("en") == "" {
			t.Fatalf("system category %q missing", k)
		}
	}
}

// Danh mục mặc định theo nhóm cha phải là danh mục có thật.
func TestSpendingByParentKnown(t *testing.T) {
	for parent, cat := range spendingByParent {
		if _, ok := CategoryOf(cat); !ok {
			t.Fatalf("%s -> %s not in categories.json", parent, cat)
		}
	}
}

// Frontend dùng bản chép; hai file phải giống hệt nhau.
func TestCategoriesFrontendCopy(t *testing.T) {
	p := filepath.Join("..", "..", "..", "frontend", "src", "i18n", "categories.json")
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		t.Skip("no frontend checkout")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, categoriesJSON) {
		t.Fatal("frontend/src/i18n/categories.json khác backend/internal/taxonomy/categories.json: chép lại file backend sang frontend")
	}
}
