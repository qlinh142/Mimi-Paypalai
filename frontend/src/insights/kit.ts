/**
 * Khung chung cho các bảng AG Studio nhỏ (bảng điểm chạm 12/11, mục "Việc chung"): mỗi điểm chạm chỉ
 * khai báo ô số, một biểu đồ và bảng chi tiết; khung lo nguồn dữ liệu, widget, bố cục một cột.
 */
import type { AgDataSourcesDefinition, AgFieldDefinition, AgFormat, AgReportState, AgWidgetLayoutState } from "ag-studio";
import { uiLang } from "../i18n";
import { formatVnd } from "../lib/text";
import type { ReportSpec } from "./reports";

export type Kpi = { label: string; value: number; caption?: string; money?: boolean };
export type Col = { name: string; format: AgFormat };
export type MiniChart = {
  kind: "bar" | "column" | "line" | "donut";
  title: string;
  /** Tên trục loại (hiện ở bảng dữ liệu khi người dùng xem nguồn). */
  cat: string;
  catFormat?: AgFormat;
  /** Mỗi chuỗi số một màu; từ 2 chuỗi thì có chú giải. */
  series: { name: string; money?: boolean }[];
  rows: { cat: string; v: number[] }[];
  /** Giữ đúng thứ tự rows (tháng, ngày); mặc định xếp giảm theo chuỗi đầu. */
  keepOrder?: boolean;
};
export type MiniGrid = { title: string; cols: Col[]; rows: (string | number | null)[][]; total?: boolean; sortBy?: number; sortDesc?: boolean };
export type Mini = {
  key: string; title: string; sub: string; note?: string; asOf?: string;
  /** Câu mô tả dữ liệu bằng tiếng Anh cho AG Studio (Hỏi về bảng sau này). */
  about: string;
  kpis?: Kpi[];
  chart?: MiniChart;
  grid?: MiniGrid;
};

const FONT = '"Be Vietnam Pro", system-ui, sans-serif';
const typo = (fontSize: number, fontWeight: "normal" | "bold" = "normal") => ({ fontFamily: FONT, fontSize, fontWeight, fontStyle: "normal" as const });
const pos = (y: number, h: number, x = 0, w = 24): AgWidgetLayoutState => ({ xTrack: x, yTrack: y, xSpan: w, ySpan: h });
const money = (): Pick<AgFieldDefinition, "createValueFormatter"> => ({
  createValueFormatter: () => (v: unknown) => (typeof v === "number" ? formatVnd(Math.round(v)) : ""),
});
const field = (id: string, name: string, format: AgFormat, hide = false): AgFieldDefinition => ({
  id, name, format, ...(format === "currencyFormat" ? money() : {}), ...(hide && { hide }),
});
const noLegend = { style: { theme: { common: { legend: { enabled: false } } } } };
const legendBottom = { style: { theme: { common: { legend: { enabled: true, position: "bottom" } } } } };

const KPI_H = 7;
/** Màu chuỗi theo thứ tự bảng màu của theme Mimi (xanh dương, xanh lá). */
const SWATCH = [uiLangIsEn() ? "Blue:" : "Xanh dương:", uiLangIsEn() ? "Green:" : "Xanh lá:"];
function uiLangIsEn() { return uiLang === "en"; }
const CHART_H = 16;
// Cột rộng cố định, tiêu đề xuống dòng: co theo nội dung thì cột số hẹp, tiêu đề thành mỗi chữ một dòng.
// Màn 393 thấy 3 cột, cột sau vuốt ngang.
const GRID_STYLE = { grandTotalRow: { enabled: false }, columnAutoSize: false, columnWidth: 104, headerRow: { wrapText: true } };
const gridH = (rows: number) => 8 + Math.ceil(Math.min(Math.max(rows, 2), 8) * 2.4);

export function mini(m: Mini): ReportSpec {
  const sources: AgDataSourcesDefinition["sources"] = [];
  const widgets: Record<string, unknown> = {};
  const layout: Record<string, AgWidgetLayoutState> = {};
  let y = 0;

  const kpis = (m.kpis ?? []).slice(0, 4);
  if (kpis.length) {
    const row: Record<string, number | string> = { id: "k" };
    kpis.forEach((k, i) => { row[`k${i}`] = k.value; });
    sources.push({ id: "k", name: "KPI", data: [row], fields: [field("id", "id", "textFormat", true), ...kpis.map((k, i) => field(`k${i}`, k.label, k.money === false ? "integerFormat" : "currencyFormat"))] });
    // Hai ô một hàng; màn hẹp (dưới 380) mỗi ô một hàng, vì ô nửa màn chỉ ~90px, số tiền bị thay bằng "…".
    const narrow = typeof window !== "undefined" && window.innerWidth < 380;
    kpis.forEach((k, i) => {
      widgets[`kpi${i}`] = {
        type: "value",
        dataMapping: { value: [{ id: `k.k${i}`, aggregation: "sum" }] },
        format: {
          title: { enabled: true, text: k.label, typography: typo(13, "bold") },
          caption: { enabled: !!k.caption, text: k.caption ?? "", typography: typo(12) },
          style: { typography: typo(17, "bold") },
        },
      };
      layout[`kpi${i}`] = narrow ? pos(y + i * KPI_H, KPI_H) : pos(y + Math.floor(i / 2) * KPI_H, KPI_H, (i % 2) * 12, 12);
    });
    y += narrow ? kpis.length * KPI_H : Math.ceil(kpis.length / 2) * KPI_H;
  }

  if (m.chart) {
    const c = m.chart;
    const rows = c.rows.map((r, i) => {
      const o: Record<string, string | number> = { id: `c${i}`, n: i, cat: r.cat };
      r.v.forEach((v, j) => { o[`v${j}`] = v; });
      return o;
    });
    sources.push({
      id: "c", name: c.title, data: rows,
      fields: [field("id", "id", "textFormat", true), field("n", "#", "integerFormat", true), field("cat", c.cat, c.catFormat ?? "textFormat"),
        ...c.series.map((s, j) => field(`v${j}`, s.name, s.money === false ? "decimalFormat" : "currencyFormat"))],
    });
    const type = { bar: "bar-chart-grouped", column: "column-chart-grouped", line: "line-chart", donut: "donut-chart" }[c.kind];
    const values = c.series.map((_, j) => ({ id: `c.v${j}`, aggregation: "sum" }));
    const dataMapping = c.kind === "donut"
      ? { categoryKey: [{ id: "c.cat" }], angleKey: values.slice(0, 1) }
      : { categoryKey: [{ id: "c.cat" }], valueKey: values };
    const sort = c.keepOrder || c.kind === "line"
      ? (c.catFormat === "dateFormat" ? [{ field: { id: "c.cat" }, direction: "asc" }] : [{ field: { id: "c.n", aggregation: "min" }, direction: "asc" }])
      : [{ field: { id: "c.v0", aggregation: "sum" }, direction: "desc" }];
    widgets.chart = {
      type, dataMapping, ...(c.kind !== "donut" && { sort }),
      // Chú giải của AG Studio thêm "(tổng)" sau tên và lật trang khi màn hẹp: thay bằng một dòng phụ ghi màu từng chuỗi.
      format: {
        title: { enabled: true, text: c.title },
        ...(c.series.length > 1 && { subtitle: { enabled: true, wrapping: true, text: c.series.map((x, j) => `${SWATCH[j] ?? ""} ${x.name}`).join(" · ") } }),
        ...(c.kind === "donut" ? legendBottom : noLegend),
      },
    };
    const h = c.kind === "bar" ? Math.max(12, 6 + c.rows.length * 3) : c.kind === "donut" ? 20 : CHART_H;
    layout.chart = pos(y, h);
    y += h;
  }

  if (m.grid) {
    const g = m.grid;
    sources.push({
      id: "g", name: g.title,
      data: g.rows.map((r, i) => Object.fromEntries([["id", `g${i}`], ...r.map((v, j) => [`c${j}`, v ?? (g.cols[j].format === "textFormat" ? "" : null)])])),
      fields: [field("id", "id", "textFormat", true), ...g.cols.map((c, j) => field(`c${j}`, c.name, c.format))],
    });
    const sortBy = g.sortBy ?? -1;
    widgets.grid = {
      type: "grid",
      dataMapping: { cols: g.cols.map((_, j) => ({ id: `g.c${j}` })) },
      ...(sortBy >= 0 && { sort: [{ field: { id: `g.c${sortBy}` }, direction: g.sortDesc === false ? "asc" : "desc" }] }),
      format: { title: { enabled: true, text: g.title }, style: { ...GRID_STYLE, grandTotalRow: { enabled: !!g.total } } },
    };
    const h = gridH(g.rows.length + (g.total ? 1 : 0));
    layout.grid = pos(y, h);
    y += h;
  }

  const report = {
    selectedPageId: m.key,
    pages: [{ id: m.key, widgets: widgets as never, widgetLayout: layout, layout: { minWidth: 280 } } as never],
  } as AgReportState;
  return {
    key: m.key, title: m.title, sub: m.sub, note: m.note, asOf: m.asOf,
    data: { description: m.about, sources }, report, height: y * 19,
  };
}
