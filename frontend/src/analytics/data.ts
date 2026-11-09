/**
 * Dữ liệu cho màn Phân tích chi tiêu: gom các API sẵn có (lịch sử hóa đơn, chi tiết từng hóa đơn,
 * đồ dùng, bảng giá đang bán của Kế hoạch) thành bốn bảng quan hệ cho AG Studio.
 * Client chỉ sắp xếp và nối bảng; mọi số tiền vẫn là số API trả về (quy ước của frontend).
 */
import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import type { AgDataSourcesDefinition, AgFieldDefinition, AgFormat, AgRelationDefinition } from "ag-studio";
import { apiGet } from "../api/client";
import { qk, useItems, useMarketPrices, usePlan, useReceiptHistory, useRecurring } from "../api/hooks";
import type { Cart, HistoryReceipt, Item, MarketPrices, ReceiptDetail, Recurring } from "../api/types";
import { categoryLabel } from "../i18n";
import { currentMonth, formatVnd } from "../lib/text";
import { at } from "./text";

/** Số hóa đơn gần nhất được tải chi tiết (mỗi hóa đơn một lời gọi API). */
export const DETAIL_LIMIT = 60;

export type ReceiptRow = {
  id: string; store: string; date: string; month: string; kind: string; buyer: string;
  total: number; discount: number; itemCount: number;
  /** Bằng total nếu hóa đơn thuộc tháng hiện tại, ngược lại 0 (thẻ "Tháng này" trên điện thoại). */
  thisMonth: number;
};
export type LineRow = {
  id: string; receiptId: string; name: string; category: string; qty: number;
  unitPrice: number | null; amount: number | null; discount: number; priceSource: string;
};
export type ItemRow = {
  key: string; name: string; category: string; daysLeft: number | null; outOn: string | null;
  status: string; statusKey: string; purchases: number; interval: number | null;
};
export type PriceRow = {
  id: string; key: string; name: string; store: string; source: string; onlinePrice: number;
  ownPrice: number | null; ownStore: string; saving: number; cheapest: boolean;
  /** 1 ở dòng rẻ nhất của món khi giá đó rẻ hơn giá nhà mình: cộng lại ra số món có giá rẻ hơn. */
  cheaperItem: number;
  currency: string; local: number | null; size: string; on: string; url: string;
};
/** Khoản định kỳ (điện, nước, internet…) sắp đến hạn, từ màn Hóa đơn định kỳ. */
export type UpcomingRow = { id: string; name: string; due: string; amount: number; category: string };
export type AnalyticsRows = { receipts: ReceiptRow[]; lines: LineRow[]; items: ItemRow[]; prices: PriceRow[]; upcoming: UpcomingRow[]; asOf: string };

const pad = (n: number) => String(n).padStart(2, "0");
/** Ngày mua theo giờ Việt Nam (như màn Lịch sử mua hàng). */
export function vnDay(iso: string) {
  const d = new Date(new Date(iso).getTime() + 7 * 3_600_000);
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
}

function receiptRow(h: HistoryReceipt, d: ReceiptDetail | undefined): ReceiptRow {
  const day = vnDay(h.purchased_at);
  const lineDiscount = (d?.lines ?? []).reduce((a, l) => a + (l.discount ?? 0), 0);
  return {
    id: h.receipt_id,
    store: h.store || "—",
    date: day,
    month: day.slice(0, 7),
    kind: at.kinds[h.kind ?? "own"],
    buyer: h.buyer_name ?? "",
    total: h.total_vnd,
    thisMonth: day.slice(0, 7) === currentMonth() ? h.total_vnd : 0,
    discount: (d?.discount ?? 0) + lineDiscount,
    itemCount: d ? d.lines.length : h.item_names.length,
  };
}

function lineRows(h: HistoryReceipt, d: ReceiptDetail | undefined): LineRow[] {
  // Khoản định kỳ không có hóa đơn gốc: một dòng duy nhất thuộc nhóm "Khoản chi định kỳ".
  if (!d) {
    return h.kind === "recurring"
      ? [{ id: `${h.receipt_id}#0`, receiptId: h.receipt_id, name: h.store, category: categoryLabel("hoa-don-dinh-ky"), qty: 1, unitPrice: h.total_vnd, amount: h.total_vnd, discount: 0, priceSource: at.price.receipt }]
      : [];
  }
  return d.lines.map((l, i) => ({
    id: `${h.receipt_id}#${i}`,
    receiptId: h.receipt_id,
    name: l.name,
    category: categoryLabel(l.category_key),
    qty: l.qty,
    unitPrice: l.unit_price,
    amount: l.amount,
    discount: l.discount ?? 0,
    priceSource: at.price[l.price_source ?? "receipt"],
  }));
}

function statusKey(i: Item): keyof typeof at.status {
  if (i.days_left == null) return "unknown";
  if (i.days_left <= 0) return "out";
  if (i.days_left <= 7) return "soon";
  if (i.days_left <= 14) return "upcoming";
  return "plenty";
}

function itemRow(i: Item): ItemRow {
  const s = statusKey(i);
  return {
    key: i.key, name: i.name, category: i.category_label || categoryLabel(i.category_key),
    daysLeft: i.days_left, outOn: i.predicted_out_on ?? null, status: at.status[s], statusKey: s,
    purchases: i.purchase_count, interval: i.interval_days ?? null,
  };
}

/** Nguồn giá: Channel3 (danh mục sản phẩm nhiều cửa hàng), tra trên mạng (Gemini), hoặc đối tác. */
function sourceLabel(src: string) {
  if (src.startsWith("channel3")) return "Channel3";
  if (src === "gemini_search") return at.sources.web;
  if (src.startsWith("partner")) return at.sources.partner;
  return src;
}

/** Bảng giá đang bán (Channel3 / tra cứu công khai) đặt cạnh giá nhà mình đã trả trong Kế hoạch. */
export function priceRows(mp: MarketPrices | undefined, cart: Cart | undefined): PriceRow[] {
  const own = new Map((cart?.items ?? []).map((c) => [c.key, c]));
  return (mp?.items ?? []).flatMap((it) => {
    const mine = own.get(it.key);
    const ownPrice = mine?.unit_price ?? null;
    const min = Math.min(...it.prices.map((p) => p.price_vnd));
    const firstMin = it.prices.findIndex((p) => p.price_vnd === min);
    return it.prices.map((p, n) => ({
      id: `${it.key}#${n}`,
      key: it.key,
      name: it.name,
      store: p.store,
      source: sourceLabel(p.source),
      onlinePrice: p.price_vnd,
      ownPrice,
      ownStore: mine?.best_store ?? "",
      saving: ownPrice != null ? Math.max(0, ownPrice - p.price_vnd) : 0,
      cheapest: p.price_vnd === min,
      cheaperItem: ownPrice != null && n === firstMin && p.price_vnd < ownPrice ? 1 : 0,
      currency: p.currency ?? "VND",
      local: p.local ?? null,
      size: p.size ?? "",
      on: p.on,
      url: p.url ?? "",
    }));
  });
}

/** Kỳ chưa trả của các khoản định kỳ đang bật, đến hạn trong 30 ngày tới (kể cả quá hạn). */
export function upcomingRows(recurring: Recurring[], now = new Date()): UpcomingRow[] {
  const until = vnDay(new Date(now.getTime() + 30 * 86_400_000).toISOString());
  return recurring
    .filter((r) => r.active && r.display !== "paused" && r.next_due_at)
    .map((r) => ({ id: r.current?.id ?? r.id, name: r.name, due: vnDay(r.current?.due_at ?? r.next_due_at), amount: r.current?.amount ?? r.amount, category: r.category_key ? categoryLabel(r.category_key) : "" }))
    .filter((r) => r.due <= until)
    .sort((a, b) => a.due.localeCompare(b.due));
}

export function buildRows(history: HistoryReceipt[], details: Map<string, ReceiptDetail>, items: Item[], mp?: MarketPrices, cart?: Cart, recurring: Recurring[] = []): AnalyticsRows {
  const receipts = history.map((h) => receiptRow(h, details.get(h.receipt_id)));
  const lines = history.flatMap((h) => lineRows(h, details.get(h.receipt_id)));
  return { receipts, lines, items: items.map(itemRow), prices: priceRows(mp, cart), upcoming: upcomingRows(recurring), asOf: new Date().toISOString() };
}

/** Tải mọi thứ màn cần. Chi tiết hóa đơn dùng chung cache với màn Chi tiết hóa đơn. */
export function useAnalyticsData() {
  const history = useReceiptHistory();
  const items = useItems({ tab: "active", horizon: 365 });
  const cart = usePlan();
  const market = useMarketPrices(true);
  const recurring = useRecurring();
  const recent = useMemo(
    () => [...(history.data ?? [])].filter((h) => h.kind !== "recurring").sort((a, b) => b.purchased_at.localeCompare(a.purchased_at)).slice(0, DETAIL_LIMIT),
    [history.data],
  );
  const details = useQueries({
    queries: recent.map((h) => ({
      queryKey: qk.receipt(h.receipt_id),
      queryFn: () => apiGet<ReceiptDetail>(`/receipts/${encodeURIComponent(h.receipt_id)}`),
      staleTime: 10 * 60_000,
    })),
  });
  const loadedDetails = details.filter((d) => d.data).length;
  const detailsDone = details.every((d) => !d.isPending);
  const rows = useMemo(() => {
    if (!history.data || !detailsDone) return undefined;
    const map = new Map<string, ReceiptDetail>();
    for (const d of details) if (d.data) map.set(d.data.receipt_id, d.data);
    return buildRows(history.data, map, items.data ?? [], market.data, cart.data, recurring.data?.items);
    // details đổi tham chiếu mỗi lần render; loadedDetails đủ để biết khi nào có thêm dữ liệu.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [history.data, detailsDone, loadedDetails, items.data, market.data, cart.data, recurring.data]);
  return {
    rows,
    isPending: history.isPending || !detailsDone,
    error: history.error,
    refetch: () => history.refetch(),
    analysed: loadedDetails,
    total: (history.data ?? []).filter((h) => h.kind !== "recurring").length,
  };
}

// ---------------- Khai báo nguồn dữ liệu cho AG Studio ----------------

type FieldSpec = [id: string, name: string, format: AgFormat, hide?: boolean];
/** Tiền luôn lưu bằng VND; hiện theo tiền tệ hiển thị của app (VND, hoặc USD ở Mỹ). */
const money = (): Pick<AgFieldDefinition, "createValueFormatter"> => ({
  createValueFormatter: () => (v: unknown) => (typeof v === "number" ? formatVnd(Math.round(v)) : ""),
});
const fields = (specs: FieldSpec[]): AgFieldDefinition[] =>
  specs.map(([id, name, format, hide]) => ({ id, name, format, ...(format === "currencyFormat" ? money() : {}), ...(hide && { hide }) }));

const manyToOne = (id: string, source: string, target: string): AgRelationDefinition => {
  const [st, sf] = source.split(".");
  const [tt, tf] = target.split(".");
  return { id, source: { tableId: st, fieldId: sf }, target: { tableId: tt, fieldId: tf }, type: "many-to-one" };
};

export const CALENDAR_ID = "calendar";

export function studioData(rows: AnalyticsRows): AgDataSourcesDefinition {
  const f = at.f;
  const dates = rows.receipts.map((r) => r.date).sort();
  const today = rows.asOf.slice(0, 10);
  return {
    description:
      "Household spending tracked by Mimi from scanned shop receipts. All money is in VND (Vietnamese dong) " +
      "and already shown in the app's display currency by the field formatters. Receipt lines carry the category of each item. " +
      "Household items carry Mimi's prediction of how many days are left before each item runs out. " +
      "Prices on sale come from public online price lookups (Channel3 product catalogue or web search) for the items in the shopping plan.",
    sources: [
      {
        id: "receipts",
        name: at.tables.receipts,
        description: "One row per receipt or paid recurring bill. Total is what was paid after discounts.",
        data: rows.receipts,
        fields: fields([
          ["id", f.receiptId, "textFormat", true],
          ["store", f.store, "textFormat"],
          ["date", f.date, "dateFormat"],
          ["month", f.month, "textFormat"],
          ["kind", f.kind, "textFormat"],
          ["buyer", f.buyer, "textFormat"],
          ["total", f.total, "currencyFormat"],
          ["thisMonth", f.thisMonth, "currencyFormat"],
          ["discount", f.discount, "currencyFormat"],
          ["itemCount", f.itemCount, "integerFormat"],
        ]),
      },
      {
        id: "lines",
        name: at.tables.lines,
        description: "One row per item line on a receipt. Amount is quantity times unit price; it is empty when the price is missing.",
        data: rows.lines,
        fields: fields([
          ["id", f.lineId, "textFormat", true],
          ["receiptId", f.receiptId, "textFormat", true],
          ["name", f.name, "textFormat"],
          ["category", f.category, "textFormat"],
          ["qty", f.qty, "decimalFormat"],
          ["unitPrice", f.unitPrice, "currencyFormat"],
          ["amount", f.amount, "currencyFormat"],
          ["discount", f.lineDiscount, "currencyFormat"],
          ["priceSource", f.priceSource, "textFormat"],
        ]),
      },
      {
        id: "items",
        name: at.tables.items,
        description: "Household consumables Mimi tracks. Days left is Mimi's prediction from the buying cycle; empty means not enough history yet.",
        data: rows.items,
        fields: fields([
          ["key", f.key, "textFormat", true],
          ["name", f.name, "textFormat"],
          ["category", f.category, "textFormat"],
          ["daysLeft", f.daysLeft, "integerFormat"],
          ["outOn", f.outOn, "dateFormat"],
          ["status", f.status, "textFormat"],
          ["statusKey", f.status, "textFormat", true],
          ["purchases", f.purchases, "integerFormat"],
          ["interval", f.interval, "integerFormat"],
        ]),
      },
      {
        id: "upcoming",
        name: at.tables.upcoming,
        description: "Unpaid recurring bills (electricity, water, internet, subscriptions…) due in the next 30 days, including overdue ones.",
        data: rows.upcoming,
        fields: fields([
          ["id", f.billId, "textFormat", true],
          ["name", f.bill, "textFormat"],
          ["due", f.due, "dateFormat"],
          ["amount", f.amount, "currencyFormat"],
          ["category", f.category, "textFormat"],
        ]),
      },
      {
        id: "prices",
        name: at.tables.prices,
        description:
          "Prices currently on sale at each store for the items in the shopping plan, next to the unit price the family last paid. " +
          "Cheaper by is how much less the online price is than our price (0 when it is not cheaper). Cheapest marks the lowest price for that item.",
        data: rows.prices,
        fields: fields([
          ["id", f.priceId, "textFormat", true],
          ["key", f.key, "textFormat", true],
          ["name", f.name, "textFormat"],
          ["store", f.store, "textFormat"],
          ["source", f.source, "textFormat"],
          ["onlinePrice", f.onlinePrice, "currencyFormat"],
          ["ownPrice", f.ownPrice, "currencyFormat"],
          ["ownStore", f.ownStore, "textFormat"],
          ["saving", f.saving, "currencyFormat"],
          ["cheapest", f.cheapest, "booleanFormat"],
          ["cheaperItem", at.r.cheaperCount, "integerFormat", true],
          ["currency", f.currency, "textFormat"],
          ["local", f.local, "decimalFormat"],
          ["size", f.size, "textFormat"],
          ["on", f.on, "dateFormat"],
          ["url", f.url, "textFormat", true],
        ]),
      },
    ],
    relationships: [
      manyToOne("line-receipt", "lines.receiptId", "receipts.id"),
      { id: "receipts-date-calendar", source: { tableId: "receipts", fieldId: "date" }, target: { calendarId: CALENDAR_ID } },
    ],
    calendars: [
      {
        id: CALENDAR_ID,
        label: "Calendar",
        range: { from: { type: "date", value: dates[0] ?? today }, to: { type: "date", value: dates.at(-1) ?? today } },
        fragments: ["year", "quarter", "month", "week", "day", "dayOfWeek"],
      },
    ],
    expressions: [
      {
        id: "avgReceipt",
        name: at.r.avgBill,
        description: "Average amount paid per receipt.",
        format: "currencyFormat",
        ...money(),
        isMeasure: true,
        expression: { operator: "divide", inputs: [{ id: "receipts.total", aggregation: "sum" }, { id: "receipts.id", aggregation: "countd" }] },
      },
    ],
  };
}
