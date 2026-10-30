import { useSyncExternalStore } from "react";

/**
 * Kế hoạch đang xem (3.1 mục 3A). "default" là Kế hoạch chính, giữ ở đường dẫn cũ của API.
 * Lưu trong máy để mở lại app vẫn ở đúng Kế hoạch; đọc / ghi bọc try vì localStorage có thể bị chặn.
 */
const KEY = "mimi.planId";
export const DEFAULT_PLAN = "default";

let current = (() => {
  try {
    return localStorage.getItem(KEY) || DEFAULT_PLAN;
  } catch {
    return DEFAULT_PLAN;
  }
})();
const listeners = new Set<() => void>();

export function currentPlanId(): string {
  return current;
}

export function setCurrentPlanId(id: string) {
  current = id || DEFAULT_PLAN;
  try {
    localStorage.setItem(KEY, current);
  } catch {
    // không lưu được thì chỉ nhớ trong phiên này
  }
  listeners.forEach((l) => l());
}

export function useCurrentPlanId(): string {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => current,
  );
}

/** Hậu tố query cho các route /plan/...; Kế hoạch chính không cần. */
export function planQuery(id: string, sep: "?" | "&" = "?"): string {
  return id === DEFAULT_PLAN ? "" : `${sep}plan_id=${encodeURIComponent(id)}`;
}
