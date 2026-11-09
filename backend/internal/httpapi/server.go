// Package httpapi là lớp HTTP của mimi-api: định tuyến, xác thực, giải mã request,
// gọi service và trả lỗi theo mã ổn định trong api/openapi.yaml. Không chứa logic
// nghiệp vụ.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/smartlife-lab/mimi/backend/internal/assistant"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/fx"
	"github.com/smartlife-lab/mimi/backend/internal/history"
	"github.com/smartlife-lab/mimi/backend/internal/home"
	"github.com/smartlife-lab/mimi/backend/internal/household"
	"github.com/smartlife-lab/mimi/backend/internal/idem"
	"github.com/smartlife-lab/mimi/backend/internal/inbox"
	"github.com/smartlife-lab/mimi/backend/internal/item"
	"github.com/smartlife-lab/mimi/backend/internal/market"
	"github.com/smartlife-lab/mimi/backend/internal/order"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/pricing"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
	"github.com/smartlife-lab/mimi/backend/internal/recurring"
	"github.com/smartlife-lab/mimi/backend/internal/spending"
	"github.com/smartlife-lab/mimi/backend/internal/user"
)

// Các phụ thuộc được khai báo ở phía dùng, nhỏ nhất có thể.
type (
	TokenVerifier interface {
		VerifyIDToken(ctx context.Context, idToken string) (uid string, err error)
	}
	// Accounts xóa tài khoản và dữ liệu (B10). SignedInAt kiểm tra token kèm thu hồi và trả
	// lúc đăng nhập gần nhất, để buộc vừa đăng nhập lại trước khi xóa.
	Accounts interface {
		Delete(ctx context.Context, p authz.Principal) error
	}
	SignInChecker interface {
		SignedInAt(ctx context.Context, idToken string) (uid string, at time.Time, err error)
	}
	AppCheckVerifier interface {
		VerifyAppCheck(ctx context.Context, token string) error
	}
	PrincipalLoader interface {
		LoadPrincipal(ctx context.Context, uid string) (authz.Principal, error)
	}
	Profiles interface {
		Profile(ctx context.Context, p authz.Principal) (user.Profile, error)
	}
	Receipts interface {
		Read(ctx context.Context, p authz.Principal, receiptID, lang string, imgs []receipt.Image) (receipt.Draft, error)
		Save(ctx context.Context, p authz.Principal, req receipt.SaveRequest) (receipt.SaveOutcome, error)
	}
	Items interface {
		List(ctx context.Context, p authz.Principal, q item.Query) ([]item.Item, error)
		Get(ctx context.Context, p authz.Principal, key string) (item.Detail, error)
		ListShared(ctx context.Context, p authz.Principal, horizon int) ([]item.SharedItem, error)
		ChangeStock(ctx context.Context, p authz.Principal, key string, c item.StockChange) (item.Item, error)
		UndoStock(ctx context.Context, p authz.Principal, key string, actedAt time.Time) (item.Item, error)
		SetQuantity(ctx context.Context, p authz.Principal, key string, q item.QuantitySet) (item.Item, error)
		Correct(ctx context.Context, p authz.Principal, key string, c item.Correction) (item.Item, error)
	}
	Home interface {
		Get(ctx context.Context, p authz.Principal) (home.Summary, error)
	}
	Spending interface {
		Get(ctx context.Context, p authz.Principal, month string) (spending.Report, error)
	}
	Plan interface {
		Get(ctx context.Context, p authz.Principal) (plan.Cart, error)
		Add(ctx context.Context, p authz.Principal, key string) (plan.Cart, error)
		UpdateLine(ctx context.Context, p authz.Principal, key string, patch plan.LinePatch) (plan.Cart, error)
		Remove(ctx context.Context, p authz.Principal, key string) error
		SetState(ctx context.Context, p authz.Principal, st plan.State) (plan.State, error)
		Prices(ctx context.Context, p authz.Principal) (plan.Prices, error)
		AddWith(ctx context.Context, p authz.Principal, key string, req plan.AddRequest) (plan.Cart, error)
		Plans(ctx context.Context, p authz.Principal) ([]plan.Meta, error)
		CreatePlan(ctx context.Context, p authz.Principal, in plan.PlanInput) (plan.Meta, error)
		UpdatePlan(ctx context.Context, p authz.Principal, id string, in plan.PlanInput) (plan.Meta, error)
		DeletePlan(ctx context.Context, p authz.Principal, id string) error
	}
	// Recurring là khoản định kỳ và Dự báo 30 ngày (2.1).
	Recurring interface {
		List(ctx context.Context, p authz.Principal) (recurring.List, error)
		Create(ctx context.Context, p authz.Principal, in recurring.Input) (recurring.View, error)
		Update(ctx context.Context, p authz.Principal, id string, in recurring.Input) (recurring.View, error)
		Delete(ctx context.Context, p authz.Principal, id string) error
		Pay(ctx context.Context, p authz.Principal, occurrenceID string, req recurring.SettleRequest) (recurring.Occurrence, error)
		Skip(ctx context.Context, p authz.Principal, occurrenceID string) (recurring.Occurrence, error)
		Forecast(ctx context.Context, p authz.Principal) (recurring.Forecast, error)
	}
	// Households là Nhà và Gửi kế hoạch mua hộ (4.1).
	Households interface {
		Get(ctx context.Context, p authz.Principal, me household.Member) (household.Household, error)
		Rename(ctx context.Context, p authz.Principal, me household.Member, patch household.Patch) (household.Household, error)
		Invite(ctx context.Context, p authz.Principal, me household.Member) (household.Invite, error)
		Join(ctx context.Context, p authz.Principal, me household.Member, req household.JoinRequest) (household.Household, error)
		Leave(ctx context.Context, p authz.Principal) error
		RemoveMember(ctx context.Context, p authz.Principal, uid string) error
		Assign(ctx context.Context, p authz.Principal, req household.AssignRequest) (household.Assignment, error)
		Cancel(ctx context.Context, p authz.Principal, id string) (household.Assignment, error)
		Complete(ctx context.Context, p authz.Principal, id string, req household.CompleteRequest) (household.Assignment, error)
		Decline(ctx context.Context, p authz.Principal, id string) (household.Assignment, error)
		Delegated(ctx context.Context, p authz.Principal) (household.Delegated, error)
	}
	// Pricing là lịch sử giá dùng chung cho Đồ dùng và Kế hoạch (8.1).
	History interface {
		History(ctx context.Context, p authz.Principal) ([]history.Entry, error)
		Detail(ctx context.Context, p authz.Principal, id string) (history.Detail, error)
	}
	// Market là bảng giá chung (ADR-004), chỉ từ nguồn công khai.
	// FXHistory đọc tỷ giá từng ngày để app quy đổi số tiền sang tiền tệ hiển thị.
	FXHistory interface {
		RateHistory(ctx context.Context, currency string, from, to domain.Date) ([]fx.Point, error)
	}
	Market interface {
		Entries(ctx context.Context, keys []string) (map[string]market.Entry, error)
	}
	// PriceRefresh là nút "Cập nhật giá": tra lại giá các món của một kế hoạch.
	PriceRefresh interface {
		Refresh(ctx context.Context, uid, day string, items []market.Wanted) (market.Result, error)
		Status(ctx context.Context, uid, day string) (market.RefreshStatus, error)
	}
	Pricing interface {
		Item(ctx context.Context, p authz.Principal, key string) (pricing.ItemHistory, error)
		Basket(ctx context.Context, p authz.Principal) (pricing.Basket, error)
	}
	Notifications interface {
		List(ctx context.Context, p authz.Principal) (inbox.List, error)
	}
	Settings interface {
		Update(ctx context.Context, p authz.Principal, patch user.SettingsPatch) (map[string]any, error)
	}
	// Orders là nhánh Mimi Online (Mimi 2.0 mục 12 đến 19).
	Orders interface {
		GetMandate(ctx context.Context, p authz.Principal) (order.Mandate, error)
		SetMandate(ctx context.Context, p authz.Principal, m order.Mandate) (order.Mandate, error)
		Basket(ctx context.Context, p authz.Principal, keys []string) (order.Basket, error)
		Checkout(ctx context.Context, p authz.Principal, req order.CheckoutRequest) (order.Order, error)
		Capture(ctx context.Context, p authz.Principal, id string) (order.Order, error)
		CardToken(ctx context.Context, p authz.Principal) (string, error)
		PayCard(ctx context.Context, p authz.Principal, id, nonce string) (order.Order, error)
		CardWebhook(ctx context.Context, signature, payload string) error
		Advance(ctx context.Context, p authz.Principal, id string) (order.Order, error)
		ReadDelivery(ctx context.Context, p authz.Principal, id string, imgs []receipt.Image) ([]order.Observed, error)
		Verify(ctx context.Context, p authz.Principal, id string, req order.VerifyRequest) (order.Order, error)
		Refund(ctx context.Context, p authz.Principal, id string, req order.RefundRequest) (order.Order, error)
		Get(ctx context.Context, p authz.Principal, id string) (order.Order, error)
		List(ctx context.Context, p authz.Principal) ([]order.Order, error)
	}
	// Assistant chạy một lượt Hỏi Mimi cho người dùng (POST /ai/turn).
	Assistant interface {
		Turn(ctx context.Context, uid string, req assistant.Request) (assistant.Response, error)
	}
	ReadinessChecker interface {
		Ready(ctx context.Context) error
	}
	Metrics interface {
		ObserveHTTP(route string, status int, d time.Duration)
	}
)

// Deps gom các phụ thuộc bắt buộc; nối thủ công trong cmd/mimi.
type Deps struct {
	Log        *slog.Logger
	Tokens     TokenVerifier
	AppCheck   AppCheckVerifier // nil khi chạy với emulator ở máy
	Principals PrincipalLoader
	Profiles   Profiles
	Receipts   Receipts
	Items      Items
	Home       Home
	Spending   Spending
	Settings   Settings
	Plan       Plan
	Notes      Notifications
	Orders     Orders     // nil: các route Mimi Online trả 501
	Recurring  Recurring  // nil: các route khoản định kỳ trả 501
	Households Households // nil: các route nhà trả 501
	Pricing    Pricing    // nil: các route lịch sử giá trả 501
	Market     Market     // nil: không có route giá đang bán
	// PriceRefresh nil: không có nút cập nhật giá (cần cả Market).
	PriceRefresh PriceRefresh
	History      History   // nil: lịch sử mua hàng trả 404
	FX           FXHistory // nil: không có GET /fx/rates (app chỉ hiện VND)
	Accounts     Accounts  // nil: DELETE /me trả 501
	Assistant    Assistant // nil: không có POST /ai/turn (ẩn Hỏi Mimi)
	SignIn       SignInChecker
	Ready        ReadinessChecker
	Metrics      Metrics // nil: không ghi metric
}

type Server struct {
	log          *slog.Logger
	tokens       TokenVerifier
	accounts     Accounts
	signIn       SignInChecker
	appCheck     AppCheckVerifier
	principals   PrincipalLoader
	profiles     Profiles
	receipts     Receipts
	items        Items
	home         Home
	spending     Spending
	settings     Settings
	plan         Plan
	notes        Notifications
	orders       Orders
	recurring    Recurring
	households   Households
	pricing      Pricing
	market       Market
	priceRefresh PriceRefresh
	history      History
	fx           FXHistory
	assistant    Assistant
	ready        ReadinessChecker
	metrics      Metrics
	limiter      *windowLimiter
	shared       SharedLimiter // nil: chỉ đếm tại instance này
	idem         idem.Store    // nil: không giữ kết quả theo Idempotency-Key

	readTimeout    time.Duration
	defaultTimeout time.Duration
}

type Option func(*Server)

// WithTimeouts đặt ngân sách thời gian: readTimeout cho POST /receipts/read (dưới
// 60 s của Hosting rewrite), defaultTimeout cho mọi route khác.
func WithTimeouts(read, def time.Duration) Option {
	return func(s *Server) { s.readTimeout, s.defaultTimeout = read, def }
}

func WithWriteLimit(perMinute int) Option {
	return func(s *Server) { s.limiter = newWindowLimiter(perMinute) }
}

func New(d Deps, opts ...Option) (*Server, error) {
	if d.Log == nil || d.Tokens == nil || d.Principals == nil || d.Receipts == nil || d.Items == nil || d.Ready == nil || d.Profiles == nil ||
		d.Home == nil || d.Spending == nil || d.Settings == nil || d.Plan == nil || d.Notes == nil {
		return nil, errors.New("httpapi: missing required dependency")
	}
	s := &Server{
		log: d.Log, tokens: d.Tokens, accounts: d.Accounts, signIn: d.SignIn, appCheck: d.AppCheck, principals: d.Principals,
		profiles: d.Profiles, receipts: d.Receipts, items: d.Items, ready: d.Ready, metrics: d.Metrics,
		home: d.Home, spending: d.Spending, settings: d.Settings, plan: d.Plan, notes: d.Notes, orders: d.Orders,
		recurring: d.Recurring, households: d.Households, pricing: d.Pricing, market: d.Market, priceRefresh: d.PriceRefresh, history: d.History, fx: d.FX, assistant: d.Assistant,
		limiter: newWindowLimiter(60), readTimeout: 55 * time.Second, defaultTimeout: 10 * time.Second,
	}
	if s.metrics == nil {
		s.metrics = noopMetrics{}
	}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

type noopMetrics struct{}

func (noopMetrics) ObserveHTTP(string, int, time.Duration) {}

// Handler trả http.Handler hoàn chỉnh. Firebase Hosting rewrite giữ nguyên tiền tố
// /api, nên router nhận cả hai dạng có và không có /api.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	public := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.observe(pattern, h))
	}
	private := func(pattern string, timeout time.Duration, h http.HandlerFunc) {
		var chain http.Handler = h
		chain = s.limitWrites(chain)
		chain = s.idempotent(chain)
		chain = s.authenticate(chain)
		chain = http.TimeoutHandler(chain, timeout, `{"code":"timeout","message":"Hết thời gian xử lý, thử lại."}`)
		mux.Handle(pattern, s.observe(pattern, chain))
	}

	public("GET /healthz", s.healthz)
	public("GET /health", s.healthz)
	public("GET /readyz", s.readyz)
	public("GET /ready", s.readyz) // như /health: Cloud Run chặn đường dẫn kết thúc bằng z ở URL công khai

	private("GET /me", s.defaultTimeout, s.getMe)
	private("PUT /settings", s.defaultTimeout, s.putSettings)
	private("GET /home", s.defaultTimeout, s.getHome)
	private("POST /receipts/read", s.readTimeout, s.readReceipt)
	private("POST /receipts", s.defaultTimeout, s.saveReceipt)
	private("GET /items", s.defaultTimeout, s.listItems)
	private("GET /items/{key}", s.defaultTimeout, s.getItem)
	private("PATCH /items/{key}/stock", s.defaultTimeout, s.changeStock)
	private("DELETE /items/{key}/override", s.defaultTimeout, s.undoStock)
	private("PUT /items/{key}/quantity", s.defaultTimeout, s.setQuantity)
	private("PATCH /items/{key}", s.defaultTimeout, s.correctItem)
	private("GET /households/needs", s.defaultTimeout, s.listSharedNeeds)
	private("GET /spending", s.defaultTimeout, s.getSpending)
	private("GET /plan", s.defaultTimeout, s.getPlan)
	private("PUT /plan/items/{key}", s.defaultTimeout, s.addPlanItem)
	private("PATCH /plan/items/{key}", s.defaultTimeout, s.patchPlanItem)
	private("DELETE /plan/items/{key}", s.defaultTimeout, s.removePlanItem)
	private("PUT /plan/state", s.defaultTimeout, s.putPlanState)
	private("GET /plan/prices", s.defaultTimeout, s.getPlanPrices)
	private("GET /notifications", s.defaultTimeout, s.getNotifications)
	private("POST /notifications/{id}/read", s.defaultTimeout, s.markNotificationRead)
	private("GET /taxonomy", s.defaultTimeout, s.getTaxonomy)
	private("GET /plans", s.defaultTimeout, s.listPlans)
	private("POST /plans", s.defaultTimeout, s.createPlan)
	private("PATCH /plans/{id}", s.defaultTimeout, s.patchPlan)
	private("DELETE /plans/{id}", s.defaultTimeout, s.deletePlan)

	if s.recurring != nil {
		private("GET /recurring", s.defaultTimeout, s.listRecurring)
		private("POST /recurring", s.defaultTimeout, s.createRecurring)
		private("PATCH /recurring/{id}", s.defaultTimeout, s.patchRecurring)
		private("DELETE /recurring/{id}", s.defaultTimeout, s.deleteRecurring)
		private("POST /recurring/occurrences/{id}/pay", s.defaultTimeout, s.payOccurrence)
		private("POST /recurring/occurrences/{id}/skip", s.defaultTimeout, s.skipOccurrence)
		private("GET /forecast", s.defaultTimeout, s.getForecast)
	}
	if s.households != nil {
		private("GET /households", s.defaultTimeout, s.getHousehold)
		private("PATCH /households", s.defaultTimeout, s.patchHousehold)
		private("POST /households/invites", s.defaultTimeout, s.postInvite)
		private("POST /households/join", s.defaultTimeout, s.postJoin)
		private("POST /households/leave", s.defaultTimeout, s.postLeave)
		private("DELETE /households/members/{uid}", s.defaultTimeout, s.deleteMember)
		private("POST /households/shopping-assignments", s.defaultTimeout, s.postAssignment)
		private("POST /households/shopping-assignments/{id}/cancel", s.defaultTimeout, s.cancelAssignment)
		private("POST /households/shopping-assignments/{id}/complete", s.defaultTimeout, s.completeAssignment)
		private("POST /households/shopping-assignments/{id}/decline", s.defaultTimeout, s.declineAssignment)
		private("GET /plan/delegated", s.defaultTimeout, s.getDelegated)
	}
	if s.history != nil {
		private("GET /receipts/history", s.defaultTimeout, s.getReceiptHistory)
		private("GET /receipts/{id}", s.defaultTimeout, s.getReceiptDetail)
	}
	if s.pricing != nil {
		private("GET /items/{key}/price-history", s.defaultTimeout, s.getItemPriceHistory)
		private("GET /plan/price-history", s.defaultTimeout, s.getPlanPriceHistory)
	}
	if s.fx != nil {
		private("GET /fx/rates", s.defaultTimeout, s.getFXRates)
	}
	if s.market != nil {
		private("GET /plan/market-prices", s.defaultTimeout, s.getPlanMarketPrices)
		private("GET /items/{key}/market-prices", s.defaultTimeout, s.getItemMarketPrices)
		private("GET /items/market-prices", s.defaultTimeout, s.listItemMarketPrices)
		if s.priceRefresh != nil {
			private("POST /plan/market-prices/refresh", s.readTimeout, s.refreshPlanMarketPrices)
		}
	}

	if s.orders != nil {
		private("GET /mandate", s.defaultTimeout, s.getMandate)
		private("PUT /mandate", s.defaultTimeout, s.putMandate)
		private("POST /orders/basket", s.defaultTimeout, s.postBasket)
		private("POST /orders", s.readTimeout, s.postOrder)
		private("GET /orders", s.defaultTimeout, s.listOrders)
		private("GET /orders/{id}", s.defaultTimeout, s.getOrder)
		private("POST /orders/{id}/capture", s.readTimeout, s.captureOrder)
		private("POST /orders/card-token", s.defaultTimeout, s.cardToken)
		private("POST /orders/{id}/card", s.readTimeout, s.payCard)
		private("POST /orders/{id}/advance", s.defaultTimeout, s.advanceOrder)
		private("POST /orders/{id}/delivery/read", s.readTimeout, s.readDelivery)
		private("POST /orders/{id}/verify", s.defaultTimeout, s.verifyOrder)
		private("POST /orders/{id}/refund", s.readTimeout, s.refundOrder)
		// Braintree gọi thẳng, không có token người dùng: tin nhờ chữ ký bt_signature.
		public("POST /webhooks/braintree", s.braintreeWebhook)
	}

	if s.assistant != nil {
		private("POST /ai/turn", s.readTimeout, s.aiTurn)
	}

	if s.accounts != nil && s.signIn != nil {
		private("DELETE /me", s.defaultTimeout, s.deleteMe)
	}

	// Các endpoint còn lại đã có trong api/openapi.yaml nhưng chưa dựng ở khung này.
	for _, p := range notYetImplemented {
		if s.implemented(p) {
			continue
		}
		private(p, s.defaultTimeout, s.notImplemented)
	}

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", mux))
	root.Handle("/", mux)
	return root
}

// implemented: route trong notYetImplemented đã có handler khi phụ thuộc tương ứng được nối.
func (s *Server) implemented(p string) bool {
	switch {
	case p == "DELETE /me":
		return s.accounts != nil && s.signIn != nil
	case strings.Contains(p, "/recurring") || p == "GET /forecast":
		return s.recurring != nil && p != "POST /recurring/scan"
	case strings.Contains(p, "/households") && !strings.Contains(p, "/asks"):
		return s.households != nil
	}
	return false
}

var notYetImplemented = []string{
	"DELETE /me",
	"GET /needs",
	"PATCH /receipts/{id}/lines/{lineId}", "GET /receipts",
	"GET /forecast",
	"GET /recurring", "POST /recurring", "PATCH /recurring/{id}", "DELETE /recurring/{id}", "POST /recurring/scan",
	"GET /households", "PATCH /households", "POST /households/invites", "POST /households/join",
	"DELETE /households/members/{uid}", "POST /households/leave",
	"POST /households/asks", "POST /households/asks/{id}/bought",
}
