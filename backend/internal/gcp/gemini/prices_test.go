package gemini

import "testing"

func TestParsePrices(t *testing.T) {
	got, err := parsePrices("```json\n{\"prices\":[{\"store\":\"WinMart\",\"price_vnd\":32000,\"size\":\"1L\",\"url\":\"https://winmart.vn/x\"}]}\n```")
	if err != nil || len(got) != 1 || got[0].Store != "WinMart" || got[0].PriceVND != 32000 || got[0].Size != "1L" {
		t.Fatalf("%+v %v", got, err)
	}
	if got, err := parsePrices(`{"prices":[]}`); err != nil || len(got) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := parsePrices("Xin lỗi, tôi không tìm thấy"); err == nil {
		t.Fatal("prose must be an error")
	}
}
