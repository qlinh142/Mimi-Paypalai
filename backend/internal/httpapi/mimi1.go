package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/household"
	"github.com/smartlife-lab/mimi/backend/internal/market"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/recurring"
	"github.com/smartlife-lab/mimi/backend/internal/taxonomy"
)

// Các route của bộ spec Mimi 1.0 (0.1 đến 8.1): taxonomy, khoản định kỳ và dự báo, nhiều
// Kế hoạch, nhà và mua hộ, lịch sử giá.

// withPlan gắn ?plan_id= vào context cho các route /plan/... (3.1 mục 3A).
func withPlan(r *http.Request) *http.Request {
	if id := strings.TrimSpace(r.URL.Query().Get("plan_id")); id != "" {
		return r.WithContext(plan.WithID(r.Context(), id))
	}
	return r
}

// ---------- Taxonomy (0.2) ----------

type taxonomyGroup struct {
	Key         string          `json:"key"`
	Parent      string          `json:"parent"`
	Label       string          `json:"label"`
	Labels      taxonomy.Labels `json:"labels"`
	Measurement string          `json:"measurement"`
	Unit        string          `json:"unit"`
}

// getTaxonomy: ?q= tìm theo nhãn (không dấu), ?lang= chọn nhãn hiển thị. Không q thì trả cả bộ.
func (s *Server) getTaxonomy(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	lang := r.URL.Query().Get("lang")
	if lang != "en" {
		lang = "vi"
	}
	var groups []taxonomy.Group
	if q != "" {
		limit := 20
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 100 {
			limit = n
		}
		groups = taxonomy.Search(q, limit)
	} else {
		groups = taxonomy.All().Groups
	}
	out := make([]taxonomyGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, taxonomyGroup{
			Key: g.Key, Parent: g.Parent, Label: g.Labels.Get(lang), Labels: g.Labels,
			Measurement: string(g.Measurement), Unit: g.Unit,
		})
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	writeJSON(w, http.StatusOK, map[string]any{"version": taxonomy.Version(), "parents": taxonomy.All().Parents, "groups": out})
}

// ---------- Khoản định kỳ và Dự báo (2.1) ----------

func (s *Server) listRecurring(w http.ResponseWriter, r *http.Request) {
	out, err := s.recurring.List(r.Context(), principalFrom(r.Context()))
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) createRecurring(w http.ResponseWriter, r *http.Request) {
	var in recurring.Input
	if err := decodeJSON(w, r, &in, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.recurring.Create(r.Context(), principalFrom(r.Context()), in)
	s.reply(w, r, http.StatusCreated, out, err)
}

func (s *Server) patchRecurring(w http.ResponseWriter, r *http.Request) {
	var in recurring.Input
	if err := decodeJSON(w, r, &in, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.recurring.Update(r.Context(), principalFrom(r.Context()), r.PathValue("id"), in)
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) deleteRecurring(w http.ResponseWriter, r *http.Request) {
	if err := s.recurring.Delete(r.Context(), principalFrom(r.Context()), r.PathValue("id")); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) payOccurrence(w http.ResponseWriter, r *http.Request) {
	var req recurring.SettleRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(w, r, &req, maxJSONBody); err != nil {
			s.writeError(w, r, err)
			return
		}
	}
	out, err := s.recurring.Pay(r.Context(), principalFrom(r.Context()), r.PathValue("id"), req)
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) skipOccurrence(w http.ResponseWriter, r *http.Request) {
	out, err := s.recurring.Skip(r.Context(), principalFrom(r.Context()), r.PathValue("id"))
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) getForecast(w http.ResponseWriter, r *http.Request) {
	out, err := s.recurring.Forecast(r.Context(), principalFrom(r.Context()))
	s.reply(w, r, http.StatusOK, out, err)
}

// ---------- Nhiều Kế hoạch (3.1) ----------

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	out, err := s.plan.Plans(r.Context(), principalFrom(r.Context()))
	s.reply(w, r, http.StatusOK, map[string]any{"plans": out}, err)
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	var in plan.PlanInput
	if err := decodeJSON(w, r, &in, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.plan.CreatePlan(r.Context(), principalFrom(r.Context()), in)
	s.reply(w, r, http.StatusCreated, out, err)
}

func (s *Server) patchPlan(w http.ResponseWriter, r *http.Request) {
	var in plan.PlanInput
	if err := decodeJSON(w, r, &in, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.plan.UpdatePlan(r.Context(), principalFrom(r.Context()), r.PathValue("id"), in)
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) deletePlan(w http.ResponseWriter, r *http.Request) {
	if err := s.plan.DeletePlan(r.Context(), principalFrom(r.Context()), r.PathValue("id")); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getPlanPriceHistory(w http.ResponseWriter, r *http.Request) {
	r = withPlan(r)
	out, err := s.pricing.Basket(r.Context(), principalFrom(r.Context()))
	s.reply(w, r, http.StatusOK, out, err)
}

// MarketItem là giá đang bán của một dòng Kế hoạch, đọc từ bảng giá chung.
type MarketItem struct {
	Key       string         `json:"key"` // khóa dòng Kế hoạch (group_key)
	Name      string         `json:"name"`
	Prices    []market.Price `json:"prices"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// getPlanMarketPrices: GET /plan/market-prices (ADR-004). Chỉ trả món đã có giá; món mới
// thêm vào Kế hoạch có giá sau lượt price-sync đêm đó, hoặc khi bấm "Cập nhật giá".
func (s *Server) getPlanMarketPrices(w http.ResponseWriter, r *http.Request) {
	r = withPlan(r)
	cart, err := s.plan.Get(r.Context(), principalFrom(r.Context()))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	p := principalFrom(r.Context())
	out, err := s.marketPrices(r.Context(), p.Country, cart)
	if err == nil && s.priceRefresh != nil {
		st, serr := s.priceRefresh.Status(r.Context(), p.UID, today(p))
		if serr != nil {
			s.log.WarnContext(r.Context(), "price refresh status failed", "err", serr) // bảng giá vẫn trả
		} else {
			out.Refresh = &st
		}
	}
	s.reply(w, r, http.StatusOK, out, err)
}

// MarketRefreshResult là kết quả một lần bấm "Cập nhật giá", kèm bảng giá mới.
type MarketRefreshResult struct {
	Items   []MarketItem          `json:"items"`
	Looked  int                   `json:"looked"` // số món vừa tra
	Found   int                   `json:"found"`  // số món tra ra giá
	Fresh   int                   `json:"fresh"`  // số món vừa có giá trong 24 giờ, giữ nguyên
	Refresh *market.RefreshStatus `json:"refresh,omitempty"`
}

// refreshPlanMarketPrices: POST /plan/market-prices/refresh. Tra lại giá cho các món của
// kế hoạch đang mở (mẹ hỏi 02/11), không phải chờ job đêm. Giới hạn lượt mỗi ngày.
func (s *Server) refreshPlanMarketPrices(w http.ResponseWriter, r *http.Request) {
	r = withPlan(r)
	p := principalFrom(r.Context())
	cart, err := s.plan.Get(r.Context(), p)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	items := make([]market.Wanted, 0, len(cart.Items))
	for _, it := range cart.Items {
		items = append(items, market.Wanted{Key: market.ItemID(p.Country, it.Name), Country: p.Country, Name: it.Name, GroupKey: it.Key, Category: it.CategoryKey})
	}
	res, err := s.priceRefresh.Refresh(r.Context(), p.UID, today(p), items)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	prices, err := s.marketPrices(r.Context(), p.Country, cart)
	out := MarketRefreshResult{Items: prices.Items, Looked: res.Looked, Found: res.Found, Fresh: res.Fresh}
	if st, serr := s.priceRefresh.Status(r.Context(), p.UID, today(p)); serr == nil {
		out.Refresh = &st
	}
	s.reply(w, r, http.StatusOK, out, err)
}

type marketPricesOut struct {
	Items   []MarketItem          `json:"items"`
	Refresh *market.RefreshStatus `json:"refresh,omitempty"` // nút Cập nhật giá: còn mấy lần, lần gần nhất
}

// marketPrices: giá chung của nước người dùng cho các món trong kế hoạch.
func (s *Server) marketPrices(ctx context.Context, country string, cart plan.Cart) (marketPricesOut, error) {
	keys := make([]string, 0, len(cart.Items))
	for _, it := range cart.Items {
		keys = append(keys, market.ItemID(country, it.Name))
	}
	entries, err := s.market.Entries(ctx, keys)
	out := marketPricesOut{Items: []MarketItem{}}
	for i, it := range cart.Items {
		if e, ok := entries[keys[i]]; ok {
			out.Items = append(out.Items, MarketItem{Key: it.Key, Name: it.Name, Prices: e.Prices, UpdatedAt: e.UpdatedAt})
		}
	}
	return out, err
}

// today là ngày hôm nay theo múi giờ của người dùng (mặc định giờ Việt Nam), để đếm lượt.
func today(p authz.Principal) string {
	loc := time.FixedZone("ICT", 7*3600)
	if p.Timezone != "" {
		if l, err := time.LoadLocation(p.Timezone); err == nil {
			loc = l
		}
	}
	return domain.Today(time.Now(), loc).String()
}

func (s *Server) getItemPriceHistory(w http.ResponseWriter, r *http.Request) {
	out, err := s.pricing.Item(r.Context(), principalFrom(r.Context()), r.PathValue("key"))
	s.reply(w, r, http.StatusOK, out, err)
}

// ---------- Nhà và mua hộ (4.1) ----------

func (s *Server) me(r *http.Request) household.Member {
	p := principalFrom(r.Context())
	m := household.Member{UID: p.UID}
	if prof, err := s.profiles.Profile(r.Context(), p); err == nil {
		m.DisplayName = prof.DisplayName
	}
	return m
}

func (s *Server) getHousehold(w http.ResponseWriter, r *http.Request) {
	out, err := s.households.Get(r.Context(), principalFrom(r.Context()), s.me(r))
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) patchHousehold(w http.ResponseWriter, r *http.Request) {
	var in household.Patch
	if err := decodeJSON(w, r, &in, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.households.Rename(r.Context(), principalFrom(r.Context()), s.me(r), in)
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) postInvite(w http.ResponseWriter, r *http.Request) {
	out, err := s.households.Invite(r.Context(), principalFrom(r.Context()), s.me(r))
	s.reply(w, r, http.StatusCreated, out, err)
}

func (s *Server) postJoin(w http.ResponseWriter, r *http.Request) {
	var in household.JoinRequest
	if err := decodeJSON(w, r, &in, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.households.Join(r.Context(), principalFrom(r.Context()), s.me(r), in)
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) postLeave(w http.ResponseWriter, r *http.Request) {
	if err := s.households.Leave(r.Context(), principalFrom(r.Context())); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteMember(w http.ResponseWriter, r *http.Request) {
	if err := s.households.RemoveMember(r.Context(), principalFrom(r.Context()), r.PathValue("uid")); err != nil {
		s.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postAssignment(w http.ResponseWriter, r *http.Request) {
	var in household.AssignRequest
	if err := decodeJSON(w, r, &in, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.households.Assign(r.Context(), principalFrom(r.Context()), in)
	s.reply(w, r, http.StatusCreated, out, err)
}

func (s *Server) cancelAssignment(w http.ResponseWriter, r *http.Request) {
	out, err := s.households.Cancel(r.Context(), principalFrom(r.Context()), r.PathValue("id"))
	s.reply(w, r, http.StatusOK, out, err)
}

// Spec 1.5 mục 2: người được nhờ báo mua xong (ghi hóa đơn từ danh sách đã mua) hoặc từ chối.
func (s *Server) completeAssignment(w http.ResponseWriter, r *http.Request) {
	var req household.CompleteRequest
	if err := decodeJSON(w, r, &req, maxJSONBody); err != nil {
		s.writeError(w, r, err)
		return
	}
	out, err := s.households.Complete(r.Context(), principalFrom(r.Context()), r.PathValue("id"), req)
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) declineAssignment(w http.ResponseWriter, r *http.Request) {
	out, err := s.households.Decline(r.Context(), principalFrom(r.Context()), r.PathValue("id"))
	s.reply(w, r, http.StatusOK, out, err)
}

func (s *Server) getDelegated(w http.ResponseWriter, r *http.Request) {
	out, err := s.households.Delegated(r.Context(), principalFrom(r.Context()))
	s.reply(w, r, http.StatusOK, out, err)
}

// reply ghi lỗi hoặc JSON.
func (s *Server) reply(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, status, v)
}

func nonNilStrings(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

// Spec 1.5 mục 4: Lịch sử mua hàng (4c) và Chi tiết hóa đơn (4d).
func (s *Server) getReceiptHistory(w http.ResponseWriter, r *http.Request) {
	out, err := s.history.History(r.Context(), principalFrom(r.Context()))
	s.reply(w, r, http.StatusOK, map[string]any{"receipts": out}, err)
}

func (s *Server) getReceiptDetail(w http.ResponseWriter, r *http.Request) {
	out, err := s.history.Detail(r.Context(), principalFrom(r.Context()), r.PathValue("id"))
	s.reply(w, r, http.StatusOK, out, err)
}
