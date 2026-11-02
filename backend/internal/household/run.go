package household

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// Spec 1.5 mục 2: người được nhờ đi mua bấm "Mua hàng xong" hoặc "Từ chối".

// ReceiptSaver là đường ghi hóa đơn duy nhất (receipt.Service.Save). Báo xong việc mua hộ
// ghi một hóa đơn từ danh sách đã mua, nên tồn kho, chi tiêu và Kế hoạch của người nhờ
// được cập nhật đúng như khi quét hóa đơn với assignment_id.
type ReceiptSaver interface {
	Save(ctx context.Context, p authz.Principal, req receipt.SaveRequest) (receipt.SaveOutcome, error)
}

func (s *Service) WithReceipts(r ReceiptSaver) *Service {
	s.receipts = r
	return s
}

// RunLine là một dòng trong danh sách đi mua của người được nhờ.
type RunLine struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Qty      float64  `json:"qty"`
	Unit     string   `json:"unit,omitempty"`
	Price    *float64 `json:"price"` // đơn giá VND; null là chưa rõ giá (A7)
	Ticked   bool     `json:"ticked"`
	Mine     bool     `json:"mine"` // món của chính người mua, mua thêm
	Category string   `json:"category,omitempty"`
}

// CompleteRequest là thân POST /households/shopping-assignments/{id}/complete.
type CompleteRequest struct {
	Scanned bool      `json:"scanned"`
	Note    string    `json:"note,omitempty"`
	Store   string    `json:"store,omitempty"`
	Lines   []RunLine `json:"lines"`
}

const maxRunLines = 100

func (s *Service) buyerAssignment(ctx context.Context, p authz.Principal, id string) (Assignment, error) {
	if p.HouseholdID == "" {
		return Assignment{}, apperr.ErrForbidden
	}
	a, err := s.store.Assignment(ctx, p.HouseholdID, id)
	if err != nil {
		return Assignment{}, err
	}
	if a.BuyerUID != p.UID {
		return Assignment{}, apperr.ErrForbidden
	}
	if !a.Active() {
		return Assignment{}, apperr.New(apperr.CodeInvalidState, "Vòng mua hộ này đã kết thúc.").With("status", a.Status)
	}
	return a, nil
}

// Decline: người được nhờ từ chối, mọi món trả về Kế hoạch của người nhờ.
func (s *Service) Decline(ctx context.Context, p authz.Principal, id string) (Assignment, error) {
	if err := authz.Can(p, authz.ActionReadPlan); err != nil {
		return Assignment{}, err
	}
	if _, err := s.buyerAssignment(ctx, p, id); err != nil {
		return Assignment{}, err
	}
	return s.store.EndByBuyer(ctx, p.HouseholdID, id, p.UID, "declined")
}

// Complete ghi các món đã tick thành một hóa đơn của người mua, gắn với vòng mua hộ.
// Món nhờ mua đã tick → done trong Kế hoạch của người nhờ, vào tồn kho của người nhờ;
// món chưa tick → trả về Kế hoạch; món "của mình" vào tồn kho người mua. Gửi lại cùng
// nội dung (mất mạng rồi tự gửi lại) không ghi trùng vì mã hóa đơn suy từ mã vòng.
func (s *Service) Complete(ctx context.Context, p authz.Principal, id string, req CompleteRequest) (Assignment, error) {
	if err := authz.Can(p, authz.ActionWriteReceipts); err != nil {
		return Assignment{}, err
	}
	if s.receipts == nil {
		return Assignment{}, apperr.ErrNotFound
	}
	if len(req.Lines) > maxRunLines {
		return Assignment{}, bad("lines", "Danh sách tối đa 100 món.")
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > 200 || strings.IndexFunc(note, unicode.IsControl) >= 0 {
		return Assignment{}, bad("note", "Ghi chú tối đa 200 ký tự.")
	}
	a, err := s.buyerAssignment(ctx, p, id)
	if err != nil {
		if apperr.From(err).Code == apperr.CodeInvalidState {
			// Gửi lại sau khi đã xong: trả vòng hiện tại để hàng đợi offline dừng.
			if done, gerr := s.store.Assignment(ctx, p.HouseholdID, id); gerr == nil && done.BuyerUID == p.UID && done.MatchedReceiptID == runReceiptID(id) {
				return done, nil
			}
		}
		return Assignment{}, err
	}
	sr, err := s.runReceipt(ctx, p, a, req)
	if err != nil {
		return Assignment{}, err
	}
	if len(sr.Lines) == 0 {
		return s.store.EndByBuyer(ctx, p.HouseholdID, id, p.UID, "closed_partial")
	}
	if _, err := s.receipts.Save(ctx, p, sr); err != nil {
		return Assignment{}, err
	}
	return s.store.Assignment(ctx, p.HouseholdID, id)
}

func runReceiptID(assignmentID string) string {
	id := "run-" + assignmentID
	if len(id) > 64 {
		id = id[:64]
	}
	return id
}

func (s *Service) runReceipt(ctx context.Context, p authz.Principal, a Assignment, req CompleteRequest) (receipt.SaveRequest, error) {
	store := strings.TrimSpace(req.Store)
	if store == "" {
		if trip, err := s.plans.PlanTrip(ctx, a.OwnerUID, a.PlanID); err == nil {
			store = trip.TripNote
		}
	}
	if store == "" {
		store = "Mua hộ"
	}
	asked := map[string]bool{}
	for _, it := range a.Items {
		asked[it.Key] = true
	}
	sr := receipt.SaveRequest{
		ReceiptID: runReceiptID(a.ID), Store: store, Currency: "VND", AssignmentID: a.ID,
		PurchasedOn:      domain.Today(s.now(), domain.Location(p.Timezone)).String(),
		UseForPrediction: true, ConfirmDuplicate: true, SourcePlanID: "",
	}
	// B7: làm thủ công (không có hóa đơn) mà không nhập giá thì lấy giá mua gần nhất, ghi "Ước tính".
	// Món của người nhờ theo giá người nhờ từng mua, món của mình theo giá của mình.
	// Chưa từng có giá thì để trống (chưa có giá), không tính 0đ.
	est := map[bool]map[string]int64{}
	if !req.Scanned {
		need := map[bool][]string{}
		for _, l := range req.Lines {
			if l.Ticked && l.Price == nil && l.Key != "" {
				need[l.Mine] = append(need[l.Mine], l.Key)
			}
		}
		for mine, keys := range need {
			uid := a.OwnerUID
			if mine {
				uid = p.UID
			}
			m, err := s.plans.LastUnitPrices(ctx, uid, keys)
			if err != nil {
				return receipt.SaveRequest{}, err
			}
			est[mine] = m
		}
	}
	for i, l := range req.Lines {
		if !l.Ticked {
			continue
		}
		if !l.Mine && !asked[l.Key] {
			return receipt.SaveRequest{}, apperr.New(apperr.CodeValidation, "Có món không thuộc lần nhờ mua này.").With("key", l.Key)
		}
		qty := l.Qty
		if qty <= 0 || math.IsNaN(qty) {
			qty = 1
		}
		amount, source := "", ""
		if l.Price != nil {
			if *l.Price < 0 || math.IsNaN(*l.Price) || *l.Price > 1e9 {
				return receipt.SaveRequest{}, apperr.New(apperr.CodeValidation, "Giá của món này chưa hợp lệ.").With("field", "lines").With("key", l.Key)
			}
			amount = fmt.Sprintf("%d", int64(math.Round(*l.Price*qty)))
			source = receipt.PriceReceipt
			if !req.Scanned {
				source = receipt.PriceManual
			}
		} else if u, ok := est[l.Mine][l.Key]; ok {
			amount, source = fmt.Sprintf("%d", int64(math.Round(float64(u)*qty))), receipt.PriceEstimate
		}
		sr.Lines = append(sr.Lines, receipt.SaveLine{
			LineID: fmt.Sprintf("l%d", i+1), Name: l.Name, Qty: qty, Unit: l.Unit, Amount: amount,
			CategoryKey: l.Category, GroupKey: l.Key, Habit: true, PriceSource: source,
		})
	}
	return sr, nil
}
