// Package household phục vụ Nhà (Settings → Household) và Gửi kế hoạch mua hộ (4.1 / 3B).
//
//   - Mỗi thành viên có tồn kho, Kế hoạch, Sổ chi tiêu riêng (4.1 mục 4). Nhà chỉ nối các
//     thành viên để gửi Kế hoạch cho nhau và nhờ mua.
//   - Một lần gửi Kế hoạch là một vòng mua hộ (Rule 1). Chỉ hóa đơn của Buyer mới xác nhận
//     món đã mua (Rule 2); món chưa mua trả lại Owner và không tự gửi lại (Rule 5, 6). Phần
//     đó nằm trong transaction lưu hóa đơn.
package household

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
)

// MaxMembers: số người tối đa của một nhà.
const MaxMembers = 8

// InviteTTL: mã mời hết hạn sau 48 giờ.
const InviteTTL = 48 * time.Hour

// MaxAssignmentItems: số món tối đa một vòng mua hộ.
const MaxAssignmentItems = 60

type Member struct {
	UID         string    `json:"uid"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"` // owner | member
	JoinedAt    time.Time `json:"joined_at"`
}

type Invite struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Household là phản hồi GET /households. Nhà một người (chưa mời ai) có ID rỗng.
type Household struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	OwnerUID string   `json:"owner_uid"`
	Members  []Member `json:"members"`
	Invite   *Invite  `json:"invite,omitempty"` // chỉ trả cho chủ nhà
	MyRole   string   `json:"my_role"`
}

// AssignmentItem là households/{hid}/shopping_assignments/{id}.items.{group_key}.
type AssignmentItem struct {
	Key    string  `json:"consumption_group_key"`
	Name   string  `json:"name_display"`
	Qty    float64 `json:"qty,omitempty"`
	Unit   string  `json:"unit,omitempty"`
	Status string  `json:"status"` // assigned | done | returned
}

// Assignment là một vòng mua hộ.
type Assignment struct {
	ID               string           `json:"assignment_id"`
	OwnerUID         string           `json:"owner_uid"`
	OwnerName        string           `json:"owner_display_name"`
	BuyerUID         string           `json:"buyer_uid"`
	BuyerName        string           `json:"buyer_display_name"`
	PlanID           string           `json:"plan_id"`
	Status           string           `json:"status"` // assigned | shopping | completed | closed_partial | cancelled
	Note             string           `json:"note,omitempty"`
	Items            []AssignmentItem `json:"items"`
	CreatedAt        time.Time        `json:"created_at"`
	ClosedAt         *time.Time       `json:"closed_at,omitempty"`
	MatchedReceiptID string           `json:"matched_receipt_id,omitempty"`
	// Trip: tên, thời gian, ghi chú của Plan gốc, đọc mới mỗi lần (3.1 mục 3B.4, 3B.5:
	// người nhận luôn thấy bản mới nhất, không phải bản sao). Chỉ có ở danh sách to_me.
	Trip *Trip `json:"trip,omitempty"`
}

// Trip là phần thông tin chuyến mua của Plan mà người được nhờ cần biết.
type Trip struct {
	PlanName  string `json:"plan_name"` // "" = Kế hoạch chính
	BuyDate   string `json:"buy_date,omitempty"`
	BuyHour   *int   `json:"buy_hour,omitempty"`
	BuyMinute *int   `json:"buy_minute,omitempty"`
	TripNote  string `json:"trip_note,omitempty"`
}

// Active: vòng còn mở, Buyer còn quét hóa đơn được.
func (a Assignment) Active() bool { return a.Status == "assigned" || a.Status == "shopping" }

type Store interface {
	// Household trả nhà theo ID (apperr.ErrNotFound khi không có).
	Household(ctx context.Context, hid string) (Household, error)
	// EnsureHousehold tạo nhà cho chủ (nếu chưa có) và trả ID.
	EnsureHousehold(ctx context.Context, owner Member, name string) (string, error)
	SetInvite(ctx context.Context, hid string, inv Invite) error
	// Join tìm nhà theo mã mời còn hạn và thêm thành viên (transaction).
	Join(ctx context.Context, m Member, code string, now time.Time, maxMembers int) (string, error)
	// RemoveMember bỏ một người khỏi nhà và đưa người đó về nhà một người. Vòng mua hộ còn
	// mở có người đó (Owner hoặc Buyer) bị hủy, món trả về Plan của Owner.
	RemoveMember(ctx context.Context, hid, uid string) error
	Rename(ctx context.Context, hid, name string) error

	// CreateAssignment đánh dấu các dòng Kế hoạch đang mở là assigned và tạo vòng mua hộ
	// trong một transaction. Dòng không còn mở trả apperr.CodeInvalidState.
	CreateAssignment(ctx context.Context, hid string, a Assignment) (Assignment, error)
	// CancelAssignment: vòng còn mở → cancelled, món trả về open (Case 4).
	CancelAssignment(ctx context.Context, hid, id, ownerUID string) (Assignment, error)
	// Assignments trả các vòng có field (buyer_uid | owner_uid) = uid, mới nhất trước.
	Assignments(ctx context.Context, hid, field, uid string, limit int) ([]Assignment, error)
}

// Plans là phần Kế hoạch mà vòng mua hộ cần: tên, số lượng của các dòng đang mở.
type Plans interface {
	OpenLineInfo(ctx context.Context, uid, planID string, keys []string) (map[string]AssignmentItem, error)
	// PlanTrip đọc tên và chuyến mua của Plan; Plan đã xóa trả Trip rỗng, không lỗi.
	PlanTrip(ctx context.Context, uid, planID string) (Trip, error)
}

type Service struct {
	store Store
	plans Plans
	now   func() time.Time
}

func New(store Store, plans Plans, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, plans: plans, now: now}
}

func bad(field, msg string) error {
	return apperr.New(apperr.CodeValidation, msg).With("field", field)
}

// Get trả nhà của người gọi; người chưa vào nhà nào thấy nhà một người của mình.
func (s *Service) Get(ctx context.Context, p authz.Principal, me Member) (Household, error) {
	if err := authz.Can(p, authz.ActionReadSelf); err != nil {
		return Household{}, err
	}
	if p.HouseholdID == "" {
		me.Role = "owner"
		return Household{OwnerUID: p.UID, Members: []Member{me}, MyRole: "owner"}, nil
	}
	h, err := s.store.Household(ctx, p.HouseholdID)
	if err != nil {
		return Household{}, err
	}
	h.MyRole = p.Role.String()
	if h.OwnerUID != p.UID || (h.Invite != nil && h.Invite.ExpiresAt.Before(s.now())) {
		h.Invite = nil
	}
	return h, nil
}

func cleanName(n string) (string, error) {
	n = strings.TrimSpace(n)
	if utf8.RuneCountInString(n) > 40 || strings.IndexFunc(n, unicode.IsControl) >= 0 {
		return "", bad("name", "Tên nhà tối đa 40 ký tự.")
	}
	return n, nil
}

// Invite tạo mã mời mới (mã cũ hết hiệu lực). Nhà một người được tạo nhà ngay lúc mời.
func (s *Service) Invite(ctx context.Context, p authz.Principal, me Member) (Invite, error) {
	if err := authz.Can(p, authz.ActionInviteMember); err != nil {
		return Invite{}, err
	}
	hid := p.HouseholdID
	if hid == "" {
		me.Role, me.JoinedAt = "owner", s.now()
		id, err := s.store.EnsureHousehold(ctx, me, "")
		if err != nil {
			return Invite{}, err
		}
		hid = id
	}
	inv := Invite{Code: newCode(), ExpiresAt: s.now().Add(InviteTTL)}
	if err := s.store.SetInvite(ctx, hid, inv); err != nil {
		return Invite{}, err
	}
	return inv, nil
}

func newCode() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]) // 8 ký tự A-Z2-7
}

// JoinRequest là body của POST /households/join.
type JoinRequest struct {
	Code string `json:"code"`
}

func (s *Service) Join(ctx context.Context, p authz.Principal, me Member, req JoinRequest) (Household, error) {
	if err := authz.Can(p, authz.ActionJoinHousehold); err != nil {
		return Household{}, err
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if len(code) != 8 {
		return Household{}, bad("code", "Mã mời gồm 8 ký tự.")
	}
	if p.HouseholdID != "" {
		h, err := s.store.Household(ctx, p.HouseholdID)
		if err == nil && len(h.Members) > 1 {
			return Household{}, apperr.New(apperr.CodeInvalidState, "Bạn đang ở trong một nhà khác. Rời nhà đó trước.")
		}
	}
	me.Role, me.JoinedAt = "member", s.now()
	hid, err := s.store.Join(ctx, me, code, s.now(), MaxMembers)
	if err != nil {
		return Household{}, err
	}
	p.HouseholdID, p.Role = hid, authz.RoleMember
	return s.Get(ctx, p, me)
}

// Leave: thành viên rời nhà. Chủ nhà không rời được khi còn thành viên khác.
func (s *Service) Leave(ctx context.Context, p authz.Principal) error {
	if p.HouseholdID == "" {
		return apperr.New(apperr.CodeInvalidState, "Bạn chưa ở trong nhà nào.")
	}
	if err := authz.Can(p, authz.ActionLeaveHousehold); err != nil {
		h, herr := s.store.Household(ctx, p.HouseholdID)
		if herr != nil || h.OwnerUID != p.UID || len(h.Members) > 1 {
			return apperr.New(apperr.CodeInvalidState, "Chủ nhà cần mời người khác ra trước khi rời nhà.")
		}
	}
	return s.store.RemoveMember(ctx, p.HouseholdID, p.UID)
}

func (s *Service) RemoveMember(ctx context.Context, p authz.Principal, uid string) error {
	if err := authz.Can(p, authz.ActionManageHousehold); err != nil {
		return err
	}
	if p.HouseholdID == "" || uid == "" {
		return apperr.ErrNotFound
	}
	if uid == p.UID {
		return bad("uid", "Chủ nhà không tự xóa mình.")
	}
	h, err := s.store.Household(ctx, p.HouseholdID)
	if err != nil {
		return err
	}
	if h.OwnerUID != p.UID {
		return apperr.ErrForbidden
	}
	if !h.has(uid) {
		return apperr.ErrNotFound
	}
	return s.store.RemoveMember(ctx, p.HouseholdID, uid)
}

// Patch là body của PATCH /households.
type Patch struct {
	Name string `json:"name"`
}

func (s *Service) Rename(ctx context.Context, p authz.Principal, me Member, patch Patch) (Household, error) {
	if err := authz.Can(p, authz.ActionManageHousehold); err != nil {
		return Household{}, err
	}
	name, err := cleanName(patch.Name)
	if err != nil {
		return Household{}, err
	}
	if p.HouseholdID == "" {
		me.Role, me.JoinedAt = "owner", s.now()
		id, err := s.store.EnsureHousehold(ctx, me, name)
		if err != nil {
			return Household{}, err
		}
		p.HouseholdID = id
	} else if err := s.store.Rename(ctx, p.HouseholdID, name); err != nil {
		return Household{}, err
	}
	return s.Get(ctx, p, me)
}

func (h Household) has(uid string) bool {
	for _, m := range h.Members {
		if m.UID == uid {
			return true
		}
	}
	return false
}

func (h Household) name(uid string) string {
	for _, m := range h.Members {
		if m.UID == uid {
			return m.DisplayName
		}
	}
	return ""
}

// AssignRequest là body của POST /households/shopping-assignments.
type AssignRequest struct {
	BuyerUID string   `json:"buyer_uid"`
	PlanID   string   `json:"plan_id,omitempty"`
	Keys     []string `json:"keys"`
	Note     string   `json:"note,omitempty"`
}

// Assign gửi các món đang mở của một Plan cho một thành viên cùng nhà (AC01, AC08).
func (s *Service) Assign(ctx context.Context, p authz.Principal, req AssignRequest) (Assignment, error) {
	if err := authz.Can(p, authz.ActionAssignShopping); err != nil {
		return Assignment{}, err
	}
	if req.BuyerUID == "" || req.BuyerUID == p.UID {
		return Assignment{}, bad("buyer_uid", "Chọn một thành viên khác trong nhà.")
	}
	keys := dedupe(req.Keys)
	if len(keys) == 0 || len(keys) > MaxAssignmentItems {
		return Assignment{}, bad("keys", "Chọn từ 1 đến 60 món để gửi.")
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > 120 || strings.IndexFunc(note, unicode.IsControl) >= 0 {
		return Assignment{}, bad("note", "Ghi chú tối đa 120 ký tự.")
	}
	h, err := s.store.Household(ctx, p.HouseholdID)
	if err != nil {
		return Assignment{}, err
	}
	if !h.has(p.UID) || !h.has(req.BuyerUID) {
		return Assignment{}, apperr.ErrForbidden // chỉ giao cho người cùng nhà
	}
	planID := req.PlanID
	if planID == "" {
		planID = "default"
	}
	info, err := s.plans.OpenLineInfo(ctx, p.UID, planID, keys)
	if err != nil {
		return Assignment{}, err
	}
	a := Assignment{
		ID:       "trip-" + strings.ToLower(newCode()),
		OwnerUID: p.UID, OwnerName: h.name(p.UID), BuyerUID: req.BuyerUID, BuyerName: h.name(req.BuyerUID),
		PlanID: planID, Status: "assigned", Note: note, CreatedAt: s.now(),
	}
	for _, k := range keys {
		it, ok := info[k]
		if !ok {
			return Assignment{}, apperr.New(apperr.CodeInvalidState, "Có món không còn mở trong Kế hoạch.").With("key", k)
		}
		it.Key, it.Status = k, "assigned"
		a.Items = append(a.Items, it)
	}
	return s.store.CreateAssignment(ctx, p.HouseholdID, a)
}

func (s *Service) Cancel(ctx context.Context, p authz.Principal, id string) (Assignment, error) {
	if err := authz.Can(p, authz.ActionAssignShopping); err != nil {
		return Assignment{}, err
	}
	return s.store.CancelAssignment(ctx, p.HouseholdID, id, p.UID)
}

// Delegated là phản hồi GET /plan/delegated: vòng được nhờ mua (Buyer) và vòng mình đã gửi.
type Delegated struct {
	ToMe   []Assignment `json:"to_me"`
	FromMe []Assignment `json:"from_me"`
}

func (s *Service) Delegated(ctx context.Context, p authz.Principal) (Delegated, error) {
	out := Delegated{ToMe: []Assignment{}, FromMe: []Assignment{}}
	if err := authz.Can(p, authz.ActionReadPlan); err != nil {
		return out, err
	}
	if p.HouseholdID == "" {
		return out, nil
	}
	to, err := s.store.Assignments(ctx, p.HouseholdID, "buyer_uid", p.UID, 20)
	if err != nil {
		return out, err
	}
	from, err := s.store.Assignments(ctx, p.HouseholdID, "owner_uid", p.UID, 20)
	if err != nil {
		return out, err
	}
	for _, a := range to {
		if a.Active() {
			trip, err := s.plans.PlanTrip(ctx, a.OwnerUID, a.PlanID)
			if err != nil {
				return out, err
			}
			a.Trip = &trip
			out.ToMe = append(out.ToMe, a)
		}
	}
	out.FromMe = append(out.FromMe, from...)
	return out, nil
}

func dedupe(keys []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] || len(k) > 80 || strings.ContainsAny(k, "/.") {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
