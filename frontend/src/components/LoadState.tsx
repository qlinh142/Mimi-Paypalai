import { t } from "../i18n";
import { ApiError, OfflineError } from "../api/client";

/** A10: câu báo lỗi khi tải màn. Mất mạng, mạng chậm và lỗi máy chủ có câu riêng; không bao giờ để quay mãi. */
export function loadErrorText(e: unknown, notFound?: string): string {
  if (e instanceof OfflineError) return t.items.loadFailedBody;
  if (e instanceof ApiError) {
    if (e.status === 404 && notFound) return notFound;
    if (e.status === 408) return t.common.timeout;
  }
  return t.common.genericError;
}

type Q = { isPending: boolean; isError: boolean; error: unknown; refetch: () => unknown };

/**
 * A10: trạng thái tải dùng chung. Đang tải lần đầu: "Đang tải". Lỗi mà chưa có dữ liệu: câu báo lỗi kèm
 * nút Thử lại. Có dữ liệu cũ (đang xem bản đã lưu) thì không che màn. Trả null khi đã có dữ liệu.
 */
export function LoadState({ q, hasData, notFound }: { q: Q; hasData: boolean; notFound?: string }) {
  if (hasData) return null;
  if (q.isError) {
    const gone = notFound && q.error instanceof ApiError && q.error.status === 404;
    return (
      <div className="empty" role="alert">
        <p>{loadErrorText(q.error, notFound)}</p>
        {!gone && <button type="button" className="btn btn--quiet" onClick={() => q.refetch()}>{t.common.retry}</button>}
      </div>
    );
  }
  if (q.isPending) return <p className="hint" role="status">{t.common.loading}</p>;
  return null;
}
