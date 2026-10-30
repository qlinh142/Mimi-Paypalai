// Package recurring xử lý khoản thanh toán định kỳ trong Sổ chi tiêu và Dự báo 30 ngày
// (tài liệu 2.1).
//
//   - Spent chỉ gồm tiền đã thực sự chi: kỳ pending không cộng; kỳ người dùng xác nhận
//     "đã trả" cộng vào tháng thanh toán; kỳ đã khớp một hóa đơn (matched) không cộng lần
//     hai vì hóa đơn đã nằm trong Spending.
//   - Forecast gồm kỳ pending có hạn trong cửa sổ dự báo. Kỳ đã paid / matched / skipped thì
//     bỏ, và kỳ tiếp theo được tạo ngay.
package recurring

import (
	"strings"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/domain"
)

// Cycle là chu kỳ lặp.
type Cycle string

const (
	Weekly    Cycle = "weekly"
	Monthly   Cycle = "monthly"
	Quarterly Cycle = "quarterly"
	Yearly    Cycle = "yearly"
)

func (c Cycle) Valid() bool {
	switch c {
	case Weekly, Monthly, Quarterly, Yearly:
		return true
	}
	return false
}

// Status của một kỳ (2.1 mục 4).
type Status string

const (
	Pending Status = "pending"
	Paid    Status = "paid"    // người dùng xác nhận đã trả: cộng Spending
	Matched Status = "matched" // đã khớp hóa đơn / giao dịch đã lưu: không cộng lại
	Skipped Status = "skipped" // kỳ này không phát sinh
)

// Recurring là users/{uid}/recurring/{id}.
type Recurring struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AmountVND   int64  `json:"amount"`
	Currency    string `json:"currency"`
	Cycle       Cycle  `json:"cycle"`
	NextDueOn   string `json:"next_due_at"` // YYYY-MM-DD: hạn của kỳ đang chờ
	Active      bool   `json:"active"`
	CategoryKey string `json:"category_key,omitempty"`
}

// Occurrence là users/{uid}/recurring_occurrences/{recurring_id}_{due}.
type Occurrence struct {
	ID          string `json:"id"`
	RecurringID string `json:"recurring_id"`
	Name        string `json:"name"`
	DueOn       string `json:"due_at"`
	AmountVND   int64  `json:"amount"`
	Status      Status `json:"status"`
	PaidOn      string `json:"paid_at,omitempty"`
	SourceType  string `json:"source_type,omitempty"` // manual | receipt
	SourceID    string `json:"source_id,omitempty"`
}

// OccurrenceID: một kỳ của một khoản là duy nhất theo hạn, nên tạo lại không nhân đôi.
func OccurrenceID(recurringID string, due domain.Date) string {
	return recurringID + "_" + due.String()
}

// NextDue trả hạn kỳ sau. Ngày 31 sang tháng ngắn hơn lùi về ngày cuối tháng (31/1 → 28/2),
// rồi các kỳ sau vẫn tính từ ngày gốc nhờ anchorDay.
func NextDue(due domain.Date, c Cycle, anchorDay int) domain.Date {
	t := due.At(12, 0, time.UTC)
	switch c {
	case Weekly:
		return due.AddDays(7)
	case Quarterly:
		return addMonths(t, 3, anchorDay)
	case Yearly:
		return addMonths(t, 12, anchorDay)
	default:
		return addMonths(t, 1, anchorDay)
	}
}

func addMonths(t time.Time, n, anchorDay int) domain.Date {
	if anchorDay <= 0 {
		anchorDay = t.Day()
	}
	first := time.Date(t.Year(), t.Month()+time.Month(n), 1, 12, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	day := min(anchorDay, last)
	d, _ := domain.ParseDate(time.Date(first.Year(), first.Month(), day, 12, 0, 0, 0, time.UTC).Format("2006-01-02"))
	return d
}

// MatchWindowDays: hóa đơn cách hạn tối đa chừng này ngày vẫn coi là cùng kỳ (2.1 mục 8,
// "date proximity").
const MatchWindowDays = 7

// MatchTolerance: lệch số tiền tối đa (làm tròn, phí nhỏ) vẫn coi là cùng khoản.
const MatchTolerance int64 = 1_000

// MatchesReceipt: hóa đơn / giao dịch được coi là kỳ thanh toán của khoản định kỳ khi tên
// nhà cung cấp khớp (tên cửa hàng hoặc tên một dòng chứa tên khoản) và số tiền khớp tổng
// hóa đơn hoặc một dòng, cùng tiền tệ VND, ngày nằm trong cửa sổ quanh hạn.
func MatchesReceipt(o Occurrence, store string, lineNames []string, total int64, lineAmounts []int64, on domain.Date) bool {
	due, err := domain.ParseDate(o.DueOn)
	if err != nil {
		return false
	}
	if d := due.DaysUntil(on); d > MatchWindowDays || d < -MatchWindowDays {
		return false
	}
	name := compact(o.Name)
	if name == "" {
		return false
	}
	nameHit := strings.Contains(compact(store), name)
	for _, l := range lineNames {
		if nameHit {
			break
		}
		nameHit = strings.Contains(compact(l), name)
	}
	if !nameHit {
		return false
	}
	near := func(v int64) bool { return abs(v-o.AmountVND) <= MatchTolerance }
	if near(total) {
		return true
	}
	for _, a := range lineAmounts {
		if near(a) {
			return true
		}
	}
	return false
}

func compact(s string) string {
	var b strings.Builder
	for _, r := range domain.NormalizeVi(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
