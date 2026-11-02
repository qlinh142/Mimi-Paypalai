package taxonomy

import (
	_ "embed"
	"encoding/json"
)

// categories.json là nguồn duy nhất của danh mục chi tiêu (khóa và nhãn vi/en). Frontend
// dùng bản chép ở frontend/src/i18n/categories.json; TestCategoriesFrontendCopy giữ hai file giống nhau.
//
//go:embed categories.json
var categoriesJSON []byte

// Hai danh mục hệ thống tự gán, không đưa cho AI chọn.
const (
	CategoryRecurring     = "hoa-don-dinh-ky" // khoản chi định kỳ (recurring)
	CategoryUncategorized = "chua-phan-loai"  // dòng chưa có danh mục
)

// Category là một danh mục chi tiêu.
type Category struct {
	Key    string `json:"key"`
	Labels Labels `json:"labels"`
}

type categoryFile struct {
	Version    int        `json:"version"`
	Categories []Category `json:"categories"`
	System     []Category `json:"system"`
}

var (
	spendingCats []Category
	catByKey     = map[string]Category{}
)

func init() {
	var f categoryFile
	if err := json.Unmarshal(categoriesJSON, &f); err != nil {
		panic("taxonomy: categories.json: " + err.Error())
	}
	spendingCats = f.Categories
	for _, c := range append(append([]Category{}, f.Categories...), f.System...) {
		if _, dup := catByKey[c.Key]; dup || c.Key == "" {
			panic("taxonomy: categories.json: khóa trùng hoặc rỗng " + c.Key)
		}
		catByKey[c.Key] = c
	}
}

// SpendingKeys trả các khóa danh mục AI và người dùng được chọn (không gồm danh mục hệ thống).
func SpendingKeys() []string {
	out := make([]string, len(spendingCats))
	for i, c := range spendingCats {
		out[i] = c.Key
	}
	return out
}

// CategoryOf tra một danh mục (kể cả danh mục hệ thống).
func CategoryOf(key string) (Category, bool) {
	c, ok := catByKey[key]
	return c, ok
}
