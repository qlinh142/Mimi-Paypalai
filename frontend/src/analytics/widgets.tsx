/**
 * Ba widget riêng của Mimi cho AG Studio:
 * - Run-out Radar: mỗi món đồ dùng là một chấm trên radar, càng gần tâm càng sắp hết.
 * - Tiết kiệm được: tiền đã tiết kiệm nhờ giảm giá, và còn rẻ hơn bao nhiêu nếu mua ở nơi rẻ nhất.
 * - Lịch chi tiêu: lịch tháng, mỗi ngày ghi số đã chi; chạm vào ngày để xem các khoản của ngày đó.
 * Cả ba lấy dữ liệu qua widgetApi.getData, nên nhận bộ lọc trang, bộ lọc chéo và đổi trường như
 * widget có sẵn của Studio.
 */
import { useEffect, useState } from "react";
import type { AgWidgetField, AgWidgetParams } from "ag-studio";
import { createWidgets, type AgWidgetDefinition } from "ag-studio-react";
import { formatVnd } from "../lib/text";
import { at } from "./text";

type Rows = Record<string, unknown>[];

/** Gọi getData mỗi khi trường hoặc bộ lọc đổi; Studio tự gọi lại khi dữ liệu lọc thay đổi. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyParams = AgWidgetParams<any>;

function useWidgetRows(params: AnyParams, fields: AgWidgetField[], queryId: string) {
  const [rows, setRows] = useState<Rows | null>(null);
  const key = fields.map((f) => `${f.key}:${f.aggregation ?? ""}`).join("|");
  useEffect(() => {
    const api = params.widgetApi;
    if (fields.length === 0) {
      api.setDisplayState("incompleteDataMapping");
      setRows(null);
      return;
    }
    let live = true;
    api.setDisplayState("loading");
    api
      .getData({ fields }, { queryId, onDataUpdated: (r: { results: { rows: Rows } }) => live && setRows(r.results.rows) } as never)
      .then((r) => {
        if (!live || (r as { replaced?: boolean }).replaced) return;
        setRows(r.results.rows);
        api.setDisplayState(r.results.rows.length ? "displayed" : "noData");
      })
      .catch(() => live && api.setDisplayState("noData"));
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, params.widgetApi]);
  return rows;
}

const num = (v: unknown) => (typeof v === "number" && Number.isFinite(v) ? v : null);

// ---------------- Run-out Radar ----------------

type RadarState = { dataMapping: { item?: AgWidgetField[]; days?: AgWidgetField[] }; format: Record<string, unknown> };

const RINGS = [0, 3, 7, 14, 30];
const color = (d: number) => (d <= 0 ? "#c5221f" : d <= 7 ? "#b4530a" : d <= 14 ? "#2a5bd7" : "#1f7a3a");

export function RunoutRadar(params: AnyParams) {
  const dm = params.dataMapping as RadarState["dataMapping"];
  const item = dm.item?.[0];
  const days = dm.days?.[0];
  const rows = useWidgetRows(params, item && days ? [item, days] : [], "radar");
  if (!rows || !item || !days) return null;
  const points = rows
    .map((r) => ({ name: String(r[item.key] ?? ""), d: num(r[days.key]) }))
    .filter((p): p is { name: string; d: number } => p.d != null)
    .sort((a, b) => a.d - b.d);
  if (points.length === 0) return <p className="radar__empty">{at.radar.empty}</p>;
  const max = RINGS.at(-1)!;
  // Bán kính theo căn bậc hai để các món sắp hết (vài ngày) không dồn hết vào tâm.
  const radius = (d: number) => 8 + Math.sqrt(Math.min(Math.max(d, 0), max) / max) * 88;
  return (
    <div className="radar">
      <svg viewBox="-100 -100 200 200" role="img" aria-label={at.radar.title}>
        {RINGS.slice(1).map((d) => (
          <g key={d}>
            <circle r={radius(d)} className="radar__ring" />
            <text y={-radius(d) + 9} className="radar__ringlabel">{at.radar.ring(d)}</text>
          </g>
        ))}
        <circle r={radius(0)} className="radar__core" />
        {points.map((p, i) => {
          const a = (i / points.length) * Math.PI * 2 - Math.PI / 2;
          const r = radius(p.d);
          return (
            <g key={`${p.name}-${i}`} transform={`translate(${Math.cos(a) * r} ${Math.sin(a) * r})`}>
              <circle r={p.d <= 7 ? 5.5 : 4} fill={color(p.d)} stroke="#fff" strokeWidth={1.5}>
                <title>{`${p.name}: ${at.radar.left(p.d)}`}</title>
              </circle>
            </g>
          );
        })}
      </svg>
      <div className="radar__list">
        <strong>{at.radar.next}</strong>
        <ul>
          {points.slice(0, 4).map((p, i) => (
            <li key={`${p.name}-${i}`}>
              <span className="radar__dot" style={{ background: color(p.d) }} aria-hidden />
              <span className="radar__name">{p.name}</span>
              <span className="radar__days" style={{ color: color(p.d) }}>{at.radar.left(p.d)}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

// ---------------- Tiết kiệm được ----------------

type SavedState = { dataMapping: { saved?: AgWidgetField[]; potential?: AgWidgetField[] }; format: Record<string, unknown> };

/** Hai số đến từ hai bảng khác nhau (hóa đơn và bảng giá), nên hỏi riêng từng số. */
function useTotal(params: AnyParams, field: AgWidgetField | undefined, queryId: string) {
  const rows = useWidgetRows(params, field ? [field] : [], queryId);
  if (!field || !rows) return null;
  return rows.reduce((a, r) => a + (num(r[field.key]) ?? 0), 0);
}

export function MoneySaved(params: AnyParams) {
  const dm = params.dataMapping as SavedState["dataMapping"];
  const savedField = dm.saved?.[0];
  const potentialField = dm.potential?.[0];
  const saved = useTotal(params, savedField, "saved");
  const potential = useTotal(params, potentialField, "potential");
  useEffect(() => {
    if (saved != null) params.widgetApi.setDisplayState("displayed");
  }, [saved, params.widgetApi]);
  if (saved == null) return null;
  return (
    <div className="saved">
      <span className="saved__badge" aria-hidden>%</span>
      <div>
        <p className="saved__num">{formatVnd(Math.round(saved))}</p>
        <p className="saved__note">{saved > 0 ? at.saved.savedNote : at.saved.none}</p>
        {potential != null && potential > 0 && <p className="saved__more">{at.saved.potentialNote(formatVnd(Math.round(potential)))}</p>}
      </div>
    </div>
  );
}

// ---------------- Lịch chi tiêu ----------------

type CalState = { dataMapping: { date?: AgWidgetField[]; amount?: AgWidgetField[]; detail?: AgWidgetField[] }; format: Record<string, unknown> };

const pad2 = (n: number) => String(n).padStart(2, "0");
/** Ngày từ dữ liệu Studio (chuỗi ISO, Date hoặc số mili giây) thành YYYY-MM-DD. */
function dayKey(v: unknown): string | null {
  if (typeof v === "string" && /^\d{4}-\d{2}-\d{2}/.test(v)) return v.slice(0, 10);
  const d = v instanceof Date ? v : typeof v === "number" ? new Date(v) : null;
  if (!d || Number.isNaN(d.getTime())) return null;
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}
/** Số tiền gọn cho ô lịch hẹp: 84K, 1,2Tr (hoặc 1.2M). */
function compact(v: number) {
  const vi = at.cal.week[0] === "T2";
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(v >= 10_000_000 ? 0 : 1).replace(".", vi ? "," : ".")}${vi ? "Tr" : "M"}`;
  if (v >= 1000) return `${Math.round(v / 1000)}K`;
  return String(Math.round(v));
}

export function SpendingCalendar(params: AnyParams) {
  const dm = params.dataMapping as CalState["dataMapping"];
  const date = dm.date?.[0];
  const amount = dm.amount?.[0];
  const detail = dm.detail?.[0];
  const rows = useWidgetRows(params, date && amount ? [date, ...(detail ? [detail] : []), amount] : [], "calendar");
  const byDay = new Map<string, { total: number; parts: { name: string; amount: number }[] }>();
  for (const r of rows ?? []) {
    const k = date ? dayKey(r[date.key]) : null;
    const v = amount ? num(r[amount.key]) : null;
    if (!k || v == null) continue;
    const e = byDay.get(k) ?? { total: 0, parts: [] };
    e.total += v;
    e.parts.push({ name: detail ? String(r[detail.key] ?? "") : "", amount: v });
    byDay.set(k, e);
  }
  const days = [...byDay.keys()].sort();
  const latest = days.at(-1);
  const [month, setMonth] = useState<string | null>(null); // YYYY-MM đang xem
  const [picked, setPicked] = useState<string | null>(null);
  const ym = month ?? latest?.slice(0, 7) ?? new Date().toISOString().slice(0, 7);
  // Dữ liệu đổi (lọc tháng khác) mà tháng đang xem không còn ngày nào thì về tháng mới nhất.
  useEffect(() => {
    if (month && latest && !days.some((d) => d.startsWith(month))) setMonth(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [latest, days.length]);
  if (!rows || !date || !amount) return null;

  const [y, m] = ym.split("-").map(Number);
  const first = new Date(y, m - 1, 1);
  const lead = (first.getDay() + 6) % 7; // thứ Hai đầu tuần
  const count = new Date(y, m, 0).getDate();
  const cells: (string | null)[] = [...Array(lead).fill(null), ...Array.from({ length: count }, (_, i) => `${ym}-${pad2(i + 1)}`)];
  while (cells.length % 7) cells.push(null);
  const inMonth = days.filter((d) => d.startsWith(ym));
  const monthTotal = inMonth.reduce((a, d) => a + byDay.get(d)!.total, 0);
  const max = Math.max(1, ...inMonth.map((d) => byDay.get(d)!.total));
  const shift = (n: number) => {
    const d = new Date(y, m - 1 + n, 1);
    setMonth(`${d.getFullYear()}-${pad2(d.getMonth() + 1)}`);
    setPicked(null);
  };
  const sel = picked && picked.startsWith(ym) ? picked : null;
  const selDay = sel ? byDay.get(sel) : undefined;
  const parts = selDay ? [...selDay.parts].sort((a, b) => b.amount - a.amount) : [];
  const today = dayKey(new Date());

  return (
    <div className="cal">
      <div className="cal__main">
        <div className="cal__head">
          <button type="button" className="cal__nav" onClick={() => shift(-1)} aria-label={at.cal.prev}>‹</button>
          <strong>{at.cal.month(y, m)}</strong>
          <button type="button" className="cal__nav" onClick={() => shift(1)} aria-label={at.cal.next}>›</button>
          <span className="cal__total">{at.cal.total}: <b>-{formatVnd(Math.round(monthTotal))}</b></span>
        </div>
        <div className="cal__grid" role="grid" aria-label={at.cal.month(y, m)}>
          {at.cal.week.map((w, i) => (
            <span key={w} className={`cal__wd${i === 5 ? " cal__wd--sat" : i === 6 ? " cal__wd--sun" : ""}`} role="columnheader">{w}</span>
          ))}
          {cells.map((d, i) => {
            if (!d) return <span key={`e${i}`} className="cal__cell cal__cell--empty" />;
            const e = byDay.get(d);
            const heat = e ? 0.12 + 0.5 * (e.total / max) : 0;
            return (
              <button
                key={d}
                type="button"
                role="gridcell"
                className={`cal__cell${d === sel ? " cal__cell--sel" : ""}${d === today ? " cal__cell--today" : ""}`}
                style={e ? { background: `rgba(210, 63, 63, ${heat.toFixed(2)})` } : undefined}
                aria-label={`${at.cal.day(d)}${e ? `: ${formatVnd(Math.round(e.total))}` : ""}`}
                aria-pressed={d === sel}
                onClick={() => setPicked(d === sel ? null : d)}
              >
                <span className="cal__num">{Number(d.slice(8))}</span>
                {e && <span className="cal__amt">-{compact(e.total)}</span>}
              </button>
            );
          })}
        </div>
      </div>
      <div className="cal__side" aria-live="polite">
        {sel ? (
          <>
            <strong className="cal__dayhead">
              {at.cal.day(sel)}
              {selDay && <span>-{formatVnd(Math.round(selDay.total))}</span>}
            </strong>
            {parts.length ? (
              <ul className="cal__list">
                {parts.map((p, i) => (
                  <li key={`${p.name}-${i}`}>
                    <span>{p.name || at.cal.amount}</span>
                    <b>-{formatVnd(Math.round(p.amount))}</b>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="cal__hint">{at.cal.none}</p>
            )}
          </>
        ) : (
          <p className="cal__hint">{at.cal.pick}</p>
        )}
      </div>
    </div>
  );
}

// ---------------- Đăng ký với Studio ----------------

const radarIcon =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1.5" fill="currentColor"/><circle cx="17" cy="8" r="1.6" fill="currentColor"/></svg>';
const savedIcon =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M20.6 13.4 13.4 20.6a2 2 0 0 1-2.8 0L3 13V3h10l7.6 7.6a2 2 0 0 1 0 2.8Z"/><circle cx="7.5" cy="7.5" r="1.5"/></svg>';

const radarDef: AgWidgetDefinition<"mimi-runout-radar"> = {
  id: "mimi-runout-radar",
  label: at.radar.label,
  icon: radarIcon,
  dataMapping: {
    item: { type: "field", supportedRoles: ["category"], required: true, intent: "category", aiDescription: "The item name, e.g. items.name." },
    days: { type: "field", supportedRoles: ["numeric"], required: true, intent: "value", aiDescription: "Days left before the item runs out, e.g. items.daysLeft (min)." },
  },
  form: ({ createDefaults }) =>
    createDefaults({ dataMappingItems: [{ key: "item", label: at.radar.item }, { key: "days", label: at.radar.days }] }),
  defaultState: {
    dataMapping: { item: [{ id: "items.name" }], days: [{ id: "items.daysLeft", aggregation: "min" }] },
    format: { title: { enabled: true, text: at.radar.title } },
  } as never,
  comp: RunoutRadar as never,
  defaultSize: { width: 10, height: 18 },
  minSize: { width: 6, height: 12 },
  ai: { label: at.radar.label, description: at.radar.aiDesc, usage: at.radar.aiUse },
};

const savedDef: AgWidgetDefinition<"mimi-money-saved"> = {
  id: "mimi-money-saved",
  label: at.saved.label,
  icon: savedIcon,
  dataMapping: {
    saved: { type: "field", supportedRoles: ["numeric"], required: true, intent: "value", aiDescription: "Money saved, e.g. receipts.discount (sum)." },
    potential: { type: "field", supportedRoles: ["numeric"], intent: "value", aiDescription: "Money that could still be saved, e.g. prices.saving (sum)." },
  },
  form: ({ createDefaults }) =>
    createDefaults({ dataMappingItems: [{ key: "saved", label: at.saved.saved }, { key: "potential", label: at.saved.potential }] }),
  defaultState: {
    dataMapping: { saved: [{ id: "receipts.discount", aggregation: "sum" }], potential: [{ id: "prices.saving", aggregation: "sum" }] },
    format: { title: { enabled: true, text: at.saved.title } },
  } as never,
  comp: MoneySaved as never,
  defaultSize: { width: 8, height: 8 },
  minSize: { width: 5, height: 6 },
  ai: { label: at.saved.label, description: at.saved.aiDesc, usage: at.saved.aiUse },
};

const calIcon =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/><rect x="7" y="13" width="3" height="3" fill="currentColor" stroke="none"/></svg>';

const calDef: AgWidgetDefinition<"mimi-spending-calendar"> = {
  id: "mimi-spending-calendar",
  label: at.cal.label,
  icon: calIcon,
  dataMapping: {
    date: { type: "field", supportedRoles: ["temporal", "category"], required: true, intent: "category", aiDescription: "The day of each payment, e.g. receipts.date." },
    amount: { type: "field", supportedRoles: ["numeric"], required: true, intent: "value", aiDescription: "Money spent, e.g. receipts.total (sum)." },
    detail: { type: "field", supportedRoles: ["category"], intent: "category", aiDescription: "What each payment was, shown when a day is tapped, e.g. receipts.store." },
  },
  form: ({ createDefaults }) =>
    createDefaults({ dataMappingItems: [{ key: "date", label: at.cal.date }, { key: "amount", label: at.cal.amount }, { key: "detail", label: at.cal.detail }] }),
  defaultState: {
    dataMapping: { date: [{ id: "receipts.date" }], amount: [{ id: "receipts.total", aggregation: "sum" }], detail: [{ id: "receipts.store" }] },
    format: { title: { enabled: true, text: at.cal.title } },
  } as never,
  comp: SpendingCalendar as never,
  defaultSize: { width: 24, height: 30 },
  minSize: { width: 8, height: 22 },
  ai: { label: at.cal.label, description: at.cal.aiDesc, usage: at.cal.aiUse },
};

export const MIMI_WIDGETS = [radarDef, savedDef, calDef];

/** Widget mặc định của Studio cộng hai widget Mimi, đặt trong nhóm "Mimi" đầu menu. */
export const mimiWidgets = createWidgets({
  additionalTypes: MIMI_WIDGETS as never,
  menu: [
    { label: "Mimi", widgetIds: ["mimi-runout-radar", "mimi-money-saved", "mimi-spending-calendar"] as never },
    { label: "Charts", widgetIds: ["column-chart-grouped", "bar-chart-grouped", "line-chart", "area-chart", "donut-chart", "pie-chart", "treemap-chart", "sunburst-chart", "combo-chart-grouped-column-line"] as never },
    { label: "Tables", widgetIds: ["grid", "pivot-grid"] as never },
    { label: "KPIs", widgetIds: ["value", "radial-gauge"] as never },
    { label: "Filters", widgetIds: ["date-filter", "button-filter", "list-filter"] as never },
    { label: "Content", widgetIds: ["text"] as never },
  ],
} as never);
