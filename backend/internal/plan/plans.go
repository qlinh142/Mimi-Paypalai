package plan

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/item"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Nhiều Kế hoạch (3.1 mục 3A): mỗi người có Plan mặc định (đường dẫn cũ plan_items) và tối
// đa MaxPlans Plan đặt tên (users/{uid}/plans/{id}, dòng ở plans/{id}/items). Mọi route
// /plan/... nhận ?plan_id=; handler gắn ID vào context bằng WithID, Store đọc lại bằng IDFrom.

// MaxPlans: số Plan đặt tên tối đa, không tính Plan mặc định.
const MaxPlans = 10

type ctxKey struct{}

// WithID gắn Plan đang thao tác vào context. "" hoặc DefaultPlanID là Plan mặc định.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// IDFrom trả Plan đang thao tác; mặc định là DefaultPlanID.
func IDFrom(ctx context.Context) string {
	if id, _ := ctx.Value(ctxKey{}).(string); id != "" {
		return id
	}
	return DefaultPlanID
}

// Meta là một Plan trong danh sách GET /plans.
type Meta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // active | archived
	Default   bool      `json:"default"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
	BuyDate   string    `json:"buy_date,omitempty"`
	Icon      string    `json:"icon"` // khóa trong PlanIcons; trống khi đọc thì trả DefaultIcon
}

// PlanIcons là bộ biểu tượng chung của app (ảnh mẫu 5b). Máy chủ chỉ lưu khóa, hình vẽ
// nằm trong app, nên đăng nhập lại ở máy nào cũng ra đúng biểu tượng.
var PlanIcons = []string{"bag", "family", "kid", "gift", "home", "food"}

// DefaultIcon dùng khi Kế hoạch chưa chọn biểu tượng.
const DefaultIcon = "bag"

func validIcon(k string) bool {
	for _, x := range PlanIcons {
		if x == k {
			return true
		}
	}
	return false
}

func iconOrDefault(k string) string {
	if validIcon(k) {
		return k
	}
	return DefaultIcon
}

func badIcon() error {
	return apperr.New(apperr.CodeValidation, "Biểu tượng không có trong bộ của app.").With("field", "icon")
}

// PlansStore là phần Firestore của danh sách Plan. ItemCount do Store đếm (dòng open + assigned).
type PlansStore interface {
	ListPlans(ctx context.Context, uid string) ([]Meta, error)
	GetPlan(ctx context.Context, uid, id string) (Meta, error) // apperr.ErrNotFound
	CreatePlan(ctx context.Context, uid string, m Meta) error
	UpdatePlan(ctx context.Context, uid string, m Meta) error
	// DeletePlan xóa Plan và các dòng của nó. Dòng đang giao cho người khác mua chặn việc xóa.
	DeletePlan(ctx context.Context, uid, id string) error
	// Plan mặc định không có tài liệu riêng: biểu tượng của nó nằm trên users/{uid}.
	DefaultPlanIcon(ctx context.Context, uid string) (string, error)
	SetDefaultPlanIcon(ctx context.Context, uid, icon string) error
}

// WithPlans bật nhiều Plan. Không có thì chỉ Plan mặc định.
func (s *Service) WithPlans(ps PlansStore) *Service {
	s.plans = ps
	return s
}

// checkPlan bảo đảm Plan trong context tồn tại; ghi vào Plan đã lưu trữ bị chặn.
func (s *Service) checkPlan(ctx context.Context, uid string, write bool) error {
	id := IDFrom(ctx)
	if id == DefaultPlanID {
		return nil
	}
	if s.plans == nil || !validPlanID(id) {
		return apperr.ErrNotFound
	}
	m, err := s.plans.GetPlan(ctx, uid, id)
	if err != nil {
		return err
	}
	if write && m.Status == "archived" {
		return apperr.New(apperr.CodeInvalidState, "Kế hoạch này đã lưu trữ.").With("plan_id", id)
	}
	return nil
}

func validPlanID(id string) bool {
	if id == "" || len(id) > 60 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// PlanInput là body của POST /plans và PATCH /plans/{id}.
type PlanInput struct {
	Name   *string `json:"name,omitempty"`
	Status *string `json:"status,omitempty"`
	Icon   *string `json:"icon,omitempty"`
}

func cleanPlanName(n string) (string, error) {
	n = strings.TrimSpace(n)
	if n == "" || utf8.RuneCountInString(n) > 40 || strings.IndexFunc(n, unicode.IsControl) >= 0 {
		return "", apperr.New(apperr.CodeValidation, "Tên Kế hoạch từ 1 đến 40 ký tự.").With("field", "name")
	}
	return n, nil
}

// Plans trả Plan mặc định trước, rồi các Plan đặt tên (đang dùng trước, mới nhất trước).
func (s *Service) Plans(ctx context.Context, p authz.Principal) ([]Meta, error) {
	if err := authz.Can(p, authz.ActionReadPlan); err != nil {
		return nil, err
	}
	lines, err := s.store.OpenLines(WithID(ctx, DefaultPlanID), p.UID)
	if err != nil {
		return nil, err
	}
	st, err := s.store.State(WithID(ctx, DefaultPlanID), p.UID)
	if err != nil {
		return nil, err
	}
	out := []Meta{{ID: DefaultPlanID, Name: "Kế hoạch chính", Status: "active", Default: true, ItemCount: len(lines), BuyDate: st.BuyDate, Icon: DefaultIcon}}
	if s.plans == nil {
		return out, nil
	}
	icon, err := s.plans.DefaultPlanIcon(ctx, p.UID)
	if err != nil {
		return nil, err
	}
	out[0].Icon = iconOrDefault(icon)
	named, err := s.plans.ListPlans(ctx, p.UID)
	if err != nil {
		return nil, err
	}
	for i := range named {
		named[i].Icon = iconOrDefault(named[i].Icon)
	}
	sort.SliceStable(named, func(i, j int) bool {
		if (named[i].Status == "archived") != (named[j].Status == "archived") {
			return named[j].Status == "archived"
		}
		return named[i].CreatedAt.After(named[j].CreatedAt)
	})
	return append(out, named...), nil
}

func (s *Service) CreatePlan(ctx context.Context, p authz.Principal, in PlanInput) (Meta, error) {
	if err := authz.Can(p, authz.ActionWritePlan); err != nil {
		return Meta{}, err
	}
	if s.plans == nil {
		return Meta{}, apperr.ErrNotFound
	}
	if in.Name == nil {
		return Meta{}, apperr.New(apperr.CodeValidation, "Cần tên Kế hoạch.").With("field", "name")
	}
	name, err := cleanPlanName(*in.Name)
	if err != nil {
		return Meta{}, err
	}
	existing, err := s.plans.ListPlans(ctx, p.UID)
	if err != nil {
		return Meta{}, err
	}
	active := 0
	for _, m := range existing {
		if m.Status != "archived" {
			active++
		}
	}
	if active >= MaxPlans {
		return Meta{}, apperr.New(apperr.CodeValidation, "Đã đủ 10 Kế hoạch đang dùng.").With("limit", MaxPlans)
	}
	icon := DefaultIcon
	if in.Icon != nil {
		if !validIcon(*in.Icon) {
			return Meta{}, badIcon()
		}
		icon = *in.Icon
	}
	m := Meta{ID: "p-" + s.now().UTC().Format("060102150405.000"), Name: name, Status: "active", CreatedAt: s.now(), Icon: icon}
	m.ID = strings.ReplaceAll(m.ID, ".", "")
	if err := s.plans.CreatePlan(ctx, p.UID, m); err != nil {
		return Meta{}, err
	}
	return m, nil
}

func (s *Service) UpdatePlan(ctx context.Context, p authz.Principal, id string, in PlanInput) (Meta, error) {
	if err := authz.Can(p, authz.ActionWritePlan); err != nil {
		return Meta{}, err
	}
	if s.plans == nil || !validPlanID(id) {
		return Meta{}, apperr.ErrNotFound
	}
	if in.Icon != nil && !validIcon(*in.Icon) {
		return Meta{}, badIcon()
	}
	if id == DefaultPlanID {
		return s.updateDefaultPlan(ctx, p, in)
	}
	m, err := s.plans.GetPlan(ctx, p.UID, id)
	if err != nil {
		return Meta{}, err
	}
	m.Icon = iconOrDefault(m.Icon)
	if in.Icon != nil {
		m.Icon = *in.Icon
	}
	if in.Name != nil {
		if m.Name, err = cleanPlanName(*in.Name); err != nil {
			return Meta{}, err
		}
	}
	if in.Status != nil {
		if *in.Status != "active" && *in.Status != "archived" {
			return Meta{}, apperr.New(apperr.CodeValidation, "Trạng thái là active hoặc archived.").With("field", "status")
		}
		m.Status = *in.Status
	}
	if err := s.plans.UpdatePlan(ctx, p.UID, m); err != nil {
		return Meta{}, err
	}
	return m, nil
}

// updateDefaultPlan: Plan mặc định chỉ đổi được biểu tượng, không đổi tên hay lưu trữ.
func (s *Service) updateDefaultPlan(ctx context.Context, p authz.Principal, in PlanInput) (Meta, error) {
	if in.Name != nil || in.Status != nil {
		return Meta{}, apperr.New(apperr.CodeValidation, "Kế hoạch chính chỉ đổi được biểu tượng.").With("field", "name")
	}
	if in.Icon == nil {
		return Meta{}, apperr.New(apperr.CodeValidation, "Chưa có thay đổi nào để lưu.")
	}
	if err := s.plans.SetDefaultPlanIcon(ctx, p.UID, *in.Icon); err != nil {
		return Meta{}, err
	}
	all, err := s.Plans(ctx, p)
	if err != nil {
		return Meta{}, err
	}
	return all[0], nil
}

func (s *Service) DeletePlan(ctx context.Context, p authz.Principal, id string) error {
	if err := authz.Can(p, authz.ActionWritePlan); err != nil {
		return err
	}
	if s.plans == nil || id == DefaultPlanID || !validPlanID(id) {
		return apperr.ErrNotFound
	}
	if _, err := s.plans.GetPlan(ctx, p.UID, id); err != nil {
		return err
	}
	return s.plans.DeletePlan(ctx, p.UID, id)
}

// AddRequest là thân tùy chọn của PUT /plan/items/{key}: thêm món chưa có trong Đồ dùng,
// từ taxonomy (3.1 mục 3A.4) hoặc món tự nhập (khóa "custom-...").
type AddRequest struct {
	Name     string  `json:"name,omitempty"`
	Qty      float64 `json:"qty,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Category string  `json:"category_key,omitempty"`
}

// lineFor dựng dòng mới khi món không có trong Đồ dùng.
func (s *Service) lineFor(key string, req AddRequest) (Line, error) {
	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) > 60 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Line{}, apperr.New(apperr.CodeValidation, "Tên món tối đa 60 ký tự.").With("field", "name")
	}
	l := Line{Key: key, AddedAt: s.now(), Qty: 1, Priority: "normal", Source: SourceManual, CategoryKey: req.Category}
	switch {
	case taxonomy.Has(key):
		l.Key = taxonomy.Normalize(key)
		l.Name = taxonomy.Label(l.Key, "vi")
		if name != "" {
			l.Name = name
		}
	case strings.HasPrefix(key, "custom-") && item.ValidKey(key):
		if name == "" {
			return Line{}, apperr.New(apperr.CodeValidation, "Món tự nhập cần tên.").With("field", "name")
		}
		l.Name = name
	default:
		return Line{}, apperr.ErrNotFound
	}
	if req.Qty > 0 && req.Qty <= 10_000 {
		l.Qty = req.Qty
	}
	l.Unit = req.Unit
	return l, nil
}
