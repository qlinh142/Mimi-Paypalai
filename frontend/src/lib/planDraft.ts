import { useSyncExternalStore } from "react";

/**
 * Người mua đã chọn và trạng thái "Chưa hoàn thành" của từng Kế hoạch (Spec 1.5 mục 1, chốt 01/11 04:36, 05:02).
 * - Chọn người mua chỉ là chọn, chưa gửi. Gửi khi bấm Lưu kế hoạch (nút máy bay giấy).
 * - Sửa kế hoạch mà rời đi chưa bấm Lưu thì kế hoạch ở trạng thái "Chưa hoàn thành", như thư nháp.
 * Hai thông tin này lưu trên máy (thư nháp chưa lên máy chủ). Người mua đã gửi lấy từ máy chủ.
 */
export type BuyerPick = "me" | string; // "me" hoặc uid của thành viên

const BUYER = (id: string) => `mimi.planBuyer.${id}`;
const DRAFT = (id: string) => `mimi.planDraft.${id}`;

const subs = new Set<() => void>();
let version = 0;
const notify = () => {
  version++;
  subs.forEach((f) => f());
};

function read(k: string): string | null {
  try {
    return localStorage.getItem(k);
  } catch {
    return null;
  }
}
function write(k: string, v: string | null) {
  try {
    if (v == null) localStorage.removeItem(k);
    else localStorage.setItem(k, v);
  } catch {
    // bỏ qua: không lưu được thì chỉ mất trạng thái nháp
  }
  notify();
}

export const buyerOf = (id: string): BuyerPick | null => read(BUYER(id));
export const setBuyer = (id: string, b: BuyerPick | null) => write(BUYER(id), b);
export const isDraft = (id: string) => read(DRAFT(id)) === "1";
export const markDraft = (id: string) => {
  if (!isDraft(id)) write(DRAFT(id), "1");
};
export const clearDraft = (id: string) => write(DRAFT(id), null);

/** Vẽ lại khi người mua hoặc trạng thái nháp đổi. */
export function usePlanDraftVersion() {
  return useSyncExternalStore(
    (f) => {
      subs.add(f);
      return () => subs.delete(f);
    },
    () => version,
  );
}
