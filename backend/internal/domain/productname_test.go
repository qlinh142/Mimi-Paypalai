package domain

import (
	"strings"
	"testing"
)

func TestFormatField(t *testing.T) {
	cases := map[string]string{
		"SUA BOT":           "Sua bot",
		"sữa bột":           "Sữa bột",
		"dầu gội nam":       "Dầu gội nam",
		"SURE DIACERNA":     "Sure diacerna",
		"ViNaMiLk":          "Vinamilk",
		"gạo st25":          "Gạo ST25",
		"hyalu b5":          "Hyalu B5",
		"bobby size l":      "Bobby size L",
		"niacinamide 10%":   "Niacinamide 10%",
		"3in1":              "3in1",
		"  nhiều   khoảng ": "Nhiều khoảng",
	}
	for in, want := range cases {
		if got := FormatField(in); got != want {
			t.Errorf("FormatField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeProductName(t *testing.T) {
	brands := func(b string) (string, bool) {
		switch strings.ToLower(strings.ReplaceAll(b, " ", "")) {
		case "p/s":
			return "P/S", true
		case "omo":
			return "OMO", true
		}
		return "", false
	}
	cases := []struct {
		raw, brand, product string
		wantBrand, wantName string
		wantDetail, wantKey string
	}{
		// 7.1 Example 1: OCR in hoa toàn bộ, có quy cách trong tên.
		{"VINAMILK SUA BOT SURE DIACERNA 900G", "VINAMILK", "SURE DIACERNA 900G", "Vinamilk", "Sure diacerna", "Vinamilk Sure diacerna", "vinamilk-sure-diacerna"},
		// 7.1 Example 2.
		{"ROMANO CLASSIC DAU GOI NAM 650ML", "romano", "CLASSIC", "Romano", "Classic", "Romano Classic", "romano-classic"},
		// Marketing text không vào tên chuẩn; brand lặp đầu tên bị bỏ.
		{"VINAMILK SURE DIACERNA 900G 37 DUONG CHAT", "Vinamilk", "Vinamilk Sure Diacerna 37 dưỡng chất", "Vinamilk", "Sure diacerna", "Vinamilk Sure diacerna", "vinamilk-sure-diacerna"},
		{"Combo 2 lon Sữa Bột Vinamilk Sure Diacerna 900g - 37 dưỡng chất với công thức có chỉ số đường huyết thấp", "Vinamilk", "Sure Diacerna 900g - 37 dưỡng chất", "Vinamilk", "Sure diacerna", "Vinamilk Sure diacerna", "vinamilk-sure-diacerna"},
		// Brand dictionary thắng quy tắc viết hoa chữ đầu.
		{"P/S BAO VE 123 180G", "p/s", "bảo vệ 123", "P/S", "Bảo vệ 123", "P/S Bảo vệ 123", "p-s-bao-ve-123"},
		{"OMO MATIC COMFORT 3.6KG", "Omo", "MATIC COMFORT", "OMO", "Matic comfort", "OMO Matic comfort", "omo-matic-comfort"},
		// Không có brand, không có tên riêng: chưa đủ để tạo product_key.
		{"RAU CAI", "", "", "", "", "", ""},
		{"Hảo Hảo 5×75g", "Hảo Hảo", "Tôm chua cay 5x75g", "Hảo hảo", "Tôm chua cay", "Hảo hảo Tôm chua cay", "hao-hao-tom-chua-cay"},
	}
	for _, c := range cases {
		p := NormalizeProductName(c.raw, c.brand, c.product, brands)
		if p.Raw != CleanItemName(c.raw) {
			t.Errorf("raw_name must be kept: %q", p.Raw)
		}
		if p.Brand != c.wantBrand || p.ProductName != c.wantName || p.DetailName != c.wantDetail || p.ProductKey != c.wantKey {
			t.Errorf("%q: got brand=%q name=%q detail=%q key=%q", c.raw, p.Brand, p.ProductName, p.DetailName, p.ProductKey)
		}
	}
}

func TestSlug(t *testing.T) {
	if got := Slug("Vinamilk Sure Diacerna"); got != "vinamilk-sure-diacerna" {
		t.Fatal(got)
	}
	if got := Slug("  Đậu nành / Simply  "); got != "dau-nanh-simply" {
		t.Fatal(got)
	}
}
