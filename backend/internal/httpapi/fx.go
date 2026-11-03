package httpapi

import (
	"net/http"
	"slices"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/fx"
)

// getFXRates: GET /fx/rates?currency=USD&from=YYYY-MM-DD&to=YYYY-MM-DD. Tỷ giá từng ngày để app
// quy đổi số tiền (lưu bằng VND) sang tiền tệ hiển thị theo đúng ngày mua (quy tắc FX2, FX4).
// Vắng to: hôm nay. Vắng from: to lùi MaxHistoryDays ngày.
func (s *Server) getFXRates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cur := q.Get("currency")
	if cur == "VND" || !slices.Contains(fx.Currencies, cur) {
		s.writeError(w, r, apperr.New(apperr.CodeValidation, "Tiền tệ không hỗ trợ.").With("field", "currency"))
		return
	}
	to := domain.Today(time.Now(), domain.Location("Asia/Ho_Chi_Minh"))
	if v := q.Get("to"); v != "" {
		d, err := domain.ParseDate(v)
		if err != nil {
			s.writeError(w, r, apperr.New(apperr.CodeValidation, "Ngày không hợp lệ.").With("field", "to"))
			return
		}
		to = d
	}
	from := to.AddDays(-fx.MaxHistoryDays)
	if v := q.Get("from"); v != "" {
		d, err := domain.ParseDate(v)
		if err != nil || d.After(to) || d.DaysUntil(to) > fx.MaxHistoryDays {
			s.writeError(w, r, apperr.New(apperr.CodeValidation, "Khoảng ngày không hợp lệ (tối đa 800 ngày).").With("field", "from"))
			return
		}
		from = d
	}
	pts, err := s.fx.RateHistory(r.Context(), cur, from, to)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	writeJSON(w, http.StatusOK, map[string]any{"currency": cur, "rates": pts})
}
