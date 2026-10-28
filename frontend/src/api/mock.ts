import { uiLang } from "../i18n";
/**
 * Backend giả cho chế độ VITE_API_MODE=mock. Dữ liệu mẫu, không phải dữ liệu thật.
 * Quy tắc trạng thái ở đây chỉ mô phỏng; bản thật do API Go tính (internal/domain).
 */
import type {
  Basket, Cart, CartItem, HomeSummary, Item, Mandate, MandateReason, Me, Notification, Order, OrderLine, OrderState,
  PlanLinePatch, PlanPrices, PlanState, PlanStop, ReceiptDraft, RefundReason, SpendingReport, StockStatus,
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
const today = new Date(2026, 9, 16);
const iso = (d: Date) => d.toISOString().slice(0, 10);

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
}));

let receiptCount = 3;
let mockSettings: Me["settings"] & { display_name?: string } = {
  lead_days: 7, lang: "vi", timezone: "Asia/Ho_Chi_Minh", remind_enabled: true, digest_time: "08:00",
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
const cart = new Set<string>(["sua-tuoi", "nuoc-rua-chen", "trung-ga"]);
/** Chi tiết dòng Kế hoạch (số lượng, ngày cần, ưu tiên); vắng: mặc định như backend. */
const planLines = new Map<string, PlanLinePatch & { source?: CartItem["source"]; bought_qty?: number }>();
let planState: PlanState = { buy_date: "", buy_hour: null, trip_note: "" };

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
  const lines = [...cart].map((k) => items.find((i) => i.key === k)).filter((i): i is Item => !!i);
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
        ...planLines.get(i.key),
      } satisfies CartItem;
    }),
    item_count: lines.length,
    estimate,
    missing_price: pr.missing_price.length,
    state: planState,
  };
}

export async function mockFetch(path: string, init: RequestInit): Promise<Response> {
  await delay();
  const url = new URL(path, "http://mock");
  const method = (init.method ?? "GET").toUpperCase();
  const p = url.pathname;

  if (method === "GET" && p === "/me") {
    const me: Me = {
      uid: "mock-uid",
      display_name: mockSettings.display_name ?? (localStorage.getItem("mimi.pendingDisplayName") || "Lan"),
      role: "owner",
      receipt_count: receiptCount,
      settings: { lead_days: mockSettings.lead_days, lang: mockSettings.lang, timezone: mockSettings.timezone,
        remind_enabled: mockSettings.remind_enabled, digest_time: mockSettings.digest_time },
    };
    return json(me);
  }

  if (method === "GET" && p === "/home") {
    const summary: HomeSummary = {
      spent_this_month: 1_284_000,
      needs_count: items.filter((i) => isSoon(i, 7)).length,
      plan: (({ item_count, estimate, missing_price }) => ({ item_count, estimate, missing_price }))(mockCart()),
      unread_notifications: 2,
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
      return json({ ...it, history });
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
      const body = JSON.parse(String(init.body ?? "{}")) as { qty: number; unit: string };
      const [qty, unit] = toBase(body.qty, body.unit);
      if (!(qty >= 0)) return json({ code: "validation_failed", message: "qty" }, 422);
      const on = iso(today);
      items = items.map((x) =>
        x.key === key
          ? { ...x, stock: { estimated: qty, unit, confirmed: qty, confirmed_on: on, confirmed_source: "manual", rate_per_day: x.stock?.rate_per_day } }
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
    const line = (line_id: string, en: string, vi: string, qty: number, vnd: number, category_key?: string) => ({
      line_id, name: L ? en : vi, qty, amount: String(vnd), amount_vnd: vnd, habit: true,
      ...(category_key ? { category_key } : {}),
    });
    const lines = [
      line("l1", "Unsweetened fresh milk 1L", "Sữa tươi không đường 1L", 2, 64_000, "sua"),
      line("l2", "Dish soap 750ml", "Nước rửa chén 750ml", 1, 38_000, "hoa-pham"),
      line("l3", "Eggs, box of 10", "Trứng gà hộp 10", 1, 36_000, "tuoi-song"),
      line("l4", "Butter cookies", "Bánh quy bơ", 1, 74_000),
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
    const top = ["receipt_id", "store", "purchased_on", "currency", "discount_total", "lines", "use_for_prediction", "confirm_duplicate"];
    const lineKeys = ["line_id", "name", "qty", "unit", "amount", "category_key", "group_key", "habit"];
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
    }
    return json({ receipt_id: body.receipt_id, status: "saved", replayed }, replayed ? 200 : 201);
  }

  if (method === "PUT" && p === "/settings") {
    const body = JSON.parse(String(init.body ?? "{}")) as Partial<typeof mockSettings>;
    mockSettings = { ...mockSettings, ...body };
    return json({ ...mockSettings, display_name: mockSettings.display_name ?? "Lan" });
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
        { key: "hoa-pham", amount: 412_000, pct: 32.1 }, { key: "sua", amount: 318_000, pct: 24.8 },
        { key: "tuoi-song", amount: 264_000, pct: 20.6 }, { key: "do-kho", amount: 190_000, pct: 14.8 },
        { key: "chua-phan-loai", amount: 100_000, pct: 7.8 },
      ],
      top_saving_receipts: [r("r2", "Co.opmart Lý Thường Kiệt", 12, 486_000, 54_000), r("r1", "Bách Hóa Xanh", 4, 312_000, 42_000)],
      recent_receipts: [r("r3", "Bách Hóa Xanh", 15, 274_000, 0), r("r2", "Co.opmart Lý Thường Kiệt", 12, 486_000, 54_000), r("r1", "Bách Hóa Xanh", 4, 312_000, 42_000)],
    } satisfies SpendingReport);
  }

  if (method === "GET" && p === "/notifications") {
    const list: Notification[] = items
      .filter((i) => isSoon(i, 7))
      .sort(order)
      .map((i) => ({ id: `item:${i.key}`, kind: "item", item_key: i.key, name: i.name, status: i.status, days_left: i.days_left }));
    return json({ items: list });
  }

  if (method === "GET" && p === "/plan") return json(mockCart());
  if (method === "GET" && p === "/plan/prices") return json(mockPrices());
  if (method === "PUT" && p === "/plan/state") {
    planState = JSON.parse(String(init.body)) as PlanState;
    return json(planState);
  }
  const planItem = p.match(/^\/plan\/items\/([^/]+)$/);
  if (planItem) {
    const key = decodeURIComponent(planItem[1]);
    if (method === "PUT") {
      if (!items.some((i) => i.key === key)) return json({ code: "not_found", message: "Not found" }, 404);
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
