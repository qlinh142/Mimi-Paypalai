// Package assistant chạy một lượt hội thoại của "Hỏi Mimi" (màn Phân tích chi tiêu, AG Studio).
//   - Trình duyệt giữ vòng lặp agent và tự chạy tool (đọc dữ liệu đã có trên máy); máy chủ chỉ
//     gọi mô hình một lượt rồi trả câu trả lời. Không có khóa nào nằm ở trình duyệt.
//   - Mô hình không ghi gì vào dữ liệu: nó chỉ đề xuất gọi tool, và tool chạy ở app với quyền
//     của chính người dùng.
//   - Giới hạn kích thước mỗi lượt và số lượt mỗi ngày cho từng người, để không ai dùng máy chủ
//     làm proxy LLM miễn phí.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
)

// Part là một phần của tin nhắn, cùng dạng với REST của Gemini (text, functionCall, functionResponse).
// ThoughtSignature (base64) là chữ ký Gemini 3 gắn vào lượt gọi tool; app gửi trả lại nguyên
// vẹn ở lượt sau, thiếu thì Gemini từ chối hội thoại có gọi tool.
type Part struct {
	Text             string            `json:"text,omitempty"`
	ThoughtSignature string            `json:"thoughtSignature,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

type FunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type FunctionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type Content struct {
	Role  string `json:"role"` // user | model
	Parts []Part `json:"parts"`
}

// Tool là khai báo một hàm mô hình được phép đề xuất gọi; Parameters là JSON Schema.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// Request là một lượt: lời dặn hệ thống, hội thoại đến giờ, các tool đang có.
type Request struct {
	System       string          `json:"system,omitempty"`
	Contents     []Content       `json:"contents"`
	Tools        []Tool          `json:"tools,omitempty"`
	ToolMode     string          `json:"tool_mode,omitempty"` // AUTO | ANY | NONE
	AllowedTools []string        `json:"allowed_tools,omitempty"`
	JSONSchema   json.RawMessage `json:"json_schema,omitempty"`
	Model        string          `json:"model,omitempty"` // bỏ qua: máy chủ dùng model đã cấu hình
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type Response struct {
	ID           string `json:"id"`
	Model        string `json:"model,omitempty"`
	Parts        []Part `json:"parts"`
	FinishReason string `json:"finish_reason,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
}

// Model gọi mô hình một lượt (Gemini trên Vertex AI ở bản chạy thật).
type Model interface {
	Turn(ctx context.Context, req Request) (Response, error)
}

const (
	MaxContents = 120 // tin nhắn mỗi lượt
	MaxTools    = 64
)

// Validate chặn lượt rỗng hoặc sai dạng trước khi tốn tiền gọi mô hình.
func (r Request) Validate() error {
	if len(r.Contents) == 0 {
		return apperr.New(apperr.CodeValidation, "Chưa có câu hỏi.")
	}
	if len(r.Contents) > MaxContents || len(r.Tools) > MaxTools {
		return apperr.New(apperr.CodePayloadTooLarge, "Cuộc trò chuyện quá dài, mở cuộc mới nhé.")
	}
	for _, c := range r.Contents {
		if c.Role != "user" && c.Role != "model" {
			return apperr.New(apperr.CodeValidation, "Vai trò tin nhắn không hợp lệ.")
		}
	}
	switch r.ToolMode {
	case "", "AUTO", "ANY", "NONE":
	default:
		return apperr.New(apperr.CodeValidation, "tool_mode không hợp lệ.")
	}
	for _, t := range r.Tools {
		if t.Name == "" {
			return apperr.New(apperr.CodeValidation, "Tool thiếu tên.")
		}
	}
	return nil
}

// Service gắn giới hạn lượt mỗi ngày quanh Model.
type Service struct {
	model Model
	daily int
	now   func() time.Time

	mu   sync.Mutex
	used map[string]int // uid|YYYY-MM-DD → số lượt
}

// New: daily <= 0 thì không giới hạn (chỉ dùng khi chạy ở máy).
func New(m Model, daily int) *Service {
	return &Service{model: m, daily: daily, now: time.Now, used: map[string]int{}}
}

// Turn chạy một lượt cho người dùng uid. Đếm tại instance này; Cloud Run nhiều instance thì
// tổng có thể vượt một chút, chấp nhận được cho giới hạn chống lạm dụng.
func (s *Service) Turn(ctx context.Context, uid string, req Request) (Response, error) {
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	if s.daily > 0 {
		key := uid + "|" + s.now().UTC().Format("2006-01-02")
		s.mu.Lock()
		if s.used[key] >= s.daily {
			s.mu.Unlock()
			return Response{}, apperr.New(apperr.CodeQuotaExceeded, "Hôm nay Mimi đã trả lời đủ lượt, mai hỏi tiếp nhé.")
		}
		s.used[key]++
		if len(s.used) > 10_000 { // dọn ngày cũ
			for k := range s.used {
				if k != key {
					delete(s.used, k)
				}
			}
		}
		s.mu.Unlock()
	}
	out, err := s.model.Turn(ctx, req)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) {
			return Response{}, err
		}
		return Response{}, apperr.Wrap(apperr.CodeAIUnavailable, "Mimi chưa trả lời được, thử lại sau ít phút.", err)
	}
	if out.Parts == nil {
		out.Parts = []Part{}
	}
	return out, nil
}
