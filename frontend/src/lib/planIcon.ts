import { Baby, Gift, House, ShoppingBag, UsersRound, UtensilsCrossed, type LucideIcon } from "lucide-react";
import type { PlanMeta } from "../api/types";

/**
 * Bộ biểu tượng chung của app cho Kế hoạch (Mimi 1.5, ảnh mẫu 5b). Máy chủ lưu khóa cùng
 * Kế hoạch (PATCH /plans/{id} {icon}), nên đăng nhập lại hay đổi máy vẫn ra đúng biểu tượng.
 * Danh sách khóa phải khớp plan.PlanIcons ở backend.
 */
export const PLAN_ICONS: { key: string; Icon: LucideIcon }[] = [
  { key: "bag", Icon: ShoppingBag },
  { key: "family", Icon: UsersRound },
  { key: "kid", Icon: Baby },
  { key: "gift", Icon: Gift },
  { key: "home", Icon: House },
  { key: "food", Icon: UtensilsCrossed },
];

export function planIcon(meta: Pick<PlanMeta, "icon"> | undefined): string {
  const v = meta?.icon;
  return v && PLAN_ICONS.some((x) => x.key === v) ? v : "bag";
}

export const iconOf = (key: string): LucideIcon => PLAN_ICONS.find((x) => x.key === key)?.Icon ?? ShoppingBag;
