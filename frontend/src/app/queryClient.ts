import { QueryClient } from "@tanstack/react-query";
import { createAsyncStoragePersister } from "@tanstack/query-async-storage-persister";
import { del, get, set } from "idb-keyval";

/**
 * Cache dữ liệu API trong IndexedDB để mất mạng vẫn xem được bản đã lưu (spec mục 11).
 * Service worker không cache /api; chỉ lớp này giữ dữ liệu, nên khi đăng xuất hoặc
 * "Xóa hết dữ liệu của mình" chỉ cần xóa một chỗ.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: 7 * 24 * 60 * 60 * 1000,
      // Lỗi 4xx (401, 403, 404, 422) gọi lại cũng vậy: chỉ thử lại lỗi mạng, hết giờ (408) và 5xx.
      retry: (count, err) => {
        const status = (err as { status?: number }).status ?? 0;
        return count < 2 && !(status >= 400 && status < 500 && status !== 408);
      },
      networkMode: "offlineFirst",
    },
    mutations: { networkMode: "always" },
  },
});

export const queryPersister = createAsyncStoragePersister({
  storage: {
    getItem: (k) => get<string>(k).then((v) => v ?? null),
    setItem: (k, v) => set(k, v),
    removeItem: (k) => del(k),
  },
  key: "mimi.queryCache.v1",
  throttleTime: 1000,
});
