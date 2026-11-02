import { queryClient, queryPersister } from "../app/queryClient";
import { clearQueue } from "./offlineQueue";

/**
 * Dữ liệu riêng của người dùng lưu trong localStorage (nháp kế hoạch, người mua đã chọn, vòng đi mua hộ,
 * kế hoạch đang mở, quan hệ thành viên, icon, đơn vị hiển thị...). Mọi khóa đều bắt đầu bằng "mimi.".
 * Đăng xuất phải xóa hết, nếu không người đăng nhập sau trên cùng máy sẽ thấy nháp của người trước
 * (vấn đề 65: logout nhưng cache vẫn còn).
 * Chỉ giữ cài đặt của chính máy, không thuộc về ai: ngôn ngữ giao diện.
 */
const PREFIX = "mimi.";
const KEEP = new Set(["mimi.uiLang"]);

export function clearLocalUserData() {
  try {
    const keys: string[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (k && k.startsWith(PREFIX) && !KEEP.has(k)) keys.push(k);
    }
    keys.forEach((k) => localStorage.removeItem(k));
  } catch {
    // localStorage bị chặn: không có gì để xóa
  }
}

/** Dọn mọi dữ liệu của người dùng trên máy trước khi đăng xuất: cache API (bộ nhớ và IndexedDB),
 * hàng đợi ghi chưa gửi và các khóa localStorage ở trên. */
export async function wipeDeviceData() {
  queryClient.clear();
  clearLocalUserData();
  await Promise.all([queryPersister.removeClient(), clearQueue()]);
}
