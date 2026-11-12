/**
 * Các điểm chạm "Mimi nhận thấy" (giai đoạn 1: vị trí, luật, câu chữ tính trước).
 * Xem bảng điểm chạm: "Điểm chạm AG Studio trong Mimi 2.2" (12/11).
 *   - Điểm 6, Chi tiết món: giá đang bán cao hơn lần mua gần nhất.
 *   - Điểm 9, Kế hoạch bước 1: kế hoạch so với ngân sách tháng còn lại.
 */
import { useState } from "react";
import { useQueries } from "@tanstack/react-query";
import { apiGet } from "../api/client";
import { qk } from "../api/hooks";
import type { ItemPriceHistory, PriceObservation } from "../api/types";
import { Touch } from "./Insight";
import { budgetMarkInsight, pricierInsight, tripInsight, type BillLine, type TripLine } from "./points";
import { useItemMarketPrices, useItemPriceHistory, useRecurring, useSpending } from "../api/hooks";
import type { Cart } from "../api/types";
import { billsLeft, monthClock, useBudget } from "../components/SpendingPlus";
import { currentMonth, formatVnd } from "../lib/text";
import { InsightCard, InsightReport } from "./Insight";
import { budgetReport, priceReport, type ReportSpec } from "./reports";
import { budgetInsight, checkOffers, priceInsight } from "./rules";
import { it } from "./text";

/** Điểm 6: dưới thẻ số lượng ở Chi tiết món. */
export function ItemPriceInsight({ itemKey, name }: { itemKey: string; name: string }) {
  const h = useItemPriceHistory(itemKey);
  const m = useItemMarketPrices(itemKey);
  const [spec, setSpec] = useState<ReportSpec | null>(null);
  const ins = priceInsight(name, h.data, m.data);
  if (!ins) return null;
  const per = (v: number) => `${formatVnd(v)}/${it.unit(ins.unit)}`;
  return (
    <>
      <InsightCard
        id={ins.id}
        severity={ins.severity}
        text={it.price.text(name, ins.pct)}
        sub={it.price.sub(per(ins.current), per(ins.last))}
        onOpen={() => setSpec(priceReport(ins, h.data?.history ?? [], checkOffers(m.data?.prices ?? [], ins.lastPaid.canonical_unit)))}
      />
      <InsightReport spec={spec} onClose={() => setSpec(null)} />
    </>
  );
}

/** Điểm 9: dưới danh sách món ở bước 1 Thêm món; cập nhật mỗi lần thêm, bỏ món hay đổi số lượng. */
/** plan: số dự kiến khác cart.estimate (bước 3 dùng tổng theo giá đã chọn). */
export function PlanBudgetInsight({ cart, plan }: { cart: Cart; plan?: number }) {
  const budget = useBudget();
  const s = useSpending(currentMonth());
  const rec = useRecurring();
  const [spec, setSpec] = useState<ReportSpec | null>(null);
  const bills = billsLeft(rec.data?.items, monthClock().end).total;
  const ins = budgetInsight(budget, s.data?.spent, bills, plan ?? cart.estimate, cart.missing_price);
  if (!ins || cart.items.length === 0) return null;
  return (
    <>
      <InsightCard
        id={ins.id}
        severity={ins.over ? "warning" : "info"}
        text={ins.over ? it.budget.over(formatVnd(-ins.left)) : it.budget.left(formatVnd(ins.left))}
        sub={[it.budget.sub(formatVnd(ins.plan)), ins.missing > 0 ? it.budget.missing(ins.missing) : ""].filter(Boolean).join(" ")}
        onOpen={() => setSpec(budgetReport(ins, cart.items))}
      />
      <InsightReport spec={spec} onClose={() => setSpec(null)} />
    </>
  );
}

// ---------- Điểm 13 và 14: màn "Đã lưu hóa đơn" ----------

export type ScanLine = { name: string; amount: number; groupKey?: string; cq?: number; cu?: string; plans: string[] };
export type ScanSnap = { lines: ScanLine[]; carts: Map<string, Cart>; spent?: number; budget: number | null; on: string };

/**
 * Lúc bấm Lưu, chụp lại kế hoạch và số đã chi tháng này: sau khi lưu, máy chủ đã cộng hóa đơn này vào.
 * Giá lần trước thì tải sau khi lưu và chỉ lấy lần mua trước ngày của hóa đơn, nên bấm Lưu nhanh cũng không sót.
 */
export function useScanSnapshot(carts: (Cart | undefined)[], planIds: string[]) {
  const s = useSpending(currentMonth());
  const budget = useBudget();
  return (lines: ScanLine[], on: string): ScanSnap => ({
    lines,
    carts: new Map(planIds.map((id, i) => [id, carts[i]]).filter((x): x is [string, Cart] => !!x[1])),
    spent: s.data?.spent,
    budget,
    on,
  });
}

export function AfterSaveInsights({ snap, planDone }: { snap: ScanSnap; planDone: string[] }) {
  const total = snap.lines.reduce((a, l) => a + l.amount, 0);
  const keys = [...new Set(snap.lines.map((l) => l.groupKey).filter((k): k is string => !!k))];
  const hist = useQueries({
    queries: keys.map((k) => ({ queryKey: qk.priceHistory(k), queryFn: () => apiGet<ItemPriceHistory>(`/items/${encodeURIComponent(k)}/price-history`) })),
  });
  const last = new Map<string, PriceObservation | null>();
  keys.forEach((k, i) => {
    const h = hist[i]?.data;
    if (!h) return;
    const before = h.history.filter((o) => o.purchased_at.slice(0, 10) < snap.on).sort((a, b) => b.purchased_at.localeCompare(a.purchased_at));
    last.set(k, before[0] ?? null);
  });
  const loaded = hist.every((q) => !q.isPending);
  // 13: chuyến mua xong, dự kiến so với thực chi.
  let trip = null;
  const done = planDone.map((id) => snap.carts.get(id)).filter((c): c is Cart => !!c);
  if (done.length) {
    const items = done.flatMap((c) => c.items);
    const keys = new Set(items.map((i) => i.key));
    const rows: TripLine[] = items.map((i) => ({
      name: i.name,
      est: i.unit_price != null ? i.unit_price * i.qty : null,
      spent: snap.lines.filter((l) => l.groupKey === i.key).reduce((a, l) => a + l.amount, 0),
    }));
    for (const l of snap.lines) if (!l.groupKey || !keys.has(l.groupKey)) rows.push({ name: l.name, est: null, spent: l.amount });
    trip = tripInsight(rows, done.reduce((a, c) => a + (c.estimate ?? 0), 0));
  }
  // 14: món đắt hơn lần trước; không có thì mốc 80% / 100% ngân sách.
  const bill: BillLine[] = snap.lines
    .filter((l) => l.groupKey && l.cq && l.cu && l.amount > 0)
    .map((l) => ({ name: l.name, unitPrice: l.amount / l.cq!, unit: l.cu!, last: last.get(l.groupKey!) ?? null }));
  // Chờ giá lần trước tải xong rồi mới chọn card thứ hai, để card không nhảy từ ngân sách sang giá.
  const second = loaded ? pricierInsight(bill) ?? budgetMarkInsight(snap.budget, snap.spent, total) : null;
  return (
    <div className="ins22stack">
      <Touch t={trip} />
      <Touch t={second} />
    </div>
  );
}
