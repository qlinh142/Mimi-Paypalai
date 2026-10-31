import { en, type Messages } from "./en";
import { vi } from "./vi";

/**
 * Ngôn ngữ giao diện, tách riêng với `lang` (ngôn ngữ dịch hóa đơn) như spec mục 7 đề xuất.
 * Mimi 1.5: mặc định tiếng Việt. Đổi trong Cài đặt (hoặc ?ui=en), lưu lại trong máy.
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
  return "vi";
}

export const uiLang: UiLang = detect();
export const t: Messages = uiLang === "vi" ? vi : en;

if (typeof document !== "undefined") document.documentElement.lang = uiLang;

/** Nhãn nhóm hàng theo ngôn ngữ giao diện; khóa lạ hiện "Khác" thay vì để trống. */
export function categoryLabel(key: string | null | undefined): string {
  const map = t.categories as Record<string, string>;
  return (key && map[key]) || t.categories.khac;
}
