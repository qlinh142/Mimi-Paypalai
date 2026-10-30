// Package taxonomy là Shared Canonical Data của Mimi V2 (tài liệu 0.2): 300 product
// group, 20 nhóm cha, nhãn vi/en, đơn vị đo mặc định và mã đơn vị đếm.
//
// Quy ước khóa: tài liệu 0.2 viết group_key bằng gạch dưới ("sua_tuoi"); backend dùng
// khóa làm ID tài liệu Firestore và trong URL nên lưu dạng gạch nối ("sua-tuoi"). Hai dạng
// là cùng một khóa: Normalize đổi qua lại, không có bản dịch hay ánh xạ nào khác.
//
// Nhãn chỉ để hiển thị. Mọi phép tính (tồn kho, FIFO, dự báo, so khớp Kế hoạch, so giá)
// dùng khóa; đổi ngôn ngữ giao diện chỉ đổi nhãn được tra ở đây, không gọi AI.
package taxonomy

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

//go:embed groups.json
var groupsJSON []byte

// Labels là nhãn hiển thị theo ngôn ngữ. Thêm ngôn ngữ mới chỉ thêm trường ở đây.
type Labels struct {
	VI string `json:"vi"`
	EN string `json:"en"`
}

// Get trả nhãn theo ngôn ngữ, thiếu thì lùi về tiếng Việt.
func (l Labels) Get(lang string) string {
	if lang == "en" && l.EN != "" {
		return l.EN
	}
	return l.VI
}

// Measurement là chiều đo mặc định của nhóm (0.2 mục 4).
type Measurement string

const (
	MeasureMass   Measurement = "mass"
	MeasureVolume Measurement = "volume"
	MeasureCount  Measurement = "count"
	MeasureLength Measurement = "length"
	// Các giá trị còn lại ("source_dimension", "mass_or_count"...) nghĩa là giữ đúng chiều
	// in trên bao bì, không ép về g hay ml.
)

type Group struct {
	Key         string      `json:"key"`
	Parent      string      `json:"parent"`
	Labels      Labels      `json:"labels"`
	Measurement Measurement `json:"measurement"`
	// Unit là default_canonical_unit: g | ml | m | item | source ("source" = giữ đơn vị thực tế).
	Unit string `json:"unit"`
	// CountUnit là count_unit_code (egg, piece, roll...) cho nhóm đếm; UI tự dịch.
	CountUnit string `json:"count_unit,omitempty"`
	// Active=false: nhóm đã ngừng dùng (0.2 mục 10). Vẫn tra được để đọc lịch sử, nhưng
	// không nhận cho phân loại mới và không hiện trong gợi ý.
	Active bool `json:"active"`
}

type Parent struct {
	Key    string `json:"key"`
	Labels Labels `json:"labels"`
}

// Data là toàn bộ taxonomy, trả nguyên cho GET /taxonomy.
type Data struct {
	Version string   `json:"version"`
	Parents []Parent `json:"parents"`
	Groups  []Group  `json:"groups"`
}

var (
	data   Data
	byKey  map[string]*Group
	byNorm []searchEntry
)

type searchEntry struct {
	g      *Group
	vi, en string // nhãn đã bỏ dấu, chữ thường
}

func init() {
	var raw struct {
		Version string   `json:"version"`
		Parents []Parent `json:"parents"`
		Groups  []struct {
			Group
			CountUnit *string `json:"count_unit"`
			Active    *bool   `json:"active"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(groupsJSON, &raw); err != nil {
		panic("taxonomy: groups.json: " + err.Error())
	}
	data = Data{Version: raw.Version, Parents: raw.Parents}
	for _, g := range raw.Groups {
		gg := g.Group
		if g.CountUnit != nil {
			gg.CountUnit = *g.CountUnit
		}
		gg.Active = g.Active == nil || *g.Active
		data.Groups = append(data.Groups, gg)
	}
	byKey = make(map[string]*Group, len(data.Groups))
	for i := range data.Groups {
		g := &data.Groups[i]
		byKey[g.Key] = g
		if !g.Active {
			continue
		}
		byNorm = append(byNorm, searchEntry{g: g, vi: domain.NormalizeVi(g.Labels.VI), en: domain.NormalizeVi(g.Labels.EN)})
	}
}

// Version là product_taxonomy_version, lưu kèm dữ liệu phân loại để migration.
func Version() string { return data.Version }

// All trả bản sao chỉ đọc của toàn bộ taxonomy.
func All() Data { return data }

// Normalize đưa khóa về dạng lưu trữ: chữ thường, gạch nối ("sua_tuoi" → "sua-tuoi").
func Normalize(key string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "_", "-")
}

// Lookup tìm nhóm theo khóa (nhận cả dạng gạch dưới của tài liệu 0.2).
func Lookup(key string) (Group, bool) {
	g, ok := byKey[Normalize(key)]
	if !ok {
		return Group{}, false
	}
	return *g, true
}

// Has báo khóa thuộc taxonomy và đang dùng (active), tức nhận được cho phân loại mới.
func Has(key string) bool { g, ok := byKey[Normalize(key)]; return ok && g.Active }

// Keys trả mọi group_key, theo thứ tự trong tài liệu 0.2 (dùng làm enum cho Gemini).
func Keys() []string {
	out := make([]string, 0, len(data.Groups))
	for _, g := range data.Groups {
		if g.Active {
			out = append(out, g.Key)
		}
	}
	return out
}

// Label trả nhãn theo ngôn ngữ; khóa không có trong taxonomy trả "".
func Label(key, lang string) string {
	if g, ok := byKey[Normalize(key)]; ok {
		return g.Labels.Get(lang)
	}
	return ""
}

// LabelsOf trả cả hai nhãn để API gửi kèm; nil khi khóa nằm ngoài taxonomy.
func LabelsOf(key string) *Labels {
	if g, ok := byKey[Normalize(key)]; ok {
		l := g.Labels
		return &l
	}
	return nil
}

// Search gợi ý nhóm cho ô "Nhập tên sản phẩm" (3.1 mục 4.2): khớp không dấu ở cả hai
// ngôn ngữ. Nhãn bắt đầu bằng từ khóa xếp trước, rồi nhãn chứa từ khóa; tối đa limit kết quả.
func Search(q string, limit int) []Group {
	needle := domain.NormalizeVi(q)
	if needle == "" || limit <= 0 {
		return []Group{}
	}
	type hit struct {
		g     *Group
		rank  int
		label string
	}
	var hits []hit
	for _, e := range byNorm {
		rank := -1
		switch {
		case strings.HasPrefix(e.vi, needle) || strings.HasPrefix(e.en, needle):
			rank = 0
		case strings.Contains(e.vi, needle) || strings.Contains(e.en, needle):
			rank = 1
		}
		if rank >= 0 {
			hits = append(hits, hit{g: e.g, rank: rank, label: e.vi})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].rank != hits[b].rank {
			return hits[a].rank < hits[b].rank
		}
		if len(hits[a].label) != len(hits[b].label) {
			return len(hits[a].label) < len(hits[b].label)
		}
		return hits[a].label < hits[b].label
	})
	out := make([]Group, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, *h.g)
	}
	return out
}

// DefaultUnit trả đơn vị gốc mặc định của nhóm (fallback cuối cùng, 0.1 mục 8):
// ok=false khi nhóm để "source" hoặc khóa nằm ngoài taxonomy.
func DefaultUnit(key string) (domain.Unit, bool) {
	g, ok := byKey[Normalize(key)]
	if !ok {
		return "", false
	}
	switch g.Unit {
	case "g":
		return domain.UnitGram, true
	case "ml":
		return domain.UnitML, true
	case "m":
		return domain.UnitMeter, true
	case "item":
		return domain.UnitItem, true
	}
	return "", false
}

// spendingByParent: danh mục chi tiêu mặc định theo nhóm cha. Nhờ vậy khi AI hoặc người
// dùng đã chọn được nhóm sản phẩm thì dòng hóa đơn cũng có danh mục, không còn "Cần phân loại".
var spendingByParent = map[string]string{
	"staples-grains": "do-kho", "bread-noodles-starches": "do-kho", "milk-dairy": "sua",
	"eggs-meat": "tuoi-song", "fish-seafood": "tuoi-song", "vegetables-roots": "rau-cu",
	"cooking-vegetables": "rau-cu", "fruit": "trai-cay", "condiments-oils-sauces": "gia-vi",
	"beverages": "do-uong", "biscuits-sweets-snacks": "banh-keo", "laundry": "hoa-pham",
	"household-cleaning": "hoa-pham", "paper-disposables": "giay", "hair-care": "ve-sinh",
	"bath-body-care": "ve-sinh", "oral-care-shaving": "ve-sinh", "skin-care-cosmetics": "my-pham",
	"baby-maternity": "me-be", "personal-care-wellness": "ve-sinh",
}

// SpendingCategory trả danh mục chi tiêu mặc định của một nhóm; "" khi khóa ngoài taxonomy.
func SpendingCategory(groupKey string) string {
	if g, ok := byKey[Normalize(groupKey)]; ok {
		return spendingByParent[g.Parent]
	}
	return ""
}
