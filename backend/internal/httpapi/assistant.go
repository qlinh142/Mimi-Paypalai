package httpapi

import (
	"net/http"

	"github.com/smartlife-lab/mimi/backend/internal/assistant"
)

// aiMaxBytes: hội thoại kèm khai báo tool của các agent AG Studio có thể khá dài.
const aiMaxBytes = 1 << 20

// aiTurn: POST /ai/turn. Một lượt Hỏi Mimi: app gửi hội thoại và các tool, máy chủ gọi Gemini
// và trả về câu trả lời hoặc lời đề nghị gọi tool. Tool luôn chạy ở app.
func (s *Server) aiTurn(w http.ResponseWriter, r *http.Request) {
	var req assistant.Request
	if err := decodeJSON(w, r, &req, aiMaxBytes); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.assistant.Turn(r.Context(), principalFrom(r.Context()).UID, req)
	s.reply(w, r, http.StatusOK, out, err)
}
