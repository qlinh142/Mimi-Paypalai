package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// NFC đưa chuỗi về dạng dựng sẵn. Cùng chữ "ầ" có thể đến dưới hai dạng: một ký tự
// (bàn phím điện thoại, Windows) hoặc "a" + dấu mũ + dấu huyền rời (dán từ macOS, một số
// trình duyệt, chữ AI đọc từ ảnh). Hai dạng nhìn giống hệt nhưng so sánh byte thì khác,
// nên mọi chữ người dùng gửi lên đều qua đây trước khi lưu hay so sánh.
func NFC(s string) string { return norm.NFC.String(s) }

// viFold ánh xạ mọi chữ có dấu của tiếng Việt về chữ không dấu. Dùng bảng tĩnh thay vì
// golang.org/x/text: đủ cho tiếng Việt, không thêm phụ thuộc. "đ" phải map riêng vì
// chuẩn hóa NFD cũng không tách được chữ này.
var viFold = func() map[rune]rune {
	groups := map[rune]string{
		'a': "àáảãạăằắẳẵặâầấẩẫậ",
		'e': "èéẻẽẹêềếểễệ",
		'i': "ìíỉĩị",
		'o': "òóỏõọôồốổỗộơờớởỡợ",
		'u': "ùúủũụưừứửữự",
		'y': "ỳýỷỹỵ",
		'd': "đ",
	}
	m := make(map[rune]rune, 160)
	for base, accented := range groups {
		for _, r := range accented {
			m[r] = base
			m[unicode.ToUpper(r)] = base
		}
	}
	return m
}()

// NormalizeVi chuẩn hóa để tìm không dấu: "Dầu gội" và "dau goi" khớp nhau, dù chữ gõ
// ở dạng dựng sẵn hay dấu rời. Chữ được tách hẳn (NFD) rồi bỏ mọi dấu, nên kết quả
// giống nhau với mọi cách gõ.
func NormalizeVi(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if base, ok := viFold[r]; ok {
			b.WriteRune(base)
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// MaxItemNameRunes giới hạn tên món đọc từ hóa đơn.
const MaxItemNameRunes = 120

// CleanItemName làm sạch tên món do AI đọc: bỏ ký tự điều khiển, gộp khoảng trắng,
// cắt độ dài. Tên món là dữ liệu, không bao giờ được dùng làm chỉ dẫn cho model.
func CleanItemName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = NFC(strings.Join(strings.Fields(s), " "))
	if utf8.RuneCountInString(s) > MaxItemNameRunes {
		s = string([]rune(s)[:MaxItemNameRunes])
	}
	return s
}
