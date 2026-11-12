/**
 * Luật "Mimi nhận thấy" giai đoạn 1 (đặc tả Contextual Insights 12/11): vị trí, điều kiện và
 * nội dung tính trước, chỉ từ dữ liệu có thật. Thiếu dữ liệu thì không có card, không đoán.
 *
 * Bản demo tính ở máy từ các API sẵn có. Bản chạy thật chuyển luật giá về máy chủ
 * (backend/internal/insight, cùng ngưỡng) để số liệu và quyền xem giá do máy chủ giữ.
 */
import type { ItemMarketPrices, ItemPriceHistory, MarketPrice, PriceObservation } from "../api/types";

export type Severity = "info" | "warning";

// Ngưỡng thử nghiệm (đặc tả C.5): tăng từ 10%, hoặc cả kế hoạch đắt thêm từ $3 / 50.000đ.
export const MIN_PCT = 10;
export const WARN_PCT = 25;
export const MAX_OFFER_AGE_DAYS = 14;
const MIN_IMPACT_VND = 50_000;
const MIN_IMPACT_LOCAL = 3; // tiền của nơi bán khi ngoài Việt Nam (USD)

type Unit = "ml" | "g" | "m" | "item";

const UNITS: Record<string, [Unit, number]> = {
  ml: ["ml", 1], l: ["ml", 1000], lit: ["ml", 1000], g: ["g", 1], gr: ["g", 1], kg: ["g", 1000],
  floz: ["ml", 29.5735], oz: ["g", 28.3495], lb: ["g", 453.592], lbs: ["g", 453.592], gal: ["ml", 3785.41], qt: ["ml", 946.353],
  ct: ["item", 1], count: ["item", 1], "quả": ["item", 1], "cuộn": ["item", 1], "viên": ["item", 1], pcs: ["item", 1],
};

/** "1L", "750ml", "5kg", "64 fl oz", "10 cuộn", "2 x 180ml" → lượng chuẩn hóa. Không đọc được: null. */
export function offerSize(size?: string): { qty: number; unit: Unit } | null {
  const s = (size ?? "").toLowerCase().replace(/fl\.?\s*oz/g, "floz").replace(",", ".");
  const m = s.match(/(?:(\d+)\s*x\s*)?(\d+(?:\.\d+)?)\s*(floz|ml|lit|l|kg|gr|g|oz|lbs|lb|gal|qt|ct|count|quả|cuộn|viên|pcs)(?![a-z])/u);
  if (!m) return null;
  const [unit, f] = UNITS[m[3]];
  const qty = Number(m[2]) * f * (m[1] ? Number(m[1]) : 1);
  return qty > 0 ? { qty: Math.round(qty * 100) / 100, unit } : null;
}

/** Đơn giá theo đơn vị hiển thị: đ/ml → đ/L, đ/g → đ/kg. */
export function displayUnit(u?: string): { f: number; label: string } {
  if (u === "ml") return { f: 1000, label: "L" };
  if (u === "g") return { f: 1000, label: "kg" };
  if (u === "m") return { f: 1, label: "m" };
  return { f: 1, label: "item" };
}

export type OfferRow = MarketPrice & { usable: boolean; reason?: "no_size" | "other_unit" | "stale" | "no_source"; unitPrice?: number; canonical?: number };

const daysAgo = (on: string, now: Date) => (now.getTime() - new Date(`${on.slice(0, 10)}T12:00:00Z`).getTime()) / 86_400_000;

/** Xét từng giá đang bán: so được với giá đã trả không, và vì sao. */
export function checkOffers(prices: MarketPrice[], unit: string | undefined, now = new Date()): OfferRow[] {
  return prices.map((p) => {
    const size = offerSize(p.size);
    if (!p.source || !p.on) return { ...p, usable: false, reason: "no_source" };
    if (daysAgo(p.on, now) > MAX_OFFER_AGE_DAYS) return { ...p, usable: false, reason: "stale" };
    if (!size) return { ...p, usable: false, reason: "no_size" };
    if (!unit || size.unit !== unit) return { ...p, usable: false, reason: "other_unit" };
    return { ...p, usable: true, canonical: size.qty, unitPrice: p.price_vnd / size.qty };
  });
}

export type PriceInsight = {
  id: string; severity: Severity; name: string; pct: number;
  unit: string; current: number; last: number; // đơn giá theo đơn vị hiển thị (đ/L...)
  lastPaid: PriceObservation; offer: OfferRow; impact: number; asOf: string;
};

/**
 * P0-01: giá đang bán rẻ nhất (cùng chiều đo, có nguồn, còn mới) vẫn cao hơn giá đã trả lần gần
 * nhất từ 10%, hoặc cả kế hoạch đắt thêm từ ngưỡng tiền. So với giá rẻ nhất nên khi card hiện,
 * mua ở nơi nào đã tra cũng đắt hơn lần trước.
 */
export function priceInsight(name: string, h: ItemPriceHistory | undefined, m: ItemMarketPrices | undefined, qty = 1, now = new Date()): PriceInsight | null {
  const last = h?.last_purchase;
  if (!last || !(last.unit_price > 0) || !m) return null;
  const best = checkOffers(m.prices, last.canonical_unit, now)
    .filter((o) => o.usable)
    .sort((a, b) => a.unitPrice! - b.unitPrice!)[0];
  if (!best) return null;
  const delta = best.unitPrice! - last.unit_price;
  if (delta <= 0) return null;
  const pct = (delta / last.unit_price) * 100;
  const impact = delta * Math.max(1, qty) * best.canonical!;
  const minImpact = best.currency && best.currency !== "VND" && best.local ? MIN_IMPACT_LOCAL * (best.price_vnd / best.local) : MIN_IMPACT_VND;
  if (pct < MIN_PCT && impact < minImpact) return null;
  const d = displayUnit(last.canonical_unit);
  return {
    id: `pi:${m.key}:${last.purchased_at}:${last.paid_price}:${best.store}:${best.price_vnd}:${best.on}`,
    severity: pct >= WARN_PCT ? "warning" : "info",
    name, pct: Math.round(pct), unit: d.label,
    current: Math.round(best.unitPrice! * d.f), last: Math.round(last.unit_price * d.f),
    lastPaid: last, offer: best, impact: Math.round(impact),
    asOf: [best.on, last.purchased_at.slice(0, 10)].sort().at(-1)!,
  };
}

export type BudgetInsight = {
  id: string; severity: Severity; over: boolean;
  budget: number; spent: number; bills: number; plan: number; left: number; missing: number;
};

/**
 * P0-02 theo ngân sách tháng (anh chốt 12/11): tháng này còn bao nhiêu sau khi trừ đã chi, các
 * khoản định kỳ còn phải trả và dự kiến của kế hoạch. Vượt thì cảnh báo; còn dư thì chỉ báo số
 * còn lại, không gợi ý mua thêm. Chưa đặt ngân sách hoặc kế hoạch chưa có giá: không có card.
 */
export function budgetInsight(budget: number | null, spent: number | undefined, bills: number, plan: number | null | undefined, missing = 0): BudgetInsight | null {
  if (!budget || spent == null || plan == null || plan <= 0) return null;
  const left = budget - spent - bills - plan;
  const over = left < 0;
  return {
    id: `bo:${budget}:${spent}:${bills}:${plan}`,
    severity: over && -left * 10 >= budget ? "warning" : over ? "info" : "info",
    over, budget, spent, bills, plan, left, missing,
  };
}
