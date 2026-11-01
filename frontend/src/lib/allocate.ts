import type { Cart, PlanMeta } from "../api/types";

/**
 * Hóa đơn trừ vào kế hoạch nào (Spec 1.5 mục 3, việc 1, chốt 01/11 01:50):
 * trừ dần vào kế hoạch có ngày đi mua sớm nhất trước, dư thì sang kế hoạch kế tiếp.
 * Kế hoạch chưa có ngày xếp sau, theo thứ tự tạo. Kế hoạch Đã mua xong không trừ nữa.
 */
export type PlanNeed = { id: string; name: string; date: string; order: number; need: Map<string, number> };
export type Alloc = { planId: string; name: string; qty: number }[];

export function planNeeds(plans: PlanMeta[], carts: (Cart | undefined)[], nameOf: (p: PlanMeta) => string): PlanNeed[] {
  return plans
    .map((p, order) => ({ p, c: carts[order], order }))
    .filter(({ c }) => c && c.phase !== "reconciled")
    .map(({ p, c, order }) => ({
      id: p.id,
      name: nameOf(p),
      date: c!.state.buy_date || "",
      order,
      need: new Map(c!.items.filter((i) => i.line_status !== "assigned").map((i) => [i.key, i.qty])),
    }))
    .sort((a, b) => (a.date && b.date ? a.date.localeCompare(b.date) : a.date ? -1 : b.date ? 1 : a.order - b.order));
}

/** Chia từng dòng (khóa nhóm, số lượng) vào các kế hoạch theo thứ tự. Mỗi dòng trả về danh sách kế hoạch nhận. */
export function allocate(lines: { id: string; key?: string; qty: number }[], needs: PlanNeed[]): Map<string, Alloc> {
  const left = needs.map((n) => new Map(n.need));
  const out = new Map<string, Alloc>();
  for (const l of lines) {
    let q = l.qty;
    const got: Alloc = [];
    if (l.key) {
      needs.forEach((n, i) => {
        const want = left[i].get(l.key!) ?? 0;
        if (q <= 0 || want <= 0) return;
        const take = Math.min(q, want);
        left[i].set(l.key!, want - take);
        q -= take;
        got.push({ planId: n.id, name: n.name, qty: take });
      });
      // Dư sau khi đủ mọi kế hoạch: tính vào kế hoạch đầu tiên đã nhận (mua dư).
      if (q > 0 && got.length) got[0].qty += q;
    }
    out.set(l.id, got);
  }
  return out;
}
