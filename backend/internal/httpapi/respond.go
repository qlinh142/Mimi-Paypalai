package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"golang.org/x/text/unicode/norm"
)

type errorBody struct {
	Code      apperr.Code    `json:"code"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	Retryable bool           `json:"retryable"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store") // dữ liệu cá nhân, không để CDN hay proxy giữ
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError là nơi duy nhất biến lỗi thành response. Lỗi 5xx được log kèm nguyên nhân
// gốc; client chỉ thấy mã và câu ngắn.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	ae := apperr.From(err)
	status := ae.Status()
	if status >= 500 {
		s.log.ErrorContext(r.Context(), "request failed", "code", ae.Code, "err", err)
	} else {
		// Lỗi 4xx thường là lỗi người dùng, chỉ ghi mã. Riêng khi cổng thanh toán từ chối
		// thì ghi kèm lý do cổng trả về (issue của PayPal, Braintree) để tra được.
		if ae.Code == apperr.CodePaymentFailed {
			s.log.WarnContext(r.Context(), "payment rejected", "code", ae.Code, "err", err)
		} else {
			s.log.InfoContext(r.Context(), "request rejected", "code", ae.Code)
		}
	}
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "30")
	}
	writeJSON(w, status, errorBody{Code: ae.Code, Message: ae.Message, Fields: ae.Fields, Retryable: ae.Retryable()})
}

// decodeJSON đọc đúng một object JSON, từ chối trường lạ (chống ghi đè trường không
// được phép) và giới hạn kích thước.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return apperr.New(apperr.CodePayloadTooLarge, "Dữ liệu gửi lên quá lớn.")
		}
		return apperr.Wrap(apperr.CodeBadRequest, "Dữ liệu gửi lên không đúng định dạng.", err)
	}
	// A2: mọi chữ trong body về dạng NFC trước khi giải mã, để tên món, tên kế hoạch, ghi
	// chú gõ bằng dấu rời cũng lưu và so sánh giống hệt bản gõ dựng sẵn. Ký tự cấu trúc của
	// JSON đều là ASCII nên không bị ảnh hưởng.
	dec := json.NewDecoder(bytes.NewReader(norm.NFC.Bytes(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Wrap(apperr.CodeBadRequest, "Dữ liệu gửi lên không đúng định dạng.", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.New(apperr.CodeBadRequest, "Chỉ gửi một đối tượng JSON.")
	}
	return nil
}
