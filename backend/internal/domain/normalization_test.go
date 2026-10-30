package domain_test

import (
	"strings"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Nhóm trong bảng 0.1 mục 19 không trùng tên với nhãn 0.2: map tay sang nhóm gần nhất,
// "" nghĩa là taxonomy chưa có nhóm này và dòng thành món tự đặt tên (custom).
var groupAliases = map[string]string{
	"Nước uống":   "nuoc-khoang",
	"Phở ăn liền": "mi-an-lien",
	"Pin AA":      "",
}

// Bộ 100 ca 0.1 mục 19 đầu-cuối: tên nhóm của ca phải là một nhóm active trong taxonomy 0.2,
// và đơn vị canonical đọc từ tên phải khớp đơn vị mặc định của nhóm (trừ nhóm "source").
func TestSamplesMatchTaxonomy(t *testing.T) {
	byLabel := map[string]taxonomy.Group{}
	for _, g := range taxonomy.All().Groups {
		byLabel[strings.ToLower(g.Labels.VI)] = g
	}
	for _, c := range domain.Samples() {
		g, ok := byLabel[strings.ToLower(c.GroupName)]
		if key, alias := groupAliases[c.GroupName]; alias {
			if key == "" {
				continue
			}
			g, ok = taxonomy.Lookup(key)
		}
		if !ok {
			t.Errorf("#%d %q: group %q is not in taxonomy 0.2", c.N, c.Input, c.GroupName)
			continue
		}
		if !taxonomy.Has(g.Key) {
			t.Errorf("#%d: group %s is inactive", c.N, g.Key)
		}
		if g.Unit != "source" && g.Unit != string(c.Unit) {
			t.Errorf("#%d %q: canonical %s but group %s uses %s", c.N, c.Input, c.Unit, g.Key, g.Unit)
		}
		if got := domain.ParsePackaging(c.Input); got.Unit != c.Unit {
			t.Errorf("#%d: parsed unit %s, want %s", c.N, got.Unit, c.Unit)
		}
	}
}
