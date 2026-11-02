/**
 * A3: kiểm tra lúc build rằng kiểu viết tay trong types.ts khớp tên trường với OpenAPI.
 * schema.d.ts sinh từ backend/api/openapi.yaml bằng `npm run gen:api`; CI chạy lại lệnh đó
 * và báo lỗi nếu file sinh ra khác bản đã commit. Trường có ở một bên mà thiếu ở bên kia
 * làm `tsc` báo lỗi ngay tại dòng của kiểu đó, kèm tên trường.
 *
 * Chỉ so tên trường: OpenAPI chưa ghi `required` nên mọi trường đều tùy chọn, so kiểu
 * từng trường sẽ báo nhầm. File này không xuất gì vào bundle.
 */
import type { components } from "./schema";
import type {
  Assignment, Basket, BasketPrices, Cart, Delegated, Forecast, HistoryReceipt, HomeSummary, Household, Item, ItemPriceHistory,
  Mandate, Occurrence, Order, OrderLine, PlanMeta, PlanPrices, PlanState, PriceObservation, ReceiptSummary, Recurring, Stock,
} from "./types";

type S = components["schemas"];
type Keys<T> = T extends unknown ? keyof T & string : never;
/** Trường app tự gắn, không đến từ API. */
type ClientOnly = "category_label";
/** never khi hai bên cùng tên trường; khác thì là tên trường bị lệch. */
type Drift<App, Api> =
  | Exclude<Keys<App>, Keys<Required<Api>> | ClientOnly>
  | Exclude<Keys<Required<Api>>, Keys<App>>;
type NoDrift<T extends never> = T;

export type Contract = [
  NoDrift<Drift<Assignment, S["Assignment"]>>,
  NoDrift<Drift<Basket, S["Basket"]>>,
  NoDrift<Drift<BasketPrices, S["BasketPrices"]>>,
  NoDrift<Drift<Cart, S["Cart"]>>,
  NoDrift<Drift<Delegated, S["Delegated"]>>,
  NoDrift<Drift<Forecast, S["Forecast"]>>,
  NoDrift<Drift<HistoryReceipt, S["HistoryReceipt"]>>,
  NoDrift<Drift<HomeSummary, S["HomeSummary"]>>,
  NoDrift<Drift<Household, S["Household"]>>,
  NoDrift<Drift<Item, S["Item"]>>,
  NoDrift<Drift<ItemPriceHistory, S["ItemPriceHistory"]>>,
  NoDrift<Drift<Mandate, S["Mandate"]>>,
  NoDrift<Drift<Occurrence, S["Occurrence"]>>,
  NoDrift<Drift<Order, S["Order"]>>,
  NoDrift<Drift<OrderLine, S["OrderLine"]>>,
  NoDrift<Drift<PlanMeta, S["PlanMeta"]>>,
  NoDrift<Drift<PlanPrices, S["PlanPrices"]>>,
  NoDrift<Drift<PlanState, S["PlanState"]>>,
  NoDrift<Drift<PriceObservation, S["PriceObservation"]>>,
  NoDrift<Drift<ReceiptSummary, S["ReceiptSummary"]>>,
  NoDrift<Drift<Recurring, S["Recurring"]>>,
  NoDrift<Drift<Stock, S["Stock"]>>,
];
