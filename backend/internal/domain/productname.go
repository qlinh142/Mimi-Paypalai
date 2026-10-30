package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Quy tắc viết tên sản phẩm sau khi quét bill (tài liệu 7.1):
//   - Không dùng kiểu viết hoa của OCR: "VINAMILK", "vinamilk", "ViNaMiLk" là một giá trị.
//   - Mỗi field text chỉ viết hoa ký tự đầu của field, không Title Case từng từ.
//   - Brand dictionary và taxonomy được quyền override cách viết mặc định.
//   - Khối lượng / số lượng không nằm trong tên; marketing text không nằm trong tên chuẩn.

// keepUpper: mã hay gặp trên bao bì phải giữ chữ hoa (cỡ, loại pin, chỉ số chống nắng).
var keepUpper = map[string]bool{
	"xs": true, "xl": true, "xxl": true, "xxxl": true, "aa": true, "aaa": true,
	"uv": true, "uht": true, "spf": true, "pa": true, "dha": true, "ara": true,
}

// FormatField viết hoa ký tự đầu của field, phần còn lại chữ thường. Ngoại lệ có chủ đích
// (AC 7.1 "trừ canonical special case"): mã ngắn chứa cả chữ lẫn số (ST25, B5, H2O), mã
// trong keepUpper, và chữ cỡ đứng sau "size" (Size L).
func FormatField(s string) string {
	words := strings.Fields(CleanItemName(s))
	for i, w := range words {
		lw := strings.ToLower(w)
		switch {
		case isShortCode(lw), keepUpper[lw]:
			words[i] = strings.ToUpper(lw)
		case i > 0 && strings.EqualFold(words[i-1], "size") && utf8.RuneCountInString(lw) <= 3:
			words[i] = strings.ToUpper(lw)
		default:
			words[i] = lw
		}
	}
	out := strings.Join(words, " ")
	r, n := utf8.DecodeRuneInString(out)
	if r == utf8.RuneError {
		return out
	}
	return string(unicode.ToUpper(r)) + out[n:]
}

// isShortCode: tối đa 5 ký tự, có cả chữ và số, chữ không quá 3 (ST25, B5, H2O; không phải 3in1).
func isShortCode(w string) bool {
	if utf8.RuneCountInString(w) > 5 {
		return false
	}
	letters, digits := 0, 0
	for _, r := range w {
		switch {
		case unicode.IsDigit(r):
			digits++
		case unicode.IsLetter(r):
			letters++
		default:
			return false
		}
	}
	return digits > 0 && letters > 0 && letters <= 3 && !strings.Contains(w, "in")
}

// marketing: đoạn quảng cáo / claim phải bỏ khỏi tên chuẩn hóa (0.1 mục 5.5). So khớp
// trên chuỗi đã bỏ dấu; mỗi mẫu là một cụm từ đầy đủ.
var marketing = []string{
	"37 duong chat", "cong thuc co chi so duong huyet thap", "chi so duong huyet thap", "bo sung dinh duong",
	"tang suc de khang", "danh bay vet ban", "thom lau", "cao cap", "best seller", "sieu tiet kiem",
	"cong nghe dot pha", "cong nghe moi", "san pham moi", "hang moi", "chinh hang", "gia soc", "khuyen mai",
}

// StripMarketing bỏ claim marketing và phần mô tả sau dấu " - " (thường là slogan trên
// sàn TMĐT), giữ lại brand, variant, dòng sản phẩm.
func StripMarketing(s string) string {
	s = CleanItemName(s)
	if i := strings.Index(s, " - "); i > 0 {
		s = s[:i]
	}
	words := strings.Fields(s)
	norm := make([]string, len(words))
	for i, w := range words {
		norm[i] = NormalizeVi(w)
	}
	drop := make([]bool, len(words))
	for _, m := range marketing {
		mw := strings.Fields(m)
		for i := 0; i+len(mw) <= len(words); i++ {
			match := true
			for j := range mw {
				if norm[i+j] != mw[j] {
					match = false
					break
				}
			}
			if match {
				for j := range mw {
					drop[i+j] = true
				}
			}
		}
	}
	kept := words[:0:0]
	for i, w := range words {
		if !drop[i] && norm[i] != "moi" {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

// StripQuantity bỏ phần khối lượng / số lượng khỏi tên (7.1 mục 7): "Sure diacerna 900g" →
// "Sure diacerna". Chỉ bỏ cụm số + đơn vị và hệ số "2x", không đụng tới mã như ST25.
func StripQuantity(s string) string {
	words := strings.Fields(strings.NewReplacer("×", " x ", "*", " x ").Replace(s))
	out := words[:0:0]
	for i := 0; i < len(words); i++ {
		w := NormalizeVi(words[i])
		if isQtyToken(w) {
			continue
		}
		if isNumber(w) && i+1 < len(words) && (NormalizeUnit(words[i+1]) != "" || NormalizeVi(words[i+1]) == "x") {
			i++
			continue
		}
		if w == "x" {
			continue
		}
		out = append(out, words[i])
	}
	return strings.Join(out, " ")
}

// isQtyToken: "900g", "1.5l", "4x180ml", "2x".
func isQtyToken(w string) bool {
	if w == "" || !unicode.IsDigit(rune(w[0])) {
		return false
	}
	i := 0
	for i < len(w) && (unicode.IsDigit(rune(w[i])) || w[i] == '.' || w[i] == ',' || w[i] == 'x') {
		i++
	}
	rest := w[i:]
	if rest == "" {
		return strings.HasSuffix(w, "x")
	}
	return NormalizeUnit(rest) != "" && rest != "in"
}

func isNumber(w string) bool {
	if w == "" {
		return false
	}
	for _, r := range w {
		if !unicode.IsDigit(r) && r != '.' && r != ',' {
			return false
		}
	}
	return true
}

// Slug tạo khóa ổn định từ chữ hiển thị: bỏ dấu, chữ thường, gạch nối, tối đa 64 ký tự.
func Slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range NormalizeVi(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(b.String(), "-")
	if len(out) > 64 {
		out = strings.TrimSuffix(out[:64], "-")
	}
	return out
}

// ProductName là kết quả chuẩn hóa tên một dòng hóa đơn (7.1 + 1.1). Các field canonical
// không phụ thuộc ngôn ngữ giao diện; CommonName lấy từ taxonomy lúc hiển thị.
type ProductName struct {
	Raw         string // raw_name: giữ nguyên chuỗi đọc từ bill, không bao giờ ghi đè
	Brand       string
	ProductName string // tên riêng / variant, không có quy cách và marketing
	DetailName  string // Brand + ProductName, tên hiển thị ở màn chi tiết
	ProductKey  string // khóa ổn định của sản phẩm cụ thể, "" khi chưa đủ thông tin
}

// BrandLookup trả cách viết chuẩn của brand nếu có trong brand dictionary.
type BrandLookup func(brand string) (string, bool)

// NormalizeProductName áp quy tắc 7.1 lên các field AI đã tách. AI chỉ chạy một lần lúc
// đọc bill; hàm này thuần, chạy lại bao nhiêu lần cũng ra cùng kết quả.
func NormalizeProductName(raw, brand, product string, brands BrandLookup) ProductName {
	p := ProductName{Raw: CleanItemName(raw)}
	if b := strings.TrimSpace(CleanItemName(brand)); b != "" {
		if canon, ok := brands(b); ok {
			p.Brand = canon
		} else {
			p.Brand = FormatField(b)
		}
	}
	name := StripQuantity(StripMarketing(product))
	if p.Brand != "" {
		name = trimBrand(name, p.Brand)
	}
	if name != "" {
		p.ProductName = FormatField(name)
	}
	p.DetailName = strings.TrimSpace(p.Brand + " " + p.ProductName)
	if p.DetailName != "" {
		p.ProductKey = Slug(p.DetailName)
	}
	return p
}

// trimBrand bỏ brand lặp lại ở đầu tên riêng ("Vinamilk Sure Diacerna" với brand Vinamilk).
func trimBrand(name, brand string) string {
	nb := NormalizeVi(brand)
	words := strings.Fields(name)
	bw := len(strings.Fields(brand))
	if len(words) >= bw && NormalizeVi(strings.Join(words[:bw], " ")) == nb {
		return strings.Join(words[bw:], " ")
	}
	return name
}
