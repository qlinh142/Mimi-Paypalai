package domain

import "strings"

// Chuẩn hóa tên cửa hàng (8.1 mục 15): "WinMart", "WINMART", "Win Mart", "WinMart+",
// "WIN MART BAC NINH" là một store_key. So giá luôn dùng store_key; tên in trên bill giữ
// nguyên ở receipt để audit.
var storeChains = []struct {
	key, display string
	prefixes     []string // đã bỏ dấu, bỏ khoảng trắng và dấu câu
}{
	{"winmart", "WinMart", []string{"winmart", "vinmart"}},
	{"coopmart", "Co.opmart", []string{"coopmart", "coopfood", "coopxtra", "coopextra"}},
	{"bach-hoa-xanh", "Bách Hóa Xanh", []string{"bachhoaxanh", "bhx"}},
	{"aeon", "AEON", []string{"aeon"}},
	{"lotte-mart", "Lotte Mart", []string{"lottemart", "lotte"}},
	{"go", "GO!", []string{"bigc", "gosupermarket", "gomart", "tops"}},
	{"mm-mega-market", "MM Mega Market", []string{"mmmegamarket", "megamarket", "metro"}},
	{"circle-k", "Circle K", []string{"circlek"}},
	{"familymart", "FamilyMart", []string{"familymart"}},
	{"gs25", "GS25", []string{"gs25"}},
	{"7-eleven", "7-Eleven", []string{"7eleven"}},
	{"ministop", "Ministop", []string{"ministop"}},
	{"satrafoods", "Satrafoods", []string{"satrafoods", "satra"}},
	{"guardian", "Guardian", []string{"guardian"}},
	{"hasaki", "Hasaki", []string{"hasaki"}},
	{"shopee", "Shopee", []string{"shopee"}},
	{"lazada", "Lazada", []string{"lazada"}},
	{"tiki", "Tiki", []string{"tiki"}},
}

func compact(s string) string {
	var b strings.Builder
	for _, r := range NormalizeVi(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// StoreKey trả khóa và tên hiển thị của cửa hàng. Chuỗi siêu thị biết trước gộp mọi chi
// nhánh; cửa hàng khác dùng slug của tên in trên bill.
func StoreKey(name string) (key, display string) {
	c := compact(name)
	if c == "" {
		return "", ""
	}
	for _, ch := range storeChains {
		for _, p := range ch.prefixes {
			if strings.HasPrefix(c, p) {
				return ch.key, ch.display
			}
		}
	}
	return Slug(name), FormatField(CleanItemName(name))
}
