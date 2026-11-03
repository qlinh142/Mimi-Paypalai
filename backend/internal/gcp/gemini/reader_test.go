package gemini

import (
	"errors"
	"testing"

	"google.golang.org/genai"

	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// Vertex AI trả 400 khi enum có chuỗi rỗng; lỗi đó từng hiện thành "chụp lại rõ hơn".
func TestSchemaEnumsHaveNoEmptyValue(t *testing.T) {
	r := &Reader{taxonomy: []string{"thuc-pham", "do-uong"}, groups: []string{"sua"}}
	var walk func(path string, s *genai.Schema)
	walk = func(path string, s *genai.Schema) {
		if s == nil {
			return
		}
		for _, v := range s.Enum {
			if v == "" {
				t.Errorf("%s: enum has empty value", path)
			}
		}
		for k, p := range s.Properties {
			walk(path+"."+k, p)
		}
		walk(path+"[]", s.Items)
	}
	walk("$", r.schema())
}

func TestClassify(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{400, receipt.ErrAIRejected},
		{403, receipt.ErrAIRejected},
		{404, receipt.ErrAIRejected},
		{429, receipt.ErrTransient},
		{503, receipt.ErrTransient},
	}
	for _, c := range cases {
		err := classify(genai.APIError{Code: c.code, Message: "x"})
		if !errors.Is(err, c.want) || errors.Is(err, receipt.ErrUnreadable) {
			t.Errorf("%d: got %v", c.code, err)
		}
	}
}
