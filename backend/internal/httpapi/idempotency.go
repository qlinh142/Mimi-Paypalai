package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/idem"
)

// Giới hạn: request lớn hơn maxIdemBody (ảnh, multipart) không được giữ theo key; kết quả
// lớn hơn maxIdemResult không lưu được (tài liệu Firestore tối đa 1 MiB).
const (
	maxIdemBody   = 2 << 20
	maxIdemResult = 900 << 10
)

var idemKeyRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

// WithIdempotency bật giữ kết quả theo Idempotency-Key cho mọi route ghi.
func WithIdempotency(st idem.Store) Option { return func(s *Server) { s.idem = st } }

// idempotent (data model 1.5 thay đổi 5): request ghi có Idempotency-Key thì
//   - lần đầu: xử lý bình thường, lưu mã và nội dung trả về;
//   - gửi lại cùng key và cùng nội dung: trả lại đúng kết quả cũ, không ghi lần hai;
//   - cùng key nhưng nội dung khác: 422, vì client dùng lại key sai;
//   - cùng key đang xử lý dở: 409 retryable, client gửi lại sau.
//
// Lỗi tạm thời (5xx, 408, 429, hoặc lỗi có retryable: true) không được lưu, để lần gửi sau được xử lý lại.
// Request không có key vẫn chạy như cũ (ảnh hóa đơn gửi multipart không có key).
func (s *Server) idempotent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		// DELETE /me xóa cả users/{uid}/idempotency; ghi kết quả sau đó sẽ để lại rác của
		// tài khoản đã xóa. Gọi lại DELETE /me vốn an toàn nên không cần giữ kết quả.
		if s.idem == nil || key == "" || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Pattern == "DELETE /me" {
			next.ServeHTTP(w, r)
			return
		}
		if !idemKeyRe.MatchString(key) {
			s.writeError(w, r, apperr.New(apperr.CodeBadRequest, "Idempotency-Key không hợp lệ."))
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxIdemBody+1))
		if err != nil {
			s.writeError(w, r, apperr.Wrap(apperr.CodeBadRequest, "Không đọc được dữ liệu gửi lên.", err))
			return
		}
		if len(body) > maxIdemBody {
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))
			next.ServeHTTP(w, r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		uid := principalFrom(r.Context()).UID
		fp := fingerprint(r.Method, r.URL.Path, body)
		rec, err := s.idem.Begin(r.Context(), uid, key, fp, time.Now())
		switch {
		case errors.Is(err, idem.ErrMismatch):
			s.writeError(w, r, apperr.New(apperr.CodeIdempotencyMismatch, "Mã gửi lại đã dùng cho thao tác khác."))
			return
		case errors.Is(err, idem.ErrInFlight):
			s.writeError(w, r, apperr.New(apperr.CodeIdempotencyInFlight, "Thao tác này đang được xử lý, thử lại sau ít giây."))
			return
		case err != nil:
			s.writeError(w, r, err)
			return
		case rec != nil:
			if rec.ContentType != "" {
				w.Header().Set("Content-Type", rec.ContentType)
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(rec.Status)
			_, _ = w.Write(rec.Body)
			return
		}

		cw := &captureWriter{ResponseWriter: w, status: http.StatusOK}
		done := false
		defer func() {
			// Request có thể đã hết giờ (TimeoutHandler hủy ctx) nhưng việc ghi vẫn xong:
			// vẫn lưu kết quả để lần gửi lại không ghi trùng.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
			defer cancel()
			if !done || cw.overflow || temporary(cw.status, cw.buf.Bytes()) {
				if err := s.idem.Abort(ctx, uid, key); err != nil {
					s.log.WarnContext(ctx, "idempotency abort failed", "err", err)
				}
				return
			}
			rec := idem.Record{Fingerprint: fp, Status: cw.status, ContentType: cw.Header().Get("Content-Type"), Body: cw.buf.Bytes()}
			if err := s.idem.Finish(ctx, uid, key, rec, time.Now()); err != nil {
				s.log.WarnContext(ctx, "idempotency finish failed", "err", err)
			}
		}()
		next.ServeHTTP(cw, r)
		done = true
	})
}

// temporary: kết quả không nên giữ lại vì gửi lại có thể thành công.
func temporary(status int, body []byte) bool {
	if status >= 500 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests {
		return true
	}
	if status < 400 {
		return false
	}
	var e struct {
		Retryable bool `json:"retryable"`
	}
	return json.Unmarshal(body, &e) == nil && e.Retryable
}

func fingerprint(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method + " " + path + "\n"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// captureWriter chép lại mã và nội dung trả về (tối đa maxIdemResult) trong khi vẫn gửi
// cho client như bình thường.
type captureWriter struct {
	http.ResponseWriter
	status   int
	buf      bytes.Buffer
	overflow bool
}

func (c *captureWriter) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(p []byte) (int, error) {
	if !c.overflow {
		if c.buf.Len()+len(p) > maxIdemResult {
			c.overflow = true
			c.buf.Reset()
		} else {
			c.buf.Write(p)
		}
	}
	return c.ResponseWriter.Write(p)
}
