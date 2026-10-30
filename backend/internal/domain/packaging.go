package domain

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Packaging là quy cách đóng gói của MỘT dòng hóa đơn, đã nhân toàn bộ cấu trúc
// combo / lốc / thùng (0.1 mục 10): "2 combo × 3 hộp × 100 ml" → Count 6, Size 100,
// SizeUnit ml, Total 600 ml.
type Packaging struct {
	Structure string  // mô tả gốc phần quy cách, để hiển thị và audit
	Count     float64 // số gói nhỏ nhất sau khi nhân, >= 1
	Size      float64 // dung tích / khối lượng / số đếm của một gói theo SizeUnit gốc; 0 nếu không in
	SizeUnit  string  // đơn vị in trên bao bì ("ml", "kg", "qua"...), "" nếu không in
	Total     float64 // Count × Size theo Unit; 0 khi chưa đọc được
	Unit      Unit    // canonical unit của Total, "" khi chưa đọc được
}

// Known báo đã có tổng lượng canonical dùng được cho tồn kho.
func (p Packaging) Known() bool { return p.Unit != "" && p.Total > 0 }

// ForLines nhân thêm số lượng in ở cột SL của hóa đơn: 2 dòng "Sữa 1L" là 2000 ml.
func (p Packaging) ForLines(qty float64) Packaging {
	if qty <= 0 || math.IsNaN(qty) {
		qty = 1
	}
	p.Count = round3(p.Count * qty)
	p.Total = round3(p.Total * qty)
	return p
}

var (
	// Số có thể dùng dấu phẩy hoặc chấm thập phân: 1.5L, 2,5kg. Số phải đứng sau ranh giới
	// (đầu chuỗi, khoảng trắng, dấu câu) để "B5", "ST25", "3in1" không bị đọc thành quy cách.
	reSize   = regexp.MustCompile(`(?:^|[^a-z0-9.,])(\d+(?:[.,]\d+)?)\s*(kg|kgs|g|gr|gram|mg|ml|l|lit|litre|liter|m|cm)(?:[^a-z0-9]|$)`)
	reMult   = regexp.MustCompile(`(?:^|[^a-z0-9.,])(\d+)\s*(combo|loc|thung|hop|lon|chai|goi|vi|tui|bich|khay|pack|hu|tuyp|cai)?\s*x(?:\s|$)`)
	reCombo  = regexp.MustCompile(`(?:^|[^a-z0-9])combo\s*(\d+)(?:[^a-z0-9.,]|$)`)
	reOuter  = regexp.MustCompile(`(?:^|[^a-z0-9.,])(\d+)\s*(combo|loc|thung|hop|lon|chai|goi|vi|tui|bich|khay|pack|hu|tuyp)(?:[^a-z0-9]|$)`)
	reDigitX = regexp.MustCompile(`(\d)\s*x\s*(\d)`)
	reCount  = regexp.MustCompile(`(?:^|[^a-z0-9.,])(\d+)\s*(qua|trai|vien|cai|chiec|mieng|cuon|to|tui|o|pcs|pc|goi|hop|chai|lon|doi|cap|bich|vi|cu|bo|hu|tuyp|item|items)(?:[^a-z0-9]|$)`)
)

// ParsePackaging đọc quy cách từ tên in trên hóa đơn bằng luật cố định (không gọi AI).
// Thứ tự ưu tiên đơn vị theo 0.1 mục 8: đơn vị thực tế in trên bao bì (g, ml...) trước,
// rồi số đếm tự nhiên (quả, viên, cuộn...). Không quy đổi g ↔ ml.
//
//	"Vinamilk Sure Diacerna 2×900g"  → 2 × 900 g = 1800 g
//	"3 lốc × 4 hộp × 180 ml"         → 12 × 180 ml = 2160 ml
//	"Pulppy 3×180 tờ"                → 3 × 180 = 540 item
//	"Trứng Ba Huân 10 quả"            → 10 item
func ParsePackaging(raw string) Packaging {
	s := " " + strings.NewReplacer("×", " x ", "*", " x ", "✕", " x ").Replace(NormalizeVi(raw)) + " "
	s = reDigitX.ReplaceAllString(s, "$1 x $2") // "2x900g" → "2 x 900g"

	sizes := reSize.FindAllStringSubmatchIndex(s, -1)
	// Bỏ "mg" (hàm lượng hoạt chất: Vitamin C 1000mg), không phải quy cách đóng gói.
	var size []int
	for i := len(sizes) - 1; i >= 0; i-- {
		if s[sizes[i][4]:sizes[i][5]] != "mg" {
			size = sizes[i]
			break
		}
	}
	if size != nil {
		n := parseNum(s[size[2]:size[3]])
		unit := s[size[4]:size[5]]
		total, base := ToBase(n, unit)
		mult := multipliers(s, size[2])
		p := Packaging{Count: mult, Size: n, SizeUnit: unit, Total: round3(total * mult), Unit: base}
		p.Structure = strings.TrimSpace(s[firstMultStart(s, size[2]):size[5]])
		return p
	}

	counts := reCount.FindAllStringSubmatchIndex(s, -1)
	if len(counts) > 0 {
		c := counts[len(counts)-1]
		n := parseNum(s[c[2]:c[3]])
		mult := 1.0
		for _, m := range reMult.FindAllStringSubmatchIndex(s, -1) {
			if m[1] <= c[2] {
				mult *= parseNum(s[m[2]:m[3]])
			}
		}
		return Packaging{
			Structure: strings.TrimSpace(s[firstMultStart(s, c[2]):c[5]]),
			Count:     mult, Size: n, SizeUnit: s[c[4]:c[5]], Total: round3(n * mult), Unit: UnitItem,
		}
	}
	return Packaging{Count: 1}
}

// multipliers nhân mọi hệ số đóng gói đứng trước số đo: "2 combo x 3 hop x", "combo 2 lon",
// "2 lon ... 900g". Mỗi con số chỉ được tính một lần dù khớp nhiều mẫu.
func multipliers(s string, before int) float64 {
	used := map[int]bool{}
	mult := 1.0
	take := func(start, end int) {
		if end > before || used[start] {
			return
		}
		used[start] = true
		mult *= parseNum(s[start:end])
	}
	for _, m := range reMult.FindAllStringSubmatchIndex(s, -1) {
		take(m[2], m[3])
	}
	for _, m := range reOuter.FindAllStringSubmatchIndex(s, -1) {
		take(m[2], m[3])
	}
	for _, m := range reCombo.FindAllStringSubmatchIndex(s, -1) {
		take(m[2], m[3])
	}
	if mult <= 0 {
		return 1
	}
	return mult
}

func firstMultStart(s string, before int) int {
	start := before
	for _, re := range []*regexp.Regexp{reMult, reOuter, reCombo} {
		for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
			if m[3] <= before && m[2] < start {
				start = m[2]
			}
		}
	}
	return start
}

func parseNum(s string) float64 {
	n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// PackagingFrom dựng quy cách từ các trường AI đã tách (package_count, package_size,
// package_unit). Dùng khi luật cố định không đọc được tên in trên hóa đơn.
func PackagingFrom(count, size float64, unit string) Packaging {
	if count <= 0 || math.IsNaN(count) {
		count = 1
	}
	if size <= 0 || math.IsNaN(size) {
		return Packaging{Count: count}
	}
	key := NormalizeUnit(unit)
	if key == "" {
		return Packaging{Count: count}
	}
	total, base := ToBase(size, key)
	return Packaging{Count: count, Size: size, SizeUnit: key, Total: round3(total * count), Unit: base}
}
