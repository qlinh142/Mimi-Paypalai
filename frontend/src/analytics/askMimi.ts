/**
 * Agent "Hỏi Mimi" trên Studio Agent Framework.
 * - Là agent chính của khung chat: hiểu chuyện nhà (hóa đơn, đồ dùng sắp hết, giá đang bán) qua
 *   bốn tool riêng của Mimi, trả lời ngắn gọn bằng ngôn ngữ của app.
 * - Cần vẽ hay sửa bảng thì giao việc (delegate_to) cho các agent có sẵn của Studio:
 *   widget (thêm/sửa widget), page (bố cục trang), data (truy vấn, trường tính toán).
 * - Mọi agent chạy cùng một adapter Gemini qua máy chủ Mimi (./gemini.ts).
 */
import { createAiHarness, directLlmRunner, type AgAiHarnessSetup, type AgLlmAdapter, type AgStudioApi } from "ag-studio";
import { currentMonth, addMonths } from "../lib/text";
import type { AnalyticsRows } from "./data";
import { buildChart, CHART_TOPICS, type ChartTopic } from "./chatCharts";
import { geminiAdapter } from "./gemini";
import { PAGE } from "./report";
import { at } from "./text";

type Money = number;
const round = (n: number) => Math.round(n);

/** Gom theo khóa, cộng số tiền: dùng cho các tool tóm tắt. */
function groupSum<T>(rows: T[], key: (r: T) => string, value: (r: T) => number) {
  const m = new Map<string, number>();
  for (const r of rows) m.set(key(r), (m.get(key(r)) ?? 0) + value(r));
  return [...m.entries()].map(([k, v]) => ({ name: k, amount: round(v) })).sort((a, b) => b.amount - a.amount);
}

export function spendingSummary(rows: AnalyticsRows, month: string) {
  const prev = addMonths(month, -1);
  const inMonth = new Set(rows.receipts.filter((r) => r.month === month).map((r) => r.id));
  const inPrev = new Set(rows.receipts.filter((r) => r.month === prev).map((r) => r.id));
  const total = (ids: Set<string>) => round(rows.receipts.filter((r) => ids.has(r.id)).reduce((a, r) => a + r.total, 0));
  const cats = (ids: Set<string>) => groupSum(rows.lines.filter((l) => ids.has(l.receiptId)), (l) => l.category, (l) => l.amount ?? 0);
  const prevCats = new Map(cats(inPrev).map((c) => [c.name, c.amount]));
  return {
    currency_note: "Amounts are VND.",
    month,
    spent: total(inMonth),
    receipts: inMonth.size,
    previous_month: prev,
    previous_spent: total(inPrev),
    by_category: cats(inMonth).map((c) => ({ ...c, previous_month: prevCats.get(c.name) ?? 0 })),
    by_store: groupSum(rows.receipts.filter((r) => inMonth.has(r.id)), (r) => r.store, (r) => r.total),
    discounts: round(rows.receipts.filter((r) => inMonth.has(r.id)).reduce((a, r) => a + r.discount, 0)),
  };
}

export function runningOut(rows: AnalyticsRows, days: number) {
  const cheapest = new Map<string, { store: string; price: Money; source: string }>();
  for (const p of rows.prices) {
    const cur = cheapest.get(p.name);
    if (!cur || p.onlinePrice < cur.price) cheapest.set(p.name, { store: p.store, price: p.onlinePrice, source: p.source });
  }
  return rows.items
    .filter((i) => i.daysLeft != null && i.daysLeft <= days)
    .sort((a, b) => (a.daysLeft ?? 0) - (b.daysLeft ?? 0))
    .map((i) => ({ item: i.name, category: i.category, days_left: i.daysLeft, runs_out_on: i.outOn, cheapest_online: cheapest.get(i.name) ?? null }));
}

export function savings(rows: AnalyticsRows) {
  const top = [...rows.receipts].filter((r) => r.discount > 0).sort((a, b) => b.discount - a.discount).slice(0, 5);
  const byItem = new Map<string, { item: string; our_price: Money | null; our_store: string; best_store: string; best_price: Money; saving: Money }>();
  for (const p of rows.prices) {
    const cur = byItem.get(p.key);
    if (!cur || p.saving > cur.saving) byItem.set(p.key, { item: p.name, our_price: p.ownPrice, our_store: p.ownStore, best_store: p.store, best_price: p.onlinePrice, saving: p.saving });
  }
  const potential = [...byItem.values()].filter((x) => x.saving > 0).sort((a, b) => b.saving - a.saving);
  return {
    currency_note: "Amounts are VND.",
    saved_with_discounts: round(rows.receipts.reduce((a, r) => a + r.discount, 0)),
    top_discount_receipts: top.map((r) => ({ store: r.store, date: r.date, discount: r.discount, total: r.total })),
    could_save_more: round(potential.reduce((a, x) => a + x.saving, 0)),
    cheaper_elsewhere: potential,
  };
}

function instructions(api: AgStudioApi) {
  const state = api.getState();
  return [
    "You are Mimi, a warm and practical household spending assistant inside the Mimi app.",
    at.ai.lang,
    "Use the mimi_* tools for questions about the family's spending, items running out, savings and prices. Never invent numbers: every figure must come from a tool result.",
    "Money in tool results is VND. When you mention money, write it in the app's display currency: VND with a dot thousands separator (e.g. 130.000đ) in Vietnamese, or the same VND amount in English (e.g. ₫130,000).",
    "The chat shows a chart under every mimi_spending_summary, mimi_running_out, mimi_savings and mimi_chart call, so the user sees the numbers you talk about. When the question is about a trend, a breakdown or a comparison that those tools do not chart (spending by month, by store, top items, our price vs online), call mimi_chart for it. Do not repeat every number in words; point out what matters in the chart.",
    "Keep answers short: 2 to 5 sentences or a short list, lead with the answer, then one practical suggestion (what to buy, where it is cheaper).",
    "When the user asks to add, change or remove a chart, table or widget, or to rearrange the page, delegate to the widget or page agent with a precise brief (fields, aggregation, chart type). Field ids look like table.field, e.g. lines.amount, receipts.store, items.daysLeft, prices.saving.",
    "Mimi's own widgets: 'mimi-runout-radar' (items.name + items.daysLeft) and 'mimi-money-saved' (receipts.discount + prices.saving).",
    "To answer an ad-hoc analytical question the mimi_* tools do not cover, delegate to the data agent.",
    `The dashboard pages are: ${PAGE.overview} (overview), ${PAGE.prices} (price check against online prices from Channel3 and public lookups), ${PAGE.receipts} (receipts). The selected page is ${state.selectedPageId}. Use mimi_show_page to switch page when it helps the user see the answer.`,
    `Today is ${new Date().toISOString().slice(0, 10)}; the current month is ${currentMonth()}.`,
  ].join("\n");
}

/** Dựng harness: "Hỏi Mimi" là agent chính, năm agent có sẵn của Studio làm người được giao việc. */
export function buildAskMimi(getRows: () => AnalyticsRows | undefined, adapter: AgLlmAdapter = geminiAdapter()): AgAiHarnessSetup {
  return ({ api }) =>
    createAiHarness(api, ({ tools, builtIn }) => {
      const need = () => {
        const r = getRows();
        if (!r) throw new Error("data not loaded");
        return r;
      };
      const mimiTools = [
        api.defineAiTool({
          name: "mimi_chart",
          description:
            "Show a chart in the chat and return its numbers: spending_by_month (last 6 months), spending_by_store, top_items (items the family spent most on), price_compare (what the family paid vs the cheapest online price per item). Use it to answer with a picture, not to change the dashboard.",
          params: (s) =>
            s.object({
              topic: s.enum(CHART_TOPICS),
              month: s.string({ description: "Month as YYYY-MM for spending_by_store and top_items; empty string for all months." }),
            }),
          execute: (args: { topic: ChartTopic; month?: string }, ctx) => ctx.success(JSON.stringify(buildChart(need(), args.topic, args.month || undefined))),
        }),
        api.defineAiTool({
          name: "mimi_spending_summary",
          description: "Spending for one month: total, number of receipts, totals by category and by store, discounts, and the previous month for comparison.",
          params: (s) => s.object({ month: s.string({ description: "Month as YYYY-MM. Use the current month when the user does not say." }) }),
          execute: (args: { month: string }, ctx) => ctx.success(JSON.stringify(spendingSummary(need(), args.month || currentMonth()))),
        }),
        api.defineAiTool({
          name: "mimi_running_out",
          description: "Household items predicted to run out within the given number of days, soonest first, each with the cheapest online price found.",
          params: (s) => s.object({ days: s.number({ description: "Look-ahead window in days, e.g. 7." }) }),
          execute: (args: { days: number }, ctx) => ctx.success(JSON.stringify(runningOut(need(), args.days ?? 7))),
        }),
        api.defineAiTool({
          name: "mimi_savings",
          description: "Money saved through receipt discounts, the receipts with the biggest discounts, and items that are cheaper at another store than what the family paid.",
          params: (s) => s.object({}),
          execute: (_args: unknown, ctx) => ctx.success(JSON.stringify(savings(need()))),
        }),
        api.defineAiTool({
          name: "mimi_show_page",
          description: "Switch the dashboard to one of its pages so the user can see the related charts.",
          params: (s) => s.object({ page: s.enum([PAGE.overview, PAGE.prices, PAGE.receipts] as const) }),
          execute: (args: { page: string }, ctx) => {
            api.setState({ ...api.getState(), selectedPageId: args.page });
            return ctx.success(`Showing page ${args.page}.`);
          },
        }),
      ];
      const mimi = directLlmRunner({
        id: "mimi",
        name: at.ai.agent,
        description: at.ai.agentDesc,
        schema: (s) => s.undefined(),
        adapter,
        maxTurns: 12,
        instructions: () => instructions(api),
        tools: () => [...mimiTools, tools.studio.viewPage(), tools.delegateTo(["widget", "page", "data"]), tools.renameThread()],
      });
      return {
        agents: [mimi, ...Object.values(builtIn).map((d) => directLlmRunner({ ...d, adapter }))],
        primary: "mimi",
        promptStarters: at.ai.starters,
      };
    });
}
