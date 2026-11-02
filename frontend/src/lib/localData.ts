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
const OWNER_KEY = "mimi.deviceOwner";

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

/**
 * Gọi mỗi khi có người đăng nhập. Nếu máy đang giữ dữ liệu của một tài khoản KHÁC (người trước bị
 * tự đăng xuất vì hết phiên nên chưa kịp dọn), xóa cache và nháp của người đó trước khi hiện gì.
 * Hàng đợi ghi không bị xóa ở đây: offlineQueue tự giấu và giữ thao tác của người khác.
 */
export async function claimDevice(uid: string) {
  let prev: string | null = null;
  try {
    prev = localStorage.getItem(OWNER_KEY);
  } catch {
    return;
  }
  if (prev === uid) return;
  if (prev !== null) {
    queryClient.clear();
    clearLocalUserData();
    await queryPersister.removeClient();
  }
  try {
    localStorage.setItem(OWNER_KEY, uid);
  } catch {
    // bỏ qua
  }
}
