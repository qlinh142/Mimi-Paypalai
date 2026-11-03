/**
 * Tiền tệ hiển thị theo từng người (mẹ chốt 03/11: tạm VND và USD). Số tiền máy chủ trả luôn
 * là đồng; app quy đổi lúc hiển thị bằng tỷ giá của đúng ngày mua (quy tắc FX2, FX4): ngày
 * chưa có tỷ giá dùng tỷ giá gần nhất trước đó. Không truyền ngày thì dùng tỷ giá hôm nay.
 * Lựa chọn lưu trong tài khoản (`settings.currency`) và nhớ trên máy để mở app là đúng ngay.
 */
export type DisplayCurrency = "VND" | "USD";
export type FxPoint = { on: string; rate: number }; // rate: số đồng cho 1 USD

const CUR_KEY = "mimi.currency";
const RATES_KEY = "mimi.fx.USD";

function readCur(): DisplayCurrency {
  try {
    return localStorage.getItem(CUR_KEY) === "USD" ? "USD" : "VND";
  } catch {
    return "VND";
  }
}

function readRates(): FxPoint[] {
  try {
    const v = JSON.parse(localStorage.getItem(RATES_KEY) || "[]") as FxPoint[];
    return Array.isArray(v) ? v.filter((p) => typeof p.on === "string" && p.rate > 0) : [];
  } catch {
    return [];
  }
}

let current: DisplayCurrency = readCur();
let rates: FxPoint[] = readRates(); // cũ trước

export const displayCurrency = (): DisplayCurrency => current;
export const hasRates = () => rates.length > 0;

/** Ghi bảng tỷ giá mới (cũ trước); nhớ trên máy cho lần mở sau. */
export function setRates(next: FxPoint[]) {
  rates = [...next].sort((a, b) => (a.on < b.on ? -1 : a.on > b.on ? 1 : 0));
  try {
    localStorage.setItem(RATES_KEY, JSON.stringify(rates));
  } catch {
    // không lưu được: vẫn dùng trong phiên này
  }
}

function today(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** Tỷ giá dùng cho một ngày (YYYY-MM-DD hoặc ISO): ngày đó, không có thì ngày gần nhất trước đó. */
export function rateOn(on?: string | null): number | undefined {
  if (!rates.length) return undefined;
  const day = (on || today()).slice(0, 10);
  let lo = 0;
  let hi = rates.length - 1;
  let hit = -1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    if (rates[mid].on <= day) {
      hit = mid;
      lo = mid + 1;
    } else hi = mid - 1;
  }
  // Trước ngày tỷ giá đầu tiên app có: dùng ngày sớm nhất còn hơn không quy đổi được.
  return rates[hit >= 0 ? hit : 0].rate;
}

/** Đồng → tiền tệ hiển thị; undefined khi đang xem VND hoặc chưa có tỷ giá. */
export function toDisplay(vnd: number, on?: string | null): number | undefined {
  if (current === "VND") return undefined;
  const r = rateOn(on);
  return r ? vnd / r : undefined;
}

/** Đổi tiền tệ hiển thị: nhớ trên máy rồi tải lại để mọi số tiền đổi theo. */
export function switchCurrency(next: DisplayCurrency, reload = true) {
  current = next;
  try {
    localStorage.setItem(CUR_KEY, next);
  } catch {
    // vẫn đổi trong phiên này
  }
  if (reload) location.reload();
}
