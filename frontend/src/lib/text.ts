import { uiLang } from "../i18n";
import { toDisplay } from "./money";

/**
 * Chuẩn hóa để tìm không dấu: "Dầu gội" và "dau goi" khớp nhau.
 * NFD tách dấu thanh và dấu mũ nhưng KHÔNG tách "đ" (U+0111), nên phải map riêng.
 * Vẫn cần ở giao diện tiếng Anh vì tên món trên hóa đơn Việt Nam có dấu.
 */
export function normalizeVi(s: string): string {
  return s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/đ/g, "d")
    .replace(/Đ/g, "D")
    .toLowerCase()
    .trim();
}

// Tiền đã được backend tính bằng số nguyên đồng; client chỉ định dạng, và quy đổi sang tiền
// tệ hiển thị của người dùng nếu khác VND (lib/money.ts).
const vndEn = new Intl.NumberFormat("en-US", { style: "currency", currency: "VND", maximumFractionDigits: 0 });
const vndVi = new Intl.NumberFormat("vi-VN");
const usdFmt = new Intl.NumberFormat(uiLang === "en" ? "en-US" : "vi-VN", { style: "currency", currency: "USD" });

/**
 * en: "₫1,284,000" · vi: "1.284.000đ". Người chọn xem USD: "$51.36" · "51,36 US$", quy đổi theo
 * tỷ giá của ngày `on` (ngày mua; vắng là hôm nay).
 */
export function formatVnd(amount: number, on?: string | null): string {
  const usd = toDisplay(amount, on);
  if (usd !== undefined) return usdFmt.format(usd);
  return uiLang === "en" ? vndEn.format(amount) : `${vndVi.format(amount)}đ`;
}

const dayMonthEn = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", timeZone: "UTC" });

/** YYYY-MM-DD → en: "Oct 15" · vi: "15/10" */
export function formatDayMonth(iso: string): string {
  const [y, m, d] = iso.split("-").map(Number);
  if (uiLang === "en") return dayMonthEn.format(new Date(Date.UTC(y, m - 1, d)));
  return `${String(d).padStart(2, "0")}/${String(m).padStart(2, "0")}`;
}

/** Thời điểm ISO → giờ máy: "22:45 02/11" (cả hai ngôn ngữ). */
export function formatDateTime(iso: string): string {
  const t = new Date(iso);
  if (Number.isNaN(t.getTime())) return "";
  const p = (n: number) => String(n).padStart(2, "0");
  return `${p(t.getHours())}:${p(t.getMinutes())} ${p(t.getDate())}/${p(t.getMonth() + 1)}`;
}

/** "YYYY-MM" → en: "October 2026" · vi: "tháng 10/2026" */
export function formatMonth(ym: string): string {
  const [y, m] = ym.split("-").map(Number);
  if (uiLang === "en") {
    return new Intl.DateTimeFormat("en-US", { month: "long", year: "numeric", timeZone: "UTC" }).format(new Date(Date.UTC(y, m - 1, 1)));
  }
  return `tháng ${m}/${y}`;
}

/** Tháng hiện tại theo giờ máy, dạng "YYYY-MM". */
export function currentMonth(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

/** Cộng trừ tháng trên chuỗi "YYYY-MM". */
export function addMonths(ym: string, n: number): string {
  const [y, m] = ym.split("-").map(Number);
  const d = new Date(Date.UTC(y, m - 1 + n, 1));
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

const qtyFmt = new Intl.NumberFormat(uiLang === "en" ? "en-US" : "vi-VN", { maximumFractionDigits: 2 });

/**
 * Số lượng theo đơn vị chuẩn của API (0.1): 2500 g → "2.5 kg", 1500 ml → "1.5 L" (0.1 mục 6.2), 3 item → "3 pcs".
 * "pc" là tên cũ của item.
 */
export function formatQty(qty: number, unit: "g" | "ml" | "m" | "item" | "pc", pcLabel: string): string {
  if (unit === "g") return qty >= 1000 ? `${qtyFmt.format(qty / 1000)} kg` : `${qtyFmt.format(qty)} g`;
  if (unit === "ml") return qty >= 1000 ? `${qtyFmt.format(qty / 1000)} L` : `${qtyFmt.format(qty)} ml`;
  if (unit === "m") return `${qtyFmt.format(qty)} m`;
  return `${qtyFmt.format(qty)} ${pcLabel}`;
}

/** Đơn giá chuẩn hóa (8.1): 35 đ/ml, 120 đ/g, 3.400 đ/cái. */
export function formatUnitPrice(price: number, unit: string, pcLabel: string, on?: string | null): string {
  const u = unit === "item" || unit === "pc" ? pcLabel : unit;
  const usd = toDisplay(price, on);
  if (usd !== undefined) {
    return `${new Intl.NumberFormat(uiLang === "en" ? "en-US" : "vi-VN", { style: "currency", currency: "USD", maximumSignificantDigits: 3 }).format(usd)}/${u}`;
  }
  return `${new Intl.NumberFormat(uiLang === "en" ? "en-US" : "vi-VN", { maximumFractionDigits: 2 }).format(price)} ${uiLang === "en" ? "VND" : "đ"}/${u}`;
}
