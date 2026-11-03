package gemini

import (
	"strings"
	"testing"

	"github.com/smartlife-lab/mimi/backend/internal/market"
)

func TestParseReply(t *testing.T) {
	got, err := parseReply("```json\n{\"items\":[{\"id\":1,\"prices\":[{\"store\":\"WinMart\",\"price\":32000,\"size\":\"1L\",\"url\":\"https://winmart.vn/x\"}]}]," +
		"\"pages\":[{\"store\":\"WinMart\",\"kind\":\"search\",\"search_url\":\"https://winmart.vn/search?q={q}\"}]}\n```")
	if err != nil || len(got.Items) != 1 || got.Items[0].Prices[0].Price != 32000 || got.Pages[0].SearchURL == "" {
		t.Fatalf("%+v %v", got, err)
	}
	if got, err := parseReply(`{"items":[],"pages":[]}`); err != nil || len(got.Items) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := parseReply("Xin lỗi, tôi không tìm thấy"); err == nil {
		t.Fatal("prose must be an error")
	}
}

func TestPricePromptNamesCountryCurrencyAndPages(t *testing.T) {
	p := pricePrompt(market.Countries["SG"], []market.Wanted{{Name: "Fresh milk 1L"}, {Name: "Eggs 10"}},
		[]market.Page{{URL: "https://www.fairprice.com.sg/category/milk"}})
	for _, want := range []string{"Singapore, in SGD", `1: "Fresh milk 1L"`, `2: "Eggs 10"`, "FairPrice (fairprice.com.sg)", "https://www.fairprice.com.sg/category/milk", "official"} {
		if !strings.Contains(p, want) {
			t.Fatalf("missing %q in\n%s", want, p)
		}
	}
	if strings.Contains(pricePrompt(market.Countries["VN"], []market.Wanted{{Name: "Sữa"}}, nil), "First read") {
		t.Fatal("no pages: google only")
	}
}
