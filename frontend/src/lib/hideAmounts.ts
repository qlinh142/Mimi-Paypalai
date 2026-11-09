import { useSyncExternalStore } from "react";

/**
 * Ẩn số tiền (Cài đặt 2.2, B1–B4 theo Redesign): làm mờ số tiền ở thẻ tổng Sổ chi tiêu, chạm vào số để
 * xem. Chỉ lưu trên máy này, không gửi lên máy chủ.
 */
const KEY = "mimi.hideAmounts";
const subs = new Set<() => void>();

export function hideAmounts(): boolean {
  try {
    return localStorage.getItem(KEY) === "1";
  } catch {
    return false;
  }
}

export function setHideAmounts(on: boolean) {
  try {
    if (on) localStorage.setItem(KEY, "1");
    else localStorage.removeItem(KEY);
  } catch {
    /* không lưu được (chế độ riêng tư): bỏ qua */
  }
  subs.forEach((f) => f());
}

export function useHideAmounts(): boolean {
  return useSyncExternalStore(
    (f) => {
      subs.add(f);
      return () => subs.delete(f);
    },
    hideAmounts,
    () => false,
  );
}
