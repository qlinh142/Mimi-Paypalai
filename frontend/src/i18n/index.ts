import { en, type Messages } from "./en";
import { vi } from "./vi";

/**
 * Ngôn ngữ của app (mẹ chốt 03/11): một ngôn ngữ chung cho giao diện, tên món dịch, lời nhắc
 * và email; lưu vào tài khoản (`settings.lang`) và nhớ trên máy. Người mới: theo ngôn ngữ
 * điện thoại, máy tiếng Việt thì tiếng Việt, còn lại tiếng Anh. Thử nhanh: ?ui=en.
 */
export type UiLang = "en" | "vi";
const KEY = "mimi.uiLang";

function detect(): UiLang {
  try {
    const fromUrl = new URLSearchParams(location.search).get("ui");
    if (fromUrl === "en" || fromUrl === "vi") {
      localStorage.setItem(KEY, fromUrl);
      return fromUrl;
    }
    const saved = localStorage.getItem(KEY);
    if (saved === "en" || saved === "vi") return saved;
  } catch {
    // localStorage có thể bị chặn (trình duyệt riêng tư): dùng mặc định
  }
  return deviceLang();
}

/** Ngôn ngữ của điện thoại: tiếng Việt nếu máy đặt tiếng Việt, còn lại tiếng Anh. */
export function deviceLang(): UiLang {
  try {
    const langs = navigator.languages?.length ? navigator.languages : [navigator.language];
    return (langs[0] || "").toLowerCase().startsWith("vi") ? "vi" : "en";
  } catch {
    return "vi";
  }
}

/** Đổi ngôn ngữ app: nhớ trên máy rồi tải lại trang để mọi chữ đổi theo. */
export function switchUiLang(lang: UiLang) {
  try {
    localStorage.setItem(KEY, lang);
  } catch {
    // không lưu được thì vẫn đổi qua ?ui
  }
  const u = new URL(location.href);
  u.searchParams.set("ui", lang);
  location.assign(u.toString());
}

export const uiLang: UiLang = detect();
export const t: Messages = uiLang === "vi" ? vi : en;

if (typeof document !== "undefined") document.documentElement.lang = uiLang;

/** Nhãn nhóm hàng theo ngôn ngữ giao diện; khóa lạ hiện "Khác" thay vì để trống. */
export function categoryLabel(key: string | null | undefined): string {
  const map = t.categories as Record<string, string>;
  return (key && map[key]) || t.categories.khac;
}
