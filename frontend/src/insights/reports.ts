/**
 * Bảng AG Studio nhỏ mở từ card "Mimi nhận thấy": mỗi bảng 1 trang, một cột (vừa màn 320),
 * vài ô số, một biểu đồ và bảng chi tiết có nguồn. Dữ liệu là đúng các số card đã dùng.
 */
import type { AgDataSourcesDefinition, AgFieldDefinition, AgFormat, AgReportState, AgWidgetLayoutState } from "ag-studio";
import type { CartItem, PriceObservation } from "../api/types";
import { formatVnd } from "../lib/text";
import { displayUnit, type BudgetInsight, type OfferRow, type PriceInsight } from "./rules";
import { it } from "./text";

export type ReportSpec = { key: string; title: string; sub: string; note?: string; asOf?: string; data: AgDataSourcesDefinition; report: AgReportState; height: number };

const pos = (y: number, h: number, x = 0, w = 24): AgWidgetLayoutState => ({ xTrack: x, yTrack: y, xSpan: w, ySpan: h });
const FONT = '"Be Vietnam Pro", system-ui, sans-serif';
const typo = (fontSize: number, fontWeight: "normal" | "bold" = "normal") => ({ fontFamily: FONT, fontSize, fontWeight, fontStyle: "normal" as const });
const title = (text: string) => ({ title: { enabled: true, text } });
const money = (): Pick<AgFieldDefinition, "createValueFormatter"> => ({
  createValueFormatter: () => (v: unknown) => (typeof v === "number" ? formatVnd(Math.round(v)) : ""),
});
type F = [id: string, name: string, format: AgFormat, hide?: boolean];
const fields = (specs: F[]): AgFieldDefinition[] =>
  specs.map(([id, name, format, hide]) => ({ id, name, format, ...(format === "currencyFormat" ? money() : {}), ...(hide && { hide }) }));

/** Ô số: một bảng một dòng, mỗi ô là tổng của một cột (đúng bằng con số đó). */
const kpi = (text: string, id: string, caption = "") => ({
  type: "value" as const,
  dataMapping: { value: [{ id, aggregation: "sum" as const }] },
  format: {
    title: { enabled: true, text, typography: typo(13, "bold") },
    caption: { enabled: !!caption, text: caption, typography: typo(12) },
    style: { typography: typo(17, "bold") }, // ô 125px ở màn 393: 20px thì "15.000.000đ" bị thay bằng "…"
  },
});

function report(id: string, widgets: Record<string, unknown>, layout: Record<string, AgWidgetLayoutState>): AgReportState {
  return {
    selectedPageId: id,
    pages: [{ id, widgets: widgets as never, widgetLayout: layout, layout: { minWidth: 280 } } as never],
  } as AgReportState;
}

const KPI_H = 7;
/** Hai ô số cạnh nhau; màn hẹp (320–379) thì xếp chồng, vì ô nửa màn chỉ ~90px, số tiền bị thay bằng "…". */
const kpiLayout = (): { a: AgWidgetLayoutState; b: AgWidgetLayoutState; h: number } =>
  typeof window !== "undefined" && window.innerWidth < 380
    ? { a: pos(0, KPI_H), b: pos(KPI_H, KPI_H), h: KPI_H * 2 }
    : { a: pos(0, KPI_H, 0, 12), b: pos(0, KPI_H, 12, 12), h: KPI_H };
const GRID_STYLE = { grandTotalRow: { enabled: false }, columnAutoSize: true };
/** Bảng chi tiết cao vừa số dòng (mỗi dòng ~2,4 ô lưới), tối đa 8 dòng rồi cuộn trong bảng; head: tiêu đề + hàng tên cột. */
const gridH = (rows: number, head = 5) => head + Math.ceil(Math.min(Math.max(rows, 1), 8) * 2.4);

// ---------- Giá một món (product_price_history) ----------

export function priceReport(ins: PriceInsight, history: PriceObservation[], offers: OfferRow[]): ReportSpec {
  const d = displayUnit(ins.lastPaid.canonical_unit);
  const per = (unitPrice: number) => Math.round(unitPrice * d.f);
  const u = it.unit(d.label);
  const paid = [...history]
    .filter((o) => o.canonical_unit === ins.lastPaid.canonical_unit && o.unit_price > 0)
    .sort((a, b) => a.purchased_at.localeCompare(b.purchased_at))
    .map((o, i) => ({ id: `p${i}`, date: o.purchased_at.slice(0, 10), store: o.store_name, price: o.paid_price, unitPrice: per(o.unit_price) }));
  const usable = offers.filter((o) => o.usable).sort((a, b) => a.unitPrice! - b.unitPrice!);
  const rows = [
    ...paid.map((p) => ({ id: p.id, type: it.price.paid, date: p.date, store: p.store, size: "", price: p.price, unitPrice: p.unitPrice, source: it.price.receipt })),
    ...usable.map((o, i) => ({ id: `o${i}`, type: it.price.offer, date: o.on.slice(0, 10), store: o.store, size: o.size ?? "", price: o.price_vnd, unitPrice: per(o.unitPrice!), source: o.source === "gemini_search" ? "Gemini" : o.source.replace(/^partner_/, "") })),
  ];
  const kpis = [{ id: "k", last: ins.last, now: ins.current, diff: ins.current - ins.last, pct: ins.pct }];
  const data: AgDataSourcesDefinition = {
    description: `Price history of one household item (${ins.name}). Paid prices come from the family's own receipts; current prices are reference prices from store pages. Unit price is per ${d.label}. Money is in VND.`,
    sources: [
      { id: "k", name: "KPI", data: kpis, fields: fields([["id", "id", "textFormat", true], ["last", `${it.price.kLast} /${u}`, "currencyFormat"], ["now", `${it.price.kNow} /${u}`, "currencyFormat"], ["diff", `${it.price.kDiff} /${u}`, "currencyFormat"], ["pct", it.price.kPct, "integerFormat"]]) },
      { id: "paid", name: it.price.paidLine, data: paid, fields: fields([["id", "id", "textFormat", true], ["date", it.f.date, "dateFormat"], ["store", it.f.store, "textFormat"], ["price", it.f.price, "currencyFormat"], ["unitPrice", `${it.f.unitPrice} /${u}`, "currencyFormat"]]) },
      { id: "offers", name: it.price.offers, data: usable.map((o, i) => ({ id: `o${i}`, store: o.store, unitPrice: per(o.unitPrice!) })), fields: fields([["id", "id", "textFormat", true], ["store", it.f.store, "textFormat"], ["unitPrice", `${it.f.unitPrice} /${u}`, "currencyFormat"]]) },
      { id: "rows", name: it.price.grid, data: rows, fields: fields([["id", "id", "textFormat", true], ["type", it.f.type, "textFormat"], ["date", it.f.date, "dateFormat"], ["store", it.f.store, "textFormat"], ["size", it.f.size, "textFormat"], ["price", it.f.price, "currencyFormat"], ["unitPrice", `${it.f.unitPrice} /${u}`, "currencyFormat"], ["source", it.f.source, "textFormat"]]) },
    ],
  };
  const widgets = {
    last: kpi(it.price.kLast, "k.last", it.price.per(u)),
    now: kpi(it.price.kNow, "k.now", it.price.per(u)),
    // Đường chỉ nối giá đã trả; giá đang bán là cột riêng bên dưới, không nối vào đường như một lần mua thật.
    paid: { type: "line-chart", dataMapping: { categoryKey: [{ id: "paid.date" }], valueKey: [{ id: "paid.unitPrice", aggregation: "avg" }] }, format: { ...title(it.price.paidLine), style: { theme: { common: { legend: { enabled: false } } } } } },
    offers: { type: "bar-chart-grouped", dataMapping: { categoryKey: [{ id: "offers.store" }], valueKey: [{ id: "offers.unitPrice", aggregation: "min" }] }, sort: [{ field: { id: "offers.unitPrice", aggregation: "min" }, direction: "asc" }], format: { ...title(it.price.offers), style: { theme: { common: { legend: { enabled: false } } } } } },
    // Màn hẹp chỉ thấy 3 cột đầu: Loại, Đơn giá, Ngày; nơi bán, quy cách, nguồn vuốt ngang. Không dòng tổng (cộng đơn giá vô nghĩa).
    grid: { type: "grid", dataMapping: { cols: [{ id: "rows.type" }, { id: "rows.unitPrice" }, { id: "rows.date" }, { id: "rows.store" }, { id: "rows.size" }, { id: "rows.source" }] }, sort: [{ field: { id: "rows.date" }, direction: "desc" }], format: { ...title(it.price.grid), style: { ...GRID_STYLE, headerRow: { wrapText: true } } } },
  };
  const gh = gridH(rows.length, 7);
  const kp = kpiLayout();
  const layout = { last: kp.a, now: kp.b, paid: pos(kp.h, 18), offers: pos(kp.h + 18, 14), grid: pos(kp.h + 32, gh) };
  return { key: ins.id, title: it.price.title(ins.name), sub: it.price.sheetSub, note: it.price.note, asOf: ins.asOf, data, report: report("price", widgets, layout), height: (kp.h + 32 + gh) * 19 };
}

// ---------- Ngân sách tháng (plan_budget_breakdown) ----------

export function budgetReport(ins: BudgetInsight, items: CartItem[]): ReportSpec {
  const parts = [
    { id: "a", part: it.budget.kSpent, amount: ins.spent },
    { id: "b", part: it.budget.kBills, amount: ins.bills },
    { id: "c", part: it.budget.kPlan, amount: ins.plan },
    ins.over ? { id: "d", part: it.budget.kOver, amount: -ins.left } : { id: "d", part: it.budget.kLeft, amount: ins.left },
  ];
  const lines = items
    .filter((i) => i.unit_price != null)
    .map((i) => ({ id: i.key, name: `${i.name} × ${i.qty}`, qty: i.qty, unitPrice: i.unit_price!, total: i.unit_price! * i.qty, store: i.best_store ?? "" }));
  const k = [{ id: "k", budget: ins.budget, plan: ins.plan, left: Math.abs(ins.left) }];
  const data: AgDataSourcesDefinition = {
    description: "This month's household budget versus money already spent, recurring bills still due this month, and the shopping plan estimate. Money is in VND.",
    sources: [
      { id: "k", name: "KPI", data: k, fields: fields([["id", "id", "textFormat", true], ["budget", it.budget.kBudget, "currencyFormat"], ["plan", it.budget.kPlan, "currencyFormat"], ["left", ins.over ? it.budget.kOver : it.budget.kLeft, "currencyFormat"]]) },
      { id: "parts", name: it.budget.parts, data: parts, fields: fields([["id", "id", "textFormat", true], ["part", it.f.part, "textFormat"], ["amount", it.f.amount, "currencyFormat"]]) },
      { id: "lines", name: it.budget.lines, data: lines, fields: fields([["id", "id", "textFormat", true], ["name", it.f.name, "textFormat"], ["qty", it.f.qty, "integerFormat"], ["unitPrice", it.f.price, "currencyFormat"], ["total", it.f.lineTotal, "currencyFormat"], ["store", it.f.store, "textFormat"]]) },
    ],
  };
  const widgets = {
    kBudget: kpi(it.budget.kBudget, "k.budget"),
    kLeft: kpi(ins.over ? it.budget.kOver : it.budget.kLeft, "k.left"),
    parts: { type: "bar-chart-grouped", dataMapping: { categoryKey: [{ id: "parts.part" }], valueKey: [{ id: "parts.amount", aggregation: "sum" }] }, format: { ...title(it.budget.parts), style: { theme: { common: { legend: { enabled: false } } } } } },
    lines: { type: "grid", dataMapping: { cols: [{ id: "lines.name" }, { id: "lines.total" }] }, sort: [{ field: { id: "lines.total" }, direction: "desc" }], format: { ...title(it.budget.lines), style: GRID_STYLE } },
  };
  const gh = gridH(lines.length);
  const kp = kpiLayout();
  const layout = { kBudget: kp.a, kLeft: kp.b, parts: pos(kp.h, 16), lines: pos(kp.h + 16, gh) };
  return { key: ins.id, title: it.budget.title, sub: it.budget.sheetSub, note: it.budget.note, data, report: report("budget", widgets, layout), height: (kp.h + 16 + gh) * 19 };
}
