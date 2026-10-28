/**
 * Kiểu dữ liệu tạm viết tay theo spec. Khi có api/openapi.yaml, chạy `npm run gen:api`
 * để sinh src/api/schema.d.ts và thay file này bằng các kiểu sinh ra.
 * Mọi con số tiền và số ngày đều do backend tính; client chỉ hiển thị.
 */
export type Role = "owner" | "member" | "solo";

export type Me = {
  uid: string;
  display_name: string | null;
  role: Role;
  receipt_count: number;
  settings: {
    lead_days: number;
    lang: "vi" | "en";
    timezone: string;
    remind_enabled?: boolean;
    digest_time?: string;      // HH:MM
  };
};

export type HomeSummary = {
  spent_this_month: number | null;   // null khi tháng chưa có hóa đơn
  needs_count: number;               // đếm theo đúng quy tắc Cần mua
  plan: { item_count: number; estimate: number | null; missing_price: number };
  unread_notifications: number;
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
  unit: "g" | "ml" | "pc";
  confirmed: number;
  confirmed_on: string;        // YYYY-MM-DD
  confirmed_source: "receipt" | "delivery" | "manual";
  rate_per_day?: number;
  days_of_stock?: number;
};

/** GET /items/{key}: mọi trường của Item kèm tối đa 10 lần mua gần nhất. */
export type ItemDetail = Item & {
  history: { on: string; store?: string; amount_vnd: number | null; receipt_id?: string }[];
};

export type ItemsQuery = {
  tab: "soon" | "active";
  horizon: 3 | 7 | 14;
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
  }[];
  use_for_prediction: boolean;
  confirm_duplicate: boolean;
};

export type SaveReceiptResult = { receipt_id: string; status: "saved"; replayed: boolean };

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
};

export type PlanPriority = "high" | "normal" | "low";
export type PlanSource = "forecast" | "check" | "meal" | "manual";

/** PATCH /plan/items/{key}: chỉ gửi trường thay đổi. */
export type PlanLinePatch = Partial<{ qty: number; unit: string; need_by: string; priority: PlanPriority; reason: string }>;

export type PlanState = { buy_date: string; buy_hour: number | null; trip_note: string };

export type Cart = {
  items: CartItem[];
  item_count: number;
  estimate: number | null;     // theo phương án khuyên dùng; null khi chưa món nào có giá
  missing_price: number;
  state: PlanState;
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
};

/** GET /spending?month=YYYY-MM (chỉ chủ giỏ). */
export type ReceiptSummary = { receipt_id: string; store?: string; purchased_on: string; total_vnd: number; discount_vnd: number };
export type SpendingReport = {
  month: string;
  spent: number;
  spent_prev_month_pct: number | null;
  saved: number;
  saved_ratio: number | null;
  receipt_count: number;
  online_spent?: number;       // chi qua PayPal (net sau refund), đã nằm trong spent
  online_orders?: number;
  refunded?: number;
  by_category: { key: string; amount: number; pct: number }[];
  top_saving_receipts: ReceiptSummary[];
  recent_receipts: ReceiptSummary[];
};

/** GET /notifications: suy ra lúc đọc từ món sắp hết và chuyến mua. */
export type Notification =
  | { id: string; kind: "item"; item_key: string; name: string; status: StockStatus; days_left?: number | null }
  | { id: string; kind: "trip"; date: string; hour?: number | null; item_count: number };

/** Cài đặt đọc từ GET /me (settings) và gửi qua PUT /settings (chỉ trường thay đổi). */
export type SettingsPatch = Partial<{
  display_name: string;
  lead_days: number;
  remind_enabled: boolean;
  digest_time: string;
  timezone: string;
  lang: "vi" | "en";
}>;

export type ApiErrorBody = { code: string; message: string };

// ================= Mimi Online (Mimi 2.0 mục 12 đến 19) =================

export type MandateCategory = "grocery" | "household";

/** Household Mandate: phạm vi Mimi được tự mua online. */
export type Mandate = {
  enabled: boolean;
  categories: MandateCategory[];
  price_tolerance_pct: number;
  max_order_vnd: number;
  max_day_vnd: number;
};

export type MandateResult = "ALLOW" | "ASK" | "BLOCK";
export type MandateReason = { key?: string; code: string; result: MandateResult; detail: string };

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
  snapshot_at: string;
  decision: { result: MandateResult; reasons: MandateReason[] };
};

export type OrderState = "awaiting_payment" | "paid" | "ordered" | "shipped" | "delivered" | "verified";

export type OrderRefund = {
  id: string; ref: string; amount_vnd: number; amount_usd: string;
  reason: RefundReason; status: string; at: string;
};
export type RefundReason = "missing" | "wrong" | "damaged" | "other";

export type Order = {
  id: string;
  state: OrderState;
  merchant: string;
  lines: OrderLine[];
  total_vnd: number;
  amount_usd: string;
  vnd_per_usd: number;
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
