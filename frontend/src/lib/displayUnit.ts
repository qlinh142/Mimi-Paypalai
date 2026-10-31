import type { Item } from "../api/types";
import { m15 } from "../i18n/m15";
import { normalizeVi } from "./text";

/**
 * Đơn vị hiển thị cho món đếm được (chai, hộp, quả...). API lưu tồn theo đơn vị chuẩn (g, ml, item),
 * nên chữ "chai" hay "hộp" là lựa chọn hiển thị: nhớ trên máy theo lần mẹ bấm Cập nhật gần nhất,
 * chưa có thì đoán theo tên món.
 */
export const COUNT_UNITS = ["chai", "hop", "qua", "goi", "cai"] as const;
export const MEASURE_UNITS = ["kg", "g", "l", "ml"] as const;
export const ALL_UNITS = [...COUNT_UNITS, ...MEASURE_UNITS];

const KEY = (k: string) => `mimi.unit.${k}`;

function guess(name: string): string {
  const n = normalizeVi(name);
  if (/trung|egg/.test(n)) return "qua";
  if (/sua|milk/.test(n)) return "hop";
  if (/nuoc|dau|chai|sauce|soap|shampoo|oil|gel/.test(n)) return "chai";
  if (/giay|bot|keo|mi |snack|paper|detergent/.test(n)) return "goi";
  return "cai";
}

export function savedUnit(item: Pick<Item, "key" | "name">): string {
  try {
    const v = localStorage.getItem(KEY(item.key));
    if (v && (ALL_UNITS as readonly string[]).includes(v)) return v;
  } catch {
    // localStorage bị chặn: dùng đoán theo tên
  }
  return guess(item.name);
}

export function rememberUnit(key: string, unit: string) {
  try {
    localStorage.setItem(KEY(key), unit);
  } catch {
    // bỏ qua
  }
}

/** Số lượng hiện có theo đơn vị hiển thị: [số, đơn vị]. Chưa có tồn thì null. */
export function stockInDisplay(item: Item): [number, string] | null {
  const st = item.stock;
  if (!st) return null;
  if (st.unit === "g") return st.estimated >= 1000 || st.estimated === 0 ? [st.estimated / 1000, "kg"] : [st.estimated, "g"];
  if (st.unit === "ml") return st.estimated >= 1000 || st.estimated === 0 ? [st.estimated / 1000, "l"] : [st.estimated, "ml"];
  const u = savedUnit(item);
  return [st.estimated, (COUNT_UNITS as readonly string[]).includes(u) ? u : "cai"];
}

export const unitLabel = (u: string) => m15.units[u] ?? u;

const fmt = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 2 });
export const formatNum = (n: number) => fmt.format(n);

/** Nhóm ở màn Đồ dùng (chốt 31/10): Cần mua ngay ≤ 3 ngày hoặc tồn 0 · Sắp hết ≤ 7 ngày · Còn nhiều. */
export type GoodsTier = "now" | "soon" | "plenty";
export function goodsTier(i: Item): GoodsTier {
  if (i.status === "out" || i.status === "buy_now") return "now";
  if (i.stock && i.stock.estimated <= 0) return "now";
  if (i.status === "waiting" || i.days_left == null) return "plenty";
  if (i.days_left <= 3) return "now";
  if (i.days_left <= 7) return "soon";
  return "plenty";
}

/** Số ngày từ hôm nay (giờ máy) tới ngày YYYY-MM-DD. */
export function daysUntil(isoDate: string): number {
  const [y, m, d] = isoDate.split("-").map(Number);
  const now = new Date();
  const a = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  return Math.round((Date.UTC(y, m - 1, d) - a) / 86_400_000);
}
