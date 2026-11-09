/**
 * Biểu đồ ngay trong khung chat Hỏi Mimi. Mỗi tool mimi_* trả về số liệu (dạng JSON mà mô hình
 * đọc); phần "detail" của tool trong khung chat đọc lại đúng số liệu đó và vẽ thành biểu đồ, nên
 * hỏi tới chuyện gì thì thấy biểu đồ của chuyện ấy, không cần mở bảng.
 * Studio để phần detail gập sẵn; useAutoOpenCharts mở sẵn những bước có biểu đồ.
 */
import { useEffect, useMemo, useRef } from "react";
import {
  AgCharts,
  BarSeriesModule,
  CategoryAxisModule,
  LegendModule,
  LineSeriesModule,
  ModuleRegistry,
  NumberAxisModule,
  type AgCartesianChartOptions,
  type AgChartInstance,
} from "ag-charts-community";
import type { AgAiToolDetailParams } from "ag-studio";
import { formatVnd } from "../lib/text";
import type { AnalyticsRows } from "./data";
import { MIMI_CHART } from "./theme";
import { at } from "./text";

/** Số liệu một biểu đồ: các nhãn, và một hoặc hai dãy số cùng độ dài với nhãn. */
export type ChatChart = {
  title: string;
  kind: "bar" | "column" | "line";
  labels: string[];
  series: { name: string; values: number[] }[];
  unit: "vnd" | "days";
};

// AG Charts 14 chỉ nạp phần được đăng ký: thanh, đường, hai trục và chú thích là đủ cho khung chat.
ModuleRegistry.registerModules([BarSeriesModule, LineSeriesModule, CategoryAxisModule, NumberAxisModule, LegendModule]);

const FONT = '"Be Vietnam Pro", system-ui, sans-serif';
const urgency = (d: number) => (d <= 0 ? "#c5221f" : d <= 7 ? "#b4530a" : d <= 14 ? "#2a5bd7" : "#1f7a3a");
const short = (s: string) => (s.length > 16 ? `${s.slice(0, 15)}…` : s);

// ---------------- Từ kết quả tool → biểu đồ ----------------

type Json = Record<string, unknown>;
const list = (v: unknown) => (Array.isArray(v) ? (v as Json[]) : []);
const n = (v: unknown) => (typeof v === "number" && Number.isFinite(v) ? v : 0);

/** Đọc kết quả của một tool mimi_* và chọn biểu đồ hợp với nó. Không có gì để vẽ thì trả null. */
export function chartFor(name: string, response: string): ChatChart | null {
  let r: unknown;
  try {
    r = JSON.parse(response);
  } catch {
    return null;
  }
  const t = at.ai.charts;
  if (name === "mimi_spending_summary" && r && typeof r === "object") {
    const o = r as Json;
    const cats = list(o.by_category).slice(0, 8);
    if (!cats.length) return null;
    return {
      title: t.byCategory(String(o.month ?? "")),
      kind: "bar",
      labels: cats.map((c) => String(c.name)),
      series: [
        { name: t.thisMonth, values: cats.map((c) => n(c.amount)) },
        { name: t.lastMonth, values: cats.map((c) => n(c.previous_month)) },
      ],
      unit: "vnd",
    };
  }
  if (name === "mimi_running_out") {
    const items = list(r).slice(0, 10);
    if (!items.length) return null;
    return { title: t.runningOut, kind: "bar", labels: items.map((i) => String(i.item)), series: [{ name: t.daysLeft, values: items.map((i) => Math.max(0, n(i.days_left))) }], unit: "days" };
  }
  if (name === "mimi_savings" && r && typeof r === "object") {
    const more = list((r as Json).cheaper_elsewhere).slice(0, 8);
    if (!more.length) return null;
    return {
      title: t.savings,
      kind: "bar",
      labels: more.map((m) => `${m.item} · ${m.best_store}`),
      series: [{ name: t.saving, values: more.map((m) => n(m.saving)) }],
      unit: "vnd",
    };
  }
  if (name === "mimi_chart" && r && typeof r === "object" && Array.isArray((r as Json).labels)) {
    const c = r as ChatChart;
    return c.labels.length ? c : null;
  }
  return null;
}

// ---------------- Tool mimi_chart: các biểu đồ còn lại của bảng ----------------

export const CHART_TOPICS = ["spending_by_month", "spending_by_store", "top_items", "price_compare"] as const;
export type ChartTopic = (typeof CHART_TOPICS)[number];

const sumBy = <T,>(rows: T[], key: (r: T) => string, value: (r: T) => number) => {
  const m = new Map<string, number>();
  for (const r of rows) m.set(key(r), (m.get(key(r)) ?? 0) + value(r));
  return [...m.entries()];
};

/** Số liệu cho tool mimi_chart, tính thẳng từ dữ liệu của bảng. `month` rỗng là mọi tháng. */
export function buildChart(rows: AnalyticsRows, topic: ChartTopic, month?: string): ChatChart {
  const t = at.ai.charts;
  const receipts = month ? rows.receipts.filter((r) => r.month === month) : rows.receipts;
  const ids = new Set(receipts.map((r) => r.id));
  if (topic === "spending_by_month") {
    const m = sumBy(rows.receipts, (r) => r.month, (r) => r.total).sort(([a], [b]) => a.localeCompare(b)).slice(-6);
    return { title: t.byMonth, kind: "column", labels: m.map(([k]) => k), series: [{ name: t.spent, values: m.map(([, v]) => Math.round(v)) }], unit: "vnd" };
  }
  if (topic === "spending_by_store") {
    const m = sumBy(receipts, (r) => r.store, (r) => r.total).sort((a, b) => b[1] - a[1]).slice(0, 8);
    return { title: t.byStore(month), kind: "bar", labels: m.map(([k]) => k), series: [{ name: t.spent, values: m.map(([, v]) => Math.round(v)) }], unit: "vnd" };
  }
  if (topic === "top_items") {
    const m = sumBy(rows.lines.filter((l) => ids.has(l.receiptId)), (l) => l.name, (l) => l.amount ?? 0).sort((a, b) => b[1] - a[1]).slice(0, 8);
    return { title: t.topItems(month), kind: "bar", labels: m.map(([k]) => k), series: [{ name: t.spent, values: m.map(([, v]) => Math.round(v)) }], unit: "vnd" };
  }
  // price_compare: mỗi món, giá nhà mình đã mua so với giá online rẻ nhất.
  const best = new Map<string, { name: string; online: number; own: number }>();
  for (const p of rows.prices) {
    if (p.ownPrice == null) continue;
    const cur = best.get(p.key);
    if (!cur || p.onlinePrice < cur.online) best.set(p.key, { name: p.name, online: p.onlinePrice, own: p.ownPrice });
  }
  const items = [...best.values()].sort((a, b) => b.own - b.online - (a.own - a.online)).slice(0, 8);
  return {
    title: t.priceCompare,
    kind: "bar",
    labels: items.map((i) => i.name),
    series: [
      { name: t.ourPrice, values: items.map((i) => Math.round(i.own)) },
      { name: t.cheapestOnline, values: items.map((i) => Math.round(i.online)) },
    ],
    unit: "vnd",
  };
}

// ---------------- Vẽ ----------------

function options(c: ChatChart): AgCartesianChartOptions {
  const data = c.labels.map((label, i) => Object.fromEntries([["label", short(label)], ...c.series.map((s, j) => [`v${j}`, s.values[i] ?? 0])]));
  const fmt = (v: number) => (c.unit === "vnd" ? formatVnd(v) : at.radar.left(v));
  const axisFmt = (v: number) => (c.unit === "vnd" ? (v >= 1_000_000 ? `${+(v / 1_000_000).toFixed(1)}M` : v >= 1000 ? `${Math.round(v / 1000)}K` : `${v}`) : `${v}`);
  const horizontal = c.kind === "bar";
  return {
    data,
    background: { visible: false },
    // Nhãn số ở đầu thanh nằm ngoài thanh nên chừa chỗ bên phải.
    padding: { top: 4, right: horizontal && c.series.length === 1 ? 64 : 12, bottom: 4, left: 8 },
    title: { enabled: !!c.title, text: c.title, fontFamily: FONT, fontSize: 13, fontWeight: "bold", color: "#1b2340", textAlign: "left" },
    legend: { enabled: c.series.length > 1, position: "bottom", item: { label: { fontFamily: FONT, fontSize: 11 } } },
    series: c.series.map((s, j) =>
      c.kind === "line"
        ? { type: "line" as const, xKey: "label", yKey: `v${j}`, yName: s.name, stroke: j === 0 ? "#1595a8" : "#9aa3c4", strokeWidth: j === 0 ? 3 : 2, ...(j > 0 && { lineDash: [6, 4] }), marker: { enabled: false }, interpolation: { type: "smooth" as const }, tooltip: { renderer: ({ datum }: { datum: Json }) => fmt(n(datum[`v${j}`])) } }
        : {
            type: "bar" as const,
            direction: horizontal ? ("horizontal" as const) : ("vertical" as const),
            xKey: "label",
            yKey: `v${j}`,
            yName: s.name,
            fill: MIMI_CHART[j],
            cornerRadius: 4,
            ...(c.unit === "days" && { itemStyler: ({ datum }: { datum: Json }) => ({ fill: urgency(n(datum[`v${j}`])) }) }),
            tooltip: { renderer: ({ datum }: { datum: Json }) => `${datum.label}: ${fmt(n(datum[`v${j}`]))}` },
            ...(c.series.length === 1 && { label: { enabled: true, placement: "outside-end" as const, fontFamily: FONT, fontSize: 10, color: "#56618a", formatter: ({ value }: { value: number }) => (c.unit === "vnd" ? axisFmt(value) : at.radar.left(value)) } }),
          },
    ) as AgCartesianChartOptions["series"],
    axes: {
      x: { type: "category", position: horizontal ? "left" : "bottom", label: { fontFamily: FONT, fontSize: 11 } },
      y: { type: "number", position: horizontal ? "bottom" : "left", label: { fontFamily: FONT, fontSize: 10, formatter: ({ value }: { value: number }) => axisFmt(value) }, gridLine: { style: [{ stroke: "#e3ebff" }] } },
    } as never,
  };
}

/** Biểu đồ nhỏ trong khung chat. Cao theo số dòng để thanh không bị ép. */
export function ChatChartView({ chart }: { chart: ChatChart }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!host.current) return;
    const inst: AgChartInstance = AgCharts.create({ ...options(chart), container: host.current } as never);
    return () => inst.destroy();
  }, [chart]);
  const height = chart.kind === "bar" ? 64 + chart.labels.length * (chart.series.length > 1 ? 34 : 26) + (chart.series.length > 1 ? 24 : 0) : 220;
  return <div ref={host} className="chatchart" style={{ height }} role="img" aria-label={chart.title} />;
}

/** Phần detail của các tool có biểu đồ, cắm vào aiToolDisplay của Studio. */
export function ToolChart(p: AgAiToolDetailParams) {
  const response = p.result?.success ? p.result.response : undefined;
  // Studio vẽ lại phần detail mỗi lần khung chat đổi; chỉ dựng lại biểu đồ khi số liệu đổi.
  const chart = useMemo(() => (response ? chartFor(p.name, response) : null), [p.name, response]);
  return chart ? <ChatChartView chart={chart} /> : null;
}

/** Các tool có biểu đồ trong khung chat. */
export const CHART_TOOLS = new Set(["mimi_spending_summary", "mimi_running_out", "mimi_savings", "mimi_chart"]);

/**
 * Studio gập sẵn phần detail của mỗi bước; với bước có biểu đồ thì mở sẵn một lần (người dùng vẫn
 * gập lại được). Bước có biểu đồ nhận ra nhờ nhãn nhỏ (pill) at.ai.chartPill.
 */
export function useAutoOpenCharts(root: React.RefObject<HTMLElement | null>) {
  useEffect(() => {
    const el = root.current;
    if (!el) return;
    const open = () => {
      for (const row of el.querySelectorAll<HTMLElement>(".ag-studio-ai-step-row-interactive:not([data-mimi-opened])")) {
        if (row.querySelector(".ag-studio-ai-step-pill")?.textContent !== at.ai.chartPill) continue;
        row.dataset.mimiOpened = "1";
        if (row.getAttribute("aria-expanded") !== "true") row.click();
      }
    };
    const mo = new MutationObserver(open);
    mo.observe(el, { subtree: true, childList: true, attributes: true, attributeFilter: ["class"] });
    return () => mo.disconnect();
  }, [root]);
}
