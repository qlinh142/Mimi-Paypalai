import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { categoryLabel } from "../i18n";
import { apiGet, apiSend, apiSendQueued, apiUploadReceipt } from "./client";
import type {
  Cart, HomeSummary, Item, ItemDetail, ItemsQuery, Me, Notification, PlanPrices, PlanState, QuickStock,
  ReceiptDraft, SaveReceiptRequest, SaveReceiptResult, SettingsPatch, SpendingReport,
} from "./types";

/** API chỉ trả category_key; nhãn hiển thị lấy theo ngôn ngữ giao diện. */
function withLabel<T extends Item>(i: T): T {
  return { ...i, category_label: categoryLabel(i.category_key) };
}

/** Khóa cache gom một chỗ để invalidate đúng phạm vi sau mỗi thao tác ghi. */
export const qk = {
  me: ["me"] as const,
  home: ["home"] as const,
  items: (q?: ItemsQuery) => (q ? (["items", q] as const) : (["items"] as const)),
  item: (key: string) => ["item", key] as const,
  plan: ["plan"] as const,
  spending: (month: string) => ["spending", month] as const,
  notifications: ["notifications"] as const,
  planPrices: ["plan", "prices"] as const,
};

export const useMe = () => useQuery({ queryKey: qk.me, queryFn: () => apiGet<Me>("/me") });

export const useHome = () => useQuery({ queryKey: qk.home, queryFn: () => apiGet<HomeSummary>("/home") });

export const useItems = (q: ItemsQuery) =>
  useQuery({
    queryKey: qk.items(q),
    // API trả { items: [...] } (api/openapi.yaml).
    queryFn: async () =>
      (await apiGet<{ items: Item[] }>(`/items?tab=${q.tab}&horizon=${q.horizon}`)).items.map(withLabel),
    placeholderData: (prev) => prev, // đổi chip 3/7/14 không nháy trắng
  });

export const useItem = (key: string) =>
  useQuery({
    queryKey: qk.item(key),
    queryFn: async () => withLabel(await apiGet<ItemDetail>(`/items/${encodeURIComponent(key)}`)),
    enabled: key !== "",
  });

const itemPath = (key: string) => `/items/${encodeURIComponent(key)}`;

/**
 * "Còn nhiều" / "Sắp hết" / "Đã hết". acted_at là lúc bấm, để thao tác xếp hàng khi mất mạng
 * gửi muộn không đè lên thao tác mới hơn. Mimi không tự đổi tồn nếu người dùng chưa bấm (spec mục 4).
 */
export function useQuickStock() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ item, quick }: { item: Item; quick: QuickStock }) => {
      const result = await apiSendQueued("PATCH", `${itemPath(item.key)}/stock`, {
        quick,
        acted_at: new Date().toISOString(),
      });
      return { result, previous: item };
    },
    onSettled: (_d, _e, v) => {
      qc.invalidateQueries({ queryKey: qk.items() });
      qc.invalidateQueries({ queryKey: qk.item(v.item.key) });
      qc.invalidateQueries({ queryKey: qk.home });
    },
  });
}

/** Hoàn tác: xóa bản sửa tay, không gửi lại cả tài liệu món. */
export function useUndoStock() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (previous: Item) =>
      apiSendQueued("DELETE", `${itemPath(previous.key)}/override?acted_at=${encodeURIComponent(new Date().toISOString())}`),
    onSettled: (_d, _e, previous) => {
      qc.invalidateQueries({ queryKey: qk.items() });
      qc.invalidateQueries({ queryKey: qk.item(previous.key) });
      qc.invalidateQueries({ queryKey: qk.home });
    },
  });
}

export function useReadReceipt() {
  return useMutation({
    mutationFn: (v: { receiptId: string; images: Blob[]; lang: string }) =>
      apiUploadReceipt<ReceiptDraft>(v.receiptId, v.images, v.lang),
  });
}

/** Lưu hóa đơn: gửi trực tiếp, không xếp hàng. receipt_id làm khóa idempotent. */
export function useSaveReceipt() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: SaveReceiptRequest) => apiSend<SaveReceiptResult>("POST", "/receipts", req, req.receipt_id),
    onSuccess: () => qc.invalidateQueries(),
  });
}

/**
 * Dựng thân POST /receipts từ bản nháp. Chỉ gửi các trường backend nhận (SaveReceipt);
 * gửi kèm trường lạ như sum_vnd hay total_mismatch sẽ bị trả 400.
 */
export function toSaveRequest(
  draft: ReceiptDraft,
  opts: { purchasedOn: string; usePrediction: boolean; confirmDuplicate: boolean },
): SaveReceiptRequest {
  return {
    receipt_id: draft.receipt_id,
    store: draft.store ?? "",
    purchased_on: opts.purchasedOn,
    currency: draft.currency || "VND",
    discount_total: draft.discount_total ?? "",
    lines: draft.lines.map((l) => ({
      line_id: l.line_id,
      name: l.name,
      qty: l.qty,
      unit: l.unit,
      amount: l.amount,
      category_key: l.category_key ?? "",
      group_key: l.group_key ?? "",
      habit: l.habit,
    })),
    use_for_prediction: opts.usePrediction,
    confirm_duplicate: opts.confirmDuplicate,
  };
}

// ================= Kế hoạch mua sắm =================

export const usePlan = () => useQuery({ queryKey: qk.plan, queryFn: () => apiGet<Cart>("/plan") });

export const usePlanPrices = (enabled: boolean) =>
  useQuery({ queryKey: qk.planPrices, queryFn: () => apiGet<PlanPrices>("/plan/prices"), enabled });

/** Thêm hoặc bỏ món khỏi giỏ. Gửi trực tiếp (không xếp hàng) vì cần giá và ước tính mới ngay. */
export function usePlanItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ key, add }: { key: string; add: boolean }): Promise<Cart | null> => {
      const path = `/plan/items/${encodeURIComponent(key)}`;
      if (add) return apiSend<Cart>("PUT", path);
      await apiSend<void>("DELETE", path);
      return null;
    },
    onSuccess: (data, v) => {
      if (v.add && data) qc.setQueryData(qk.plan, data);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: qk.plan });
      qc.invalidateQueries({ queryKey: qk.home });
    },
  });
}

export function useSavePlanState() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (st: PlanState) => apiSend<PlanState>("PUT", "/plan/state", st),
    onSuccess: (st) => qc.setQueryData<Cart>(qk.plan, (c) => (c ? { ...c, state: st } : c)),
  });
}

// ================= Sổ chi tiêu, Thông báo, Cài đặt =================

export const useSpending = (month: string) =>
  useQuery({
    queryKey: qk.spending(month),
    queryFn: () => apiGet<SpendingReport>(`/spending?month=${month}`),
    placeholderData: (prev) => prev, // đổi tháng không nháy trắng
  });

export const useNotifications = () =>
  useQuery({ queryKey: qk.notifications, queryFn: async () => (await apiGet<{ items: Notification[] }>("/notifications")).items });

export function useSaveSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: SettingsPatch) => apiSend<Record<string, unknown>>("PUT", "/settings", patch),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.me });
      qc.invalidateQueries({ queryKey: qk.home });
      qc.invalidateQueries({ queryKey: qk.items() });
    },
  });
}
