// Package order là nhánh Mimi Online của Mimi 2.0 (mục 12 đến 19):
//
//	Plan → Online Basket → Household Mandate → PayPal → Fulfillment → Verify → Refund → Reconcile
//
// Nguyên tắc giữ nguyên từ spec:
//   - Payment success chỉ đổi Financial State và Spending, không cộng tồn kho.
//   - Delivered không phải Verified; chỉ lượng đã Verify mới vào Confirmed Inventory.
//   - Refund giữ cả giao dịch gốc và refund để audit; Spending tính theo net.
package order

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/smartlife-lab/mimi/backend/internal/apperr"
	"github.com/smartlife-lab/mimi/backend/internal/authz"
	"github.com/smartlife-lab/mimi/backend/internal/braintree"
	"github.com/smartlife-lab/mimi/backend/internal/domain"
	"github.com/smartlife-lab/mimi/backend/internal/paypal"
	"github.com/smartlife-lab/mimi/backend/internal/plan"
	"github.com/smartlife-lab/mimi/backend/internal/receipt"
)

// State là trạng thái đơn online (Mimi 2.0 mục 16).
type State string

const (
	StateAwaitingPayment State = "awaiting_payment" // đã tạo PayPal order, chờ người dùng approve
	StatePaid            State = "paid"
	StateOrdered         State = "ordered"
	StateShipped         State = "shipped"
	StateDelivered       State = "delivered"
	StateVerified        State = "verified" // đã kiểm hàng và reconcile vào tồn kho, Kế hoạch
)

// nextFulfillment: tiến trình giao hàng giả lập của merchant Sandbox.
var nextFulfillment = map[State]State{StatePaid: StateOrdered, StateOrdered: StateShipped, StateShipped: StateDelivered}

// Line là một dòng giỏ online / đơn hàng. Qty tính theo gói của sản phẩm.
type Line struct {
	Key              string   `json:"key"`
	Name             string   `json:"name"` // tên nhóm trong Kế hoạch
	ProductName      string   `json:"product_name"`
	CategoryKey      string   `json:"category_key,omitempty"` // danh mục chi tiêu
	Category         string   `json:"category"`               // grocery | household (mandate)
	Qty              float64  `json:"qty"`                    // số gói
	Unit             string   `json:"unit"`                   // đơn vị của gói
	PackQty          float64  `json:"pack_qty"`
	PlanQty          float64  `json:"plan_qty"`
	PlanUnit         string   `json:"plan_unit,omitempty"`
	UnitPriceVND     int64    `json:"unit_price_vnd"`     // giá đang bán mỗi gói
	SnapshotPriceVND int64    `json:"snapshot_price_vnd"` // giá tốt nhất ở lần quét gần nhất
	TotalVND         int64    `json:"total_vnd"`
	NeedBy           string   `json:"need_by,omitempty"`
	ETA              string   `json:"eta"`
	InPlan           bool     `json:"-"`
	OverPlan         bool     `json:"-"`
	VerifiedQty      *float64 `json:"verified_qty,omitempty"` // số gói thực nhận sau Verify
	Match            string   `json:"match,omitempty"`        // match | partial | missing
}

// Basket là giỏ online đã resolve product, giá, ETA và đã qua Household Mandate.
type Basket struct {
	Merchant   string    `json:"merchant"`
	Lines      []Line    `json:"lines"`
	Missing    []string  `json:"missing"` // món trong Kế hoạch merchant không bán
	TotalVND   int64     `json:"total_vnd"`
	AmountUSD  string    `json:"amount_usd"`
	VNDPerUSD  int64     `json:"vnd_per_usd"`
	RateSource string    `json:"rate_source,omitempty"` // tỷ giá lấy từ đâu
	RateAsOf   string    `json:"rate_as_of,omitempty"`  // tỷ giá của ngày nào, YYYY-MM-DD
	SnapshotAt time.Time `json:"snapshot_at"`
	Decision   Decision  `json:"decision"`
	// CardEnabled: server có cấu hình Braintree, app hiện thêm nút "Trả bằng thẻ".
	CardEnabled bool `json:"card_enabled"`
}

// Refund là một lần hoàn tiền; giữ nguyên để audit cùng giao dịch gốc.
type Refund struct {
	ID        string    `json:"id"` // ID refund của PayPal
	Ref       string    `json:"ref"`
	AmountVND int64     `json:"amount_vnd"`
	AmountUSD string    `json:"amount_usd"`
	Reason    string    `json:"reason"` // missing | wrong | damaged | other
	Status    string    `json:"status"`
	At        time.Time `json:"at"`
}

type Event struct {
	State State     `json:"state"`
	At    time.Time `json:"at"`
}

// Order là đơn Mimi Online, lưu nguyên khối (users/{uid}/orders/{id}).
type Order struct {
	ID             string   `json:"id"`
	State          State    `json:"state"`
	Merchant       string   `json:"merchant"`
	Lines          []Line   `json:"lines"`
	TotalVND       int64    `json:"total_vnd"`
	AmountUSD      string   `json:"amount_usd"`
	VNDPerUSD      int64    `json:"vnd_per_usd"`
	RateSource     string   `json:"rate_source,omitempty"`
	RateAsOf       string   `json:"rate_as_of,omitempty"`
	Decision       Decision `json:"decision"`
	ApprovedByUser bool     `json:"approved_by_user"` // người dùng duyệt đơn ASK
	// PlanID là Kế hoạch của đơn ("" là Giỏ mua sắm chính); lúc kiểm hàng, Mimi trừ vào đúng Kế hoạch này.
	PlanID string `json:"plan_id,omitempty"`
	// Provider là cổng đã nhận tiền (paypal | braintree). Đơn lưu trước khi có ô này là paypal.
	Provider        string `json:"provider"`
	ProviderOrderID string `json:"provider_order_id,omitempty"`
	ProviderTxnID   string `json:"provider_txn_id,omitempty"`
	// Ô cũ của PayPal, vẫn ghi cho đơn PayPal để app và đơn đã lưu đọc được.
	PayPalOrderID      string     `json:"paypal_order_id"`
	ApproveURL         string     `json:"approve_url,omitempty"`
	CaptureID          string     `json:"paypal_capture_id,omitempty"`
	PaidAt             *time.Time `json:"paid_at,omitempty"`
	ETA                string     `json:"eta"`
	Events             []Event    `json:"events"`
	Refunds            []Refund   `json:"refunds"`
	RefundedVND        int64      `json:"refunded_vnd"`
	NetVND             int64      `json:"net_vnd"` // = total - refunded, chỉ có nghĩa sau khi paid
	SuggestedRefundVND int64      `json:"suggested_refund_vnd"`
	// CardStatus: trạng thái tiền thẻ Braintree báo qua webhook sau khi thu
	// (settled | settlement_declined | disputed | dispute_won | dispute_lost); rỗng: chưa có tin.
	CardStatus   string     `json:"card_status,omitempty"`
	CardStatusAt *time.Time `json:"card_status_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Phụ thuộc, khai báo ở phía dùng.
type (
	Payments interface {
		CreateOrder(ctx context.Context, ref, amountUSD, description string) (paypal.Order, error)
		CaptureOrder(ctx context.Context, paypalOrderID, ref string) (paypal.Capture, error)
		RefundCapture(ctx context.Context, captureID, amountUSD, ref, note string) (paypal.Refund, error)
	}
	// Cards là cổng thẻ (Braintree): app lấy client token, Drop-in trả nonce, server thu tiền.
	Cards interface {
		ClientToken(ctx context.Context) (string, error)
		Charge(ctx context.Context, nonce, amountUSD, ref string) (braintree.Transaction, error)
		// Refund: full=true khi hoàn hết số còn lại (giao dịch chưa quyết toán thì hủy).
		Refund(ctx context.Context, txnID, amountUSD, ref string, full bool) (braintree.Refund, error)
		// ParseWebhook kiểm chữ ký và đọc thông báo webhook Braintree.
		ParseWebhook(signature, payload string) (braintree.Notification, error)
	}
	PlanLines interface {
		OpenLines(ctx context.Context, uid string) ([]plan.Line, error)
	}
	// Verifier đọc ảnh hàng nhận và đếm từng món (Mimi 2.0 mục 17).
	Verifier interface {
		VerifyDelivery(ctx context.Context, imgs []receipt.Image, expected []Expected) ([]Observed, error)
	}
	Store interface {
		Mandate(ctx context.Context, uid string) (Mandate, error)
		SetMandate(ctx context.Context, uid string, m Mandate) error
		CreateOrder(ctx context.Context, uid string, o Order) error
		GetOrder(ctx context.Context, uid, id string) (Order, error) // apperr.ErrNotFound
		ListOrders(ctx context.Context, uid string, limit int) ([]Order, error)
		// UpdateOrder ghi đơn nếu trạng thái đang lưu vẫn là from (chống ghi đè đồng thời).
		UpdateOrder(ctx context.Context, uid string, o Order, from State) error
		// RecordPayment: đơn sang paid và ghi Spending trong một transaction.
		RecordPayment(ctx context.Context, uid string, o Order, tz *time.Location) error
		// ApplyDelivery: đơn sang verified, cộng lượng đã Verify vào Confirmed Inventory
		// (baseline nguồn delivery) và reconcile Kế hoạch, trong một transaction.
		ApplyDelivery(ctx context.Context, uid string, o Order, tz *time.Location) error
		// RecordRefund: thêm refund vào đơn và trừ Spending trong một transaction.
		RecordRefund(ctx context.Context, uid string, o Order, r Refund, tz *time.Location) error
		// CardOrder tìm chủ đơn theo mã giao dịch Braintree (apperr.ErrNotFound).
		CardOrder(ctx context.Context, txnID string) (uid, orderID string, err error)
	}
)

// Expected là một món Mimi chờ nhận, gửi cho AI để so với ảnh.
type Expected struct {
	Key  string
	Name string
	Qty  float64
	Unit string
}

// Observed là kết quả AI đếm được cho một món; Confidence low thì người dùng phải xác nhận.
type Observed struct {
	Key        string  `json:"key"`
	Qty        float64 `json:"qty"`
	Confidence string  `json:"confidence"` // low | mid | high
}

type Service struct {
	store     Store
	plans     PlanLines
	catalog   Catalog
	payments  Payments // nil: chưa cấu hình PayPal
	cards     Cards    // nil: chưa cấu hình Braintree
	verifier  Verifier // nil: chỉ cho nhập tay
	vndPerUSD int64
	rateSrc   string
	rateAsOf  string
	log       *slog.Logger
	now       func() time.Time
}

func New(store Store, plans PlanLines, catalog Catalog, payments Payments, verifier Verifier, vndPerUSD int64, log *slog.Logger, now func() time.Time) (*Service, error) {
	if vndPerUSD <= 0 {
		return nil, errors.New("order: vndPerUSD must be > 0")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, plans: plans, catalog: catalog, payments: payments, verifier: verifier, vndPerUSD: vndPerUSD, log: log, now: now}, nil
}

// WithCards bật lựa chọn trả bằng thẻ qua Braintree.
func (s *Service) WithCards(c Cards) *Service {
	s.cards = c
	return s
}

// CardsEnabled: app chỉ hiện nút thẻ khi server đã cấu hình Braintree.
func (s *Service) CardsEnabled() bool { return s.cards != nil }

// WithRateInfo ghi nguồn và ngày của tỷ giá vào mỗi giỏ và đơn hàng (vấn đề 44).
func (s *Service) WithRateInfo(source, asOf string) *Service {
	s.rateSrc, s.rateAsOf = source, asOf
	return s
}

// ToUSD quy đồng sang USD 2 chữ số thập phân, làm tròn nửa lên. PayPal không nhận VND.
func ToUSD(vnd, vndPerUSD int64) string {
	cents := (vnd*100 + vndPerUSD/2) / vndPerUSD
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// ---------- Mandate ----------

func (s *Service) GetMandate(ctx context.Context, p authz.Principal) (Mandate, error) {
	if err := authz.Can(p, authz.ActionManageMandate); err != nil {
		return Mandate{}, err
	}
	return s.store.Mandate(ctx, p.UID)
}

func (s *Service) SetMandate(ctx context.Context, p authz.Principal, m Mandate) (Mandate, error) {
	if err := authz.Can(p, authz.ActionManageMandate); err != nil {
		return Mandate{}, err
	}
	if err := m.Validate(); err != nil {
		return Mandate{}, apperr.Wrap(apperr.CodeValidation, "Cài đặt ủy quyền chưa hợp lệ.", err)
	}
	if m.Categories == nil {
		m.Categories = []string{}
	}
	if err := s.store.SetMandate(ctx, p.UID, m); err != nil {
		return Mandate{}, err
	}
	return m, nil
}

// ---------- Giỏ online ----------

// Basket dựng giỏ online từ các dòng Kế hoạch đang mở (keys rỗng: tất cả) và đánh giá
// Household Mandate. Chưa tạo giao dịch.
func (s *Service) Basket(ctx context.Context, p authz.Principal, keys []string) (Basket, error) {
	if err := authz.Can(p, authz.ActionBuyOnline); err != nil {
		return Basket{}, err
	}
	all, err := s.plans.OpenLines(ctx, p.UID)
	if err != nil {
		return Basket{}, err
	}
	lines := make([]plan.Line, 0, len(all))
	for _, l := range all {
		if !l.Assigned() { // dòng đang nhờ người khác mua không vào giỏ online
			lines = append(lines, l)
		}
	}
	today := domain.Today(s.now(), domain.Location(p.Timezone))
	b := buildBasket(s.catalog, lines, keys, today)
	b.VNDPerUSD, b.AmountUSD = s.vndPerUSD, ToUSD(b.TotalVND, s.vndPerUSD)
	b.RateSource, b.RateAsOf = s.rateSrc, s.rateAsOf
	m, err := s.store.Mandate(ctx, p.UID)
	if err != nil {
		return Basket{}, err
	}
	b.Decision = Evaluate(m, b)
	b.CardEnabled = s.cards != nil
	return b, nil
}

// buildBasket: product resolution theo group (exact product do merchant Sandbox bán).
// Số gói = làm tròn lên lượng Kế hoạch / cỡ gói khi cùng loại đơn vị; khác loại thì mua
// đúng số lượng Kế hoạch. Không thay thế sản phẩm (substitution để phase sau).
func buildBasket(c Catalog, lines []plan.Line, keys []string, today domain.Date) Basket {
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	inPlan := map[string]plan.Line{}
	for _, l := range lines {
		inPlan[l.Key] = l
	}
	b := Basket{Merchant: c.Merchant(), Lines: []Line{}, Missing: []string{}, SnapshotAt: c.SnapshotAt()}
	ordered := keys
	if len(keys) == 0 {
		for _, l := range lines {
			ordered = append(ordered, l.Key)
		}
		sort.Strings(ordered)
	}
	seen := map[string]bool{}
	for _, k := range ordered {
		if seen[k] {
			continue
		}
		seen[k] = true
		pl, ok := inPlan[k]
		if !ok {
			// Món ngoài Kế hoạch: vẫn đưa vào để Mandate chặn và người dùng thấy lý do.
			if pr, found := c.Product(k); found {
				b.Lines = append(b.Lines, Line{Key: k, Name: pr.Name, ProductName: pr.Name, Category: pr.Category, Unit: pr.Unit, PackQty: pr.PackQty})
			} else {
				b.Missing = append(b.Missing, k)
			}
			continue
		}
		pr, found := c.Product(k)
		if !found {
			b.Missing = append(b.Missing, k)
			continue
		}
		planQty := pl.Qty
		if planQty <= 0 {
			planQty = 1
		}
		packs := planQty
		wantBase, wantUnit := domain.ToBase(planQty, pl.Unit)
		packBase, packUnit := domain.ToBase(pr.PackQty, pr.Unit)
		overPlan := false
		if wantUnit == packUnit && packBase > 0 {
			packs = math.Ceil(wantBase/packBase - 1e-9)
			// Quantity guard: làm tròn lên gói được vượt tối đa một gói.
			overPlan = packs*packBase > wantBase+packBase
		}
		l := Line{
			Key: k, Name: pl.Name, ProductName: pr.Name, CategoryKey: pl.CategoryKey, Category: pr.Category,
			Qty: packs, Unit: pr.Unit, PackQty: pr.PackQty, PlanQty: planQty, PlanUnit: pl.Unit,
			UnitPriceVND: pr.OfferVND, SnapshotPriceVND: pr.SnapshotVND, NeedBy: pl.NeedBy,
			ETA: today.AddDays(pr.EtaDays).String(), InPlan: true, OverPlan: overPlan,
		}
		l.TotalVND = int64(math.Round(packs * float64(pr.OfferVND)))
		b.TotalVND += l.TotalVND
		b.Lines = append(b.Lines, l)
	}
	return b
}

// ---------- Checkout (PayPal) ----------

// Cổng thanh toán của đơn.
const (
	ProviderPayPal    = "paypal"
	ProviderBraintree = "braintree"
)

// ProviderOf trả cổng của đơn; đơn cũ chưa có ô provider là PayPal.
func ProviderOf(o Order) string {
	if o.Provider == "" {
		return ProviderPayPal
	}
	return o.Provider
}

// Description là mô tả đơn gửi cổng thanh toán: tiếng Anh, chỉ ASCII, tối đa 127 ký tự
// (giới hạn của PayPal purchase_units.description).
func Description(items int) string {
	unit := "items"
	if items == 1 {
		unit = "item"
	}
	return domain.ASCII(fmt.Sprintf("Mimi order - %d %s", items, unit), 127)
}

type CheckoutRequest struct {
	Keys    []string `json:"keys"`    // dòng Kế hoạch mua online; rỗng: tất cả
	Approve bool     `json:"approve"` // người dùng duyệt các ngoại lệ ASK
	// Provider: paypal (mặc định) hoặc braintree (trả bằng thẻ).
	Provider string `json:"provider,omitempty"`
}

// Checkout áp Mandate rồi tạo đơn chờ thanh toán. BLOCK luôn bị từ chối; ASK cần approve=true.
// PayPal: tạo PayPal order ngay. Braintree: app lấy client token, rồi gọi PayCard với nonce.
func (s *Service) Checkout(ctx context.Context, p authz.Principal, req CheckoutRequest) (Order, error) {
	switch req.Provider {
	case "", ProviderPayPal:
		req.Provider = ProviderPayPal
		if s.payments == nil {
			return Order{}, apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình PayPal.")
		}
	case ProviderBraintree:
		if s.cards == nil {
			return Order{}, apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình thanh toán thẻ.")
		}
	default:
		return Order{}, apperr.New(apperr.CodeValidation, "Cổng thanh toán không hợp lệ.").With("field", "provider")
	}
	if len(req.Keys) > plan.MaxCartLines {
		return Order{}, apperr.New(apperr.CodeValidation, "Quá nhiều món.").With("field", "keys")
	}
	b, err := s.Basket(ctx, p, req.Keys)
	if err != nil {
		return Order{}, err
	}
	// Mandate chặn trước: món ngoài Kế hoạch không có giá nên giỏ có thể rỗng tiền, nhưng lý
	// do người dùng cần thấy là vi phạm ủy quyền.
	if b.Decision.Result == Block {
		return Order{}, apperr.New(apperr.CodeMandateBlocked, "Đơn vi phạm phạm vi ủy quyền.").With("reasons", b.Decision.Reasons)
	}
	if len(b.Lines) == 0 || b.TotalVND <= 0 {
		return Order{}, apperr.New(apperr.CodeValidation, "Giỏ online chưa có món nào merchant bán.").With("missing", b.Missing)
	}
	switch b.Decision.Result {
	case Ask:
		if !req.Approve {
			return Order{}, apperr.New(apperr.CodeApprovalRequired, "Đơn cần bạn duyệt.").With("reasons", b.Decision.Reasons)
		}
	}
	now := s.now()
	o := Order{
		ID: newID(), State: StateAwaitingPayment, Merchant: b.Merchant, Lines: b.Lines,
		TotalVND: b.TotalVND, AmountUSD: b.AmountUSD, VNDPerUSD: b.VNDPerUSD, RateSource: b.RateSource, RateAsOf: b.RateAsOf, Decision: b.Decision,
		ApprovedByUser: b.Decision.Result == Ask, Refunds: []Refund{}, CreatedAt: now, UpdatedAt: now, PlanID: plan.IDFrom(ctx),
		Events: []Event{{State: StateAwaitingPayment, At: now}},
	}
	for _, l := range o.Lines {
		if l.ETA > o.ETA {
			o.ETA = l.ETA
		}
	}
	if req.Provider == ProviderBraintree {
		o.Provider = ProviderBraintree
	} else {
		ppo, err := s.payments.CreateOrder(ctx, o.ID, o.AmountUSD, Description(len(o.Lines)))
		if err != nil {
			return Order{}, paymentErr(err)
		}
		o.Provider, o.ProviderOrderID = ProviderPayPal, ppo.ID
		o.PayPalOrderID, o.ApproveURL = ppo.ID, ppo.ApproveURL
	}
	if err := s.store.CreateOrder(ctx, p.UID, o); err != nil {
		return Order{}, err
	}
	return o, nil
}

// Capture thu tiền sau khi người dùng approve trên PayPal; gọi lại khi đã paid trả đơn cũ.
func (s *Service) Capture(ctx context.Context, p authz.Principal, id string) (Order, error) {
	o, err := s.owned(ctx, p, id)
	if err != nil {
		return Order{}, err
	}
	if o.State != StateAwaitingPayment {
		if o.CaptureID != "" {
			return o, nil // đã capture: idempotent
		}
		return Order{}, invalidState(o.State)
	}
	if ProviderOf(o) != ProviderPayPal {
		return Order{}, invalidState(o.State)
	}
	if s.payments == nil {
		return Order{}, apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình PayPal.")
	}
	cp, err := s.payments.CaptureOrder(ctx, o.PayPalOrderID, o.ID)
	if err != nil {
		return Order{}, paymentErr(err)
	}
	if cp.Status != "COMPLETED" {
		return Order{}, apperr.New(apperr.CodePaymentFailed, "PayPal chưa hoàn tất thanh toán.").With("paypal_status", cp.Status)
	}
	now := s.now()
	o.State, o.CaptureID, o.ProviderTxnID, o.PaidAt, o.UpdatedAt = StatePaid, cp.ID, cp.ID, &now, now
	o.NetVND = o.TotalVND
	o.Events = append(o.Events, Event{State: StatePaid, At: now})
	if err := s.store.RecordPayment(ctx, p.UID, o, domain.Location(p.Timezone)); err != nil {
		return Order{}, err
	}
	return o, nil
}

// CardToken trả client token cho Drop-in UI của Braintree ở app.
func (s *Service) CardToken(ctx context.Context, p authz.Principal) (string, error) {
	if err := authz.Can(p, authz.ActionBuyOnline); err != nil {
		return "", err
	}
	if s.cards == nil {
		return "", apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình thanh toán thẻ.")
	}
	tok, err := s.cards.ClientToken(ctx)
	if err != nil {
		return "", cardErr(err)
	}
	return tok, nil
}

// maxNonceLen: nonce Braintree ngắn; giới hạn để không nhận chuỗi bất kỳ.
const maxNonceLen = 512

// PayCard thu tiền đơn Braintree bằng nonce Drop-in trả về; gọi lại khi đã paid trả đơn cũ.
func (s *Service) PayCard(ctx context.Context, p authz.Principal, id, nonce string) (Order, error) {
	o, err := s.owned(ctx, p, id)
	if err != nil {
		return Order{}, err
	}
	if ProviderOf(o) != ProviderBraintree {
		return Order{}, invalidState(o.State)
	}
	if o.State != StateAwaitingPayment {
		if o.ProviderTxnID != "" {
			return o, nil // đã thu: idempotent
		}
		return Order{}, invalidState(o.State)
	}
	nonce = strings.TrimSpace(nonce)
	if nonce == "" || len(nonce) > maxNonceLen || strings.IndexFunc(nonce, func(r rune) bool { return r <= ' ' || r > '~' }) >= 0 {
		return Order{}, apperr.New(apperr.CodeValidation, "Thiếu thông tin thẻ.").With("field", "nonce")
	}
	if s.cards == nil {
		return Order{}, apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình thanh toán thẻ.")
	}
	tx, err := s.cards.Charge(ctx, nonce, o.AmountUSD, o.ID)
	if err != nil {
		return Order{}, cardErr(err)
	}
	if !braintree.Paid(tx.Status) {
		return Order{}, apperr.New(apperr.CodePaymentFailed, "Thẻ chưa thanh toán được.").With("braintree_status", tx.Status)
	}
	now := s.now()
	o.State, o.ProviderOrderID, o.ProviderTxnID, o.PaidAt, o.UpdatedAt = StatePaid, tx.ID, tx.ID, &now, now
	o.NetVND = o.TotalVND
	o.Events = append(o.Events, Event{State: StatePaid, At: now})
	if err := s.store.RecordPayment(ctx, p.UID, o, domain.Location(p.Timezone)); err != nil {
		return Order{}, err
	}
	return o, nil
}

// Advance đẩy đơn sang bước giao hàng tiếp theo (merchant Sandbox giả lập:
// PAID → ORDERED → SHIPPED → DELIVERED). Không đổi tồn kho.
func (s *Service) Advance(ctx context.Context, p authz.Principal, id string) (Order, error) {
	o, err := s.owned(ctx, p, id)
	if err != nil {
		return Order{}, err
	}
	next, ok := nextFulfillment[o.State]
	if !ok {
		return Order{}, invalidState(o.State)
	}
	from := o.State
	now := s.now()
	o.State, o.UpdatedAt = next, now
	o.Events = append(o.Events, Event{State: next, At: now})
	if err := s.store.UpdateOrder(ctx, p.UID, o, from); err != nil {
		return Order{}, err
	}
	return o, nil
}

// ---------- Verify + Reconcile ----------

// ReadDelivery: AI đếm món trong ảnh hàng nhận. Chỉ là gợi ý; người dùng xác nhận bằng Verify.
func (s *Service) ReadDelivery(ctx context.Context, p authz.Principal, id string, imgs []receipt.Image) ([]Observed, error) {
	o, err := s.owned(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if o.State != StateDelivered {
		return nil, invalidState(o.State)
	}
	if s.verifier == nil {
		return nil, apperr.New(apperr.CodeAIUnavailable, "Chưa bật kiểm hàng bằng ảnh, hãy nhập số lượng nhận.")
	}
	if len(imgs) == 0 || len(imgs) > 4 {
		return nil, apperr.New(apperr.CodeValidation, "Gửi 1 đến 4 ảnh hàng nhận.")
	}
	exp := make([]Expected, 0, len(o.Lines))
	for _, l := range o.Lines {
		exp = append(exp, Expected{Key: l.Key, Name: l.ProductName, Qty: l.Qty, Unit: l.Unit})
	}
	obs, err := s.verifier.VerifyDelivery(ctx, imgs, exp)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeAIUnavailable, "Mimi chưa đọc được ảnh, thử lại hoặc nhập tay.", err)
	}
	return obs, nil
}

type VerifyRequest struct {
	// Actual: số gói thực nhận theo key, người dùng đã xem gợi ý AI và xác nhận.
	Actual map[string]float64 `json:"actual"`
}

// Verify ghi kết quả kiểm hàng và reconcile: chỉ lượng thực nhận được cộng vào tồn kho;
// Kế hoạch đủ thì xong, thiếu thì giữ mở. Trả số tiền nên refund cho phần thiếu.
func (s *Service) Verify(ctx context.Context, p authz.Principal, id string, req VerifyRequest) (Order, error) {
	o, err := s.owned(ctx, p, id)
	if err != nil {
		return Order{}, err
	}
	if o.State != StateDelivered {
		return Order{}, invalidState(o.State)
	}
	var shortfall int64
	for i := range o.Lines {
		l := &o.Lines[i]
		got, ok := req.Actual[l.Key]
		if !ok {
			return Order{}, apperr.New(apperr.CodeValidation, "Thiếu số lượng nhận của một món.").With("key", l.Key)
		}
		if !(got >= 0 && got <= l.Qty*2) {
			return Order{}, apperr.New(apperr.CodeValidation, "Số lượng nhận không hợp lệ.").With("key", l.Key)
		}
		v := got
		l.VerifiedQty = &v
		switch {
		case got >= l.Qty:
			l.Match = "match"
		case got == 0:
			l.Match = "missing"
		default:
			l.Match = "partial"
		}
		if got < l.Qty {
			shortfall += int64(math.Round((l.Qty - got) * float64(l.UnitPriceVND)))
		}
	}
	now := s.now()
	o.State, o.UpdatedAt = StateVerified, now
	o.SuggestedRefundVND = max(0, shortfall-o.RefundedVND)
	o.Events = append(o.Events, Event{State: StateVerified, At: now})
	if err := s.store.ApplyDelivery(ctx, p.UID, o, domain.Location(p.Timezone)); err != nil {
		return Order{}, err
	}
	return o, nil
}

// ---------- Refund ----------

type RefundRequest struct {
	Reason    string `json:"reason"`     // missing | wrong | damaged | other
	AmountVND int64  `json:"amount_vnd"` // 0: hoàn đúng số Mimi đề xuất cho phần thiếu
	Note      string `json:"note"`
}

var validRefundReason = map[string]bool{"missing": true, "wrong": true, "damaged": true, "other": true}

// Refund hoàn tiền qua PayPal cho phần thiếu / sai / hỏng, rồi điều chỉnh net spending.
// Tổng refund không vượt số đã thanh toán.
func (s *Service) Refund(ctx context.Context, p authz.Principal, id string, req RefundRequest) (Order, error) {
	o, err := s.owned(ctx, p, id)
	if err != nil {
		return Order{}, err
	}
	txn := o.ProviderTxnID
	if txn == "" {
		txn = o.CaptureID // đơn PayPal lưu trước khi có provider_txn_id
	}
	if o.State != StateVerified || txn == "" {
		return Order{}, invalidState(o.State)
	}
	if !validRefundReason[req.Reason] {
		return Order{}, apperr.New(apperr.CodeValidation, "Chọn lý do: thiếu, sai, hỏng hoặc khác.").With("field", "reason")
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > 200 || strings.IndexFunc(note, unicode.IsControl) >= 0 {
		return Order{}, apperr.New(apperr.CodeValidation, "Ghi chú tối đa 200 ký tự.").With("field", "note")
	}
	amount := req.AmountVND
	if amount == 0 {
		amount = o.SuggestedRefundVND
	}
	if amount <= 0 || amount > o.TotalVND-o.RefundedVND {
		return Order{}, apperr.New(apperr.CodeValidation, "Số tiền hoàn không hợp lệ.").With("max_vnd", o.TotalVND-o.RefundedVND)
	}
	ref := fmt.Sprintf("%s-r%d", o.ID, len(o.Refunds)+1)
	usd := ToUSD(amount, o.VNDPerUSD)
	var refID, refStatus string
	if ProviderOf(o) == ProviderBraintree {
		if s.cards == nil {
			return Order{}, apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình thanh toán thẻ.")
		}
		br, err := s.cards.Refund(ctx, txn, usd, ref, amount == o.TotalVND-o.RefundedVND)
		if err != nil {
			return Order{}, cardErr(err)
		}
		refID, refStatus = br.ID, br.Status
	} else {
		if s.payments == nil {
			return Order{}, apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình PayPal.")
		}
		pr, err := s.payments.RefundCapture(ctx, txn, usd, ref, note)
		if err != nil {
			return Order{}, paymentErr(err)
		}
		refID, refStatus = pr.ID, pr.Status
	}
	now := s.now()
	r := Refund{ID: refID, Ref: ref, AmountVND: amount, AmountUSD: usd, Reason: req.Reason, Status: refStatus, At: now}
	o.Refunds = append(o.Refunds, r)
	o.RefundedVND += amount
	o.NetVND = o.TotalVND - o.RefundedVND
	o.SuggestedRefundVND = max(0, o.SuggestedRefundVND-amount)
	o.UpdatedAt = now
	if err := s.store.RecordRefund(ctx, p.UID, o, r, domain.Location(p.Timezone)); err != nil {
		return Order{}, err
	}
	return o, nil
}

// ---------- Đọc ----------

func (s *Service) Get(ctx context.Context, p authz.Principal, id string) (Order, error) {
	return s.owned(ctx, p, id)
}

func (s *Service) List(ctx context.Context, p authz.Principal) ([]Order, error) {
	if err := authz.Can(p, authz.ActionBuyOnline); err != nil {
		return nil, err
	}
	out, err := s.store.ListOrders(ctx, p.UID, 50)
	if out == nil {
		out = []Order{}
	}
	return out, err
}

func (s *Service) owned(ctx context.Context, p authz.Principal, id string) (Order, error) {
	if err := authz.Can(p, authz.ActionBuyOnline); err != nil {
		return Order{}, err
	}
	if !ValidID(id) {
		return Order{}, apperr.ErrNotFound
	}
	return s.store.GetOrder(ctx, p.UID, id)
}

// ValidID: "o-" + 16 ký tự hex do server sinh.
func ValidID(id string) bool {
	if len(id) != 18 || !strings.HasPrefix(id, "o-") {
		return false
	}
	_, err := hex.DecodeString(id[2:])
	return err == nil
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "o-" + hex.EncodeToString(b)
}

func invalidState(st State) error {
	return apperr.New(apperr.CodeInvalidState, "Đơn hàng không ở bước cho phép thao tác này.").With("state", string(st))
}

func paymentErr(err error) error {
	if errors.Is(err, paypal.ErrDeclined) {
		return apperr.Wrap(apperr.CodePaymentFailed, "PayPal từ chối giao dịch. Kiểm tra lại rồi thử lại.", err)
	}
	return apperr.Wrap(apperr.CodePaymentUnavailable, "PayPal đang bận, thử lại sau ít phút.", err)
}

func cardErr(err error) error {
	switch {
	case errors.Is(err, braintree.ErrNotSettled):
		return apperr.Wrap(apperr.CodePaymentFailed, "Giao dịch thẻ chưa quyết toán nên chưa hoàn một phần được. Thử lại sau vài giờ, hoặc hoàn toàn bộ.", err)
	case errors.Is(err, braintree.ErrDeclined):
		return apperr.Wrap(apperr.CodePaymentFailed, "Thẻ bị từ chối. Kiểm tra lại rồi thử lại.", err)
	}
	return apperr.Wrap(apperr.CodePaymentUnavailable, "Cổng thẻ đang bận, thử lại sau ít phút.", err)
}

// cardStatusOf: kind webhook Braintree → CardStatus; "" là kind Mimi không theo dõi.
func cardStatusOf(kind string) string {
	switch kind {
	case braintree.KindTransactionSettled:
		return "settled"
	case braintree.KindTransactionSettlementDecl:
		return "settlement_declined"
	case braintree.KindDisputeOpened:
		return "disputed"
	case braintree.KindDisputeWon:
		return "dispute_won"
	case braintree.KindDisputeLost, braintree.KindDisputeAccepted, braintree.KindDisputeAutoAccepted, braintree.KindDisputeExpired:
		return "dispute_lost"
	}
	return ""
}

// CardWebhook áp thông báo Braintree (đã kiểm chữ ký) vào đơn thẻ: chỉ ghi CardStatus, không
// đổi bước giao hàng hay Spending. Thông báo không khớp đơn nào, kind không theo dõi, hoặc cũ
// hơn tin đã ghi thì bỏ qua (trả nil để Braintree không gửi lại).
func (s *Service) CardWebhook(ctx context.Context, signature, payload string) error {
	if s.cards == nil {
		return apperr.New(apperr.CodePaymentNotEnabled, "Chưa cấu hình thanh toán thẻ.")
	}
	n, err := s.cards.ParseWebhook(signature, payload)
	if err != nil {
		return apperr.Wrap(apperr.CodeBadRequest, "Webhook không hợp lệ.", err)
	}
	status := cardStatusOf(n.Kind)
	if status == "" || n.TxnID == "" {
		return nil
	}
	uid, oid, err := s.store.CardOrder(ctx, n.TxnID)
	if errors.Is(err, apperr.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	o, err := s.store.GetOrder(ctx, uid, oid)
	if errors.Is(err, apperr.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if ProviderOf(o) != ProviderBraintree || o.ProviderTxnID != n.TxnID {
		return nil
	}
	at := n.Timestamp
	if at.IsZero() {
		at = s.now()
	}
	if o.CardStatusAt != nil && at.Before(*o.CardStatusAt) {
		return nil
	}
	o.CardStatus, o.CardStatusAt, o.UpdatedAt = status, &at, s.now()
	return s.store.UpdateOrder(ctx, uid, o, o.State)
}
