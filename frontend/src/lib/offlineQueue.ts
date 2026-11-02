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
  /** Tài khoản đã tạo thao tác. Chỉ gửi bằng đúng tài khoản này (thao tác cũ chưa có thì coi như của người đang dùng). */
  uid?: string;
};

/**
 * Người đang đăng nhập. App gọi setQueueOwner mỗi khi phiên đổi.
 * Lỗi N3 giữ hàng đợi khi hết phiên (401); nếu sau đó một người KHÁC đăng nhập trên cùng máy,
 * thao tác của người trước không được gửi bằng tài khoản người sau và cũng không hiện cho họ thấy.
 * Thao tác đó nằm lại trong máy, người cũ đăng nhập lại thì gửi tiếp.
 */
let owner: string | null = null;
export function setQueueOwner(uid: string | null) {
  if (owner === uid) return;
  owner = uid;
  notify();
}
const mine = (w: QueuedWrite) => owner !== null && (w.uid === undefined || w.uid === owner);

async function readAll(): Promise<QueuedWrite[]> {
  return ((await get(STORE_KEY)) as QueuedWrite[] | undefined) ?? [];
}

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
    return (await readAll()).filter(mine);
  } catch {
    return [];
  }
}

const STORE_KEY = "mimi.writeQueue.v1";

export async function enqueue(w: Omit<QueuedWrite, "queuedAt" | "uid">) {
  const q = await readAll();
  if (q.some((x) => x.key === w.key)) return;
  q.push({ ...w, uid: owner ?? undefined, queuedAt: Date.now() });
  await set(STORE_KEY, q);
  notify();
}

export async function pendingCount(): Promise<number> {
  return (await listQueue()).length;
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
  if (flushing || !navigator.onLine || owner === null) return;
  flushing = true;
  try {
    for (;;) {
      // Đọc lại mỗi vòng: thao tác mới có thể được thêm trong lúc đang gửi.
      const head = (await readAll()).find(mine);
      if (!head) break;
      let res: Response;
      try {
        res = await send(head);
      } catch {
        break; // vẫn mất mạng
      }
      if (res.status >= 500 || res.status === 401 || res.status === 408 || res.status === 429) break;
      // Lỗi tạm thời dạng 409 (đang xử lý cùng mã gửi lại, ghi đồng thời): giữ lại, lần sau gửi tiếp.
      if (res.status === 409 && (await retryable(res))) break;
      await set(STORE_KEY, (await readAll()).filter((x) => x.key !== head.key));
      notify();
    }
  } finally {
    flushing = false;
  }
}

/** Đăng xuất chủ động: xóa thao tác của người đang dùng, giữ thao tác của người khác trên máy. */
export async function clearQueue() {
  await set(STORE_KEY, (await readAll()).filter((w) => !mine(w)));
  notify();
}
