package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// A2: tên gửi lên bằng dấu rời được lưu ở dạng dựng sẵn.
func TestDecodeJSONNormalizesNFC(t *testing.T) {
	body := "{\"name\":\"Dầu gội\"}"
	r := httptest.NewRequest("POST", "/x", strings.NewReader(body))
	var v struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(httptest.NewRecorder(), r, &v, 1<<10); err != nil {
		t.Fatal(err)
	}
	if v.Name != "Dầu gội" {
		t.Fatalf("%q", v.Name)
	}
	big := httptest.NewRequest("POST", "/x", strings.NewReader("{\"name\":\""+strings.Repeat("a", 2000)+"\"}"))
	if err := decodeJSON(httptest.NewRecorder(), big, &v, 1<<10); err == nil || !strings.Contains(err.Error(), "quá lớn") {
		t.Fatalf("want too large, got %v", err)
	}
}
