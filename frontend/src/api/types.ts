/**
 * Kiểu dữ liệu tạm viết tay theo spec. Khi có api/openapi.yaml, chạy `npm run gen:api`
 * để sinh src/api/schema.d.ts và thay file này bằng các kiểu sinh ra.
 * Mọi con số tiền và số ngày đều do backend tính; client chỉ hiển thị.
 */
/** Nước lấy giá online. Cài đặt cho chọn VN, US (mẹ chốt 03/11); SG vẫn giữ cho người đã chọn. */
export type Country = "VN" | "US" | "SG";

export type Role = "owner" | "member" | "solo";

export type Me = {
  uid: string;
  display_name: string | null;
  role: Role;
  receipt_count: number;
  settings: {
    lead_days: number;
    lang?: "vi" | "en";      // ngôn ngữ của tài khoản; vắng khi người mới chưa ghi
    timezone: string;
    country?: Country;     // nước lấy giá online; vắng thì máy chủ đoán theo múi giờ
    currency?: "VND" | "USD";  // tiền tệ hiển thị của người này; vắng là VND
    remind_enabled?: boolean;
    digest_time?: string;      // HH:MM
    relations?: Record<string, string>; // tên gọi người nhà theo uid, lưu trên máy chủ
    monthly_budget_vnd?: number; // Mimi 2.2: ngân sách tháng (đồng); vắng là chưa đặt
  };
};

export type HomeSummary = {
  spent_this_month: number | null;   // null khi tháng chưa có hóa đơn
  needs_count: number;               // đếm theo đúng quy tắc Cần mua
  plan: { item_count: number; estimate: number | null; missing_price: number };
  unread_notifications: number;
  receipt_count: number;             // số hóa đơn đã lưu, mọi thời gian
};

/** Nhãn trạng thái dùng chung mọi màn (spec 6.2), theo thứ tự ưu tiên. */
export type StockStatus = "out" | "buy_now" | "soon" | "upcoming" | "plenty" | "waiting";

export type Item = {
  key: string;                 // consumption_group_key
  name: string;
  category_key: string;
  /** Không có trong API: client gắn từ category_key theo ngôn ngữ giao diện (src/api/hooks.ts). */
  category_label: string;
  status: StockStatus;
  days_left: number | null;    // null: chưa đủ dữ liệu
  interval_days?: number;      // chu kỳ mua trung bình (ngày), khi đã đủ lịch sử
  purchase_count: number;
  // API bỏ qua hai trường dưới khi chưa có giá trị, nên có thể là undefined.
  confidence?: "low" | "mid" | "high" | "" | null;
  source: "predicted" | "manual";
  predicted_out_on?: string | null; // YYYY-MM-DD
  stock?: Stock;               // tồn theo số lượng; vắng khi chưa có hóa đơn nào ghi số lượng
};

/** Mimi 2.0 mục 4: Confirmed Inventory (có bằng chứng) tách khỏi Estimated Inventory (Mimi ước tính). */
export type Stock = {
  estimated: number;
  unit: CanonicalUnit;
  confirmed: number;
  confirmed_on: string;        // YYYY-MM-DD
  confirmed_source: "receipt" | "delivery" | "manual";
  rate_per_day?: number;
  days_of_stock?: number;
  /** Chiều đo khác của cùng nhóm (800 g và 900 ml): hiện riêng, không cộng (0.1 mục 20). */
  parts?: { estimated: number; unit: CanonicalUnit; confirmed: number; confirmed_on: string; days_of_stock?: number }[];
};

/** Đơn vị chuẩn (0.1). "pc" là tên cũ của item. */
export type CanonicalUnit = "g" | "ml" | "m" | "item" | "pc";

/** Purchase lot (0.1 mục 11): phần còn lại chia FIFO từ tồn ước tính. */
export type Lot = {
  lot_id: string;
  detail_name?: string;
  brand?: string;
  store?: string;
  purchased_at: string;
  initial_quantity: number;
  canonical_unit: CanonicalUnit;
  remaining_quantity: number;
  inventory_source: "receipt" | "delivery";
};

/** GET /items/{key}: mọi trường của Item kèm tối đa 10 lần mua gần nhất. */
export type ItemDetail = Item & {
  history: { on: string; store?: string; amount_vnd: number | null; receipt_id?: string }[];
  lots?: Lot[];
};

export type ItemsQuery = {
  tab: "soon" | "active";
  horizon: number;
};

/** Còn nhiều = 30 ngày, Sắp hết = 3 ngày, Đã hết = 0 (spec mục 4). */
export type QuickStock = "plenty" | "soon" | "out";

/** Một dòng trong bản nháp do POST /receipts/read trả về (api/openapi.yaml, DraftLine). */
export type ReceiptDraftLine = {
  line_id: string;
  name: string;
  qty: number;
  unit?: string;                // đơn vị đã chuẩn hóa: kg, g, l, ml, hop, goi...
  amount: string;               // nguyên văn in trên hóa đơn, ví dụ "64000" hoặc "12.50"
  amount_vnd: number | null;    // null khi chưa quy đổi được sang VND; âm với dòng khuyến mãi
  discount?: boolean;           // dòng khuyến mãi: trừ vào tổng, không phải món
  category_key?: string;        // vắng: hiện nút "Chọn nhóm"
  group_key?: string;
  habit: boolean;
  // Chuẩn hóa sản phẩm (0.1, 1.1, 7.1): tên gốc, brand, quy cách, lượng canonical.
  raw_name?: string;
  brand?: string;
  product_name?: string;
  detail_name?: string;
  product_key?: string;
  source_language?: string;
  /** Tên theo bill khi name là tên đã dịch sang ngôn ngữ app; hiện chữ nhỏ dưới tên. */
  original_name?: string;
  package_count?: number;
  package_size?: number;
  package_unit?: string;
  canonical_quantity?: number;
  canonical_unit?: CanonicalUnit;
  /** Lưu được nhưng chưa dùng cho tồn kho / dự đoán (6.1 mục 5.3). */
  needs_review?: boolean;
  review_reasons?: ("missing_category" | "missing_group" | "unknown_quantity" | "count_for_measured_group")[];
};

/** Bản nháp hóa đơn (Draft). Mọi con số do backend tính; client chỉ hiển thị. */
export type ReceiptDraft = {
  receipt_id: string;
  store?: string;
  purchased_on?: string;        // YYYY-MM-DD
  currency: string;
  total_printed?: string;       // nguyên văn, vắng nếu hóa đơn không in tổng
  discount_total?: string;
  lines: ReceiptDraftLine[];
  sum_vnd: number;
  total_mismatch: boolean;
  fx_missing: boolean;
  pages: number;
  unreadable_pages: number;
};

/** Thân POST /receipts (SaveReceipt). Backend từ chối mọi trường không có ở đây. */
export type SaveReceiptRequest = {
  receipt_id: string;
  store: string;
  purchased_on: string;
  currency: string;
  discount_total: string;
  lines: {
    line_id: string;
    name: string;
    qty: number;
    unit?: string;
    amount: string;
    category_key: string;
    group_key: string;
    habit: boolean;
    raw_name?: string;
    brand?: string;
    product_name?: string;
    source_language?: string;
    original_name?: string;
    package_count?: number;
    package_size?: number;
    /** B3: kế hoạch mà dòng này trừ vào, theo thứ tự đã chia. Vắng: theo source_plan_id. [] = không trừ. */
    plans?: { plan_id: string; qty: number }[];
    package_unit?: string;
  }[];
  use_for_prediction: boolean;
  confirm_duplicate: boolean;
  /** Quét từ trong một Plan: chỉ đối chiếu Plan đó (3.1). */
  source_plan_id?: string;
  /** Buyer quét hóa đơn cho một vòng mua hộ (4.1). */
  assignment_id?: string;
};

export type SaveReceiptResult = {
  receipt_id: string;
  status: "saved";
  replayed: boolean;
  plan_done?: string[];
  assignment?: { assignment_id: string; status: "completed" | "closed_partial"; done: string[]; returned: string[] };
  recurring_matched?: string[];
  /** B6: món chưa mua của kế hoạch vừa chốt đã sang kế hoạch mới "· còn lại". */
  split_plan?: { from_plan_id: string; plan_id: string; name: string; moved: number };
};

/** Kế hoạch mua sắm (GET /plan). Giá là giá mỗi đơn vị người dùng từng trả, lấy từ hóa đơn đã lưu. */
export type CartItem = {
  key: string;
  name: string;
  category_key: string;
  status: StockStatus;
  days_left: number | null;
  unit_price: number | null;   // null: chưa có giá
  best_store?: string;
  qty: number;
  unit?: string;
  need_by?: string;            // YYYY-MM-DD
  priority: PlanPriority;
  source: PlanSource;
  reason?: string;
  bought_qty?: number;         // > 0: đã mua một phần
  line_status?: "open" | "assigned"; // assigned: đang nhờ người nhà mua (4.1)
  assigned_to_uid?: string;
};

export type PlanPriority = "high" | "normal" | "low";
export type PlanSource = "forecast" | "check" | "meal" | "manual";

/** PATCH /plan/items/{key}: chỉ gửi trường thay đổi. */
export type PlanLinePatch = Partial<{ qty: number; unit: string; need_by: string; priority: PlanPriority; reason: string }>;

/** rev: máy chủ tăng mỗi lần lưu. base_rev: rev máy này đã đọc, lệch thì máy chủ trả 409 (A1). */
export type PlanState = { buy_date: string; buy_hour: number | null; buy_minute?: number | null; trip_note: string; reconciled_on?: string; rev?: number; base_rev?: number };
export type PlanPhase = "planning" | "shopping" | "reconciled";

export type Cart = {
  plan_id?: string;
  items: CartItem[];
  item_count: number;
  estimate: number | null;     // theo phương án khuyên dùng; null khi chưa món nào có giá
  missing_price: number;
  state: PlanState;
  phase?: PlanPhase;            // 3.1 mục 18
};

export type PlanStop = { store: string; groups: string[]; total: number };

/** GET /plan/prices: Nơi mua tối ưu. groups là khóa món (key), tên lấy từ Cart. */
export type PlanPrices = {
  single_store: PlanStop | null;
  split: PlanStop[];
  split_total: number;
  saving: number;
  recommend: "single" | "split";
  missing_price: string[];
  single_source: string[];
  sale_offers?: unknown[];     // chưa tìm giá sale bên ngoài: luôn rỗng
};

/** GET /spending?month=YYYY-MM (chỉ chủ giỏ). */
/** Spec 1.5 mục 4 (việc 2): Lịch sử mua hàng mọi tháng và Chi tiết hóa đơn (chỉ xem, không ảnh, đã lọc thông tin cá nhân). */
export type HistoryReceipt = {
  receipt_id: string; store: string; purchased_at: string; total_vnd: number; /** Tên các món, để tìm theo món. */ item_names: string[];
  /** "delegated" = khoản người nhà mua hộ, chỉ phần món của mình, không có hóa đơn gốc. */
  kind?: "own" | "delegated" | "recurring"; buyer_uid?: string; buyer_name?: string; plan_name?: string;
  /** Số món chưa rõ giá, không tính vào total_vnd. */
  missing_price?: number;
  /** kind "recurring": khoản định kỳ đã trả, bấm thì mở chi tiết khoản (góp ý 01/11). */
  recurring_id?: string;
};
/** Một dòng Lịch sử đơn hàng (2.2, chỉ ở giao diện): hóa đơn, hoặc đơn online (kind "online") gộp vào, bấm thì mở chi tiết đơn. */
export type HistoryRow = Omit<HistoryReceipt, "kind"> & { kind?: HistoryReceipt["kind"] | "online"; order_id?: string; order_state?: string };
/** unit_price, amount null: món chưa rõ giá, không tính vào tổng. */
export type ReceiptDetailLine = { name: string; /** Tên theo bill khi name là tên đã dịch. */ original_name?: string; spec?: string; qty: number; unit_price: number | null; amount: number | null; category_key?: string; discount?: number; tag?: "for_owner" | "mine";
  /** B7: estimate = không nhập giá, lấy giá mua gần nhất; missing = chưa có giá. "online" để dành cho bước 13. */
  price_source?: "receipt" | "manual" | "estimate" | "missing" | "online" };
export type ReceiptDetail = HistoryReceipt & { plans: { id: string; name: string }[]; lines: ReceiptDetailLine[]; subtotal: number; discount: number; split?: { for_owner: number; mine: number; owner_name: string } };
export type ReceiptSummary = { receipt_id: string; store?: string; purchased_on: string; total_vnd: number; discount_vnd: number; delegated?: boolean };
export type SpendingReport = {
  month: string;
  spent: number;
  spent_prev_month_pct: number | null;
  /** Mimi 2.2: chỉ tháng hiện tại, so với ngày 1 đến same_range_day của tháng trước. */
  spent_prev_same_range_pct?: number | null;
  same_range_day?: number;
  saved: number;
  saved_ratio: number | null;
  receipt_count: number;
  online_spent?: number;       // chi qua PayPal (net sau refund), đã nằm trong spent
  online_orders?: number;
  refunded?: number;
  delegated_spent?: number;    // người nhà mua hộ cho mình, đã nằm trong spent
  by_category: { key: string; amount: number; pct: number }[];
  top_saving_receipts: ReceiptSummary[];
  recent_receipts: ReceiptSummary[];
};

/** GET /notifications: suy ra lúc đọc từ món sắp hết và chuyến mua. */
export type Notification =
  | { id: string; kind: "item"; item_key: string; name: string; status: StockStatus; days_left?: number | null }
  | { id: string; kind: "trip"; date: string; hour?: number | null; item_count: number }
  /** B9: sự kiện giữa người trong nhà, đã lưu, có trạng thái đã đọc. */
  | { id: string; kind: "event"; event: HouseEvent };

export type HouseEvent = {
  id: string;
  kind: "plan_sent" | "plan_updated" | "plan_cancelled" | "run_done" | "run_declined" | "plan_split";
  at: string;
  actor_name?: string;
  plan_id?: string;
  plan_name?: string;
  assignment_id?: string;
  items?: number;
  returned?: number;
  read: boolean;
};

/** Cài đặt đọc từ GET /me (settings) và gửi qua PUT /settings (chỉ trường thay đổi). */
export type SettingsPatch = Partial<{
  display_name: string;
  lead_days: number;
  remind_enabled: boolean;
  digest_time: string;
  timezone: string;
  lang: "vi" | "en";
  country: Country;
  currency: "VND" | "USD";
  /** Chỉ gửi phần đổi; "" là xóa tên gọi của uid đó. */
  relations: Record<string, string>;
  /** Mimi 2.2: ngân sách tháng (đồng); 0 là bỏ ngân sách. */
  monthly_budget_vnd: number;
}>;

export type ApiErrorBody = { code: string; message: string; fields?: Record<string, unknown>; retryable?: boolean };

// ================= Mimi Online (Mimi 2.0 mục 12 đến 19) =================

export type MandateCategory = "grocery" | "household";

/** Household Mandate: phạm vi Mimi được tự mua online. */
export type Mandate = {
  enabled: boolean;
  categories: MandateCategory[];
  price_tolerance_pct: number;
  max_order_vnd: number;
};

export type MandateResult = "ALLOW" | "ASK" | "BLOCK";
export type MandateReason = { key?: string; code: string; result: MandateResult; detail: string };

/** Trạng thái tiền thẻ Braintree báo qua webhook sau khi thu (chỉ đơn braintree). */
export type CardStatus = "settled" | "settlement_declined" | "disputed" | "dispute_won" | "dispute_lost";

export type OrderLine = {
  key: string;
  name: string;
  product_name: string;
  category_key?: string;
  category: MandateCategory;
  qty: number;                 // số gói
  unit: string;
  pack_qty: number;
  plan_qty: number;
  plan_unit?: string;
  unit_price_vnd: number;
  snapshot_price_vnd: number;
  total_vnd: number;
  need_by?: string;
  eta: string;
  verified_qty?: number;
  match?: "match" | "partial" | "missing";
};

export type Basket = {
  merchant: string;
  lines: OrderLine[];
  missing: string[];
  total_vnd: number;
  amount_usd: string;
  vnd_per_usd: number;
  rate_source?: string;
  rate_as_of?: string;
  snapshot_at: string;
  decision: { result: MandateResult; reasons: MandateReason[] };
  /** Server đã cấu hình Braintree: hiện thêm nút "Trả bằng thẻ". */
  card_enabled?: boolean;
};

export type OrderState = "awaiting_payment" | "paid" | "ordered" | "shipped" | "delivered" | "verified";

export type OrderRefund = {
  id: string; ref: string; amount_vnd: number; amount_usd: string;
  reason: RefundReason; status: string; at: string;
};
export type RefundReason = "missing" | "wrong" | "damaged" | "other";

export type Order = {
  id: string;
  plan_id?: string;
  provider?: "paypal" | "braintree";
  provider_order_id?: string;
  provider_txn_id?: string;
  card_status?: CardStatus;
  card_status_at?: string;
  state: OrderState;
  merchant: string;
  lines: OrderLine[];
  total_vnd: number;
  amount_usd: string;
  vnd_per_usd: number;
  rate_source?: string;
  rate_as_of?: string;
  decision: { result: MandateResult; reasons: MandateReason[] };
  approved_by_user: boolean;
  paypal_order_id: string;
  approve_url?: string;
  paypal_capture_id?: string;
  paid_at?: string;
  eta: string;
  events: { state: OrderState; at: string }[];
  refunds: OrderRefund[];
  refunded_vnd: number;
  net_vnd: number;
  suggested_refund_vnd: number;
  created_at: string;
};

export type DeliveryObservation = { key: string; qty: number; confidence: "low" | "mid" | "high" };

// ================= Bộ spec Mimi 1.0 =================

/** 0.2: nhóm sản phẩm dùng chung. */
export type TaxonomyGroup = {
  key: string; parent: string; label: string; labels: { vi: string; en: string };
  measurement: string; unit: string;
};

/** 3.1: một Kế hoạch trong danh sách. */
export type PlanMeta = {
  id: string; name: string; status: "active" | "archived"; default: boolean; item_count: number; buy_date?: string;
  icon?: string; // khóa trong PLAN_ICONS, máy chủ lưu
};

/** 2.1: khoản định kỳ và kỳ thanh toán. */
export type RecurringCycle = "weekly" | "monthly" | "quarterly" | "yearly";
export type Occurrence = {
  id: string; recurring_id: string; name: string; due_at: string; amount: number;
  status: "pending" | "paid" | "matched" | "skipped"; paid_at?: string; source_type?: "manual" | "receipt"; source_id?: string;
};
export type Recurring = {
  id: string; name: string; amount: number; currency: "VND"; cycle: RecurringCycle; next_due_at: string;
  active: boolean; category_key?: string;
  display: "overdue" | "due_today" | "due_soon" | "upcoming" | "paused";
  current?: Occurrence; history: Occurrence[];
};
export type RecurringInput = Partial<{ name: string; amount: number; cycle: RecurringCycle; next_due_at: string; category_key: string; active: boolean }>;
export type Forecast = {
  from: string; to: string; total: number; recurring_total: number; shopping_total: number;
  recurring: Occurrence[]; shopping: { key: string; name: string; out_on: string; amount: number | null }[]; missing_prices: number;
};

/** 4.1: nhà và vòng mua hộ. */
export type HouseholdMember = { uid: string; display_name: string; role: "owner" | "member"; joined_at: string };
export type Household = {
  id: string; name: string; owner_uid: string; my_role: "owner" | "member";
  members: HouseholdMember[]; invite?: { code: string };
};
export type AssignmentItem = { consumption_group_key: string; name_display: string; qty?: number; unit?: string; status: "assigned" | "done" | "returned"; unit_price?: number; category_key?: string };
export type Assignment = {
  assignment_id: string; owner_uid: string; owner_display_name: string; buyer_uid: string; buyer_display_name: string;
  plan_id: string; status: "assigned" | "shopping" | "completed" | "closed_partial" | "cancelled" | "declined"; note?: string;
  items: AssignmentItem[]; created_at: string; updated_at?: string;
  /** Tên, thời gian, ghi chú của Plan gốc, luôn là bản mới nhất (3.1 mục 3B.4). */
  trip?: { plan_name: string; buy_date?: string; buy_hour?: number; buy_minute?: number; trip_note?: string };
};
export type Delegated = { to_me: Assignment[]; from_me: Assignment[]; /** 2.2: vòng được nhờ đã mua xong. */ to_me_done?: Assignment[] };

/** 8.1: lịch sử giá theo đơn giá chuẩn hóa. */
export type PriceObservation = {
  store_name: string; paid_price: number; purchased_at: string; canonical_quantity: number; canonical_unit: CanonicalUnit; unit_price: number;
};
export type ItemPriceHistory = {
  group_key: string; purchase_count: number; canonical_unit?: CanonicalUnit;
  last_purchase: PriceObservation | null; lowest_historical: PriceObservation | null; average_unit_price: number | null;
  other_dimension_count: number; history: PriceObservation[]; window_days: number;
};
/** ADR-004: giá đang bán từ bảng giá chung (chỉ nguồn công khai). */
/** local, currency: số tiền gốc nơi bán ghi, chỉ có ngoài Việt Nam (ví dụ 4.5 USD). */
export type MarketPrice = { store: string; price_vnd: number; local?: number; currency?: string; size?: string; source: string; url?: string; on: string };
/** Nút "Cập nhật giá": còn mấy lần hôm nay, lần bấm gần nhất. */
export type MarketRefreshStatus = { daily: number; left: number; last_at?: string };
/** Mimi 2.2: giá chung của một món trong Đồ dùng. updated_at null: Mimi chưa tra món này lần nào. */
export type ItemMarketPrices = { key: string; name: string; prices: MarketPrice[]; updated_at: string | null };
export type MarketPrices = { items: { key: string; name: string; prices: MarketPrice[]; updated_at: string }[]; refresh?: MarketRefreshStatus };
/** Nút "Cập nhật giá": bảng giá mới kèm số món vừa tra. */
export type MarketRefresh = MarketPrices & { looked: number; found: number; fresh: number };

export type BasketPrices = {
  basket_item_count: number;
  store_options: { store_name: string; store_key: string; covered_items: number; total_items: number; estimated_total: number | null; covered_subtotal: number; missing_keys: string[] }[];
  split_store_option: { estimated_total: number; stores: { store_name: string; keys: string[]; total: number }[]; covered_items: number } | null;
  recommend: "single" | "split" | "none";
  no_history: string[];
  window_days: number;
};
