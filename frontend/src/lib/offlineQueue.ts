import { get, set } from "idb-keyval";
import { useEffect, useState } from "react";

/**
 * Hàng đợi thao tác ghi khi mất mạng (sửa tồn, tick giỏ, đánh dấu voucher).
 * - Mỗi thao tác mang idempotency_key sinh lúc người dùng bấm, nên gửi lại bao nhiêu lần
 *   backend cũng chỉ ghi một lần.
 * - iOS Safari không có Background Sync, nên hàng đợi được xả khi app mở lại
 *   và khi trình duyệt báo có mạng ("online").
 * - Lưu hóa đơn KHÔNG đi qua hàng đợi này: người dùng phải thấy kết quả lưu ngay.
 */
export type QueuedWrite = {
  key: string;            // idempotency key
  method: "PATCH" | "POST" | "PUT" | "DELETE";
  path: string;           // ví dụ /items/sua-tuoi
  body?: unknown;
  queuedAt: number;
  /** Spec 1.5 mục 8: tên việc hiện ở "Đang chờ đồng bộ", ví dụ "Kết quả mua hàng cho Minh". */
  label?: string;
};

/* Báo cho màn hình khi hàng đợi đổi (thêm, gửi xong). */
const subs = new Set<() => void>();
const notify = () => subs.forEach((f) => f());

/** Danh sách việc đang chờ gửi, tự cập nhật. */
export function useQueue(): QueuedWrite[] {
  const [q, setQ] = useState<QueuedWrite[]>([]);
  useEffect(() => {
    let alive = true;
    const load = () => listQueue().then((x) => alive && setQ(x));
    load();
    subs.add(load);
    return () => {
      alive = false;
      subs.delete(load);
    };
  }, []);
  return q;
}

export async function listQueue(): Promise<QueuedWrite[]> {
  try {
    return ((await get(STORE_KEY)) as QueuedWrite[] | undefined) ?? [];
  } catch {
    return [];
  }
}

const STORE_KEY = "mimi.writeQueue.v1";

export async function enqueue(w: Omit<QueuedWrite, "queuedAt">) {
  const q = ((await get(STORE_KEY)) as QueuedWrite[] | undefined) ?? [];
  if (q.some((x) => x.key === w.key)) return;
  q.push({ ...w, queuedAt: Date.now() });
  await set(STORE_KEY, q);
  notify();
}

export async function pendingCount(): Promise<number> {
  return (((await get(STORE_KEY)) as QueuedWrite[] | undefined) ?? []).length;
}

let flushing = false;

async function retryable(res: Response): Promise<boolean> {
  try {
    return ((await res.clone().json()) as { retryable?: boolean }).retryable === true;
  } catch {
    return false;
  }
}

/**
 * Gửi lần lượt theo thứ tự. Dừng ở lỗi mạng đầu tiên; bỏ thao tác bị backend từ chối hẳn (4xx).
 * Lỗi N3 (Spec 1.5 mục 8): hết phiên đăng nhập (401) KHÔNG phải từ chối hẳn. Giữ nguyên thao tác,
 * dừng lại, lần sau (đã làm mới phiên hoặc đăng nhập lại) gửi tiếp. Không bao giờ xóa khi chưa gửi được.
 */
export async function flush(send: (w: QueuedWrite) => Promise<Response>) {
  if (flushing || !navigator.onLine) return;
  flushing = true;
  try {
    let q = ((await get(STORE_KEY)) as QueuedWrite[] | undefined) ?? [];
    while (q.length) {
      const [head, ...rest] = q;
      let res: Response;
      try {
        res = await send(head);
      } catch {
        break; // vẫn mất mạng
      }
      if (res.status >= 500 || res.status === 401 || res.status === 408 || res.status === 429) break;
      // Lỗi tạm thời dạng 409 (đang xử lý cùng mã gửi lại, ghi đồng thời): giữ lại, lần sau gửi tiếp.
      if (res.status === 409 && (await retryable(res))) break;
      q = rest;
      await set(STORE_KEY, q);
      notify();
    }
  } finally {
    flushing = false;
  }
}

export async function clearQueue() {
  await set(STORE_KEY, []);
  notify();
}
