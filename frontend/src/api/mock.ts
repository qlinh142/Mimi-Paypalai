import { uiLang } from "../i18n";
import taxonomyGroups from "./taxonomy-groups.json";
/**
 * Backend giả cho chế độ VITE_API_MODE=mock. Dữ liệu mẫu, không phải dữ liệu thật.
 * Quy tắc trạng thái ở đây chỉ mô phỏng; bản thật do API Go tính (internal/domain).
 */
import type {
  Basket, CanonicalUnit, Cart, CartItem, HomeSummary, Item, Mandate, MandateReason, Me, Notification, Order, OrderLine, OrderState,
  PlanLinePatch, PlanPrices, PlanState, PlanStop, ReceiptDetail, ReceiptDetailLine, ReceiptDraft, RefundReason, SpendingReport, StockStatus,
} from "./types";

const delay = (ms = 250) => new Promise((r) => setTimeout(r, ms));

function statusFor(days: number | null, purchases: number, forcedOut = false): StockStatus {
  if (forcedOut) return "out";
  if (days === null || purchases < 3) return "waiting";
  if (days <= 0) return "buy_now";
  if (days <= 7) return "soon";
  if (days <= 14) return "upcoming";
  return "plenty";
}

type Seed = [key: string, name: [vi: string, en: string], cat: string, catLabel: [vi: string, en: string], days: number | null, purchases: number];
const L = uiLang === "vi" ? 0 : 1;
const seeds: Seed[] = [
  ["sua-tuoi", ["Sữa tươi không đường", "Unsweetened fresh milk"], "sua", ["Sữa và đồ uống", "Dairy & drinks"], 0, 6],
  ["nuoc-rua-chen", ["Nước rửa chén", "Dish soap"], "hoa-pham", ["Hóa phẩm gia đình", "Household cleaning"], 0, 4],
  ["trung-ga", ["Trứng gà", "Eggs"], "tuoi-song", ["Thịt, cá, trứng", "Meat, fish & eggs"], 2, 9],
  ["dau-goi", ["Dầu gội", "Shampoo"], "ve-sinh", ["Chăm sóc cá nhân", "Personal care"], 5, 3],
  ["gao", ["Gạo ST25", "ST25 rice"], "do-kho", ["Đồ khô", "Pantry"], 6, 5],
  ["giay-ve-sinh", ["Giấy vệ sinh", "Toilet paper"], "hoa-pham", ["Hóa phẩm gia đình", "Household cleaning"], 11, 4],
  ["bot-giat", ["Bột giặt", "Laundry detergent"], "hoa-pham", ["Hóa phẩm gia đình", "Household cleaning"], 13, 3],
  ["kem-danh-rang", ["Kem đánh răng", "Toothpaste"], "ve-sinh", ["Chăm sóc cá nhân", "Personal care"], 19, 4],
  ["nuoc-mam", ["Nước mắm", "Fish sauce"], "gia-vi", ["Gia vị", "Condiments"], 26, 3],
  ["ca-phe", ["Cà phê hạt", "Coffee beans"], "sua", ["Sữa và đồ uống", "Dairy & drinks"], null, 1],
];
// Mimi 1.5: bản chạy thử dùng ngày thật (trước đây cố định 16/10, lỗi V5).
const today = (() => { const d = new Date(); return new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate(), 12)); })();
const iso = (d: Date) => d.toISOString().slice(0, 10);
/** Tồn mẫu (đơn vị chuẩn) để thẻ Đồ dùng hiện "0 chai", "2 quả"... */
const SEED_STOCK: Record<string, [number, "pc" | "g" | "ml"]> = {
  "sua-tuoi": [1, "pc"], "nuoc-rua-chen": [0, "pc"], "trung-ga": [2, "pc"], "dau-goi": [1, "pc"], gao: [2000, "g"],
  "giay-ve-sinh": [4, "pc"], "bot-giat": [1500, "g"], "kem-danh-rang": [2, "pc"], "nuoc-mam": [1, "pc"],
};

let items: Item[] = seeds.map(([key, names, category_key, labels, days, purchase_count]) => ({
  key,
  name: names[L],
  category_key,
  category_label: labels[L],
  days_left: purchase_count < 2 ? null : days,
  purchase_count,
  status: statusFor(days, purchase_count, key === "nuoc-rua-chen"),
  confidence: purchase_count >= 4 ? "high" : purchase_count === 3 ? "mid" : purchase_count === 2 ? "low" : null,
  source: "predicted",
  predicted_out_on: days === null ? null : iso(new Date(today.getTime() + days * 86_400_000)),
  ...(SEED_STOCK[key]
    ? { stock: { estimated: SEED_STOCK[key][0], unit: SEED_STOCK[key][1], confirmed: SEED_STOCK[key][0], confirmed_on: iso(today), confirmed_source: "receipt" as const } }
    : {}),
}));

let receiptCount = 5;
let defaultPlanIcon = "bag";
let mockSettings: Me["settings"] & { display_name?: string } = {
  lead_days: 7, lang: uiLang, timezone: "Asia/Ho_Chi_Minh", remind_enabled: true, digest_time: "08:00",
};
/** Bản trước lần sửa tồn gần nhất của từng món, để Hoàn tác trả lại đúng trạng thái cũ. */
const beforeOverride = new Map<string, Item>();
const savedReceipts = new Set<string>();

function json(body: unknown, status = 200) {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function isSoon(i: Item, horizon: number) {
  return i.status === "out" || i.status === "buy_now" || (i.days_left !== null && i.status !== "waiting" && i.days_left <= horizon);
}

const order = (a: Item, b: Item) => (a.days_left ?? 999) - (b.days_left ?? 999);

// ================= Kế hoạch (bản thử) =================
// Giá mỗi đơn vị từng trả ở mỗi cửa hàng: dữ liệu mẫu, không phải giá thật.
const MOCK_PRICES: Record<string, Record<string, number>> = {
  "sua-tuoi": { "Co.opmart": 32_000, "Bách Hóa Xanh": 30_000 },
  "nuoc-rua-chen": { "Co.opmart": 38_000, "Bách Hóa Xanh": 41_000 },
  "trung-ga": { "Bách Hóa Xanh": 36_000, "Co.opmart": 38_000 },
  gao: { "Co.opmart": 120_000, "Bách Hóa Xanh": 140_000 },
  "dau-goi": { "Co.opmart": 95_000 },
  "giay-ve-sinh": { "Co.opmart": 78_000, "Bách Hóa Xanh": 72_000 },
  "bot-giat": { "Bách Hóa Xanh": 155_000, "Co.opmart": 189_000 },
};
let cart = new Set<string>(["sua-tuoi", "nuoc-rua-chen", "trung-ga"]);
/** Chi tiết dòng Kế hoạch (số lượng, ngày cần, ưu tiên); vắng: mặc định như backend. */
type MockLine = PlanLinePatch & { source?: CartItem["source"]; bought_qty?: number; line_status?: "open" | "assigned"; assigned_to_uid?: string };
let planLines = new Map<string, MockLine>();
// Mimi 1.5: Kế hoạch chính mẫu theo ảnh 5a (đi mua sau 2 ngày, 09:30, WinMart gần nhà).
let planState: PlanState = { buy_date: new Date(Date.now() + 2 * 86_400_000).toISOString().slice(0, 10), buy_hour: 9, buy_minute: 30, trip_note: uiLang === "en" ? "WinMart nearby" : "WinMart gần nhà" };
/** Món chưa có trong Đồ dùng: nhóm taxonomy hoặc món tự nhập (3.1). */
const extraNames = new Map<string, string>();

/** Nhiều Kế hoạch (3.1): mỗi Kế hoạch đặt tên có giỏ riêng; "default" dùng các biến ở trên. */
type MockPlan = { name: string; status: "active" | "archived"; icon?: string; created: number; cart: Set<string>; lines: Map<string, MockLine>; state: PlanState };
const namedPlans = new Map<string, MockPlan>([
  ["p-be", {
    name: L ? "Baby shopping" : "Mua đồ cho bé", status: "active", created: 1, cart: new Set(["sua-tuoi", "giay-ve-sinh", "nuoc-mam"]), lines: new Map([
      ["sua-tuoi", { line_status: "assigned", assigned_to_uid: "mock-minh" }],
      ["giay-ve-sinh", { line_status: "assigned", assigned_to_uid: "mock-minh" }],
      ["nuoc-mam", { line_status: "assigned", assigned_to_uid: "mock-minh" }],
    ]),
    state: { buy_date: new Date(Date.now() + 5 * 86_400_000).toISOString().slice(0, 10), buy_hour: 18, buy_minute: 0, trip_note: L ? "Bach Hoa Xanh" : "Bách Hóa Xanh" },
  }],
]);

/** Chạy fn trên Kế hoạch plan_id của request bằng cách đổi tạm các biến giỏ. */
function inPlan<T>(id: string | null, fn: () => T): T {
  const p = id && id !== "default" ? namedPlans.get(id) : undefined;
  if (!p) return fn();
  const saved = { cart, planLines, planState };
  cart = p.cart;
  planLines = p.lines;
  planState = p.state;
  try {
    return fn();
  } finally {
    p.state = planState;
    ({ cart, planLines, planState } = saved);
  }
}

/** Cùng quy tắc với domain.PlanShopping: tách nơi khi tiết kiệm từ 10.000đ. */
function mockPrices(): PlanPrices {
  const keys = [...cart];
  const priced = keys.filter((k) => MOCK_PRICES[k]);
  const missing_price = keys.filter((k) => !MOCK_PRICES[k]);
  const single_source = priced.filter((k) => Object.keys(MOCK_PRICES[k]).length === 1);
  const stops = new Map<string, PlanStop>();
  for (const k of priced) {
    const [store, price] = Object.entries(MOCK_PRICES[k]).sort((a, b) => a[1] - b[1] || a[0].localeCompare(b[0]))[0];
    const st = stops.get(store) ?? { store, groups: [], total: 0 };
    st.groups.push(k);
    st.total += price;
    stops.set(store, st);
  }
  const split = [...stops.values()].sort((a, b) => b.groups.length - a.groups.length || a.store.localeCompare(b.store));
  const split_total = split.reduce((a, s) => a + s.total, 0);
  let single_store: PlanStop | null = null;
  const stores = new Set(priced.flatMap((k) => Object.keys(MOCK_PRICES[k])));
  for (const store of stores) {
    if (!priced.every((k) => MOCK_PRICES[k][store] != null)) continue;
    const total = priced.reduce((a, k) => a + MOCK_PRICES[k][store], 0);
    if (!single_store || total < single_store.total) single_store = { store, groups: priced, total };
  }
  const saving = single_store ? single_store.total - split_total : 0;
  const recommend = !priced.length ? "single" : !single_store || (saving >= 10_000 && split.length > 1) ? "split" : "single";
  return { single_store, split, split_total, saving, recommend, missing_price, single_source };
}

function mockCart(): Cart {
  const pr = mockPrices();
  const lines = [...cart]
    .map((k) => items.find((i) => i.key === k) ?? (extraNames.has(k) ? extraItem(k) : undefined))
    .filter((i): i is Item => !!i);
  const estimate = !lines.some((i) => MOCK_PRICES[i.key])
    ? null
    : pr.recommend === "single" && pr.single_store
      ? pr.single_store.total
      : pr.split_total;
  return {
    items: lines.map((i) => {
      const best = MOCK_PRICES[i.key] && Object.entries(MOCK_PRICES[i.key]).sort((a, b) => a[1] - b[1])[0];
      return {
        key: i.key, name: i.name, category_key: i.category_key, status: i.status, days_left: i.days_left,
        unit_price: best ? best[1] : null, ...(best ? { best_store: best[0] } : {}),
        qty: 1, priority: "normal", source: isSoon(i, 7) ? "forecast" : "check",
        ...(i.predicted_out_on && isSoon(i, 7) ? { need_by: i.predicted_out_on } : {}),
        line_status: "open",
        ...planLines.get(i.key),
      } satisfies CartItem;
    }),
    item_count: lines.length,
    estimate,
    missing_price: pr.missing_price.length,
    state: planState,
    // Như backend (3.1 mục 18): reconciled khi có hóa đơn sau ngày mua, shopping từ ngày mua.
    phase: planState.reconciled_on && (!planState.buy_date || planState.buy_date <= planState.reconciled_on)
      ? "reconciled"
      : planState.buy_date && planState.buy_date <= iso(today) ? "shopping" : "planning",
  };
}

function extraItem(key: string): Item {
  return {
    key, name: extraNames.get(key) ?? key, category_key: "khac", category_label: "", status: "waiting",
    days_left: null, purchase_count: 0, source: "predicted",
  };
}

export async function mockFetch(path: string, init: RequestInit): Promise<Response> {
  await delay();
  const url = new URL(path, "http://mock");
  const method = (init.method ?? "GET").toUpperCase();
  const p = url.pathname;
  const extra = mockM1(method, url, init);
  if (extra) return extra;
  if (p.startsWith("/plan")) return inPlan(url.searchParams.get("plan_id"), () => mockCore(method, url, init));
  return mockCore(method, url, init);
}

async function mockCore(method: string, url: URL, init: RequestInit): Promise<Response> {
  const p = url.pathname;

  // B10: như máy chủ, chỉ xóa khi vừa "đăng nhập lại" trong 5 phút (khóa MOCK_REAUTH_KEY ở lib/auth).
  if (method === "DELETE" && p === "/me") {
    const at = Number(sessionStorage.getItem("mimi.mock.reauthAt") ?? 0);
    if (Date.now() - at > 5 * 60_000) {
      return json({ code: "reauth_required", message: L ? "Please sign in again to confirm deleting your account." : "Mẹ đăng nhập lại để xác nhận xóa tài khoản nhé." }, 403);
    }
    sessionStorage.removeItem("mimi.mock.reauthAt");
    return new Response(null, { status: 204 });
  }
  if (method === "GET" && p === "/me") {
    const me: Me = {
      uid: "mock-uid",
      display_name: mockSettings.display_name ?? (localStorage.getItem("mimi.pendingDisplayName") || "Lan"),
      role: "owner",
      receipt_count: receiptCount,
      settings: { lead_days: mockSettings.lead_days, lang: mockSettings.lang, timezone: mockSettings.timezone,
        remind_enabled: mockSettings.remind_enabled, digest_time: mockSettings.digest_time, relations: mockSettings.relations ?? {},
        ...(mockSettings.country ? { country: mockSettings.country } : {}),
        // Bản thử không có máy chủ: tiền tệ hiển thị nhớ theo máy.
        currency: mockSettings.currency ?? (localStorage.getItem("mimi.currency") === "USD" ? "USD" : "VND") },
    };
    return json(me);
  }

  if (method === "GET" && p === "/fx/rates") {
    // Tỷ giá mẫu: 90 ngày gần nhất, mỗi ngày nhích nhẹ quanh 25.300 đ/USD.
    const rates = Array.from({ length: 90 }, (_, i) => {
      const d = new Date(Date.now() - (89 - i) * 86_400_000);
      return { on: d.toISOString().slice(0, 10), rate: String(25_000 + i * 5) };
    });
    return json({ currency: "USD", rates });
  }

  if (method === "GET" && p === "/home") {
    const summary: HomeSummary = {
      spent_this_month: 1_284_000,
      needs_count: items.filter((i) => isSoon(i, 7)).length,
      plan: (({ item_count, estimate, missing_price }) => ({ item_count, estimate, missing_price }))(mockCart()),
      unread_notifications: 2,
      receipt_count: 12 + savedReceipts.size,
    };
    return json(summary);
  }

  if (method === "GET" && p === "/items") {
    const tab = url.searchParams.get("tab") ?? "soon";
    const horizon = Number(url.searchParams.get("horizon") ?? 7);
    const list = tab === "soon" ? items.filter((i) => isSoon(i, horizon)) : items;
    return json({ items: [...list].sort(order) }); // cùng dạng với API thật
  }

  // Cùng các đường dẫn với API thật: GET /items/{key}, PATCH /items/{key}/stock, PUT /items/{key}/quantity,
  // DELETE /items/{key}/override.
  const itemMatch = p.match(/^\/items\/([^/]+)(\/stock|\/quantity|\/override)?$/);
  if (itemMatch) {
    const key = decodeURIComponent(itemMatch[1]);
    const sub = itemMatch[2] ?? "";
    const it = items.find((i) => i.key === key);
    if (!it) return json({ code: "not_found", message: L ? "Item not found." : "Không tìm thấy món này." }, 404);
    if (method === "GET" && sub === "") {
      const history = Array.from({ length: Math.min(it.purchase_count, 3) }, (_, n) => ({
        on: iso(new Date(today.getTime() - (n + 1) * 12 * 86_400_000)),
        store: "Co.opmart",
        amount_vnd: 32_000,
      }));
      return json({ ...it, history, lots: mockLots(it) });
    }
    // 0.1 mục 3.2: sửa tên chuẩn hóa hoặc chuyển sang nhóm đúng (gộp nếu nhóm đó đã có món).
    if (method === "PATCH" && sub === "") {
      const body = JSON.parse(String(init.body ?? "{}")) as { name?: string; group_key?: string };
      let next: Item = it;
      if (body.name !== undefined) {
        const name = body.name.trim();
        if (!name || name.length > 60) return json({ code: "validation_failed", message: "name", field: "name" }, 422);
        next = { ...next, name };
      }
      if (body.group_key !== undefined && body.group_key !== key) {
        const g = taxonomyGroups.groups.find((x) => x.key === body.group_key);
        if (!g) return json({ code: "validation_failed", message: "group_key", field: "group_key" }, 422);
        const dest = items.find((x) => x.key === g.key);
        next = dest
          ? { ...dest, purchase_count: dest.purchase_count + it.purchase_count }
          : { ...next, key: g.key };
        items = items.filter((x) => x.key !== key && x.key !== g.key);
        items.push(next);
        return json(next);
      }
      items = items.map((x) => (x.key === key ? next : x));
      return json(next);
    }
    if (method === "PATCH" && sub === "/stock") {
      const body = JSON.parse(String(init.body ?? "{}")) as { quick?: "plenty" | "soon" | "out" };
      if (!beforeOverride.has(key)) beforeOverride.set(key, it);
      const days = body.quick === "plenty" ? 30 : body.quick === "soon" ? 3 : 0;
      items = items.map((x) =>
        x.key === key
          ? { ...x, days_left: days, source: "manual", status: body.quick === "out" ? "out" : statusFor(days, 9) }
          : x,
      );
      return json(items.find((x) => x.key === key));
    }
    if (method === "PUT" && sub === "/quantity") {
      const body = JSON.parse(String(init.body ?? "{}")) as { qty: number; unit: string; action?: string };
      let [qty, unit] = toBase(body.qty, body.unit);
      // 0.1 mục 14: increase / decrease theo tồn hiện tại, mark_out về 0.
      if (body.action === "mark_out") [qty, unit] = [0, it.stock?.unit === "pc" || !it.stock ? "pc" : (it.stock.unit as "g" | "ml")];
      else if (body.action === "increase" || body.action === "decrease") {
        if (!it.stock) return json({ code: "invalid_state", message: L ? "Set a quantity first." : "Hãy nhập số lượng hiện có trước." }, 409);
        qty = Math.max(0, it.stock.estimated + (body.action === "increase" ? qty : -qty));
      }
      if (!(qty >= 0)) return json({ code: "validation_failed", message: "qty" }, 422);
      const on = iso(today);
      items = items.map((x) =>
        x.key === key
          ? {
              ...x,
              stock: { estimated: qty, unit, confirmed: qty, confirmed_on: on, confirmed_source: "manual", rate_per_day: x.stock?.rate_per_day },
              // Mimi 1.5: tồn 0 thì Cần mua ngay; có tồn thì ước lượng lại số ngày (mô phỏng).
              ...(qty <= 0
                ? { status: "out" as const, days_left: 0, source: "manual" as const }
                : x.status === "out" || x.status === "buy_now"
                  ? (() => { const d = Math.max(1, Math.round(x.stock?.rate_per_day ? qty / x.stock.rate_per_day : unit === "pc" ? qty * 4 : qty / 150)); return { status: statusFor(d, 9), days_left: d, source: "manual" as const, predicted_out_on: iso(new Date(today.getTime() + d * 86_400_000)) }; })()
                  : {}),
            }
          : x,
      );
      return json(items.find((x) => x.key === key));
    }
    if (method === "DELETE" && sub === "/override") {
      const prev = beforeOverride.get(key);
      if (prev) {
        items = items.map((x) => (x.key === key ? prev : x));
        beforeOverride.delete(key);
      }
      return json(items.find((x) => x.key === key));
    }
  }

  if (method === "POST" && p === "/receipts/read") {
    await delay(1400);
    const form = init.body as FormData;
    // Cùng dạng với Draft của API thật (api/openapi.yaml).
    type P = { brand?: string; product_name?: string; group_key?: string; canonical_quantity?: number; canonical_unit?: CanonicalUnit; raw_name: string };
    const line = (line_id: string, en: string, vi: string, qty: number, vnd: number, category_key: string | undefined, p: P) => ({
      line_id, name: L ? en : vi, qty, amount: String(vnd), amount_vnd: vnd, habit: true,
      ...(category_key ? { category_key } : {}),
      ...p,
      detail_name: [p.brand, p.product_name].filter(Boolean).join(" ") || undefined,
    });
    const lines = [
      line("l1", "Unsweetened fresh milk 1L", "Sữa tươi không đường 1L", 2, 64_000, "sua",
        { raw_name: "VINAMILK SUA TUOI KHONG DUONG 1L", brand: "Vinamilk", product_name: L ? "Unsweetened" : "Không đường", group_key: "sua-tuoi", canonical_quantity: 2000, canonical_unit: "ml" }),
      line("l2", "Dish soap 750ml", "Nước rửa chén 750ml", 1, 38_000, "hoa-pham",
        { raw_name: "SUNLIGHT CHANH 750ML", brand: "Sunlight", product_name: L ? "Lemon" : "Chanh", group_key: "nuoc-rua-chen", canonical_quantity: 750, canonical_unit: "ml" }),
      line("l3", "Eggs, box of 10", "Trứng gà hộp 10", 1, 36_000, "tuoi-song",
        { raw_name: "TRUNG GA HOP 10", group_key: "trung-ga", canonical_quantity: 10, canonical_unit: "item" }),
      { ...line("l4", "Butter cookies", "Bánh quy bơ", 1, 74_000, undefined, { raw_name: "BANH QUY BO" }),
        needs_review: true, review_reasons: ["missing_category", "missing_group"] as ("missing_category" | "missing_group")[] },
    ];
    const draft: ReceiptDraft = {
      receipt_id: String(form.get("receipt_id")),
      store: "Co.opmart Lý Thường Kiệt",
      purchased_on: iso(today),
      currency: "VND",
      total_printed: "212000",
      lines,
      sum_vnd: lines.reduce((a, l) => a + l.amount_vnd, 0),
      total_mismatch: false,
      fx_missing: false,
      pages: form.getAll("images").length || 1,
      unreadable_pages: 0,
    };
    return json(draft);
  }

  if (method === "POST" && p === "/receipts") {
    // Như backend: từ chối trường lạ (DisallowUnknownFields), để bản thử bắt được lỗi lệch hợp đồng.
    const body = JSON.parse(String(init.body)) as Record<string, unknown> & { receipt_id: string; lines?: Record<string, unknown>[] };
    const top = ["receipt_id", "store", "purchased_on", "currency", "discount_total", "lines", "use_for_prediction", "confirm_duplicate", "source_plan_id", "assignment_id"];
    const lineKeys = ["line_id", "name", "qty", "unit", "amount", "category_key", "group_key", "habit",
      "raw_name", "brand", "product_name", "source_language", "original_name", "package_count", "package_size", "package_unit", "plans", "price_source"];
    const unknown =
      Object.keys(body).some((k) => !top.includes(k)) ||
      (body.lines ?? []).some((l) => Object.keys(l).some((k) => !lineKeys.includes(k)));
    if (unknown) return json({ code: "bad_request", message: "Unknown field" }, 400);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(String(body.purchased_on ?? ""))) {
      return json({ code: "validation_failed", message: "purchased_on", fields: { field: "purchased_on" } }, 422);
    }
    const replayed = savedReceipts.has(body.receipt_id);
    if (!replayed) {
      savedReceipts.add(body.receipt_id);
      receiptCount += 1;
      if (!body.source_plan_id && !body.assignment_id) planState = { ...planState, reconciled_on: String(body.purchased_on) };
    }
    return json({ receipt_id: body.receipt_id, status: "saved", replayed }, replayed ? 200 : 201);
  }

  if (method === "PUT" && p === "/settings") {
    const body = JSON.parse(String(init.body ?? "{}")) as Partial<typeof mockSettings>;
    const rels = { ...(mockSettings.relations ?? {}) };
    for (const [uid, rel] of Object.entries(body.relations ?? {})) {
      if (rel) rels[uid] = rel;
      else delete rels[uid];
    }
    mockSettings = { ...mockSettings, ...body, relations: rels };
    return json({
      ...mockSettings, display_name: mockSettings.display_name ?? "Lan",
      currency: mockSettings.currency ?? (localStorage.getItem("mimi.currency") === "USD" ? "USD" : "VND"),
    });
  }

  if (method === "GET" && p === "/spending") {
    const month = url.searchParams.get("month") ?? "";
    if (!/^\d{4}-\d{2}$/.test(month)) return json({ code: "validation_failed", message: "month" }, 422);
    const [y, m] = month.split("-").map(Number);
    const sameAsMock = y === today.getFullYear() && m === today.getMonth() + 1;
    const prevOfMock = (y * 12 + m) === today.getFullYear() * 12 + today.getMonth(); // tháng liền trước
    const base: SpendingReport = {
      month, spent: 0, spent_prev_month_pct: null, saved: 0, saved_ratio: null, receipt_count: 0,
      by_category: [], top_saving_receipts: [], recent_receipts: [],
    };
    if (prevOfMock) return json({ ...base, spent: 1_070_000, receipt_count: 4 });
    if (!sameAsMock) return json(base);
    const r = (id: string, store: string, d: number, total: number, discount: number) => ({
      receipt_id: id, store, purchased_on: `${month}-${String(d).padStart(2, "0")}`, total_vnd: total, discount_vnd: discount,
    });
    return json({
      ...base, spent: 1_284_000, spent_prev_month_pct: 20, saved: 96_000, saved_ratio: 0.07, receipt_count: receiptCount,
      by_category: [
        { key: "hoa-don-dinh-ky", amount: 354_499, pct: 27.6 },
        { key: "hoa-pham", amount: 412_000, pct: 32.1 }, { key: "sua", amount: 318_000, pct: 24.8 },
        { key: "tuoi-song", amount: 264_000, pct: 20.6 }, { key: "do-kho", amount: 190_000, pct: 14.8 },
        { key: "chua-phan-loai", amount: 100_000, pct: 7.8 },
      ],
      top_saving_receipts: [r("r2", "Co.opmart Lý Thường Kiệt", 12, 486_000, 54_000), r("r1", "Bách Hóa Xanh", 4, 312_000, 42_000)],
      // Mimi 1.5: Lịch sử mua hàng liệt kê mọi hóa đơn của tháng (API trả tối đa 200).
      recent_receipts: [r("r3", "Bách Hóa Xanh", 15, 274_000, 0), r("r2", "Co.opmart Lý Thường Kiệt", 12, 486_000, 54_000), r("r4", "WinMart gần nhà", 8, 140_000, 0), r("r1", "Bách Hóa Xanh", 4, 312_000, 42_000), r("r5", "Chợ Bến Thành", 2, 72_000, 0)],
    } satisfies SpendingReport);
  }

  if (method === "GET" && p === "/notifications") {
    const list: Notification[] = items
      .filter((i) => isSoon(i, 7))
      .sort(order)
      .map((i) => ({ id: `item:${i.key}`, kind: "item", item_key: i.key, name: i.name, status: i.status, days_left: i.days_left }));
    // B9: sự kiện trong nhà đứng đầu.
    const events: Notification[] = [
      { id: "event:done-mock", kind: "event", event: { id: "done-mock", kind: "run_done", at: new Date(Date.now() - 3600_000).toISOString(), actor_name: "Minh", plan_name: "Mua đồ cuối tuần", items: 4, returned: 1, read: mockEventRead.has("event:done-mock") } },
    ];
    return json({ items: [...events, ...list] });
  }
  const readM = p.match(/^\/notifications\/([^/]+)\/read$/);
  if (readM && method === "POST") {
    mockEventRead.add(decodeURIComponent(readM[1]));
    return json(undefined, 204);
  }

  if (method === "GET" && p === "/plan") return json(mockCart());
  if (method === "GET" && p === "/plan/prices") return json(mockPrices());
  if (method === "PUT" && p === "/plan/state") {
    const { base_rev, ...next } = JSON.parse(String(init.body)) as PlanState;
    const cur = planState.rev ?? 0;
    // A1: máy khác đã lưu sau lần đọc thì báo xung đột, không ghi đè.
    if (base_rev != null && base_rev !== cur) {
      return json({ code: "conflict", message: L ? "This plan was just changed on another device. Mimi reloaded it, please check and edit again." : "Kế hoạch vừa được sửa trên máy khác. Mimi đã tải lại bản mới, mẹ xem rồi sửa lại nhé.", details: { rev: cur } }, 409);
    }
    planState = { ...next, rev: cur + 1, reconciled_on: planState.reconciled_on }; // reconciled_on chỉ do lưu hóa đơn ghi
    return json(planState);
  }
  const planItem = p.match(/^\/plan\/items\/([^/]+)$/);
  if (planItem) {
    const key = decodeURIComponent(planItem[1]);
    if (method === "PUT") {
      if (!items.some((i) => i.key === key)) {
        const body = init.body ? (JSON.parse(String(init.body)) as { name?: string }) : {};
        const tax = TAXONOMY.find((g) => g.key === key);
        if (!tax && !(key.startsWith("custom-") && body.name)) return json({ code: "not_found", message: "Not found" }, 404);
        extraNames.set(key, body.name || (tax ? tax.labels[L ? "en" : "vi"] : key));
      }
      cart.add(key);
      return json(mockCart());
    }
    if (method === "PATCH") {
      if (!cart.has(key)) return json({ code: "not_found", message: "Not found" }, 404);
      const body = JSON.parse(String(init.body ?? "{}")) as PlanLinePatch;
      if (body.qty != null && !(body.qty > 0 && body.qty <= 10_000)) return json({ code: "validation_failed", message: "qty" }, 422);
      planLines.set(key, { ...planLines.get(key), ...body });
      return json(mockCart());
    }
    if (method === "DELETE") {
      cart.delete(key);
      planLines.delete(key);
      return json(undefined, 204);
    }
  }

  const online = mockOnline(method, p, init);
  if (online) return online;

  return json({ code: "not_found", message: `Prototype: ${method} ${p} is not available` }, 404);
}

// ================= Mimi Online (bản thử, mô phỏng backend internal/order) =================

const UNIT_BASE: Record<string, [number, "g" | "ml" | "pc"]> = {
  g: [1, "g"], kg: [1000, "g"], ml: [1, "ml"], l: [1000, "ml"],
};
function toBase(qty: number, unit: string): [number, "g" | "ml" | "pc"] {
  const [f, u] = UNIT_BASE[unit] ?? [1, "pc"];
  return [qty * f, u];
}

type Product = { name: string; unit: string; pack: number; snapshot: number; offer: number; eta: number; category: "grocery" | "household" };
// Cửa hàng sandbox; giá mẫu. Dầu gội có giá cao hơn giá quét để thấy luồng ASK.
const CATALOG: Record<string, Product> = {
  "sua-tuoi": { name: "Sữa tươi không đường 1L", unit: "hộp", pack: 1, snapshot: 30_000, offer: 31_000, eta: 1, category: "grocery" },
  "nuoc-rua-chen": { name: "Nước rửa chén 750ml", unit: "chai", pack: 1, snapshot: 38_000, offer: 37_000, eta: 1, category: "household" },
  "trung-ga": { name: "Trứng gà vỉ 10", unit: "quả", pack: 10, snapshot: 36_000, offer: 34_000, eta: 1, category: "grocery" },
  gao: { name: "Gạo ST25 5kg", unit: "túi", pack: 1, snapshot: 120_000, offer: 125_000, eta: 2, category: "grocery" },
  "dau-goi": { name: "Dầu gội 650g", unit: "chai", pack: 1, snapshot: 95_000, offer: 118_000, eta: 2, category: "household" },
  "giay-ve-sinh": { name: "Giấy vệ sinh 10 cuộn", unit: "bịch", pack: 1, snapshot: 72_000, offer: 72_000, eta: 1, category: "household" },
  "bot-giat": { name: "Bột giặt 3kg", unit: "túi", pack: 1, snapshot: 155_000, offer: 149_000, eta: 2, category: "household" },
};
const VND_PER_USD = 25_000;
let mandate: Mandate = { enabled: false, categories: ["grocery", "household"], price_tolerance_pct: 10, max_order_vnd: 1_000_000, max_day_vnd: 0 };
const orders = new Map<string, Order>();
const addDays = (n: number) => iso(new Date(today.getTime() + n * 86_400_000));
const usd = (vnd: number) => (Math.round((vnd / VND_PER_USD) * 100) / 100).toFixed(2);

function buildBasket(keys: string[]): Basket {
  const c = mockCart();
  const lines: OrderLine[] = [];
  const missing: string[] = [];
  for (const k of keys) {
    const ci = c.items.find((x) => x.key === k);
    const pr = CATALOG[k];
    if (!ci) continue;
    if (!pr) {
      missing.push(ci.name);
      continue;
    }
    const want = ci.qty - (ci.bought_qty ?? 0);
    const packs = Math.max(1, Math.ceil(want / pr.pack));
    lines.push({
      key: k, name: ci.name, product_name: pr.name, category_key: ci.category_key, category: pr.category,
      qty: packs, unit: pr.unit, pack_qty: pr.pack, plan_qty: ci.qty, plan_unit: ci.unit,
      unit_price_vnd: pr.offer, snapshot_price_vnd: pr.snapshot, total_vnd: pr.offer * packs,
      need_by: ci.need_by, eta: addDays(pr.eta),
    });
  }
  const total = lines.reduce((a, l) => a + l.total_vnd, 0);
  const reasons: MandateReason[] = [];
  let result: Basket["decision"]["result"] = "ALLOW";
  const add = (r: MandateReason) => {
    reasons.push(r);
    if (r.result === "BLOCK" || (r.result === "ASK" && result === "ALLOW")) result = r.result;
  };
  for (const l of lines) {
    if (!mandate.categories.includes(l.category)) add({ key: l.key, code: "category", result: "BLOCK", detail: `Danh mục ${l.category} chưa được ủy quyền` });
    const maxP = Math.floor((l.snapshot_price_vnd * (100 + mandate.price_tolerance_pct)) / 100);
    if (l.unit_price_vnd > maxP) add({ key: l.key, code: "price_guard", result: "ASK", detail: `Giá ${l.unit_price_vnd}đ cao hơn mức cho phép ${maxP}đ` });
    if (l.need_by && l.eta > l.need_by) add({ key: l.key, code: "delivery_guard", result: "ASK", detail: `Giao dự kiến ${l.eta}, sau ngày cần ${l.need_by}` });
  }
  if (mandate.max_order_vnd > 0 && total > mandate.max_order_vnd) add({ code: "budget_order", result: "ASK", detail: `Tổng đơn vượt trần ${mandate.max_order_vnd}đ/đơn` });
  if (!mandate.enabled && result === "ALLOW") add({ code: "mandate_off", result: "ASK", detail: "Chưa bật ủy quyền mua online: cần bạn duyệt đơn này" });
  return {
    merchant: "Mimi Mart Sandbox", lines, missing, total_vnd: total, amount_usd: usd(total), vnd_per_usd: VND_PER_USD,
    snapshot_at: iso(today), decision: { result, reasons },
  };
}

const NEXT: Partial<Record<OrderState, OrderState>> = { paid: "ordered", ordered: "shipped", shipped: "delivered" };

function mockOnline(method: string, p: string, init: RequestInit): Response | null {
  const body = () => JSON.parse(String(init.body ?? "{}")) as Record<string, unknown>;
  const now = () => new Date().toISOString();
  if (p === "/mandate") {
    if (method === "GET") return json(mandate);
    if (method === "PUT") {
      mandate = body() as Mandate;
      return json(mandate);
    }
  }
  if (method === "POST" && p === "/orders/basket") return json(buildBasket((body().keys as string[]) ?? [...cart]));
  if (method === "GET" && p === "/orders") return json({ orders: [...orders.values()].reverse() });
  if (method === "POST" && p === "/orders") {
    const b = body() as { keys?: string[]; approve?: boolean };
    const basket = buildBasket(b.keys ?? [...cart]);
    if (!basket.lines.length) return json({ code: "validation_failed", message: "empty" }, 422);
    if (basket.decision.result === "BLOCK") return json({ code: "mandate_blocked", message: "Blocked" }, 403);
    if (basket.decision.result === "ASK" && !b.approve) return json({ code: "approval_required", message: "Approve" }, 409);
    const id = "o-" + Array.from({ length: 16 }, () => Math.floor(Math.random() * 16).toString(16)).join("");
    const o: Order = {
      id, state: "awaiting_payment", merchant: basket.merchant, lines: basket.lines, total_vnd: basket.total_vnd,
      amount_usd: basket.amount_usd, vnd_per_usd: VND_PER_USD, decision: basket.decision, approved_by_user: !!b.approve,
      paypal_order_id: `FAKE-${basket.amount_usd}-mock`, eta: basket.lines.reduce((a, l) => (l.eta > a ? l.eta : a), ""),
      events: [{ state: "awaiting_payment", at: now() }], refunds: [], refunded_vnd: 0, net_vnd: basket.total_vnd,
      suggested_refund_vnd: 0, created_at: now(),
    };
    orders.set(id, o);
    return json(o, 201);
  }
  const m = p.match(/^\/orders\/(o-[0-9a-f]{16})(\/capture|\/advance|\/delivery\/read|\/verify|\/refund)?$/);
  if (!m) return null;
  const o = orders.get(m[1]);
  if (!o) return json({ code: "not_found", message: "Order not found" }, 404);
  const set = (next: Order) => {
    orders.set(next.id, next);
    return json(next);
  };
  const bad = () => json({ code: "invalid_state", message: o.state }, 409);
  switch (method + (m[2] ?? "")) {
    case "GET":
      return json(o);
    case "POST/capture":
      if (o.state !== "awaiting_payment") return o.state === "paid" ? json(o) : bad();
      return set({ ...o, state: "paid", paypal_capture_id: "FAKECAP-mock", paid_at: now(), events: [...o.events, { state: "paid", at: now() }] });
    case "POST/advance": {
      const next = NEXT[o.state];
      if (!next) return bad();
      return set({ ...o, state: next, events: [...o.events, { state: next, at: now() }] });
    }
    case "POST/delivery/read":
      if (o.state !== "delivered") return bad();
      // Bản thử: "thấy" thiếu một gói ở dòng đầu để demo luồng refund.
      return json({ items: o.lines.map((l, n) => ({ key: l.key, qty: n === 0 ? Math.max(0, l.qty - 1) : l.qty, confidence: n === 0 ? "mid" : "high" })) });
    case "POST/verify": {
      if (o.state !== "delivered") return bad();
      const actual = (body().actual ?? {}) as Record<string, number>;
      let suggested = 0;
      const lines = o.lines.map((l) => {
        const got = Math.min(l.qty, Math.max(0, Math.floor(actual[l.key] ?? 0)));
        suggested += (l.qty - got) * l.unit_price_vnd;
        if (got > 0 && cart.has(l.key)) {
          const ci = mockCart().items.find((x) => x.key === l.key);
          const bought = (ci?.bought_qty ?? 0) + got * l.pack_qty;
          if (ci && bought >= ci.qty) {
            cart.delete(l.key);
            planLines.delete(l.key);
          } else planLines.set(l.key, { ...planLines.get(l.key), bought_qty: bought });
        }
        const match: OrderLine["match"] = got >= l.qty ? "match" : got > 0 ? "partial" : "missing";
        return { ...l, verified_qty: got, match };
      });
      return set({ ...o, state: "verified", lines, suggested_refund_vnd: suggested, events: [...o.events, { state: "verified", at: now() }] });
    }
    case "POST/refund": {
      if (o.state !== "verified") return bad();
      const b = body() as { reason: RefundReason; amount_vnd: number };
      const amount = Math.round(b.amount_vnd);
      if (!(amount > 0) || amount > o.net_vnd) return json({ code: "validation_failed", message: "amount_vnd" }, 422);
      const r = { id: `FAKEREF-${o.refunds.length + 1}`, ref: `${o.id}-r${o.refunds.length + 1}`, amount_vnd: amount, amount_usd: usd(amount), reason: b.reason, status: "COMPLETED", at: now() };
      return set({ ...o, refunds: [...o.refunds, r], refunded_vnd: o.refunded_vnd + amount, net_vnd: o.net_vnd - amount, suggested_refund_vnd: Math.max(0, o.suggested_refund_vnd - amount) });
    }
  }
  return null;
}

// ================= Bộ spec Mimi 1.0 (bản thử) =================

// Bản sao của backend/internal/taxonomy/groups.json (0.2) để bản thử có đủ 300 nhóm.
// Khi taxonomy đổi: chép lại file đó vào src/api/taxonomy-groups.json.
const TAXONOMY = taxonomyGroups.groups.map((g) => ({
  key: g.key, parent: g.parent, labels: g.labels, label: L ? g.labels.en : g.labels.vi, measurement: g.measurement, unit: g.unit,
}));

const fold = (s: string) => s.normalize("NFD").replace(/[\u0300-\u036f]/g, "").replace(/đ/g, "d").toLowerCase();

function mockLots(it: Item) {
  if (!it.stock || it.purchase_count === 0) return [];
  let left = it.stock.estimated;
  const unit = it.stock.unit === "pc" ? "item" : it.stock.unit;
  return Array.from({ length: Math.min(3, it.purchase_count) }, (_, n) => {
    const initial = unit === "item" ? 10 : 1000;
    const remaining = Math.max(0, Math.min(initial, left));
    left -= remaining;
    return {
      lot_id: `${it.key}-${n}`, detail_name: it.name, store: "Co.opmart", purchased_at: iso(new Date(today.getTime() - n * 12 * 86_400_000)),
      initial_quantity: initial, canonical_unit: unit, remaining_quantity: remaining, inventory_source: "receipt",
    };
  });
}

type MockRecurring = { id: string; name: string; amount: number; cycle: "weekly" | "monthly" | "quarterly" | "yearly"; next_due_at: string; active: boolean };
const dueIn = (n: number) => iso(new Date(today.getTime() + n * 86_400_000));
let recurring: MockRecurring[] = [
  { id: "dien", name: L ? "Electricity" : "Tiền điện", amount: 680_000, cycle: "monthly", next_due_at: dueIn(0), active: true },
  { id: "internet", name: "Internet", amount: 250_000, cycle: "monthly", next_due_at: dueIn(5), active: true },
  { id: "netflix", name: "Netflix", amount: 260_000, cycle: "monthly", next_due_at: dueIn(12), active: true },
  { id: "dien-thoai", name: L ? "Phone" : "Điện thoại", amount: 150_000, cycle: "monthly", next_due_at: dueIn(28), active: true },
];
const occHistory: { id: string; recurring_id: string; name: string; due_at: string; amount: number; status: "paid" | "skipped" | "matched"; paid_at?: string; source_type?: "manual" | "receipt"; source_id?: string }[] = [];
// Góp ý 01/11 (Phần E): lịch sử các kỳ đã trả, để thấy ở màn chi tiết khoản định kỳ và Lịch sử mua hàng.
for (const [rid, name, amount, monthsAgo, day, via] of [
  ["dien", L ? "Electricity" : "Tiền điện", 642_000, 1, 5, "manual"], ["dien", L ? "Electricity" : "Tiền điện", 598_000, 2, 5, "manual"],
  ["internet", "Internet", 250_000, 1, 10, "manual"], ["internet", "Internet", 250_000, 2, 10, "manual"],
  ["netflix", "Netflix", 260_000, 1, 17, "manual"], ["dien-thoai", L ? "Phone" : "Điện thoại", 150_000, 1, 3, "manual"],
] as const) {
  const d = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth() - monthsAgo, day, 12));
  occHistory.push({ id: `${rid}_${iso(d)}`, recurring_id: rid, name, due_at: iso(d), amount, status: "paid", paid_at: iso(d), source_type: via });
}
occHistory.sort((a, b) => a.due_at.localeCompare(b.due_at));
const occId = (r: MockRecurring) => `${r.id}_${r.next_due_at}`;
const addPeriod = (d: string, c: MockRecurring["cycle"]) => {
  const x = new Date(`${d}T12:00:00Z`);
  if (c === "weekly") x.setUTCDate(x.getUTCDate() + 7);
  else x.setUTCMonth(x.getUTCMonth() + (c === "monthly" ? 1 : c === "quarterly" ? 3 : 12));
  return x.toISOString().slice(0, 10);
};
function recurringView(r: MockRecurring) {
  const days = Math.round((new Date(`${r.next_due_at}T12:00:00Z`).getTime() - today.getTime()) / 86_400_000);
  const display = !r.active ? "paused" : days < 0 ? "overdue" : days === 0 ? "due_today" : days <= 7 ? "due_soon" : "upcoming";
  return {
    ...r, currency: "VND", display,
    current: r.active ? { id: occId(r), recurring_id: r.id, name: r.name, due_at: r.next_due_at, amount: r.amount, status: "pending" } : undefined,
    history: occHistory.filter((o) => o.recurring_id === r.id).reverse(),
  };
}

const ME = "mock-uid";
let household: { id: string; name: string; members: { uid: string; display_name: string; role: "owner" | "member"; joined_at: string }[]; invite?: { code: string } } = {
  id: "nha-lan", name: "Nhà Lan", members: [
    { uid: ME, display_name: "Lan", role: "owner", joined_at: iso(today) },
    { uid: "mock-minh", display_name: "Minh", role: "member", joined_at: iso(today) },
    { uid: "mock-bo", display_name: L ? "Dad" : "Bố", role: "member", joined_at: iso(today) },
    { uid: "mock-me", display_name: L ? "Grandma" : "Bà", role: "member", joined_at: iso(today) },
  ],
};
const mockEventRead = new Set<string>();
type MockAssignment = { assignment_id: string; owner_uid: string; owner_display_name: string; buyer_uid: string; buyer_display_name: string; plan_id: string; status: string; items: { consumption_group_key: string; name_display: string; status: string; qty?: number; unit?: string; unit_price?: number; category_key?: string }[]; created_at: string; updated_at?: string; note?: string;
  trip?: { plan_name: string; buy_date?: string; buy_hour?: number; buy_minute?: number; trip_note?: string } };
// Spec 1.5 mục 2: một lần nhờ Minh mua đang dở, để thấy màn mẹ xem kết quả (tiến độ, ghi chú).
const assignments: MockAssignment[] = [{
  assignment_id: "trip-be", owner_uid: ME, owner_display_name: "Lan", buyer_uid: "mock-minh", buyer_display_name: "Minh", plan_id: "p-be",
  status: "shopping", created_at: iso(today), note: L ? "1L milk was out, got 900ml instead." : "Sữa 1L hết hàng, con lấy loại 900ml.",
  items: [
    { consumption_group_key: "sua-tuoi", name_display: L ? "Fresh milk" : "Sữa tươi", status: "done" },
    { consumption_group_key: "giay-ve-sinh", name_display: L ? "Toilet paper" : "Giấy vệ sinh", status: "done" },
    { consumption_group_key: "nuoc-mam", name_display: L ? "Fish sauce" : "Nước mắm", status: "assigned" },
  ],
}];
// Một lời nhờ mẫu từ người nhà, để thấy luồng Buyer.
const incoming: MockAssignment = {
  assignment_id: "trip-mau", owner_uid: "mock-minh", owner_display_name: "Minh", buyer_uid: ME, buyer_display_name: "Lan", plan_id: "default",
  status: "assigned", created_at: iso(today),
  // Spec 1.5 mục 2: đủ món, số lượng và đơn giá dự kiến để thấy màn đi mua hộ.
  items: [
    { consumption_group_key: "sua-tuoi", name_display: L ? "Unsweetened fresh milk 1L" : "Sữa tươi không đường 1L", status: "assigned", qty: 2, unit: L ? "box" : "hộp", unit_price: 30_000, category_key: "sua" },
    { consumption_group_key: "trung-ga", name_display: L ? "Chicken eggs (10)" : "Trứng gà hộp 10", status: "assigned", qty: 1, unit: L ? "box" : "hộp", unit_price: 36_000, category_key: "tuoi-song" },
    { consumption_group_key: "banh-mi", name_display: L ? "Bread" : "Bánh mì", status: "assigned", qty: 4, unit: L ? "pc" : "ổ", unit_price: 5_000, category_key: "do-kho" },
    { consumption_group_key: "nuoc-rua-chen", name_display: L ? "Dish soap 750ml" : "Nước rửa chén 750ml", status: "assigned", qty: 1, unit: L ? "bottle" : "chai", unit_price: 38_000, category_key: "hoa-pham" },
    { consumption_group_key: "dau-an", name_display: L ? "Cooking oil 1L" : "Dầu ăn 1L", status: "assigned", qty: 1, unit: L ? "bottle" : "chai", unit_price: 52_000, category_key: "gia-vi" },
  ],
  trip: { plan_name: L ? "Weekend shopping" : "Mua đồ cuối tuần", buy_date: iso(new Date(today.getTime() + 2 * 86_400_000)), buy_hour: 9, buy_minute: 30, trip_note: L ? "WinMart near home" : "WinMart gần nhà" },
};

function priceObs(key: string) {
  const prices = MOCK_PRICES[key];
  if (!prices) return [];
  return Object.entries(prices).flatMap(([store, price], n) => [
    { store_name: store, paid_price: price, purchased_at: iso(new Date(today.getTime() - (n * 9 + 3) * 86_400_000)), canonical_quantity: 1000, canonical_unit: "ml", unit_price: price / 1000 },
  ]);
}


/* Spec 1.5 mục 4: lịch sử mua hàng nhiều tháng, chi tiết hóa đơn (dữ liệu mẫu). */
const HIST_EN: Record<string, string> = {
  "Sữa tươi Vinamilk không đường": "Vinamilk unsweetened fresh milk", "Trứng gà": "Chicken eggs",
  "Nước rửa chén Sunlight chanh": "Sunlight lemon dish soap", "Bánh mì sandwich": "Sandwich bread",
  "Sữa chua Vinamilk": "Vinamilk yogurt", "Nước giặt Omo": "Omo laundry liquid",
};
type HLine = [string, string, number, number, string, ("for_owner" | "mine")?];
const HIST_SRC: [string, string, number, number, HLine[], number?][] = [
  // id, cửa hàng, số ngày trước, giờ, các dòng [tên, quy cách, sl, đơn giá, danh mục], giảm giá
  ["h1", "Bách Hóa Xanh", 1, 18, [["Sữa tươi Vinamilk không đường", "1L", 2, 32_000, "sua"], ["Trứng gà", "hộp 10", 1, 36_000, "tuoi-song"], ["Nước rửa chén Sunlight chanh", "750ml", 1, 38_000, "hoa-pham"]], 10_000],
  ["h2", "Co.opmart Lý Thường Kiệt", 3, 10, [["Gạo ST25", "5kg", 1, 185_000, "do-kho"], ["Dầu ăn Neptune", "1L", 2, 52_000, "gia-vi"], ["Giấy vệ sinh Pulppy", "10 cuộn", 1, 72_000, "hoa-pham"]], 24_000],
  ["h3", "WinMart gần nhà", 6, 9, [["Bánh mì sandwich", "ổ", 4, 5_000, "do-kho"], ["Sữa chua Vinamilk", "lốc 4", 2, 28_000, "sua"]]],
  ["h4", "Chợ Bến Thành", 9, 7, [["Thịt heo ba chỉ", "500g", 1, 75_000, "tuoi-song"], ["Rau muống", "bó", 2, 8_000, "tuoi-song"]]],
  ["h5", "Bách Hóa Xanh", 12, 19, [["Nước mắm Nam Ngư", "750ml", 1, 42_000, "gia-vi"], ["Dầu gội Clear", "650g", 1, 165_000, "ve-sinh"]]],
  ["h6", "Co.opmart Lý Thường Kiệt", 34, 11, [["Sữa tươi Vinamilk không đường", "1L", 4, 31_000, "sua"], ["Nước giặt Omo", "3.6kg", 1, 189_000, "hoa-pham"]], 15_000],
  ["h7", "WinMart gần nhà", 38, 17, [["Trứng gà", "hộp 10", 2, 35_000, "tuoi-song"], ["Mì gói Hảo Hảo", "thùng", 1, 115_000, "do-kho"]]],
  ["h8", "Bách Hóa Xanh", 45, 8, [["Gạo ST25", "5kg", 1, 180_000, "do-kho"]]],
  ["h9", "Chợ Bến Thành", 52, 7, [["Cá basa", "1kg", 1, 85_000, "tuoi-song"]]],
  ["h10", "Bách Hóa Xanh", 66, 18, [["Kem đánh răng P/S", "200g", 2, 34_000, "ve-sinh"], ["Sữa tươi Vinamilk không đường", "1L", 2, 30_000, "sua"]]],
];
function histDate(daysAgo: number, hour: number) {
  const d = new Date(today.getTime() - daysAgo * 86_400_000);
  d.setUTCHours(hour - 7, 15);
  return d.toISOString();
}
function histDetail(id: string): ReceiptDetail | null {
  if (id === "h-minh") {
    const lines: ReceiptDetailLine[] = [
      { name: L ? "Fresh milk" : "Sữa tươi Vinamilk", spec: "900ml", qty: 2, unit_price: 29_000, amount: 58_000, category_key: "sua" },
      { name: L ? "Toilet paper" : "Giấy vệ sinh Pulppy", spec: "10 cuộn", qty: 1, unit_price: 72_000, amount: 72_000, category_key: "hoa-pham" },
    ];
    return { receipt_id: id, store: "Bách Hóa Xanh", purchased_at: histDate(0, 17), total_vnd: 130_000, item_names: [], kind: "delegated", buyer_uid: "mock-minh", buyer_name: "Minh", plan_name: L ? "Baby shopping" : "Mua đồ cho bé", plans: [{ id: "p-be", name: L ? "Baby shopping" : "Mua đồ cho bé" }], lines, subtotal: 130_000, discount: 0 };
  }
  const src = HIST_SRC.find((h) => h[0] === id);
  if (!src) return null;
  const [rid, store, ago, hour, ls, disc = 0] = src;
  // Bản tiếng Anh: tên đã dịch lúc quét, tên theo bill (tiếng Việt) giữ ở original_name.
  const lines = ls.map(([name, spec, qty, unit_price, category_key]) => ({
    name: (L && HIST_EN[name]) || name, ...(L && HIST_EN[name] ? { original_name: name } : {}),
    spec, qty, unit_price, amount: qty * unit_price, category_key,
  }));
  const subtotal = lines.reduce((a, l) => a + l.amount, 0);
  const plans = rid === "h1" ? [{ id: "default", name: L ? "Shopping cart" : "Giỏ mua sắm" }] : rid === "h2" ? [{ id: "default", name: L ? "Shopping cart" : "Giỏ mua sắm" }, { id: "p-be", name: L ? "Baby shopping" : "Mua đồ cho bé" }] : [];
  return { receipt_id: rid, store, purchased_at: histDate(ago, hour), total_vnd: subtotal - disc, item_names: [], kind: "own", plans, lines, subtotal, discount: disc };
}

/** Bản thử: nút Cập nhật giá 3 lần mỗi ngày như máy chủ. */
let mockRefresh: { daily: number; left: number; last_at?: string } = { daily: 3, left: 3 };

function mockMarketItems() {
  // Bản thử: nước ngoài Việt Nam thì giả giá tiền gốc ở cửa hàng nước đó, quy ra VND theo tỷ giá mẫu.
  const abroad = ({
    US: { currency: "USD", rate: 25_300, stores: { "Co.opmart": "Walmart", "Bách Hóa Xanh": "Target", "WinMart": "Costco" } as Record<string, string> },
    SG: { currency: "SGD", rate: 19_500, stores: { "Co.opmart": "FairPrice", "Bách Hóa Xanh": "Sheng Siong" } as Record<string, string> },
  } as Record<string, { currency: string; rate: number; stores: Record<string, string> } | undefined>)[mockSettings.country ?? "VN"];
  const on = new Date().toISOString().slice(0, 10);
  // Trứng gà: giả như chưa tra ra giá online; nước rửa chén: online đắt hơn giá nhà mình từng
  // mua. Cả hai để bản thử hiện dòng "giá nhà mình từng mua".
  return mockCart().items.filter((i) => MOCK_PRICES[i.key] && i.key !== "trung-ga").map((i) => ({
    key: i.key, name: i.name, updated_at: new Date().toISOString(),
    prices: Object.entries(MOCK_PRICES[i.key])
      .map(([store, v]) => {
        const vnd = Math.round((v * (i.key === "nuoc-rua-chen" ? 1.1 : 0.97)) / 100) * 100;
        if (!abroad) return { store, price_vnd: vnd, currency: "VND", source: "gemini_search", on };
        const local = Math.round((vnd / abroad.rate) * 100) / 100;
        return { store: abroad.stores[store] ?? store, price_vnd: Math.round(local * abroad.rate), local, currency: abroad.currency, source: "gemini_search", on };
      })
      .sort((a, b) => a.price_vnd - b.price_vnd),
  }));
}

function mockM1(method: string, url: URL, init: RequestInit): Response | null {
  const p = url.pathname;
  const body = () => (init.body ? (JSON.parse(String(init.body)) as Record<string, unknown>) : {});
  const notFound = () => json({ code: "not_found", message: L ? "Not found." : "Không tìm thấy." }, 404);

  if (method === "GET" && p === "/receipts/history") {
    const all = ["h-minh", ...HIST_SRC.map((h) => h[0])].map((id) => histDetail(id)!);
    // Góp ý 01/11: Lịch sử mua hàng có thêm các khoản định kỳ đã trả (kỳ trả qua hóa đơn chỉ hiện hóa đơn).
    const rec = occHistory.filter((o) => o.status === "paid" && o.source_type !== "receipt").map((o) => ({
      receipt_id: `rec-${o.id}`, store: o.name, purchased_at: `${o.paid_at ?? o.due_at}T02:00:00Z`, total_vnd: o.amount, item_names: [o.name], kind: "recurring" as const, recurring_id: o.recurring_id,
    }));
    return json({ receipts: [...all.map(({ lines, plans: _p, subtotal: _s, discount: _d, ...r }) => ({ ...r, item_names: lines.map((l) => l.name) })), ...rec] });
  }
  const histM = p.match(/^\/receipts\/(h[\w-]+)$/);
  if (method === "GET" && histM) {
    const d = histDetail(decodeURIComponent(histM[1]));
    return d ? json(d) : json({ code: "not_found", message: "Không tìm thấy." }, 404);
  }
  if (method === "GET" && p === "/taxonomy") {
    const q = fold(url.searchParams.get("q") ?? "");
    const hits = TAXONOMY.filter((g) => !q || fold(g.labels.vi).includes(q) || g.labels.en.toLowerCase().includes(q));
    return json({ version: taxonomyGroups.version, groups: q ? hits.slice(0, 8) : hits });
  }

  // Nhiều Kế hoạch
  if (p === "/plans" && method === "GET") {
    const main = { id: "default", name: "", status: "active", default: true, item_count: cart.size, buy_date: planState.buy_date, icon: defaultPlanIcon };
    const named = [...namedPlans.entries()].map(([id, x]) => ({ id, name: x.name, status: x.status, icon: x.icon ?? "bag", default: false, item_count: x.cart.size, created: x.created }))
      .sort((a, b) => b.created - a.created);
    return json({ plans: [main, ...named] });
  }
  if (p === "/plans" && method === "POST") {
    const name = String(body().name ?? "").trim();
    if (!name) return json({ code: "validation_failed", message: "name" }, 422);
    const id = `p-${Date.now()}`;
    const icon = typeof body().icon === "string" ? String(body().icon) : "bag";
    namedPlans.set(id, { name, status: "active", icon, created: Date.now(), cart: new Set(), lines: new Map(), state: { buy_date: "", buy_hour: null, trip_note: "" } });
    return json({ id, name, status: "active", icon, default: false, item_count: 0 }, 201);
  }
  const planM = p.match(/^\/plans\/([^/]+)$/);
  if (planM && planM[1] === "default" && method === "PATCH") {
    const b = body() as { icon?: string };
    if (b.icon) defaultPlanIcon = b.icon;
    return json({ id: "default", name: "", status: "active", default: true, item_count: cart.size, icon: defaultPlanIcon });
  }
  if (planM) {
    const x = namedPlans.get(planM[1]);
    if (!x) return notFound();
    if (method === "PATCH") {
      const b = body() as { name?: string; status?: "active" | "archived"; icon?: string };
      if (b.name) x.name = b.name;
      if (b.status) x.status = b.status;
      if (b.icon) x.icon = b.icon;
      return json({ id: planM[1], name: x.name, status: x.status, icon: x.icon ?? "bag", default: false, item_count: x.cart.size });
    }
    if (method === "DELETE") {
      namedPlans.delete(planM[1]);
      return json(undefined, 204);
    }
  }

  // Lịch sử giá
  const ph = p.match(/^\/items\/([^/]+)\/price-history$/);
  if (ph && method === "GET") {
    const key = decodeURIComponent(ph[1]);
    const hist = priceObs(key);
    const sorted = [...hist].sort((a, b) => a.unit_price - b.unit_price);
    return json({
      group_key: key, purchase_count: hist.length, canonical_unit: hist.length ? "ml" : undefined,
      last_purchase: hist[0] ?? null, lowest_historical: hist.length > 1 ? sorted[0] : null,
      average_unit_price: hist.length > 1 ? Math.round((hist.reduce((a, o) => a + o.unit_price, 0) / hist.length) * 100) / 100 : null,
      other_dimension_count: 0, history: hist, window_days: 90,
    });
  }
  if (p === "/plan/market-prices" && method === "GET") {
    // Bản thử: giá "tra trên mạng" giả, lệch nhẹ so với giá hóa đơn mẫu.
    return inPlan(url.searchParams.get("plan_id"), () => json({ items: mockMarketItems(), refresh: mockRefresh }));
  }
  if (p === "/plan/market-prices/refresh" && method === "POST") {
    // Bản thử: không tra thật, trả lại bảng giá giả như vừa tra xong.
    return inPlan(url.searchParams.get("plan_id"), () => {
      const items = mockMarketItems();
      if (mockRefresh.left <= 0) {
        return json({ code: "quota_exceeded", message: `Hôm nay bạn đã cập nhật giá ${mockRefresh.daily} lần, mai cập nhật tiếp nhé. Mimi vẫn tự tra giá mỗi đêm.` }, 429);
      }
      mockRefresh = { ...mockRefresh, left: mockRefresh.left - 1, last_at: new Date().toISOString() };
      return json({ items, looked: mockCart().items.length, found: items.length, fresh: 0, refresh: mockRefresh });
    });
  }
  if (p === "/plan/price-history" && method === "GET") {
    return inPlan(url.searchParams.get("plan_id"), () => {
      const keys = [...cart].filter((k) => planLines.get(k)?.line_status !== "assigned");
      const stores = new Map<string, { covered: number; total: number; missing: string[] }>();
      for (const k of keys) for (const s of Object.keys(MOCK_PRICES[k] ?? {})) if (!stores.has(s)) stores.set(s, { covered: 0, total: 0, missing: [] });
      for (const [s, agg] of stores) for (const k of keys) {
        const v = MOCK_PRICES[k]?.[s];
        if (v == null) agg.missing.push(k);
        else {
          agg.covered++;
          agg.total += v;
        }
      }
      const pr = mockPrices();
      const options = [...stores.entries()].map(([s, a]) => ({
        store_name: s, store_key: fold(s).replace(/[^a-z0-9]+/g, "-"), covered_items: a.covered, total_items: keys.length,
        estimated_total: a.covered === keys.length ? a.total : null, covered_subtotal: a.total, missing_keys: a.missing,
      })).sort((a, b) => b.covered_items - a.covered_items || (a.estimated_total ?? 0) - (b.estimated_total ?? 0));
      return json({
        basket_item_count: keys.length, store_options: options,
        split_store_option: pr.split.length ? { estimated_total: pr.split_total, stores: pr.split.map((s) => ({ store_name: s.store, keys: s.groups, total: s.total })), covered_items: keys.length - pr.missing_price.length } : null,
        recommend: pr.recommend, no_history: pr.missing_price, window_days: 90,
      });
    });
  }

  // Khoản định kỳ và Dự báo
  if (p === "/recurring" && method === "GET") {
    const items = recurring.map(recurringView);
    const monthly = recurring.filter((r) => r.active).reduce((a, r) => a + (r.cycle === "weekly" ? Math.round((r.amount * 52) / 12) : r.cycle === "quarterly" ? Math.round(r.amount / 3) : r.cycle === "yearly" ? Math.round(r.amount / 12) : r.amount), 0);
    return json({ items, monthly_total: monthly });
  }
  if (p === "/recurring" && method === "POST") {
    const b = body() as Partial<MockRecurring>;
    if (!b.name || !b.amount || !b.next_due_at) return json({ code: "validation_failed", message: "name" }, 422);
    const r: MockRecurring = { id: `r-${Date.now()}`, name: b.name, amount: b.amount, cycle: b.cycle ?? "monthly", next_due_at: b.next_due_at, active: true };
    recurring.push(r);
    return json(recurringView(r), 201);
  }
  const recM = p.match(/^\/recurring\/([^/]+)$/);
  if (recM) {
    const r = recurring.find((x) => x.id === recM[1]);
    if (!r) return notFound();
    if (method === "PATCH") {
      Object.assign(r, body());
      return json(recurringView(r));
    }
    if (method === "DELETE") {
      recurring = recurring.filter((x) => x !== r);
      return json(undefined, 204);
    }
  }
  const occM = p.match(/^\/recurring\/occurrences\/([^/]+)\/(pay|skip)$/);
  if (occM && method === "POST") {
    const r = recurring.find((x) => occId(x) === decodeURIComponent(occM[1]));
    if (!r) return notFound();
    const o = { id: occId(r), recurring_id: r.id, name: r.name, due_at: r.next_due_at, amount: r.amount, status: occM[2] === "pay" ? "paid" as const : "skipped" as const, paid_at: occM[2] === "pay" ? iso(today) : undefined };
    occHistory.push(o);
    r.next_due_at = addPeriod(r.next_due_at, r.cycle);
    return json(o);
  }
  if (p === "/forecast" && method === "GET") {
    const to = iso(new Date(today.getTime() + 30 * 86_400_000));
    const occ = recurring.filter((r) => r.active && r.next_due_at <= to).map((r) => recurringView(r).current!);
    const shopping = items.filter((i) => i.predicted_out_on && i.predicted_out_on <= to).map((i) => {
      const prices = Object.values(MOCK_PRICES[i.key] ?? {});
      return { key: i.key, name: i.name, out_on: i.predicted_out_on!, amount: prices.length ? Math.min(...prices) : null };
    });
    const recurringTotal = occ.reduce((a, o) => a + o.amount, 0);
    const shoppingTotal = shopping.reduce((a, s) => a + (s.amount ?? 0), 0);
    return json({
      from: iso(today), to, total: recurringTotal + shoppingTotal, recurring_total: recurringTotal, shopping_total: shoppingTotal,
      recurring: occ, shopping, missing_prices: shopping.filter((s) => s.amount == null).length,
    });
  }

  // Nhà và mua hộ
  if (p === "/households" && method === "GET") return json({ ...household, owner_uid: ME, my_role: "owner" });
  if (p === "/households" && method === "PATCH") {
    household.name = String(body().name ?? "");
    household.id ||= `h-${ME}`;
    return json({ ...household, owner_uid: ME, my_role: "owner" });
  }
  if (p === "/households/invites" && method === "POST") {
    household.id ||= `h-${ME}`;
    household.invite = { code: Math.random().toString(36).slice(2, 10).toUpperCase().padEnd(8, "X") };
    // Bản thử: có mã là có ngay một người nhà mẫu để thử gửi Kế hoạch.
    if (household.members.length === 1) household.members.push({ uid: "mock-minh", display_name: "Minh", role: "member", joined_at: iso(today) });
    return json(household.invite, 201);
  }
  if (p === "/households/join" && method === "POST") return json({ code: "not_found", message: L ? "That invite code is wrong." : "Mã mời không đúng." }, 404);
  if (p === "/households/leave" && method === "POST") return json({ code: "invalid_state", message: "owner" }, 409);
  const memM = p.match(/^\/households\/members\/([^/]+)$/);
  if (memM && method === "DELETE") {
    household.members = household.members.filter((m) => m.uid !== decodeURIComponent(memM[1]));
    return json(undefined, 204);
  }
  if (p === "/households/shopping-assignments" && method === "POST") {
    const b = body() as { buyer_uid: string; keys: string[]; plan_id?: string };
    const buyer = household.members.find((m) => m.uid === b.buyer_uid);
    if (!buyer || buyer.uid === ME) return json({ code: "forbidden", message: "buyer" }, 403);
    return inPlan(b.plan_id ?? null, () => {
      // B5: kế hoạch đang gửi cho cùng người thì ghi đè bản đã gửi.
      const cur = assignments.find((x) => x.plan_id === (b.plan_id ?? "default") && x.buyer_uid === buyer.uid && (x.status === "assigned" || x.status === "shopping"));
      if (cur) {
        const done = cur.items.filter((i) => i.status === "done");
        for (const i of cur.items) if (i.status === "assigned" && !b.keys.includes(i.consumption_group_key)) planLines.set(i.consumption_group_key, { ...planLines.get(i.consumption_group_key), line_status: "open", assigned_to_uid: undefined });
        cur.items = [...done, ...b.keys.filter((k) => !done.some((d) => d.consumption_group_key === k)).map((k) => ({ consumption_group_key: k, name_display: items.find((i) => i.key === k)?.name ?? extraNames.get(k) ?? k, status: "assigned" as const }))];
        cur.updated_at = new Date().toISOString();
        for (const k of b.keys) planLines.set(k, { ...planLines.get(k), line_status: "assigned", assigned_to_uid: buyer.uid });
        return json(cur, 201);
      }
      const a: MockAssignment = {
        assignment_id: `trip-${Date.now()}`, owner_uid: ME, owner_display_name: "Lan", buyer_uid: buyer.uid, buyer_display_name: buyer.display_name,
        plan_id: b.plan_id ?? "default", status: "assigned", created_at: new Date().toISOString(),
        items: b.keys.map((k) => ({ consumption_group_key: k, name_display: items.find((i) => i.key === k)?.name ?? extraNames.get(k) ?? k, status: "assigned" })),
      };
      for (const k of b.keys) planLines.set(k, { ...planLines.get(k), line_status: "assigned", assigned_to_uid: buyer.uid });
      assignments.unshift(a);
      return json(a, 201);
    });
  }
  const asgM = p.match(/^\/households\/shopping-assignments\/([^/]+)\/cancel$/);
  if (asgM && method === "POST") {
    const a = assignments.find((x) => x.assignment_id === decodeURIComponent(asgM[1]));
    if (!a) return notFound();
    a.status = "cancelled";
    inPlan(a.plan_id, () => {
      for (const it of a.items) {
        const l = planLines.get(it.consumption_group_key);
        if (l) planLines.set(it.consumption_group_key, { ...l, line_status: "open", assigned_to_uid: undefined });
        it.status = "returned";
      }
    });
    return json(a);
  }
  // Spec 1.5 mục 2: người được nhờ bấm Mua hàng xong, hoặc từ chối.
  const runM = p.match(/^\/households\/shopping-assignments\/([^/]+)\/(complete|decline)$/);
  if (runM && method === "POST" && decodeURIComponent(runM[1]) === incoming.assignment_id) {
    incoming.status = runM[2] === "complete" ? "completed" : "declined";
    return json(incoming);
  }
  if (p === "/plan/delegated" && method === "GET") {
    return json({ to_me: incoming.status === "assigned" ? [incoming] : [], from_me: assignments });
  }
  if (method === "POST" && p === "/receipts") {
    const b = body() as { assignment_id?: string };
    if (b.assignment_id === incoming.assignment_id) incoming.status = "closed_partial";
  }
  return null;
}
