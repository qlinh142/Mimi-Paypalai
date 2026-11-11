import { useMemo } from "react";
import { useForecast, useItems, useItemsMarketPrices, useRecurring } from "../api/hooks";
import { recurringTier } from "../components/RecurringList";
import { m15 } from "../i18n/m15";
import { s22 } from "../i18n/s22";
import { daysUntil, goodsTier } from "./displayUnit";
import { formatDayMonth, formatVnd } from "./text";

export type UpcomingLine = { key: string; name: string; sub: string; amount: number | null };

/**
 * "Sắp chi 7 ngày tới" (anh chốt 10/11), dùng chung cho tab Sắp chi của Sổ chi tiêu và màn
 * Phân tích chi tiêu, để hai nơi luôn ra cùng một con số:
 * - Khoản chi định kỳ: các kỳ đến hạn trong 7 ngày tới (kể cả quá hạn).
 * - Hàng tiêu dùng: món sắp hết (nhóm Còn 3 ngày, Còn 7 ngày) × mức dùng mỗi ngày × 7 ngày.
 */
export function useUpcoming7() {
  const rec = useRecurring();
  const items = useItems({ tab: "active", horizon: 7 });
  const fc = useForecast();
  const market = useItemsMarketPrices();

  const recRows = useMemo<UpcomingLine[]>(
    () => (rec.data?.items ?? []).filter((r) => recurringTier(r) !== "later").map((r) => ({ key: r.id, name: r.name, sub: `${formatDayMonth(r.next_due_at)} · ${s22.due(daysUntil(r.next_due_at))}`, amount: r.amount as number | null })),
    [rec.data],
  );
  // Giá mỗi món: hóa đơn gần nhất (Dự báo của API), chưa có thì giá rẻ nhất trong bảng giá chung.
  const goodsRows = useMemo<UpcomingLine[]>(() => {
    const paid = new Map((fc.data?.shopping ?? []).map((s) => [s.key, s.amount]));
    const cheapest = new Map<string, { price: number; store: string }>();
    for (const m of market.data?.items ?? []) {
      const best = m.prices.reduce<{ price: number; store: string } | null>((a, x) => (a && a.price <= x.price_vnd ? a : { price: x.price_vnd, store: x.store }), null);
      if (best) cheapest.set(m.key, best);
    }
    // Số tiền mỗi món = mức dùng trung bình mỗi ngày × 7 ngày,
    // mức dùng mỗi ngày = giá một lần mua ÷ số ngày giữa hai lần mua (chu kỳ mua).
    return (items.data ?? []).filter((i) => goodsTier(i) !== "plenty").map((i) => {
      const own = paid.get(i.key);
      const m = own == null ? cheapest.get(i.key) : undefined;
      const price = own ?? m?.price ?? null;
      const from = m ? m15.spending.marketPrice(m.store) : "";
      if (price == null) return { key: i.key, name: i.name, sub: "", amount: null };
      const interval = i.interval_days && i.interval_days > 0 ? i.interval_days : null;
      if (!interval) return { key: i.key, name: i.name, sub: [m15.spending.oneBuy, from].filter(Boolean).join(" · "), amount: price };
      // Làm tròn mức dùng mỗi ngày tới 100đ rồi mới nhân 7, để số trên dòng phụ nhân ra đúng số tiền.
      const perDay = Math.round(price / interval / 100) * 100;
      return { key: i.key, name: i.name, sub: [m15.spending.perDay(formatVnd(perDay)), from].filter(Boolean).join(" · "), amount: perDay * 7 };
    });
  }, [items.data, fc.data, market.data]);

  const sum = (rows: UpcomingLine[]) => rows.reduce((a, x) => a + (x.amount ?? 0), 0);
  const recTotal = sum(recRows);
  const goodsTotal = sum(goodsRows);
  return {
    rec,
    items,
    recRows,
    goodsRows,
    recTotal,
    goodsTotal,
    total: recTotal + goodsTotal,
    missing: [...recRows, ...goodsRows].filter((x) => x.amount == null).length,
    pending: rec.isPending || items.isPending,
  };
}
