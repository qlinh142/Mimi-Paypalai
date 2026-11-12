import { uiLang } from "../i18n";

/** Chữ của 16 điểm chạm còn lại (bảng điểm chạm 12/11). Câu trung tính: báo số, không thúc mua thêm. */
const vi = {
  ana: "Phân tích",
  days: "ngày",
  times: "lần",
  pctUnit: "%",
  thisMonth: "Tháng này", lastMonth: "Tháng trước", diff: "Chênh lệch",
  f: { cat: "Nhóm", store: "Nơi mua", date: "Ngày", amount: "Số tiền", item: "Món", daysLeft: "Còn (ngày)", outOn: "Dự kiến hết", month: "Tháng", day: "Ngày trong tháng", count: "Số hóa đơn", reason: "Lý do", status: "Trạng thái", pct: "Chênh (%)" },
  // 1. Sổ chi tiêu · Xu hướng
  trend: {
    title: (cat: string) => `Chi cho ${cat}`, sub: "Tháng này so với tháng trước, theo nhóm hàng",
    chart: "Các nhóm hàng, hai tháng", grid: "Từng nhóm",
    note: "Tính từ hóa đơn đã lưu. Tháng này mới tính đến hôm nay.",
  },
  // 2. Cửa hàng
  store: {
    title: (s: string) => `Chi ở ${s}`, sub: "Phần của từng nơi mua trong tháng này",
    kTotal: "Chi ở đây", kPct: "Phần trong tháng", chart: "Theo nơi mua", grid: (s: string) => `Hóa đơn ở ${s}`,
  },
  // 3. Định kỳ sắp đến hạn
  bill: {
    title: (n: string) => n, sub: "Kỳ sắp tới và các kỳ đã trả",
    kAmount: "Kỳ này", kDays: "Đến hạn sau", chart: "Các kỳ đã trả", grid: "Sắp chi 7 ngày tới",
  },
  // 4. Món sắp hết
  low: {
    title: "Món sắp hết", sub: "Mimi ước tính theo mức dùng mỗi ngày",
    kDays: "Còn khoảng", kCycle: "Chu kỳ mua", chart: "Số ngày còn lại", grid: "Các món cần để ý",
    note: "Ước tính từ các lần mua trước, có thể lệch vài ngày.",
  },
  // 5. Còn tiêu được
  safe: {
    title: "Còn tiêu được", sub: "Ngân sách tháng sau khi trừ đã chi và định kỳ",
    kBudget: "Ngân sách", kSpent: "Đã chi", kBills: "Định kỳ còn trả", kFree: "Còn tiêu được", kOver: "Vượt",
    chart: "Chi cộng dồn trong tháng", grid: "Định kỳ còn phải trả tháng này",
    note: "Tháng trước tính đến cùng ngày để so cho công bằng.",
  },
  // 7. Chi tiết món · tần suất mua
  freq: {
    text: (name: string, n: number, days: number | null) => `3 tháng qua nhà mình mua **${name}** **${n} lần**${days ? `, khoảng **${days} ngày** một lần` : ""}.`,
    sub: (total: string) => `Tổng ${total}`,
    title: (n: string) => `Mua ${n} theo tháng`, sub2: "Số lần mua và tiền mỗi tháng",
    kCount: "Số lần (3 tháng)", kTotal: "Tổng tiền (3 tháng)", chart: "Tiền mỗi tháng", grid: "Các lần mua",
  },
  // 8. Chi tiết khoản định kỳ
  rise: {
    text: (n: string, pct: number) => `**${n}** kỳ này cao hơn trung bình các kỳ trước **${pct}%**.`,
    sub: (cur: string, avg: string) => `Kỳ này ${cur} · Trung bình ${avg}`,
    title: (n: string) => n, sub2: "Kỳ này so với các kỳ đã trả",
    kCur: "Kỳ này", kAvg: "Trung bình các kỳ trước", chart: "Số tiền từng kỳ", grid: "Các kỳ",
    paid: "Đã trả", due: "Sắp đến hạn",
  },
  // 10. Kế hoạch bước 2 · giá từng món
  cmp: {
    text: (n: string, pct: number) => `**${n}**: nơi bán rẻ nhất vẫn đắt hơn lần mua trước **${pct}%**.`,
    sub: (now: string, last: string) => `Rẻ nhất đang bán ${now} · Lần trước ${last}`,
    title: "Giá kế hoạch so với lần trước", sub2: "Từng món: giá lần trước và giá rẻ nhất đang bán",
    kLast: "Theo giá lần trước", kNow: "Theo giá rẻ nhất", chart: "Từng món", grid: "Chi tiết",
    sLast: "Lần trước", sNow: "Rẻ nhất đang bán",
    note: "Giá đang bán là giá tham khảo Mimi tra trên trang bán hàng, chưa phải giá thanh toán.",
  },
  // 11. Cả kế hoạch sau khi chọn giá
  picked: {
    less: (v: string) => `Với giá đã chọn, kế hoạch này **rẻ hơn lần mua trước ${v}**.`,
    more: (v: string) => `Với giá đã chọn, kế hoạch này **đắt hơn lần mua trước ${v}**.`,
    sub: (n: number) => `So trên ${n} món đã từng mua`,
    title: "Giá đã chọn so với lần trước", sub2: "Từng món: lần trước, giá đã chọn và nơi mua",
    kLast: "Theo giá lần trước", kNow: "Theo giá đã chọn", chart: "Chênh lệch từng món", grid: "Chi tiết",
    sDiff: "Chênh so với lần trước",
  },
  // 13. Hoàn thành chuyến mua
  trip: {
    text: (spent: string, est: string) => `Chuyến này, các món trong kế hoạch chi **${spent}**, dự kiến **${est}**.`,
    whole: (v: string) => `Cả hóa đơn ${v}`,
    extra: (n: number, v: string) => `${n} món ngoài kế hoạch ${v}`,
    title: "Chuyến mua vừa xong", sub: "Dự kiến lúc lên kế hoạch so với hóa đơn",
    kEst: "Dự kiến", kSpent: "Thực chi", chart: "Từng món", grid: "Chi tiết", sEst: "Dự kiến", sSpent: "Thực chi",
    outside: "Ngoài kế hoạch",
  },
  // 14. Sau khi lưu hóa đơn
  bill14: {
    pricier: (n: number) => `Hóa đơn này có **${n} món đắt hơn lần mua trước**.`,
    pricierSub: (names: string) => names,
    budget: (pct: number) => (pct >= 100 ? `Với hóa đơn này, tháng này **đã vượt ngân sách**.` : `Với hóa đơn này, tháng này **đã dùng ${pct}% ngân sách**.`),
    budgetSub: (spent: string, budget: string) => `Đã chi ${spent} / ${budget}`,
    title: "Giá trong hóa đơn này", sub: "Từng món so với lần mua trước (cùng quy cách)",
    kUp: "Món đắt hơn", kExtra: "Đắt thêm tất cả", chart: "Chênh so với lần trước (%)", grid: "Chi tiết",
    sNow: "Lần này", sLast: "Lần trước",
    bTitle: "Ngân sách tháng này", bSub: "Trước và sau hóa đơn này",
    kBefore: "Trước hóa đơn", kAfter: "Sau hóa đơn", bChart: "Ngân sách tháng", bGrid: "Các phần",
    parts: { spent: "Đã chi trước đó", bill: "Hóa đơn này", left: "Còn lại", over: "Vượt" },
  },
  // 15. Đơn online · hoàn tiền
  refund: {
    text: (v: string) => `Đã hoàn **${v}**, Sổ chi tiêu đã trừ.`,
    sub: (net: string) => `Thực chi đơn này ${net}`,
    title: "Tiền của đơn này", sub2: "Đã trả, đã hoàn và thực chi",
    kPaid: "Đã trả", kRefund: "Đã hoàn", kNet: "Thực chi", chart: "Đơn này", grid: "Các lần hoàn",
    reasons: { missing: "Thiếu hàng", wrong: "Sai hàng", damaged: "Hàng hỏng", other: "Khác" } as Record<string, string>,
  },
  // 16. Đặt ngân sách tháng
  avg: {
    text: (avg: string) => `3 tháng qua nhà mình chi trung bình **${avg}** mỗi tháng.`,
    sub: (rec: string) => `Các khoản định kỳ hiện khoảng ${rec} mỗi tháng`,
    title: "Chi mấy tháng gần đây", sub2: "Để đặt ngân sách sát với thực tế",
    kAvg: "Trung bình 3 tháng", kRec: "Định kỳ mỗi tháng", chart: "Đã chi theo tháng", grid: "Từng tháng",
  },
  // 17. Trang chủ
  home: {
    used: (pct: number) => `Đã dùng ${pct}% ngân sách`,
    over: "Đã vượt ngân sách",
  },
  // 18. Thông báo · tóm tắt tuần
  week: {
    text: (v: string, store: string) => `Tuần trước nhà mình chi **${v}**, nhiều nhất ở **${store}**.`,
    sub: (n: number, pct: number | null) => `${n} hóa đơn${pct == null ? "" : pct === 0 ? " · bằng tuần trước nữa" : ` · ${pct > 0 ? "nhiều" : "ít"} hơn tuần trước nữa ${Math.abs(pct)}%`}`,
    title: "Tuần trước", sub2: "Chi theo ngày và theo nơi mua",
    kWeek: "Tuần trước", kPrev: "Tuần trước nữa", chart: "Theo ngày", grid: "Các hóa đơn",
    wd: ["T2", "T3", "T4", "T5", "T6", "T7", "CN"],
  },
};

const en: typeof vi = {
  ana: "Analysis",
  days: "days",
  times: "times",
  pctUnit: "%",
  thisMonth: "This month", lastMonth: "Last month", diff: "Difference",
  f: { cat: "Category", store: "Store", date: "Date", amount: "Amount", item: "Item", daysLeft: "Days left", outOn: "Runs out", month: "Month", day: "Day of month", count: "Receipts", reason: "Reason", status: "Status", pct: "Diff (%)" },
  trend: {
    title: (cat) => `Spending on ${cat}`, sub: "This month versus last month, by category",
    chart: "Categories, two months", grid: "By category",
    note: "From saved receipts. This month counts up to today.",
  },
  store: {
    title: (s) => `Spending at ${s}`, sub: "Each store's share of this month",
    kTotal: "Spent here", kPct: "Share of month", chart: "By store", grid: (s) => `Receipts at ${s}`,
  },
  bill: {
    title: (n) => n, sub: "Next payment and past payments",
    kAmount: "This time", kDays: "Due in", chart: "Past payments", grid: "Due in the next 7 days",
  },
  low: {
    title: "Running low", sub: "Mimi's estimate from daily use",
    kDays: "About", kCycle: "Buy every", chart: "Days left", grid: "Items to watch",
    note: "Estimated from past purchases; may be off by a few days.",
  },
  safe: {
    title: "Safe to spend", sub: "Monthly budget after spending and bills",
    kBudget: "Budget", kSpent: "Spent", kBills: "Bills still due", kFree: "Safe to spend", kOver: "Over",
    chart: "Cumulative spending this month", grid: "Bills still due this month",
    note: "Last month is counted to the same day for a fair comparison.",
  },
  freq: {
    text: (name, n, days) => `In the last 3 months we bought **${name}** **${n} times**${days ? `, about every **${days} days**` : ""}.`,
    sub: (total) => `Total ${total}`,
    title: (n) => `${n} by month`, sub2: "Purchases and money each month",
    kCount: "Times (3 months)", kTotal: "Total (3 months)", chart: "Money per month", grid: "Purchases",
  },
  rise: {
    text: (n, pct) => `**${n}** is **${pct}%** higher than the average of past payments.`,
    sub: (cur, avg) => `This time ${cur} · Average ${avg}`,
    title: (n) => n, sub2: "This payment versus past payments",
    kCur: "This time", kAvg: "Past average", chart: "Each payment", grid: "Payments",
    paid: "Paid", due: "Due",
  },
  cmp: {
    text: (n, pct) => `**${n}**: the cheapest offer is still **${pct}%** above what we paid last time.`,
    sub: (now, last) => `Cheapest now ${now} · Last time ${last}`,
    title: "Plan prices versus last time", sub2: "Each item: last price paid and cheapest offer now",
    kLast: "At last prices", kNow: "At cheapest offers", chart: "Each item", grid: "Details",
    sLast: "Last time", sNow: "Cheapest now",
    note: "Offers are reference prices Mimi found on store pages, not checkout prices.",
  },
  picked: {
    less: (v) => `With the prices you picked, this plan is **${v} cheaper than last time**.`,
    more: (v) => `With the prices you picked, this plan is **${v} more than last time**.`,
    sub: (n) => `Compared on ${n} items bought before`,
    title: "Picked prices versus last time", sub2: "Each item: last time, picked price and store",
    kLast: "At last prices", kNow: "At picked prices", chart: "Difference per item", grid: "Details",
    sDiff: "Versus last time",
  },
  trip: {
    text: (spent, est) => `This trip, planned items cost **${spent}**, estimate was **${est}**.`,
    whole: (v) => `Whole receipt ${v}`,
    extra: (n, v) => `${n} items outside the plan ${v}`,
    title: "Trip just finished", sub: "Plan estimate versus the receipt",
    kEst: "Estimate", kSpent: "Spent", chart: "Each item", grid: "Details", sEst: "Estimate", sSpent: "Spent",
    outside: "Outside the plan",
  },
  bill14: {
    pricier: (n) => `This receipt has **${n} items pricier than last time**.`,
    pricierSub: (names) => names,
    budget: (pct) => (pct >= 100 ? `With this receipt, this month is **over budget**.` : `With this receipt, **${pct}% of this month's budget** is used.`),
    budgetSub: (spent, budget) => `Spent ${spent} / ${budget}`,
    title: "Prices on this receipt", sub: "Each item versus last purchase (same size)",
    kUp: "Pricier items", kExtra: "Extra in total", chart: "Versus last time (%)", grid: "Details",
    sNow: "This time", sLast: "Last time",
    bTitle: "This month's budget", bSub: "Before and after this receipt",
    kBefore: "Before receipt", kAfter: "After receipt", bChart: "Monthly budget", bGrid: "Parts",
    parts: { spent: "Spent before", bill: "This receipt", left: "Left", over: "Over" },
  },
  refund: {
    text: (v) => `**${v}** refunded, already taken off Spending.`,
    sub: (net) => `Net for this order ${net}`,
    title: "Money for this order", sub2: "Paid, refunded and net",
    kPaid: "Paid", kRefund: "Refunded", kNet: "Net", chart: "This order", grid: "Refunds",
    reasons: { missing: "Missing", wrong: "Wrong item", damaged: "Damaged", other: "Other" },
  },
  avg: {
    text: (avg) => `Over the last 3 months we spent **${avg}** a month on average.`,
    sub: (rec) => `Recurring bills are about ${rec} a month`,
    title: "Recent months", sub2: "To set a realistic budget",
    kAvg: "3-month average", kRec: "Bills per month", chart: "Spent by month", grid: "Each month",
  },
  home: {
    used: (pct) => `${pct}% of budget used`,
    over: "Over budget",
  },
  week: {
    text: (v, store) => `Last week we spent **${v}**, most at **${store}**.`,
    sub: (n, pct) => `${n} receipts${pct == null ? "" : pct === 0 ? " · same as the week before" : ` · ${Math.abs(pct)}% ${pct > 0 ? "more" : "less"} than the week before`}`,
    title: "Last week", sub2: "By day and by store",
    kWeek: "Last week", kPrev: "Week before", chart: "By day", grid: "Receipts",
    wd: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"],
  },
};

export const pt = uiLang === "en" ? en : vi;
