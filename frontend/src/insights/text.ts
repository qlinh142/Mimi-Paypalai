import { uiLang } from "../i18n";

/** Chữ của "Mimi nhận thấy" và các bảng phân tích nhỏ (đặc tả Contextual Insights 12/11, mục B3). */
const vi = {
  header: "Mimi nhận thấy",
  open: "Xem phân tích",
  dismiss: "Ẩn nhận xét",
  back: "Quay lại",
  loading: "Đang mở phân tích…",
  error: "Chưa mở được bảng phân tích.",
  retry: "Thử lại",
  noData: "Chưa đủ dữ liệu để phân tích",
  updated: (d: string) => `Cập nhật lần cuối ${d}`,
  price: {
    text: (name: string, pct: number) => `Giá **${name}** đang cao hơn **${pct}%** so với lần mua gần nhất.`,
    sub: (cur: string, last: string) => `Đang bán ${cur} · Lần trước ${last}`,
    title: (name: string) => `Giá ${name}`,
    sheetSub: "Giá nhà mình đã trả so với giá đang bán tham khảo",
    kLast: "Lần mua gần nhất", kNow: "Đang bán rẻ nhất", per: (u: string) => `đ/${u}`, kDiff: "Chênh lệch", kPct: "Tăng (%)",
    paidLine: "Giá đã trả theo thời gian", offers: "Giá đang bán từng nơi (tham khảo)", grid: "Chi tiết và nguồn",
    paid: "Đã mua", offer: "Đang bán",
    receipt: "Hóa đơn của nhà mình",
    note: "Giá đang bán là giá tham khảo Mimi tra trên trang bán hàng, chưa phải giá thanh toán. Đơn giá đã quy về cùng quy cách.",
  },
  budget: {
    over: (v: string) => `Thêm kế hoạch này thì tháng này **vượt ngân sách ${v}**.`,
    left: (v: string) => `Sau kế hoạch này, tháng này còn **${v}** ngân sách.`,
    sub: (plan: string) => `Kế hoạch dự kiến ${plan} · đã tính các khoản định kỳ còn phải trả`,
    missing: (n: number) => `${n} món chưa có giá, chưa tính vào dự kiến.`,
    title: "Ngân sách tháng này",
    sheetSub: "Đã chi, định kỳ sắp trả và kế hoạch này so với ngân sách tháng",
    kBudget: "Ngân sách tháng", kSpent: "Đã chi", kBills: "Định kỳ còn phải trả", kPlan: "Kế hoạch này", kLeft: "Còn lại", kOver: "Vượt",
    parts: "Ngân sách tháng được dùng thế nào", lines: "Các món trong kế hoạch",
    note: "Dự kiến tính theo giá gần nhất nhà mình đã trả. Phí giao hàng, thuế (nếu có) chưa gồm.",
  },
  trend: {
    title: (cat: string) => `Chi cho ${cat}`,
    sheetSub: "So với tháng trước, và các món làm thay đổi",
    kNow: "Tháng này", kPrev: "Tháng trước", kDiff: "Chênh lệch",
    months: "6 tháng gần đây", items: "Các món tháng này",
  },
  f: {
    type: "Loại", date: "Ngày", store: "Nơi bán", size: "Quy cách", price: "Giá", unitPrice: "Đơn giá", source: "Nguồn", on: "Ngày tra",
    part: "Phần", amount: "Số tiền", name: "Món", qty: "Số lượng", lineTotal: "Thành tiền", month: "Tháng",
  },
  unit: (u: string) => (u === "item" ? "cái" : u),
};

const en: typeof vi = {
  header: "Mimi noticed",
  open: "View analysis",
  dismiss: "Hide",
  back: "Back",
  loading: "Opening analysis…",
  error: "Couldn't open the analysis.",
  retry: "Try again",
  noData: "Not enough data to analyse yet",
  updated: (d) => `Last updated ${d}`,
  price: {
    text: (name, pct) => `The current price of **${name}** is **${pct}%** higher than your last purchase.`,
    sub: (cur, last) => `Now ${cur} · Last paid ${last}`,
    title: (name) => `${name} price`,
    sheetSub: "What we paid versus current reference prices",
    kLast: "Last paid", kNow: "Cheapest now", per: (u) => `per ${u}`, kDiff: "Difference", kPct: "Increase (%)",
    paidLine: "What we paid over time", offers: "Current prices by store (reference)", grid: "Details and sources",
    paid: "Paid", offer: "On sale",
    receipt: "Our receipt",
    note: "Current prices are reference prices Mimi found on store pages, not checkout prices. Unit prices use the same pack size.",
  },
  budget: {
    over: (v) => `With this plan, this month is **${v} over budget**.`,
    left: (v) => `After this plan, **${v}** of this month's budget is left.`,
    sub: (plan) => `Plan estimate ${plan} · upcoming bills included`,
    missing: (n) => `${n} items have no price yet and are not in the estimate.`,
    title: "This month's budget",
    sheetSub: "Spent, upcoming bills and this plan against the monthly budget",
    kBudget: "Monthly budget", kSpent: "Spent", kBills: "Bills still due", kPlan: "This plan", kLeft: "Left", kOver: "Over",
    parts: "How the monthly budget is used", lines: "Items in this plan",
    note: "Estimate uses the last price we paid. Delivery fees and taxes are not included.",
  },
  trend: {
    title: (cat) => `Spending on ${cat}`,
    sheetSub: "Versus last month, and the items behind the change",
    kNow: "This month", kPrev: "Last month", kDiff: "Difference",
    months: "Last 6 months", items: "Items this month",
  },
  f: {
    type: "Type", date: "Date", store: "Store", size: "Size", price: "Price", unitPrice: "Unit price", source: "Source", on: "Checked on",
    part: "Part", amount: "Amount", name: "Item", qty: "Qty", lineTotal: "Line total", month: "Month",
  },
  unit: (u) => (u === "item" ? "ea" : u),
};

export const it = uiLang === "en" ? en : vi;
