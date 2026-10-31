import { Baby, Gift, House, ShoppingBag, UsersRound, UtensilsCrossed, type LucideIcon } from "lucide-react";

/**
 * Biểu tượng của Kế hoạch (Mimi 1.5, ảnh mẫu 5b). API chưa có trường này nên tạm lưu trên máy;
 * khi backend thêm trường icon thì chuyển sang lưu cùng Kế hoạch.
 */
export const PLAN_ICONS: { key: string; Icon: LucideIcon }[] = [
  { key: "bag", Icon: ShoppingBag },
  { key: "family", Icon: UsersRound },
  { key: "kid", Icon: Baby },
  { key: "gift", Icon: Gift },
  { key: "home", Icon: House },
  { key: "food", Icon: UtensilsCrossed },
];

const KEY = (id: string) => `mimi.planIcon.${id}`;

export function planIcon(id: string): string {
  try {
    const v = localStorage.getItem(KEY(id));
    if (v && PLAN_ICONS.some((x) => x.key === v)) return v;
  } catch {
    // bỏ qua
  }
  return "bag";
}

export function setPlanIcon(id: string, key: string) {
  try {
    localStorage.setItem(KEY(id), key);
  } catch {
    // bỏ qua
  }
}

export const iconOf = (key: string): LucideIcon => PLAN_ICONS.find((x) => x.key === key)?.Icon ?? ShoppingBag;
