import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { categoryLabel } from "../i18n";
import { ApiError, UPLOAD_TIMEOUT_MS, apiGet, apiSend, apiSendQueued, apiUploadImages, apiUploadReceipt } from "./client";
import { currentPlanId, planQuery, useCurrentPlanId } from "../lib/currentPlan";
import type {
  Assignment, HistoryReceipt, ReceiptDetail, Basket, BasketPrices, MarketPrices, MarketRefresh, Cart, Delegated, DeliveryObservation, Forecast, HomeSummary, Household, Item, ItemDetail,
  ItemPriceHistory, ItemsQuery, Mandate, Me, Notification, Occurrence, Order, PlanLinePatch, PlanMeta, PlanPrices, PlanState,
  QuickStock, ReceiptDraft, ReceiptDraftLine, Recurring, RecurringInput, RefundReason, SaveReceiptRequest, SaveReceiptResult, SettingsPatch,
  SpendingReport, TaxonomyGroup,
} from "./types";

/** API chỉ trả category_key; nhãn hiển thị lấy theo ngôn ngữ giao diện. */
function withLabel<T extends Item>(i: T): T {
  return { ...i, category_label: categoryLabel(i.category_key) };
}

/** Khóa cache gom một chỗ để invalidate đúng phạm vi sau mỗi thao tác ghi. */
export const qk = {
  receiptHistory: ["receipts", "history"] as const,
  receipt: (id: string) => ["receipts", id] as const,
  me: ["me"] as const,
  home: ["home"] as const,
  items: (q?: ItemsQuery) => (q ? (["items", q] as const) : (["items"] as const)),
  item: (key: string) => ["item", key] as const,
  plan: ["plan"] as const,
  cart: (planId: string) => ["plan", "cart", planId] as const,
  spending: (month: string) => ["spending", month] as const,
  notifications: ["notifications"] as const,
  planPrices: ["plan", "prices"] as const,
  plans: ["plans"] as const,
  recurring: ["recurring"] as const,
  forecast: ["forecast"] as const,
  household: ["household"] as const,
  delegated: ["plan", "delegated"] as const,
  priceHistory: (key: string) => ["item", key, "prices"] as const,
  taxonomy: (q: string) => ["taxonomy", q] as const,
  mandate: ["mandate"] as const,
  orders: ["orders"] as const,
  order: (id: string) => ["orders", id] as const,
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

/** SET số lượng thực tế: tạo Confirmed Inventory mới, không sửa lịch sử mua (Mimi 2.0 mục 4). */
export function useSetQuantity() {
  const qc = useQueryClient();
  return useMutation({
    // action (0.1 mục 14): set (mặc định), increase / decrease (cộng / trừ vào tồn ước tính), mark_out.
    mutationFn: (v: { key: string; qty: number; unit: string; action?: "set" | "increase" | "decrease" | "mark_out" }) =>
      apiSend<Item>("PUT", `${itemPath(v.key)}/quantity`, { qty: v.qty, unit: v.unit, ...(v.action ? { action: v.action } : {}) }),
    onSettled: (_d, _e, v) => {
      qc.invalidateQueries({ queryKey: qk.items() });
      qc.invalidateQueries({ queryKey: qk.item(v.key) });
    },
  });
}

/** PATCH /items/{key}: sửa tên chuẩn hóa hoặc chuyển sang nhóm đúng (0.1 mục 3.2). */
export function useCorrectItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { key: string; name?: string; group_key?: string }) =>
      apiSend<Item>("PATCH", itemPath(v.key), { ...(v.name ? { name: v.name } : {}), ...(v.group_key ? { group_key: v.group_key } : {}) }),
    onSettled: (_d, _e, v) => {
      qc.invalidateQueries({ queryKey: qk.items() });
      qc.invalidateQueries({ queryKey: qk.item(v.key) });
      qc.invalidateQueries({ queryKey: qk.plan });
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
  opts: {
    purchasedOn: string; usePrediction: boolean; confirmDuplicate: boolean; planId?: string; assignmentId?: string;
    /** B3: phần chia từng dòng vào kế hoạch (theo line_id), như màn Xác nhận đang hiện. */
    linePlans?: Map<string, { plan_id: string; qty: number }[]>;
  },
  lines: ReceiptDraftLine[] = draft.lines,
): SaveReceiptRequest {
  return {
    ...(opts.planId && opts.planId !== "default" ? { source_plan_id: opts.planId } : {}),
    ...(opts.assignmentId ? { assignment_id: opts.assignmentId } : {}),
    receipt_id: draft.receipt_id,
    store: draft.store ?? "",
    purchased_on: opts.purchasedOn,
    currency: draft.currency || "VND",
    discount_total: draft.discount_total ?? "",
    lines: lines.map((l) => ({
      line_id: l.line_id,
      name: l.name,
      qty: l.qty,
      unit: l.unit,
      amount: l.amount,
      category_key: l.category_key ?? "",
      group_key: l.group_key ?? "",
      habit: l.habit,
      // Kết quả chuẩn hóa một lần lúc đọc (1.1): gửi lại nguyên vẹn để server tính lượng canonical.
      raw_name: l.raw_name,
      brand: l.brand,
      product_name: l.product_name,
      source_language: l.source_language,
      original_name: l.original_name,
      package_count: l.package_count,
      package_size: l.package_size,
      package_unit: l.package_unit,
      ...(opts.linePlans?.has(l.line_id) ? { plans: opts.linePlans.get(l.line_id) } : {}),
    })),
    use_for_prediction: opts.usePrediction,
    confirm_duplicate: opts.confirmDuplicate,
  };
}

// ================= Kế hoạch mua sắm =================

// Mọi route /plan/... đi theo Kế hoạch đang xem (lib/currentPlan); Kế hoạch chính không gửi plan_id.

export function usePlan() {
  const id = useCurrentPlanId();
  return useQuery({ queryKey: qk.cart(id), queryFn: () => apiGet<Cart>(`/plan${planQuery(id)}`) });
}

export function usePlanPrices(enabled: boolean) {
  const id = useCurrentPlanId();
  return useQuery({ queryKey: [...qk.planPrices, id], queryFn: () => apiGet<PlanPrices>(`/plan/prices${planQuery(id)}`), enabled });
}

/** Thêm hoặc bỏ món khỏi giỏ. Gửi trực tiếp (không xếp hàng) vì cần giá và ước tính mới ngay. */
export function usePlanItem() {
  const qc = useQueryClient();
  return useMutation({
    // body: món chưa có trong Đồ dùng (nhóm taxonomy hoặc "custom-..."), 3.1 mục 3A.4.
    // planId: thêm vào một Kế hoạch đích cụ thể (3.1 mục 3A.4), mặc định là Kế hoạch đang xem.
    mutationFn: async ({ key, add, body, planId }: { key: string; add: boolean; body?: { name?: string; qty?: number; unit?: string }; planId?: string }): Promise<Cart | null> => {
      const path = `/plan/items/${encodeURIComponent(key)}${planQuery(planId ?? currentPlanId())}`;
      if (add) return apiSend<Cart>("PUT", path, body);
      await apiSend<void>("DELETE", path);
      return null;
    },
    onSuccess: (data, v) => {
      if (v.add && data) qc.setQueryData(qk.cart(v.planId ?? currentPlanId()), data);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: qk.plan });
      qc.invalidateQueries({ queryKey: qk.home });
    },
  });
}

/** Sửa số lượng, đơn vị, need-by, ưu tiên của một dòng Kế hoạch. */
export function usePlanLine() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { key: string; patch: PlanLinePatch }) =>
      apiSend<Cart>("PATCH", `/plan/items/${encodeURIComponent(v.key)}${planQuery(currentPlanId())}`, v.patch),
    onSuccess: (data) => qc.setQueryData(qk.cart(currentPlanId()), data),
    onSettled: () => qc.invalidateQueries({ queryKey: qk.plan }),
  });
}

export function useSavePlanState() {
  const qc = useQueryClient();
  return useMutation({
    // A1: gửi kèm rev đã đọc (base_rev). Máy khác lưu trước thì máy chủ trả 409: tải lại kế hoạch
    // để màn hình hiện bản mới, lỗi mang câu báo của máy chủ.
    mutationFn: ({ rev, base_rev, ...st }: PlanState) =>
      apiSend<PlanState>("PUT", `/plan/state${planQuery(currentPlanId())}`, { ...st, base_rev: base_rev ?? rev }),
    onSuccess: (st) => {
      qc.setQueryData<Cart>(qk.cart(currentPlanId()), (c) => (c ? { ...c, state: st } : c));
      qc.invalidateQueries({ queryKey: qk.plans });
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) qc.invalidateQueries({ queryKey: qk.cart(currentPlanId()) });
    },
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

/** B9: đánh dấu một thông báo sự kiện đã đọc. */
export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiSend<void>("POST", `/notifications/${encodeURIComponent(id)}/read`),
    onSettled: () => qc.invalidateQueries({ queryKey: qk.notifications }),
  });
}

export function useSaveSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patch: SettingsPatch) => apiSend<Record<string, unknown>>("PUT", "/settings", patch),
    onSuccess: (saved, patch) => {
      // Ghi ngay vào bản nhớ: app có thể tải lại trang (đổi ngôn ngữ, tiền tệ) trước khi kịp tải lại /me.
      // Lấy cài đặt máy chủ trả về (đủ mọi ô), không chỉ phần vừa đổi: bản nhớ có thể là bản cũ
      // khôi phục từ máy, ghép phần vừa đổi vào đó sẽ làm ô khác (tiền tệ) trông như mới mà sai.
      qc.setQueryData<Me>(qk.me, (old) => {
        if (!old) return old;
        const { display_name, relations: _rel, ...rest } = patch;
        const { display_name: savedName, ...savedSettings } = (saved ?? {}) as Partial<Me["settings"]> & { display_name?: string };
        return {
          ...old,
          display_name: display_name ?? savedName ?? old.display_name,
          settings: { ...old.settings, ...savedSettings, ...rest },
        };
      });
      qc.invalidateQueries({ queryKey: qk.me });
      // Đổi nước thì bảng giá online đổi theo nước.
      if (patch.country) qc.invalidateQueries({ queryKey: ["plan", "market-prices"] });
      qc.invalidateQueries({ queryKey: qk.home });
      qc.invalidateQueries({ queryKey: qk.items() });
    },
  });
}

// ================= Mimi Online: Mandate, giỏ online, PayPal, giao hàng, Verify, Refund =================

export const useMandate = () => useQuery({ queryKey: qk.mandate, queryFn: () => apiGet<Mandate>("/mandate") });

export function useSaveMandate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (m: Mandate) => apiSend<Mandate>("PUT", "/mandate", m),
    onSuccess: (m) => qc.setQueryData(qk.mandate, m),
  });
}

/** Giỏ online dựng từ Kế hoạch, kèm kết quả Household Mandate. Chưa tạo giao dịch. */
export const useBasket = (keys: string[], enabled = true) =>
  useQuery({
    queryKey: ["basket", keys] as const,
    queryFn: () => apiSend<Basket>("POST", "/orders/basket", { keys }),
    enabled,
  });

export const useOrders = () =>
  useQuery({ queryKey: qk.orders, queryFn: async () => (await apiGet<{ orders: Order[] }>("/orders")).orders });

export const useOrder = (id: string) =>
  useQuery({ queryKey: qk.order(id), queryFn: () => apiGet<Order>(`/orders/${encodeURIComponent(id)}`), enabled: id !== "" });

/** Mọi thao tác trên đơn trả về đơn mới; cập nhật cache và làm mới các màn liên quan. */
function useOrderAction<V>(fn: (v: V) => Promise<Order>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: (o) => {
      qc.setQueryData(qk.order(o.id), o);
      qc.invalidateQueries({ queryKey: qk.orders });
      qc.invalidateQueries({ queryKey: qk.plan });
      qc.invalidateQueries({ queryKey: qk.items() });
      qc.invalidateQueries({ queryKey: ["spending"] });
      qc.invalidateQueries({ queryKey: qk.home });
    },
  });
}

const orderPath = (id: string) => `/orders/${encodeURIComponent(id)}`;

export const useCheckout = () =>
  useOrderAction((v: { keys: string[]; approve: boolean }) => apiSend<Order>("POST", "/orders", v));
export const useCaptureOrder = () => useOrderAction((id: string) => apiSend<Order>("POST", `${orderPath(id)}/capture`));
export const useAdvanceOrder = () => useOrderAction((id: string) => apiSend<Order>("POST", `${orderPath(id)}/advance`));
export const useVerifyOrder = () =>
  useOrderAction((v: { id: string; actual: Record<string, number> }) =>
    apiSend<Order>("POST", `${orderPath(v.id)}/verify`, { actual: v.actual }));
export const useRefundOrder = () =>
  useOrderAction((v: { id: string; reason: RefundReason; amount_vnd: number; note: string }) =>
    apiSend<Order>("POST", `${orderPath(v.id)}/refund`, { reason: v.reason, amount_vnd: v.amount_vnd, note: v.note }));

export function useReadDelivery() {
  return useMutation({
    mutationFn: async (v: { id: string; images: Blob[] }) =>
      (await apiUploadImages<{ items: DeliveryObservation[] }>(`${orderPath(v.id)}/delivery/read`, v.images)).items,
  });
}

// ================= Bộ spec Mimi 1.0 =================

/** 0.2: tìm nhóm sản phẩm theo nhãn (không dấu). */
/** Cả bộ taxonomy 0.2 (300 nhóm): tra nhãn hiển thị theo group_key, không gọi AI (1.1). */
export function useTaxonomyLabels() {
  const q = useQuery({
    queryKey: qk.taxonomy(`all:${uiLangParam()}`),
    queryFn: async () => (await apiGet<{ groups: TaxonomyGroup[] }>(`/taxonomy?lang=${uiLangParam()}`)).groups,
    staleTime: Infinity,
  });
  const map = new Map((q.data ?? []).map((g) => [g.key, g.label]));
  return (key?: string | null) => (key ? map.get(key.replace(/_/g, "-")) ?? "" : "");
}

export const useTaxonomySearch = (q: string) =>
  useQuery({
    queryKey: qk.taxonomy(q),
    queryFn: async () =>
      (await apiGet<{ groups: TaxonomyGroup[] }>(`/taxonomy?q=${encodeURIComponent(q)}&limit=8&lang=${uiLangParam()}`)).groups,
    enabled: q.trim().length >= 2,
    staleTime: 3_600_000,
  });

function uiLangParam() {
  return document.documentElement.lang === "vi" ? "vi" : "en";
}

/** 3.1: danh sách Kế hoạch. */
export const usePlans = () =>
  useQuery({ queryKey: qk.plans, queryFn: async () => (await apiGet<{ plans: PlanMeta[] }>("/plans")).plans });

/**
 * Mimi 1.5 (góp ý 31/10 16:04): "Xóa" Giỏ mua sắm. Máy chủ giữ sẵn Kế hoạch mặc định, nên xóa ở đây
 * là bỏ hết món và xóa ngày giờ, nơi mua; Giỏ trống thì tự ẩn khỏi danh sách.
 */
export function useClearDefaultPlan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (keys: string[]) => {
      for (const k of keys) await apiSend<void>("DELETE", `/plan/items/${encodeURIComponent(k)}${planQuery("default")}`);
      await apiSend<PlanState>("PUT", `/plan/state${planQuery("default")}`, { buy_date: "", buy_hour: null, buy_minute: null, trip_note: "" });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: qk.plan });
      qc.invalidateQueries({ queryKey: qk.plans });
      qc.invalidateQueries({ queryKey: qk.home });
    },
  });
}

/** Mimi 1.5: đọc giỏ của nhiều Kế hoạch một lúc (thẻ ở màn danh sách Kế hoạch). Cùng khóa cache với usePlan. */
export function usePlanCarts(ids: string[]) {
  return useQueries({
    queries: ids.map((id) => ({ queryKey: qk.cart(id), queryFn: () => apiGet<Cart>(`/plan${planQuery(id)}`) })),
  });
}

export function usePlanMeta() {
  const qc = useQueryClient();
  const done = () => {
    qc.invalidateQueries({ queryKey: qk.plans });
    qc.invalidateQueries({ queryKey: qk.plan });
  };
  return {
    create: useMutation({ mutationFn: (name: string) => apiSend<PlanMeta>("POST", "/plans", { name }), onSettled: done }),
    update: useMutation({
      mutationFn: (v: { id: string; name?: string; status?: PlanMeta["status"]; icon?: string }) =>
        apiSend<PlanMeta>("PATCH", `/plans/${encodeURIComponent(v.id)}`, { name: v.name, status: v.status, icon: v.icon }),
      onSettled: done,
    }),
    remove: useMutation({ mutationFn: (id: string) => apiSend<void>("DELETE", `/plans/${encodeURIComponent(id)}`), onSettled: done }),
  };
}

/** 8.1: ước tính giỏ theo lịch sử giá, độ phủ từng nơi. */
export function useBasketPrices(enabled: boolean) {
  const id = useCurrentPlanId();
  return useQuery({
    queryKey: ["plan", "price-history", id],
    queryFn: () => apiGet<BasketPrices>(`/plan/price-history${planQuery(id)}`),
    enabled,
  });
}

/** ADR-004: giá đang bán của các món trong Kế hoạch (bảng giá chung, cập nhật mỗi đêm). */
export function useMarketPrices(enabled: boolean) {
  const id = useCurrentPlanId();
  return useQuery({
    queryKey: ["plan", "market-prices", id],
    queryFn: () => apiGet<MarketPrices>(`/plan/market-prices${planQuery(id)}`),
    enabled,
    staleTime: 30 * 60_000, // bảng chỉ đổi mỗi đêm hoặc khi bấm "Cập nhật giá"
  });
}

/** Nút "Cập nhật giá": máy chủ tra lại giá các món của kế hoạch đang mở (có thể mất 20 đến 40 giây). */
export function useRefreshMarketPrices() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiSend<MarketRefresh>("POST", `/plan/market-prices/refresh${planQuery(currentPlanId())}`, undefined, crypto.randomUUID(), UPLOAD_TIMEOUT_MS),
    onSuccess: (d) => qc.setQueryData<MarketPrices>(["plan", "market-prices", currentPlanId()], { items: d.items, refresh: d.refresh }),
    // Hết lượt hoặc lỗi: tải lại để số lần còn lại đúng với máy chủ.
    onError: () => qc.invalidateQueries({ queryKey: ["plan", "market-prices"] }),
  });
}

export const useItemPriceHistory = (key: string) =>
  useQuery({
    queryKey: qk.priceHistory(key),
    queryFn: () => apiGet<ItemPriceHistory>(`${itemPath(key)}/price-history`),
    enabled: key !== "",
  });

/** 2.1: khoản định kỳ và Dự báo 30 ngày. */
export const useRecurring = () =>
  useQuery({ queryKey: qk.recurring, queryFn: () => apiGet<{ items: Recurring[]; monthly_total: number }>("/recurring") });

export const useForecast = () => useQuery({ queryKey: qk.forecast, queryFn: () => apiGet<Forecast>("/forecast") });

export function useRecurringActions() {
  const qc = useQueryClient();
  // Trả / bỏ qua một kỳ đổi cả Spending, Forecast và Trang chủ (2.1 mục 9).
  const done = () => {
    qc.invalidateQueries({ queryKey: qk.recurring });
    qc.invalidateQueries({ queryKey: qk.forecast });
    qc.invalidateQueries({ queryKey: ["spending"] });
    qc.invalidateQueries({ queryKey: qk.home });
  };
  return {
    create: useMutation({ mutationFn: (v: RecurringInput) => apiSend<Recurring>("POST", "/recurring", v), onSettled: done }),
    update: useMutation({
      mutationFn: (v: { id: string; patch: RecurringInput }) => apiSend<Recurring>("PATCH", `/recurring/${encodeURIComponent(v.id)}`, v.patch),
      onSettled: done,
    }),
    remove: useMutation({ mutationFn: (id: string) => apiSend<void>("DELETE", `/recurring/${encodeURIComponent(id)}`), onSettled: done }),
    pay: useMutation({
      mutationFn: (occId: string) => apiSend<Occurrence>("POST", `/recurring/occurrences/${encodeURIComponent(occId)}/pay`),
      onSettled: done,
    }),
    skip: useMutation({
      mutationFn: (occId: string) => apiSend<Occurrence>("POST", `/recurring/occurrences/${encodeURIComponent(occId)}/skip`),
      onSettled: done,
    }),
  };
}

/** 4.1: nhà, mã mời, thành viên. */
export const useHousehold = () => useQuery({ queryKey: qk.household, queryFn: () => apiGet<Household>("/households") });

/** B10: xóa tài khoản. 403 reauth_required nghĩa là cần đăng nhập lại rồi gọi lại. */
export function useDeleteAccount() {
  return useMutation({ mutationFn: () => apiSend<void>("DELETE", "/me") });
}

export function useHouseholdActions() {
  const qc = useQueryClient();
  const done = () => {
    qc.invalidateQueries({ queryKey: qk.household });
    qc.invalidateQueries({ queryKey: qk.me });
  };
  return {
    invite: useMutation({ mutationFn: () => apiSend<{ code: string }>("POST", "/households/invites"), onSettled: done }),
    join: useMutation({ mutationFn: (code: string) => apiSend<Household>("POST", "/households/join", { code }), onSettled: done }),
    leave: useMutation({ mutationFn: () => apiSend<void>("POST", "/households/leave"), onSettled: () => qc.invalidateQueries() }),
    rename: useMutation({ mutationFn: (name: string) => apiSend<Household>("PATCH", "/households", { name }), onSettled: done }),
    removeMember: useMutation({
      mutationFn: (uid: string) => apiSend<void>("DELETE", `/households/members/${encodeURIComponent(uid)}`),
      onSettled: done,
    }),
  };
}

/** 4.1 mục 9: gửi Kế hoạch cho người nhà mua hộ, hủy, xem vòng được nhờ. */
export const useDelegated = (enabled = true) =>
  useQuery({ queryKey: qk.delegated, queryFn: () => apiGet<Delegated>("/plan/delegated"), enabled });

export function useAssignments() {
  const qc = useQueryClient();
  const done = () => qc.invalidateQueries({ queryKey: qk.plan });
  return {
    assign: useMutation({
      mutationFn: (v: { buyer_uid: string; keys: string[]; note?: string }) =>
        apiSend<Assignment>("POST", "/households/shopping-assignments", { ...v, plan_id: currentPlanId() }),
      onSettled: done,
    }),
    /** Spec 1.5 mục 2: kết quả đi mua hộ (món đã tick, giá, ghi chú, có quét bill hay không). */
    complete: useMutation({
      mutationFn: (v: { id: string; result: unknown }) =>
        apiSend<Assignment>("POST", `/households/shopping-assignments/${encodeURIComponent(v.id)}/complete`, v.result),
      onSettled: done,
    }),
    /** Spec 1.5 câu 5: người được nhờ từ chối. */
    decline: useMutation({
      mutationFn: (id: string) => apiSend<Assignment>("POST", `/households/shopping-assignments/${encodeURIComponent(id)}/decline`),
      onSettled: done,
    }),
    cancel: useMutation({
      mutationFn: (id: string) => apiSend<Assignment>("POST", `/households/shopping-assignments/${encodeURIComponent(id)}/cancel`),
      onSettled: done,
    }),
  };
}

/** Spec 1.5 mục 4: Lịch sử mua hàng (mọi tháng) và Chi tiết hóa đơn. */
export const useReceiptHistory = () =>
  useQuery({ queryKey: qk.receiptHistory, queryFn: async () => (await apiGet<{ receipts: HistoryReceipt[] }>("/receipts/history")).receipts });
export const useReceiptDetail = (id: string) =>
  useQuery({ queryKey: qk.receipt(id), queryFn: () => apiGet<ReceiptDetail>(`/receipts/${encodeURIComponent(id)}`) });
