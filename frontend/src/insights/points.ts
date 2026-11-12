/**
 * Luật và bảng phân tích của 16 điểm chạm còn lại (bảng điểm chạm 12/11). Mỗi hàm luật trả về null
 * khi không đủ dữ liệu hoặc chưa tới ngưỡng: không có card, không đoán.
 */
import type { CartItem, Forecast, HistoryReceipt, Item, ItemDetail, Order, PriceObservation, ReceiptSummary, Recurring, SpendingReport } from "../api/types";
import { categoryLabel } from "../i18n";
import { formatVnd } from "../lib/text";
import { mini } from "./kit";
import type { Severity } from "./rules";
import type { ReportSpec } from "./reports";
import { pt } from "./ptext";

export type Touchpoint = { id: string; severity: Severity; text: string; sub?: string; report: () => ReportSpec };

const REC = "hoa-don-dinh-ky";
const DAY = 86_400_000;
const pctOf = (a: number, b: number) => Math.round(((a - b) / b) * 100);
const dayOf = (iso: string) => iso.slice(0, 10);
const daysFromToday = (iso: string, now = new Date()) => Math.round((new Date(`${dayOf(iso)}T12:00:00`).getTime() - now.getTime()) / DAY);
const sum = (xs: number[]) => xs.reduce((a, b) => a + b, 0);

// ---------- 1. Sổ chi tiêu · Xu hướng ----------
export function trendReport(r: SpendingReport, prev: SpendingReport, key: string): ReportSpec {
  const before = new Map(prev.by_category.map((c) => [c.key, c.amount]));
  const keys = [...new Set([...r.by_category.map((c) => c.key), ...prev.by_category.map((c) => c.key)])].filter((k) => k !== REC);
  const now = new Map(r.by_category.map((c) => [c.key, c.amount]));
  const rows = keys.map((k) => ({ k, cat: categoryLabel(k), a: now.get(k) ?? 0, b: before.get(k) ?? 0 }));
  const cat = categoryLabel(key);
  return mini({
    key: `trend:${key}`, title: pt.trend.title(cat), sub: pt.trend.sub, note: pt.trend.note,
    about: `Household spending by category, this month versus last month. Highlighted category: ${cat}. Money in VND.`,
    kpis: [{ label: pt.thisMonth, value: now.get(key) ?? 0 }, { label: pt.lastMonth, value: before.get(key) ?? 0 }],
    chart: { kind: "bar", title: pt.trend.chart, cat: pt.f.cat, series: [{ name: pt.thisMonth }, { name: pt.lastMonth }], rows: rows.map((x) => ({ cat: x.cat, v: [x.a, x.b] })) },
    grid: { title: pt.trend.grid, cols: [{ name: pt.f.cat, format: "textFormat" }, { name: pt.thisMonth, format: "currencyFormat" }, { name: pt.lastMonth, format: "currencyFormat" }, { name: pt.diff, format: "currencyFormat" }], rows: rows.map((x) => [x.cat, x.a, x.b, x.a - x.b]), sortBy: 1 },
  });
}

// ---------- 2. Sổ chi tiêu · Cửa hàng ----------
export function storeReport(r: SpendingReport, store: string): ReportSpec {
  const by = new Map<string, number>();
  for (const x of r.recent_receipts) if (!x.delegated && x.store) by.set(x.store, (by.get(x.store) ?? 0) + x.total_vnd);
  const total = by.get(store) ?? 0;
  const mine = r.recent_receipts.filter((x) => x.store === store && !x.delegated);
  return mini({
    key: `store:${store}`, title: pt.store.title(store), sub: pt.store.sub,
    about: `This month's household spending by store; receipts at ${store}. Money in VND.`,
    kpis: [{ label: pt.store.kTotal, value: total }, { label: pt.store.kPct, value: r.spent > 0 ? Math.round((total / r.spent) * 100) : 0, money: false, caption: pt.pctUnit }],
    chart: { kind: "bar", title: pt.store.chart, cat: pt.f.store, series: [{ name: pt.f.amount }], rows: [...by].map(([cat, v]) => ({ cat, v: [v] })) },
    grid: { title: pt.store.grid(store), cols: [{ name: pt.f.date, format: "dateFormat" }, { name: pt.f.amount, format: "currencyFormat" }], rows: mine.map((x) => [x.purchased_on, x.total_vnd]), sortBy: 0 },
  });
}

// ---------- 3. Sổ chi tiêu · Định kỳ sắp đến hạn ----------
export function billReport(bill: Recurring, fc: Forecast | undefined): ReportSpec {
  const past = [...bill.history].filter((o) => o.status !== "skipped").sort((a, b) => a.due_at.localeCompare(b.due_at));
  const soon = (fc?.recurring ?? []).filter((o) => daysFromToday(o.due_at) <= 7);
  return mini({
    key: `bill:${bill.id}`, title: pt.bill.title(bill.name), sub: pt.bill.sub,
    about: `Recurring bill ${bill.name}: upcoming amount, past payments and all bills due in the next 7 days. Money in VND.`,
    kpis: [{ label: pt.bill.kAmount, value: bill.amount }, { label: pt.bill.kDays, value: Math.max(0, daysFromToday(bill.next_due_at)), money: false, caption: pt.days }],
    chart: past.length ? { kind: "column", title: pt.bill.chart, cat: pt.f.date, catFormat: "dateFormat", series: [{ name: pt.f.amount }], rows: past.map((o) => ({ cat: dayOf(o.due_at), v: [o.amount] })), keepOrder: true } : undefined,
    grid: { title: pt.bill.grid, cols: [{ name: pt.f.item, format: "textFormat" }, { name: pt.f.date, format: "dateFormat" }, { name: pt.f.amount, format: "currencyFormat" }], rows: soon.map((o) => [o.name, dayOf(o.due_at), o.amount]), sortBy: 1, sortDesc: false },
  });
}

// ---------- 4. Sổ chi tiêu · Món sắp hết ----------
export function lowReport(items: Item[], low: Item): ReportSpec {
  const watch = items.filter((i) => i.days_left != null && i.days_left <= 14).sort((a, b) => a.days_left! - b.days_left!).slice(0, 8);
  return mini({
    key: `low:${low.key}`, title: pt.low.title, sub: `${low.name} · ${pt.low.sub}`, note: pt.low.note,
    about: "Household items running low: estimated days of stock left from purchase history.",
    kpis: [
      { label: pt.low.kDays, value: Math.max(0, low.days_left ?? 0), money: false, caption: pt.days },
      ...(low.interval_days ? [{ label: pt.low.kCycle, value: Math.round(low.interval_days), money: false, caption: pt.days }] : []),
    ],
    chart: { kind: "bar", title: pt.low.chart, cat: pt.f.item, series: [{ name: pt.f.daysLeft, money: false }], rows: watch.map((i) => ({ cat: i.name, v: [Math.max(0, i.days_left!)] })), keepOrder: true },
    grid: { title: pt.low.grid, cols: [{ name: pt.f.item, format: "textFormat" }, { name: pt.f.daysLeft, format: "integerFormat" }, { name: pt.f.outOn, format: "dateFormat" }], rows: watch.map((i) => [i.name, Math.max(0, i.days_left!), i.predicted_out_on ?? null]), sortBy: 1, sortDesc: false },
  });
}

// ---------- 5. Sổ chi tiêu · Còn tiêu được ----------
function cumulative(rows: ReceiptSummary[], upTo: number) {
  const per = new Array(upTo + 1).fill(0);
  for (const x of rows) {
    const d = Number(x.purchased_on.slice(8, 10));
    if (d >= 1 && d <= upTo) per[d] += x.total_vnd;
  }
  for (let d = 1; d <= upTo; d++) per[d] += per[d - 1];
  return per;
}
export function safeReport(budget: number, r: SpendingReport, prev: SpendingReport | undefined, bills: Recurring[], day: number, days: number): ReportSpec {
  const billTotal = sum(bills.map((b) => b.amount));
  const free = budget - r.spent - billTotal;
  const a = cumulative(r.recent_receipts, day);
  const b = prev ? cumulative(prev.recent_receipts, Math.min(days, 31)) : null;
  return mini({
    key: "safe", title: pt.safe.title, sub: pt.safe.sub, note: pt.safe.note,
    about: "This month's household budget: spent so far, recurring bills still due, money left; cumulative spending by day versus last month. Money in VND.",
    kpis: [{ label: pt.safe.kBudget, value: budget }, { label: pt.safe.kSpent, value: r.spent }, { label: pt.safe.kBills, value: billTotal }, { label: free < 0 ? pt.safe.kOver : pt.safe.kFree, value: Math.abs(free) }],
    chart: { kind: "line", title: pt.safe.chart, cat: pt.f.day, series: b ? [{ name: pt.thisMonth }, { name: pt.lastMonth }] : [{ name: pt.thisMonth }], rows: Array.from({ length: day }, (_, i) => ({ cat: String(i + 1), v: b ? [a[i + 1], b[i + 1] ?? b[b.length - 1]] : [a[i + 1]] })), keepOrder: true },
    grid: bills.length ? { title: pt.safe.grid, cols: [{ name: pt.f.item, format: "textFormat" }, { name: pt.f.date, format: "dateFormat" }, { name: pt.f.amount, format: "currencyFormat" }], rows: bills.map((x) => [x.name, dayOf(x.next_due_at), x.amount]), sortBy: 1, sortDesc: false } : undefined,
  });
}

// ---------- 7. Chi tiết món · tần suất mua ----------
export function freqInsight(item: ItemDetail | undefined, now = new Date()): Touchpoint | null {
  if (!item) return null;
  const since = now.getTime() - 90 * DAY;
  const rows = item.history.filter((h) => new Date(`${dayOf(h.on)}T12:00:00`).getTime() >= since);
  if (rows.length < 3) return null;
  const total = sum(rows.map((h) => h.amount_vnd ?? 0));
  const days = item.interval_days ? Math.round(item.interval_days) : null;
  const months = new Map<string, number>();
  for (let i = 2; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    months.set(`${String(d.getMonth() + 1).padStart(2, "0")}/${d.getFullYear()}`, 0);
  }
  for (const h of rows) {
    const k = `${h.on.slice(5, 7)}/${h.on.slice(0, 4)}`;
    if (months.has(k)) months.set(k, months.get(k)! + (h.amount_vnd ?? 0));
  }
  return {
    id: `fq:${item.key}:${rows.length}:${rows[0]?.on}`,
    severity: "info",
    text: pt.freq.text(item.name, rows.length, days),
    sub: total > 0 ? pt.freq.sub(formatVnd(total)) : undefined,
    report: () => mini({
      key: `fq:${item.key}`, title: pt.freq.title(item.name), sub: pt.freq.sub2,
      about: `Purchases of ${item.name} in the last 3 months: count, money per month, each purchase. Money in VND.`,
      kpis: [{ label: pt.freq.kCount, value: rows.length, money: false, caption: pt.times }, { label: pt.freq.kTotal, value: total }],
      chart: { kind: "column", title: pt.freq.chart, cat: pt.f.month, series: [{ name: pt.f.amount }], rows: [...months].map(([cat, v]) => ({ cat, v: [v] })), keepOrder: true },
      grid: { title: pt.freq.grid, cols: [{ name: pt.f.date, format: "dateFormat" }, { name: pt.f.store, format: "textFormat" }, { name: pt.f.amount, format: "currencyFormat" }], rows: rows.map((h) => [dayOf(h.on), h.store ?? "", h.amount_vnd]), sortBy: 0 },
    }),
  };
}

// ---------- 8. Chi tiết khoản định kỳ ----------
export function riseInsight(r: Recurring | undefined): Touchpoint | null {
  if (!r) return null;
  const paid = r.history.filter((o) => o.status !== "skipped" && o.status !== "pending").sort((a, b) => b.due_at.localeCompare(a.due_at)).slice(0, 3);
  if (paid.length < 2) return null;
  const cur = r.current?.amount ?? r.amount;
  const avg = sum(paid.map((o) => o.amount)) / paid.length;
  const pct = pctOf(cur, avg);
  if (pct < 20) return null;
  const all = [...r.history.filter((o) => o.status !== "skipped")].sort((a, b) => a.due_at.localeCompare(b.due_at)).slice(-11);
  const rows = [...all.map((o) => ({ d: dayOf(o.due_at), v: o.amount, s: pt.rise.paid })), { d: dayOf(r.current?.due_at ?? r.next_due_at), v: cur, s: pt.rise.due }];
  return {
    id: `rs:${r.id}:${cur}:${Math.round(avg)}`,
    severity: pct >= 40 ? "warning" : "info",
    text: pt.rise.text(r.name, pct),
    sub: pt.rise.sub(formatVnd(cur), formatVnd(Math.round(avg))),
    report: () => mini({
      key: `rs:${r.id}`, title: pt.rise.title(r.name), sub: pt.rise.sub2,
      about: `Recurring bill ${r.name}: this payment versus the average of up to 3 past payments; up to 12 payments. Money in VND.`,
      kpis: [{ label: pt.rise.kCur, value: cur }, { label: pt.rise.kAvg, value: Math.round(avg) }],
      chart: { kind: "column", title: pt.rise.chart, cat: pt.f.date, catFormat: "dateFormat", series: [{ name: pt.f.amount }], rows: rows.map((x) => ({ cat: x.d, v: [x.v] })), keepOrder: true },
      grid: { title: pt.rise.grid, cols: [{ name: pt.f.date, format: "dateFormat" }, { name: pt.f.amount, format: "currencyFormat" }, { name: pt.f.status, format: "textFormat" }], rows: rows.map((x) => [x.d, x.v, x.s]), sortBy: 0 },
    }),
  };
}

// ---------- 10 & 11. Kế hoạch bước 2 ----------
export type PlanRow = { item: CartItem; last: number | null; cheapest: number | null; picked: number | null; pickedStore?: string };

export function cmpInsight(rows: PlanRow[]): Touchpoint | null {
  const up = rows
    .filter((r) => r.last != null && r.last > 0 && r.cheapest != null)
    .map((r) => ({ r, pct: pctOf(r.cheapest!, r.last!) }))
    .filter((x) => x.pct >= 10)
    .sort((a, b) => b.pct - a.pct)[0];
  if (!up) return null;
  const known = rows.filter((r) => r.last != null && r.cheapest != null);
  return {
    id: `cp:${up.r.item.key}:${up.r.last}:${up.r.cheapest}`,
    severity: up.pct >= 25 ? "warning" : "info",
    text: pt.cmp.text(up.r.item.name, up.pct),
    sub: pt.cmp.sub(formatVnd(up.r.cheapest!), formatVnd(up.r.last!)),
    report: () => mini({
      key: "cmp", title: pt.cmp.title, sub: pt.cmp.sub2, note: pt.cmp.note,
      about: "Shopping plan items: total at the last price the family paid versus total at the cheapest current offer. Money in VND.",
      kpis: [{ label: pt.cmp.kLast, value: sum(known.map((r) => r.last!)) }, { label: pt.cmp.kNow, value: sum(known.map((r) => r.cheapest!)) }],
      chart: { kind: "bar", title: pt.cmp.chart, cat: pt.f.item, series: [{ name: pt.cmp.sLast }, { name: pt.cmp.sNow }], rows: known.map((r) => ({ cat: r.item.name, v: [r.last!, r.cheapest!] })) },
      grid: { title: pt.cmp.grid, cols: [{ name: pt.f.item, format: "textFormat" }, { name: pt.cmp.sLast, format: "currencyFormat" }, { name: pt.cmp.sNow, format: "currencyFormat" }, { name: pt.f.pct, format: "integerFormat" }], rows: known.map((r) => [r.item.name, r.last, r.cheapest, pctOf(r.cheapest!, r.last!)]), sortBy: 3 },
    }),
  };
}

export function pickedInsight(rows: PlanRow[]): Touchpoint | null {
  const known = rows.filter((r) => r.last != null && r.picked != null);
  if (known.length === 0) return null;
  const last = sum(known.map((r) => r.last!));
  const picked = sum(known.map((r) => r.picked!));
  const diff = picked - last;
  if (Math.abs(diff) < 1000) return null;
  return {
    id: `pk:${known.map((r) => `${r.item.key}=${r.picked}`).join(",")}`,
    severity: "info",
    text: diff < 0 ? pt.picked.less(formatVnd(-diff)) : pt.picked.more(formatVnd(diff)),
    sub: pt.picked.sub(known.length),
    report: () => mini({
      key: "picked", title: pt.picked.title, sub: pt.picked.sub2,
      about: "Shopping plan: total at the prices the user picked versus the last prices the family paid, per item. Money in VND.",
      kpis: [{ label: pt.picked.kLast, value: last }, { label: pt.picked.kNow, value: picked }],
      chart: { kind: "bar", title: pt.picked.chart, cat: pt.f.item, series: [{ name: pt.picked.sDiff }], rows: known.map((r) => ({ cat: r.item.name, v: [r.picked! - r.last!] })) },
      grid: { title: pt.picked.grid, cols: [{ name: pt.f.item, format: "textFormat" }, { name: pt.cmp.sLast, format: "currencyFormat" }, { name: pt.picked.kNow, format: "currencyFormat" }, { name: pt.f.store, format: "textFormat" }], rows: known.map((r) => [r.item.name, r.last, r.picked, r.pickedStore ?? ""]) },
    }),
  };
}

// ---------- 13. Hoàn thành chuyến mua ----------
export type TripLine = { name: string; est: number | null; spent: number };
export function tripInsight(lines: TripLine[], estimate: number): Touchpoint | null {
  if (estimate <= 0 || lines.length === 0) return null;
  const spent = sum(lines.map((l) => l.spent));
  const extra = lines.filter((l) => l.est == null);
  // So đúng phần trong kế hoạch với dự kiến; món mua thêm nói riêng, không cộng vào để khỏi thành "vượt dự kiến".
  const inPlan = spent - sum(extra.map((l) => l.spent));
  return {
    id: `tp:${spent}:${estimate}`,
    severity: "info",
    text: pt.trip.text(formatVnd(inPlan), formatVnd(estimate)),
    sub: [pt.trip.whole(formatVnd(spent)), extra.length ? pt.trip.extra(extra.length, formatVnd(sum(extra.map((l) => l.spent)))) : ""].filter(Boolean).join(" · "),
    report: () => mini({
      key: "trip", title: pt.trip.title, sub: pt.trip.sub,
      about: "A finished shopping trip: plan estimate per item versus the amount on the receipt; items outside the plan have no estimate. Money in VND.",
      kpis: [{ label: pt.trip.kEst, value: estimate }, { label: pt.trip.kSpent, value: spent }],
      chart: { kind: "bar", title: pt.trip.chart, cat: pt.f.item, series: [{ name: pt.trip.sEst }, { name: pt.trip.sSpent }], rows: lines.map((l) => ({ cat: l.name, v: [l.est ?? 0, l.spent] })) },
      grid: { title: pt.trip.grid, cols: [{ name: pt.f.item, format: "textFormat" }, { name: pt.trip.sEst, format: "currencyFormat" }, { name: pt.trip.sSpent, format: "currencyFormat" }, { name: pt.f.status, format: "textFormat" }], rows: lines.map((l) => [l.name, l.est, l.spent, l.est == null ? pt.trip.outside : ""]) },
    }),
  };
}

// ---------- 14. Sau khi lưu hóa đơn ----------
export type BillLine = { name: string; unitPrice: number; unit: string; last: PriceObservation | null };
export function pricierInsight(lines: BillLine[]): Touchpoint | null {
  const up = lines
    .filter((l) => l.last && l.last.canonical_unit === l.unit && l.last.unit_price > 0)
    .map((l) => ({ l, pct: pctOf(l.unitPrice, l.last!.unit_price) }))
    .filter((x) => x.pct >= 5);
  if (up.length === 0) return null;
  const known = lines.filter((l) => l.last && l.last.canonical_unit === l.unit && l.last.unit_price > 0);
  const f = (u: string) => (u === "ml" || u === "g" ? 1000 : 1);
  return {
    id: `pr:${up.map((x) => `${x.l.name}:${x.l.unitPrice}`).join(",")}`,
    severity: "info",
    text: pt.bill14.pricier(up.length),
    sub: pt.bill14.pricierSub(up.map((x) => `${x.l.name} +${x.pct}%`).join(" · ")),
    report: () => mini({
      key: "pricier", title: pt.bill14.title, sub: pt.bill14.sub,
      about: "A just-saved receipt: unit price of each item versus the last time the family bought it (same unit). Money in VND per litre, kilogram or piece.",
      kpis: [{ label: pt.bill14.kUp, value: up.length, money: false, caption: `/ ${known.length}` }],
      chart: { kind: "bar", title: pt.bill14.chart, cat: pt.f.item, series: [{ name: pt.f.pct, money: false }], rows: known.map((l) => ({ cat: l.name, v: [pctOf(l.unitPrice, l.last!.unit_price)] })) },
      grid: { title: pt.bill14.grid, cols: [{ name: pt.f.item, format: "textFormat" }, { name: pt.bill14.sNow, format: "currencyFormat" }, { name: pt.bill14.sLast, format: "currencyFormat" }, { name: pt.f.store, format: "textFormat" }], rows: known.map((l) => [l.name, Math.round(l.unitPrice * f(l.unit)), Math.round(l.last!.unit_price * f(l.unit)), l.last!.store_name]) },
    }),
  };
}

export function budgetMarkInsight(budget: number | null, before: number | undefined, bill: number): Touchpoint | null {
  if (!budget || before == null || bill <= 0) return null;
  const after = before + bill;
  const a = (before / budget) * 100, b = (after / budget) * 100;
  const mark = [100, 80].find((m) => a < m && b >= m);
  if (!mark) return null;
  const pct = Math.floor(b);
  const left = budget - after;
  return {
    id: `bm:${budget}:${Math.round(before)}:${bill}`,
    severity: mark === 100 ? "warning" : "info",
    text: pt.bill14.budget(pct),
    sub: pt.bill14.budgetSub(formatVnd(after), formatVnd(budget)),
    report: () => mini({
      key: "bmark", title: pt.bill14.bTitle, sub: pt.bill14.bSub,
      about: "This month's household budget before and after the receipt just saved. Money in VND.",
      kpis: [{ label: pt.bill14.kBefore, value: Math.round(a), money: false, caption: pt.pctUnit }, { label: pt.bill14.kAfter, value: Math.round(b), money: false, caption: pt.pctUnit }],
      chart: { kind: "bar", title: pt.bill14.bChart, cat: pt.f.cat, series: [{ name: pt.f.amount }], keepOrder: true, rows: [
        { cat: pt.bill14.parts.spent, v: [before] }, { cat: pt.bill14.parts.bill, v: [bill] }, { cat: left < 0 ? pt.bill14.parts.over : pt.bill14.parts.left, v: [Math.abs(left)] },
      ] },
    }),
  };
}

// ---------- 15. Đơn online · hoàn tiền ----------
export function refundInsight(o: Order | undefined): Touchpoint | null {
  if (!o || !(o.refunded_vnd > 0)) return null;
  return {
    id: `rf:${o.id}:${o.refunded_vnd}`,
    severity: "info",
    text: pt.refund.text(formatVnd(o.refunded_vnd)),
    sub: pt.refund.sub(formatVnd(o.net_vnd)),
    report: () => mini({
      key: `rf:${o.id}`, title: pt.refund.title, sub: pt.refund.sub2,
      about: "An online order: amount paid, refunds and the net amount counted in Spending. Money in VND.",
      kpis: [{ label: pt.refund.kPaid, value: o.total_vnd }, { label: pt.refund.kNet, value: o.net_vnd }],
      chart: { kind: "bar", title: pt.refund.chart, cat: pt.f.cat, series: [{ name: pt.f.amount }], keepOrder: true, rows: [
        { cat: pt.refund.kPaid, v: [o.total_vnd] }, { cat: pt.refund.kRefund, v: [o.refunded_vnd] }, { cat: pt.refund.kNet, v: [o.net_vnd] },
      ] },
      grid: { title: pt.refund.grid, cols: [{ name: pt.f.date, format: "dateFormat" }, { name: pt.f.reason, format: "textFormat" }, { name: pt.f.amount, format: "currencyFormat" }], rows: o.refunds.map((r) => [dayOf(r.at), pt.refund.reasons[r.reason] ?? r.reason, r.amount_vnd]) },
    }),
  };
}

// ---------- 16. Đặt ngân sách tháng ----------
export function avgInsight(months: { month: string; r?: SpendingReport }[], recurringMonthly: number | undefined): Touchpoint | null {
  const have = months.filter((m) => m.r && m.r.spent > 0);
  if (have.length < 2) return null;
  const avg = Math.round(sum(have.map((m) => m.r!.spent)) / have.length / 1000) * 1000;
  const label = (m: string) => `${m.slice(5, 7)}/${m.slice(0, 4)}`;
  return {
    id: `av:${have.map((m) => `${m.month}=${m.r!.spent}`).join(",")}`,
    severity: "info",
    text: pt.avg.text(formatVnd(avg)),
    sub: recurringMonthly ? pt.avg.sub(formatVnd(recurringMonthly)) : undefined,
    report: () => mini({
      key: "avg", title: pt.avg.title, sub: pt.avg.sub2,
      about: "Household spending in each of the last 3 full months, to help set a monthly budget. Money in VND.",
      kpis: [{ label: pt.avg.kAvg, value: avg }, ...(recurringMonthly ? [{ label: pt.avg.kRec, value: recurringMonthly }] : [])],
      chart: { kind: "column", title: pt.avg.chart, cat: pt.f.month, series: [{ name: pt.f.amount }], keepOrder: true, rows: [...have].reverse().map((m) => ({ cat: label(m.month), v: [m.r!.spent] })) },
      grid: { title: pt.avg.grid, cols: [{ name: pt.f.month, format: "textFormat" }, { name: pt.f.amount, format: "currencyFormat" }, { name: pt.f.count, format: "integerFormat" }], rows: have.map((m) => [label(m.month), m.r!.spent, m.r!.receipt_count]) },
    }),
  };
}

// ---------- 18. Thông báo · tóm tắt tuần ----------
function weekStart(d: Date) {
  const x = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  x.setDate(x.getDate() - ((x.getDay() + 6) % 7));
  return x;
}
export function weekInsight(rows: HistoryReceipt[] | undefined, now = new Date()): Touchpoint | null {
  if (!rows) return null;
  const thisWeek = weekStart(now);
  const from = new Date(thisWeek.getTime() - 7 * DAY), before = new Date(thisWeek.getTime() - 14 * DAY);
  const at = (x: HistoryReceipt) => new Date(`${dayOf(x.purchased_at)}T12:00:00`);
  const own = rows.filter((x) => x.kind !== "recurring");
  const last = own.filter((x) => at(x) >= from && at(x) < thisWeek);
  const prev = own.filter((x) => at(x) >= before && at(x) < from);
  if (last.length === 0) return null;
  const total = sum(last.map((x) => x.total_vnd));
  const prevTotal = sum(prev.map((x) => x.total_vnd));
  const by = new Map<string, number>();
  for (const x of last) by.set(x.store, (by.get(x.store) ?? 0) + x.total_vnd);
  const top = [...by].sort((a, b) => b[1] - a[1])[0][0];
  const days = pt.week.wd.map((cat, i) => ({ cat, v: [sum(last.filter((x) => (at(x).getDay() + 6) % 7 === i).map((x) => x.total_vnd))] }));
  return {
    id: `wk:${dayOf(from.toISOString())}:${total}`,
    severity: "info",
    text: pt.week.text(formatVnd(total), top),
    sub: pt.week.sub(last.length, prevTotal > 0 ? pctOf(total, prevTotal) : null),
    report: () => mini({
      key: "week", title: pt.week.title, sub: pt.week.sub2,
      about: "Household spending last week (Monday to Sunday) by day and by receipt, versus the week before. Money in VND.",
      kpis: [{ label: pt.week.kWeek, value: total }, { label: pt.week.kPrev, value: prevTotal }],
      chart: { kind: "column", title: pt.week.chart, cat: pt.f.day, series: [{ name: pt.f.amount }], keepOrder: true, rows: days },
      grid: { title: pt.week.grid, cols: [{ name: pt.f.date, format: "dateFormat" }, { name: pt.f.store, format: "textFormat" }, { name: pt.f.amount, format: "currencyFormat" }], rows: last.map((x) => [dayOf(x.purchased_at), x.store, x.total_vnd]), sortBy: 0 },
    }),
  };
}
