import { t } from "../i18n";
import { API_MODE, appCheckToken } from "../lib/firebase";
import { enqueue, flush, type QueuedWrite } from "../lib/offlineQueue";
import { mockFetch } from "./mock";
import type { ApiErrorBody } from "./types";

/**
 * Mọi lời gọi đi qua /api trên cùng tên miền (Firebase Hosting rewrite sang Cloud Run),
 * nên không cần CORS. Mỗi request mang:
 *   Authorization: Bearer <Firebase ID token>
 *   X-Firebase-AppCheck: <token>
 *   Idempotency-Key: <uuid> cho thao tác ghi
 */
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    /** Chi tiết theo mã lỗi, ví dụ image_index của ảnh cần chụp lại (1.1). */
    public fields: Record<string, unknown> = {},
    /** true: gửi lại đúng yêu cầu này có thể thành công. */
    public retryable = false,
  ) {
    super(message);
  }
}
export class OfflineError extends Error {}

type TokenGetter = () => Promise<string | null>;
let getToken: TokenGetter = async () => null;
let onUnauthorized: () => void = () => {};

export function configureApi(opts: { getToken: TokenGetter; onUnauthorized: () => void }) {
  getToken = opts.getToken;
  onUnauthorized = opts.onUnauthorized;
}

/**
 * Dựng header TRƯỚC khi gọi mạng, để lỗi lấy token không bị báo nhầm thành "mất mạng".
 * - ID token: lỗi mạng khi làm mới token mới là OfflineError; lỗi khác giữ nguyên để hiện đúng.
 * - App Check: lấy token lỗi (key sai, tên miền chưa khai báo...) thì vẫn gửi request không kèm
 *   header. Backend bật APP_CHECK_REQUIRED sẽ trả 401 app_check_failed, nhìn thấy được trong Network.
 */
async function headers(extra?: Record<string, string>) {
  const h: Record<string, string> = { Accept: "application/json", ...extra };
  let token: string | null;
  try {
    token = await getToken();
  } catch (e) {
    if ((e as { code?: string }).code === "auth/network-request-failed") throw new OfflineError("offline");
    console.error("[mimi] Không lấy được ID token", e);
    throw e;
  }
  if (token) h.Authorization = `Bearer ${token}`;
  try {
    const ac = await appCheckToken();
    if (ac) h["X-Firebase-AppCheck"] = ac;
  } catch (e) {
    console.error("[mimi] Không lấy được App Check token, gửi request không kèm token", e);
  }
  return h;
}

/**
 * Thời gian chờ tối đa phía app (vấn đề 13, 90): mạng chập chờn thì không để vòng quay mãi.
 * Đặt dài hơn giới hạn của backend (10 s cho thao tác thường, 55 s cho đọc ảnh) để backend
 * luôn kịp trả lỗi có mã trước khi app tự cắt.
 */
export const TIMEOUT_MS = 20_000;
export const UPLOAD_TIMEOUT_MS = 65_000;

async function rawFetch(path: string, init: RequestInit, timeoutMs = TIMEOUT_MS): Promise<Response> {
  if (API_MODE === "mock") return mockFetch(path, init);
  try {
    return await fetch(`/api${path}`, { ...init, signal: AbortSignal.timeout(timeoutMs) });
  } catch (e) {
    // Hết giờ: chưa biết backend đã làm xong hay chưa. Báo lỗi riêng, không coi là mất mạng,
    // để thao tác ghi không bị xếp hàng gửi lại một cách mù quáng.
    if ((e as { name?: string }).name === "TimeoutError") {
      throw new ApiError(408, "timeout", t.common.timeout, {}, true);
    }
    throw e;
  }
}

async function parse<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    onUnauthorized();
    throw new ApiError(401, "unauthenticated", t.common.sessionExpired);
  }
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ApiErrorBody | null;
    throw new ApiError(res.status, body?.code ?? "unknown", body?.message ?? t.common.genericError, body?.fields ?? {}, !!body?.retryable);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export async function apiGet<T>(path: string): Promise<T> {
  const h = await headers();
  let res: Response;
  try {
    res = await rawFetch(path, { headers: h });
  } catch (e) {
    if (e instanceof ApiError) throw e;
    throw new OfflineError("offline");
  }
  return parse<T>(res);
}

/** Ghi trực tiếp, báo lỗi ngay. Dùng cho lưu hóa đơn, xóa dữ liệu, cài đặt. */
export async function apiSend<T>(
  method: "POST" | "PUT" | "PATCH" | "DELETE",
  path: string,
  body?: unknown,
  idempotencyKey: string = crypto.randomUUID(),
  timeoutMs: number = TIMEOUT_MS,
): Promise<T> {
  const h = await headers({ "Content-Type": "application/json", "Idempotency-Key": idempotencyKey });
  let res: Response;
  try {
    res = await rawFetch(path, {
      method,
      headers: h,
      body: body === undefined ? undefined : JSON.stringify(body),
    }, timeoutMs);
  } catch (e) {
    if (e instanceof ApiError) throw e;
    throw new OfflineError("offline");
  }
  return parse<T>(res);
}

/**
 * Ghi có thể xếp hàng khi mất mạng (sửa tồn nhanh, tick giỏ).
 * Trả về "queued" để UI hiện "Sẽ đồng bộ khi có mạng".
 */
export async function apiSendQueued(
  method: QueuedWrite["method"],
  path: string,
  body?: unknown,
  label?: string,
): Promise<"sent" | "queued"> {
  const key = crypto.randomUUID();
  // Spec 1.5 mục 8: biết chắc mất mạng thì lưu vào hộp thư đi luôn, không chờ lỗi.
  if (!navigator.onLine) {
    await enqueue({ key, method, path, body, label });
    return "queued";
  }
  try {
    await apiSend(method, path, body, key);
    return "sent";
  } catch (e) {
    if (e instanceof OfflineError) {
      await enqueue({ key, method, path, body, label });
      return "queued";
    }
    throw e;
  }
}

export async function flushQueue() {
  await flush(async (w) =>
    rawFetch(w.path, {
      method: w.method,
      headers: await headers({ "Content-Type": "application/json", "Idempotency-Key": w.key }),
      body: w.body === undefined ? undefined : JSON.stringify(w.body),
    }),
  );
}

/** Gửi ảnh hóa đơn dạng multipart; ảnh đã được nén ở máy. */
export async function apiUploadReceipt<T>(receiptId: string, images: Blob[], lang: string): Promise<T> {
  const form = new FormData();
  form.append("receipt_id", receiptId);
  form.append("lang", lang);
  images.forEach((b, i) => form.append("images", b, `p${i + 1}.jpg`));
  const h = await headers();
  let res: Response;
  try {
    res = await rawFetch("/receipts/read", { method: "POST", headers: h, body: form }, UPLOAD_TIMEOUT_MS);
  } catch (e) {
    if (e instanceof ApiError) throw e;
    throw new OfflineError("offline");
  }
  return parse<T>(res);
}

/** Gửi ảnh hàng nhận dạng multipart cho Verify; ảnh đã được nén ở máy và không được lưu. */
export async function apiUploadImages<T>(path: string, images: Blob[]): Promise<T> {
  const form = new FormData();
  images.forEach((b, i) => form.append("images", b, `d${i + 1}.jpg`));
  const h = await headers();
  let res: Response;
  try {
    res = await rawFetch(path, { method: "POST", headers: h, body: form }, UPLOAD_TIMEOUT_MS);
  } catch (e) {
    if (e instanceof ApiError) throw e;
    throw new OfflineError("offline");
  }
  return parse<T>(res);
}
