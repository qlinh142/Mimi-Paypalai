package httpapi

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A3: OpenAPI là hợp đồng giữa máy chủ và app (app sinh kiểu từ file này bằng
// `npm run gen:api`). Test này bắt hai loại lệch: route có trên máy chủ mà thiếu trong
// OpenAPI, và thao tác trong OpenAPI mà máy chủ không còn phục vụ. Nó cũng bắt file YAML
// hỏng (khóa trùng), lỗi từng khiến gen:api không chạy được.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	spec, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatalf("openapi.yaml không đọc được: %v", err)
	}
	documented := map[string]bool{}
	for path, ops := range doc.Paths {
		for method := range ops {
			switch method {
			case "get", "put", "post", "patch", "delete":
				documented[strings.ToUpper(method)+" "+path] = true
			}
		}
	}

	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	served := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?:public|private)\("([A-Z]+ /[^"]*)"`).FindAllSubmatch(src, -1) {
		served[string(m[1])] = true
	}
	for _, r := range notYetImplemented { // vẫn có route, trả 501 đến khi nối xong
		served[r] = true
	}
	if len(served) < 20 {
		t.Fatalf("chỉ tìm thấy %d route, regex có thể đã lỗi thời", len(served))
	}

	var missing, stale []string
	for r := range served {
		if !documented[r] {
			missing = append(missing, r)
		}
	}
	for r := range documented {
		if !served[r] {
			stale = append(stale, r)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("route chưa có trong openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("openapi.yaml có thao tác máy chủ không phục vụ:\n  %s", strings.Join(stale, "\n  "))
	}
}
