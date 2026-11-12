/**
 * Các điểm chạm "Mimi nhận thấy" (giai đoạn 1: vị trí, luật, câu chữ tính trước).
 * Xem bảng điểm chạm: "Điểm chạm AG Studio trong Mimi 2.2" (12/11).
 *   - Điểm 6, Chi tiết món: giá đang bán cao hơn lần mua gần nhất.
 *   - Điểm 9, Kế hoạch bước 1: kế hoạch so với ngân sách tháng còn lại.
 */
import { useState } from "react";
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
export function PlanBudgetInsight({ cart }: { cart: Cart }) {
  const budget = useBudget();
  const s = useSpending(currentMonth());
  const rec = useRecurring();
  const [spec, setSpec] = useState<ReportSpec | null>(null);
  const bills = billsLeft(rec.data?.items, monthClock().end).total;
  const ins = budgetInsight(budget, s.data?.spent, bills, cart.estimate, cart.missing_price);
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
