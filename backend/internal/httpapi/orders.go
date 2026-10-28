package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/order"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// maxDeliveryImages: ảnh hàng nhận gửi cho Verify.
const maxDeliveryImages = 4

func (s *Server) getMandate(w http.ResponseWriter, r *http.Request) {
	m, err := s.orders.GetMandate(r.Context(), principalFrom(r.Context()))
	s.respond(w, r, http.StatusOK, m, err)
}

func (s *Server) putMandate(w http.ResponseWriter, r *http.Request) {
	var m order.Mandate
	if err := decodeJSON(w, r, &m, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	saved, err := s.orders.SetMandate(r.Context(), principalFrom(r.Context()), m)
	s.respond(w, r, http.StatusOK, saved, err)
}

type keysBody struct {
	Keys []string `json:"keys"`
}

func (s *Server) postBasket(w http.ResponseWriter, r *http.Request) {
	var body keysBody
	if err := decodeJSON(w, r, &body, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	b, err := s.orders.Basket(r.Context(), principalFrom(r.Context()), body.Keys)
	s.respond(w, r, http.StatusOK, b, err)
}

func (s *Server) postOrder(w http.ResponseWriter, r *http.Request) {
	var req order.CheckoutRequest
	if err := decodeJSON(w, r, &req, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	o, err := s.orders.Checkout(r.Context(), principalFrom(r.Context()), req)
	s.respond(w, r, http.StatusCreated, o, err)
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	list, err := s.orders.List(r.Context(), principalFrom(r.Context()))
	s.respond(w, r, http.StatusOK, map[string]any{"orders": list}, err)
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	o, err := s.orders.Get(r.Context(), principalFrom(r.Context()), r.PathValue("id"))
	s.respond(w, r, http.StatusOK, o, err)
}

func (s *Server) captureOrder(w http.ResponseWriter, r *http.Request) {
	o, err := s.orders.Capture(r.Context(), principalFrom(r.Context()), r.PathValue("id"))
	s.respond(w, r, http.StatusOK, o, err)
}

func (s *Server) advanceOrder(w http.ResponseWriter, r *http.Request) {
	o, err := s.orders.Advance(r.Context(), principalFrom(r.Context()), r.PathValue("id"))
	s.respond(w, r, http.StatusOK, o, err)
}

// readDelivery nhận multipart "images" (1..4 ảnh), đọc thẳng vào bộ nhớ như hóa đơn;
// ảnh không được lưu.
func (s *Server) readDelivery(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
	mr, err := r.MultipartReader()
	if err != nil {
		s.writeError(w, r, apperr.New(apperr.CodeBadRequest, "Cần gửi ảnh dạng multipart."))
		return
	}
	var imgs []receipt.Image
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			s.writeError(w, r, uploadErr(err))
			return
		}
		if part.FormName() == "images" {
			if len(imgs) == maxDeliveryImages {
				s.writeError(w, r, apperr.New(apperr.CodeValidation, "Tối đa 4 ảnh hàng nhận."))
				return
			}
			data, err := io.ReadAll(io.LimitReader(part, maxImageBytes+1))
			if err != nil {
				s.writeError(w, r, uploadErr(err))
				return
			}
			if len(data) > maxImageBytes {
				s.writeError(w, r, apperr.New(apperr.CodePayloadTooLarge, "Ảnh quá lớn."))
				return
			}
			mt, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if !receipt.ValidImage(mt, data) {
				s.writeError(w, r, apperr.New(apperr.CodeUnsupportedMedia, "Chỉ nhận ảnh JPEG, PNG hoặc WebP."))
				return
			}
			imgs = append(imgs, receipt.Image{Data: data, MIME: mt})
		}
		_ = part.Close()
	}
	obs, err := s.orders.ReadDelivery(r.Context(), principalFrom(r.Context()), r.PathValue("id"), imgs)
	s.respond(w, r, http.StatusOK, map[string]any{"items": obs}, err)
}

func (s *Server) verifyOrder(w http.ResponseWriter, r *http.Request) {
	var req order.VerifyRequest
	if err := decodeJSON(w, r, &req, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	o, err := s.orders.Verify(r.Context(), principalFrom(r.Context()), r.PathValue("id"), req)
	s.respond(w, r, http.StatusOK, o, err)
}

func (s *Server) refundOrder(w http.ResponseWriter, r *http.Request) {
	var req order.RefundRequest
	if err := decodeJSON(w, r, &req, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	o, err := s.orders.Refund(r.Context(), principalFrom(r.Context()), r.PathValue("id"), req)
	s.respond(w, r, http.StatusOK, o, err)
}

// respond ghi lỗi hoặc JSON với mã status.
func (s *Server) respond(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, status, v)
}
