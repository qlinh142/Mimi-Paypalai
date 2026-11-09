package recurring

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/item"
)

// ForecastDays là cửa sổ "Dự báo 30 ngày".
const ForecastDays = 30

// MaxRecurring: giới hạn số khoản định kỳ mỗi người.
const MaxRecurring = 50

// Store là phần Firestore của khoản định kỳ.
type Store interface {
	ListRecurring(ctx context.Context, uid string) ([]Recurring, error)
	GetRecurring(ctx context.Context, uid, id string) (Recurring, error) // apperr.ErrNotFound
	// CreateRecurring ghi khoản mới cùng kỳ pending đầu tiên.
	CreateRecurring(ctx context.Context, uid string, r Recurring, anchorDay int) error
	// UpdateRecurring ghi tên, số tiền, chu kỳ, danh mục, active. Đổi hạn thì kỳ pending
	// hiện tại được thay bằng kỳ mới đúng hạn.
	UpdateRecurring(ctx context.Context, uid string, r Recurring, anchorDay int, dueChanged bool) error
	// DeleteRecurring xóa khoản và các kỳ pending; kỳ đã paid / matched giữ lại làm lịch sử.
	DeleteRecurring(ctx context.Context, uid, id string) error
	// Occurrences trả các kỳ có hạn trong [from, to].
	Occurrences(ctx context.Context, uid string, from, to domain.Date) ([]Occurrence, error)
	// Settle chuyển một kỳ pending sang paid / skipped trong một transaction: kỳ paid cộng
	// số tiền vào months/{tháng trả} (Rule R2), rồi tạo kỳ sau và dời next_due_on.
	Settle(ctx context.Context, uid, occurrenceID string, to Status, paidOn domain.Date) (Occurrence, error)
}

// Items và Prices là phần đọc mà Dự báo cần để ước tính tiền mua sắm sắp tới.
type Items interface {
	ListItems(ctx context.Context, uid string) ([]item.Record, error)
}

type Prices interface {
	Prices(ctx context.Context, uid string, groups []string) ([]domain.LastPrice, error)
}

type Service struct {
	store  Store
	items  Items
	prices Prices
	now    func() time.Time
}

func NewService(store Store, items Items, prices Prices, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, items: items, prices: prices, now: now}
}

// Input là body của POST /recurring và PATCH /recurring/{id} (PATCH: trường có mặt mới đổi).
type Input struct {
	Name        *string `json:"name,omitempty"`
	AmountVND   *int64  `json:"amount,omitempty"`
	Cycle       *Cycle  `json:"cycle,omitempty"`
	NextDueOn   *string `json:"next_due_at,omitempty"`
	CategoryKey *string `json:"category_key,omitempty"`
	Active      *bool   `json:"active,omitempty"`
}

// View là một khoản định kỳ kèm kỳ đang chờ và trạng thái hiển thị.
type View struct {
	Recurring
	Display string       `json:"display"` // overdue | due_today | due_soon | upcoming | paused
	Current *Occurrence  `json:"current,omitempty"`
	History []Occurrence `json:"history"`
}

type List struct {
	Items        []View `json:"items"`
	MonthlyTotal int64  `json:"monthly_total"` // quy đổi mọi chu kỳ về tháng, chỉ khoản active
}

func (s *Service) today(p authz.Principal) domain.Date {
	return domain.Today(s.now(), domain.Location(p.Timezone))
}

func (s *Service) List(ctx context.Context, p authz.Principal) (List, error) {
	if err := authz.Can(p, authz.ActionManageRecurring); err != nil {
		return List{}, err
	}
	recs, err := s.store.ListRecurring(ctx, p.UID)
	if err != nil {
		return List{}, err
	}
	today := s.today(p)
	occs, err := s.store.Occurrences(ctx, p.UID, today.AddDays(-120), today.AddDays(400))
	if err != nil {
		return List{}, err
	}
	byRec := map[string][]Occurrence{}
	for _, o := range occs {
		byRec[o.RecurringID] = append(byRec[o.RecurringID], o)
	}
	out := List{Items: []View{}}
	for _, r := range recs {
		v := View{Recurring: r, History: []Occurrence{}}
		os := byRec[r.ID]
		sort.Slice(os, func(i, j int) bool { return os[i].DueOn < os[j].DueOn })
		for i := range os {
			o := os[i]
			if o.Status == Pending {
				if v.Current == nil {
					v.Current = &o
				}
				continue
			}
			v.History = append(v.History, o)
		}
		sort.Slice(v.History, func(i, j int) bool { return v.History[i].DueOn > v.History[j].DueOn })
		if len(v.History) > 6 {
			v.History = v.History[:6]
		}
		v.Display = display(r, v.Current, today)
		if r.Active {
			out.MonthlyTotal += monthly(r)
		}
		out.Items = append(out.Items, v)
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].NextDueOn < out.Items[j].NextDueOn })
	return out, nil
}

func display(r Recurring, cur *Occurrence, today domain.Date) string {
	if !r.Active {
		return "paused"
	}
	if cur == nil {
		return "upcoming"
	}
	due, err := domain.ParseDate(cur.DueOn)
	if err != nil {
		return "upcoming"
	}
	switch d := today.DaysUntil(due); {
	case d < 0:
		return "overdue"
	case d == 0:
		return "due_today"
	case d <= 7:
		return "due_soon"
	}
	return "upcoming"
}

func monthly(r Recurring) int64 {
	switch r.Cycle {
	case Weekly:
		return int64(math.Round(float64(r.AmountVND) * 52 / 12))
	case Quarterly:
		return int64(math.Round(float64(r.AmountVND) / 3))
	case Yearly:
		return int64(math.Round(float64(r.AmountVND) / 12))
	}
	return r.AmountVND
}

func bad(field, msg string) error {
	return apperr.New(apperr.CodeValidation, msg).With("field", field)
}

func (s *Service) apply(r *Recurring, in Input, today domain.Date) error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || utf8.RuneCountInString(n) > 60 || strings.IndexFunc(n, unicode.IsControl) >= 0 {
			return bad("name", "Tên khoản định kỳ từ 1 đến 60 ký tự.")
		}
		r.Name = n
	}
	if in.AmountVND != nil {
		if *in.AmountVND <= 0 || *in.AmountVND > 1_000_000_000 {
			return bad("amount", "Số tiền phải lớn hơn 0.")
		}
		r.AmountVND = *in.AmountVND
	}
	if in.Cycle != nil {
		if !in.Cycle.Valid() {
			return bad("cycle", "Chu kỳ là weekly, monthly, quarterly hoặc yearly.")
		}
		r.Cycle = *in.Cycle
	}
	if in.NextDueOn != nil {
		d, err := domain.ParseDate(*in.NextDueOn)
		if err != nil || d.Before(today.AddDays(-62)) || d.After(today.AddDays(400)) {
			return bad("next_due_at", "Ngày đến hạn không hợp lệ.")
		}
		r.NextDueOn = d.String()
	}
	if in.CategoryKey != nil {
		r.CategoryKey = strings.TrimSpace(*in.CategoryKey)
	}
	if in.Active != nil {
		r.Active = *in.Active
	}
	return nil
}

func (s *Service) Create(ctx context.Context, p authz.Principal, in Input) (View, error) {
	if err := authz.Can(p, authz.ActionManageRecurring); err != nil {
		return View{}, err
	}
	today := s.today(p)
	if in.Name == nil || in.AmountVND == nil || in.NextDueOn == nil {
		return View{}, bad("name", "Cần tên, số tiền và ngày đến hạn.")
	}
	r := Recurring{Currency: "VND", Cycle: Monthly, Active: true}
	if err := s.apply(&r, in, today); err != nil {
		return View{}, err
	}
	existing, err := s.store.ListRecurring(ctx, p.UID)
	if err != nil {
		return View{}, err
	}
	if len(existing) >= MaxRecurring {
		return View{}, apperr.New(apperr.CodeValidation, "Đã đủ 50 khoản định kỳ.").With("limit", MaxRecurring)
	}
	r.ID = domain.Slug(r.Name) + "-" + s.now().UTC().Format("060102150405")
	due, _ := domain.ParseDate(r.NextDueOn)
	if err := s.store.CreateRecurring(ctx, p.UID, r, due.Day()); err != nil {
		return View{}, err
	}
	o := Occurrence{ID: OccurrenceID(r.ID, due), RecurringID: r.ID, Name: r.Name, DueOn: r.NextDueOn, AmountVND: r.AmountVND, Status: Pending}
	return View{Recurring: r, Current: &o, History: []Occurrence{}, Display: display(r, &o, today)}, nil
}

func (s *Service) Update(ctx context.Context, p authz.Principal, id string, in Input) (View, error) {
	if err := authz.Can(p, authz.ActionManageRecurring); err != nil {
		return View{}, err
	}
	r, err := s.store.GetRecurring(ctx, p.UID, id)
	if err != nil {
		return View{}, err
	}
	before := r.NextDueOn
	if err := s.apply(&r, in, s.today(p)); err != nil {
		return View{}, err
	}
	due, _ := domain.ParseDate(r.NextDueOn)
	if err := s.store.UpdateRecurring(ctx, p.UID, r, due.Day(), before != r.NextDueOn); err != nil {
		return View{}, err
	}
	return s.find(ctx, p, id)
}

func (s *Service) find(ctx context.Context, p authz.Principal, id string) (View, error) {
	l, err := s.List(ctx, p)
	if err != nil {
		return View{}, err
	}
	for _, v := range l.Items {
		if v.ID == id {
			return v, nil
		}
	}
	return View{}, apperr.ErrNotFound
}

func (s *Service) Delete(ctx context.Context, p authz.Principal, id string) error {
	if err := authz.Can(p, authz.ActionManageRecurring); err != nil {
		return err
	}
	if _, err := s.store.GetRecurring(ctx, p.UID, id); err != nil {
		return err
	}
	return s.store.DeleteRecurring(ctx, p.UID, id)
}

// SettleRequest là body của POST /recurring/occurrences/{id}/pay|skip.
type SettleRequest struct {
	PaidOn string `json:"paid_at,omitempty"` // mặc định hôm nay
}

// Pay: người dùng xác nhận đã trả (Rule R2, AC03 đến AC05).
func (s *Service) Pay(ctx context.Context, p authz.Principal, occurrenceID string, req SettleRequest) (Occurrence, error) {
	if err := authz.Can(p, authz.ActionManageRecurring); err != nil {
		return Occurrence{}, err
	}
	today := s.today(p)
	on := today
	if req.PaidOn != "" {
		d, err := domain.ParseDate(req.PaidOn)
		if err != nil || d.After(today) || d.Before(today.AddDays(-62)) {
			return Occurrence{}, bad("paid_at", "Ngày trả không hợp lệ.")
		}
		on = d
	}
	return s.store.Settle(ctx, p.UID, occurrenceID, Paid, on)
}

// Skip: kỳ này không phát sinh; không cộng Spending, rời Forecast, tạo kỳ sau.
func (s *Service) Skip(ctx context.Context, p authz.Principal, occurrenceID string) (Occurrence, error) {
	if err := authz.Can(p, authz.ActionManageRecurring); err != nil {
		return Occurrence{}, err
	}
	return s.store.Settle(ctx, p.UID, occurrenceID, Skipped, s.today(p))
}

// ForecastItem là một món dự kiến cần mua trong cửa sổ dự báo.
type ForecastItem struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	OutOn     string `json:"out_on"`
	AmountVND *int64 `json:"amount"` // null: chưa có giá đã trả
}

// Forecast là phản hồi GET /forecast (2.1 mục 7).
type Forecast struct {
	From          string         `json:"from"`
	To            string         `json:"to"`
	Total         int64          `json:"total"`
	RecurringVND  int64          `json:"recurring_total"`
	ShoppingVND   int64          `json:"shopping_total"`
	Recurring     []Occurrence   `json:"recurring"`
	Shopping      []ForecastItem `json:"shopping"`
	MissingPrices int            `json:"missing_prices"`
}

func (s *Service) Forecast(ctx context.Context, p authz.Principal) (Forecast, error) {
	if err := authz.Can(p, authz.ActionReadSpending); err != nil {
		return Forecast{}, err
	}
	today := s.today(p)
	end := today.AddDays(ForecastDays)
	out := Forecast{From: today.String(), To: end.String(), Recurring: []Occurrence{}, Shopping: []ForecastItem{}}
	// Kỳ quá hạn chưa trả vẫn là nghĩa vụ chưa trả: tính vào dự báo (Rule R1).
	occs, err := s.store.Occurrences(ctx, p.UID, today.AddDays(-62), end)
	if err != nil {
		return Forecast{}, err
	}
	for _, o := range occs {
		if o.Status != Pending {
			continue
		}
		out.Recurring = append(out.Recurring, o)
		out.RecurringVND += o.AmountVND
	}
	sort.Slice(out.Recurring, func(i, j int) bool { return out.Recurring[i].DueOn < out.Recurring[j].DueOn })

	recs, err := s.items.ListItems(ctx, p.UID)
	if err != nil {
		return Forecast{}, err
	}
	var groups []string
	due := map[string]item.Record{}
	for _, r := range recs {
		if r.Dropped || r.Prediction.OutOn.IsZero() || r.Prediction.OutOn.After(end) {
			continue
		}
		groups = append(groups, r.GroupKey)
		due[r.GroupKey] = r
	}
	sort.Strings(groups)
	latest := map[string]domain.LastPrice{}
	if len(groups) > 0 {
		lp, err := s.prices.Prices(ctx, p.UID, groups)
		if err != nil {
			return Forecast{}, err
		}
		for _, x := range lp {
			if cur, ok := latest[x.GroupKey]; !ok || x.On.After(cur.On) {
				latest[x.GroupKey] = x
			}
		}
	}
	for _, g := range groups {
		r := due[g]
		fi := ForecastItem{Key: g, Name: r.Name, OutOn: r.Prediction.OutOn.String()}
		if x, ok := latest[g]; ok {
			v := int64(x.Unit)
			fi.AmountVND = &v
			out.ShoppingVND += v
		} else {
			out.MissingPrices++
		}
		out.Shopping = append(out.Shopping, fi)
	}
	out.Total = out.RecurringVND + out.ShoppingVND
	return out, nil
}
