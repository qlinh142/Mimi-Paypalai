/**
 * Trang chi tiết của một thẻ trên Tổng quan (bản điện thoại): chạm thẻ thì mở toàn màn, có số
 * chính, biểu đồ lớn, đoạn "Đây là gì?" và nút qua thẻ trước/sau. Số liệu tính thẳng từ dữ liệu
 * của bảng (AnalyticsRows), cùng cách tính với các tool của Hỏi Mimi.
 */
import { useEffect, useMemo, useRef } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { addMonths, currentMonth, formatVnd } from "../lib/text";
import { runningOut, savings, spendingSummary } from "./askMimi";
import { chartFor, ChatChartView, type ChatChart } from "./chatCharts";
import type { AnalyticsRows } from "./data";
import { at } from "./text";

type Detail = { value: string; sub?: string; chart: ChatChart | null; list?: { name: string; note: string; amount: string }[] };

const dayOf = (iso: string) => Number(iso.slice(8, 10));

/** Cộng dồn chi theo ngày trong một tháng (YYYY-MM), độ dài = số ngày đã qua (tháng cũ: cả tháng). */
function cumulative(rows: AnalyticsRows, month: string, days: number) {
  const per = Array<number>(days).fill(0);
  for (const r of rows.receipts) if (r.month === month && dayOf(r.date) <= days) per[dayOf(r.date) - 1] += r.total;
  let acc = 0;
  return per.map((v) => (acc += v));
}

function buildDetail(id: string, rows: AnalyticsRows): Detail {
  const month = currentMonth();
  const d = at.detail;
  if (id === "kpi-spent") {
    const today = new Date().getDate();
    const now = cumulative(rows, month, today);
    const prev = [1, 2, 3].map((n) => cumulative(rows, addMonths(month, -n), today));
    const avg = now.map((_, i) => Math.round(prev.reduce((a, p) => a + p[i], 0) / 3));
    const spent = now.at(-1) ?? 0;
    const base = avg.at(-1) ?? 0;
    const pct = base > 0 ? Math.round(((spent - base) / base) * 100) : 0;
    return {
      value: formatVnd(Math.round(spent)),
      sub: base > 0 ? d.vsAvg(pct) : undefined,
      chart: { title: d.cards["kpi-spent"].title, kind: "line", labels: now.map((_, i) => String(i + 1)), series: [{ name: d.thisMonth, values: now }, { name: d.avg3, values: avg }], unit: "vnd" },
    };
  }
  if (id === "by-category") {
    const s = spendingSummary(rows, month);
    const top = s.by_category[0];
    return { value: top ? formatVnd(top.amount) : formatVnd(0), sub: top?.name, chart: chartFor("mimi_spending_summary", JSON.stringify(s)) };
  }
  if (id === "kpi-upcoming") {
    const bills = rows.upcoming;
    const total = bills.reduce((a, b) => a + b.amount, 0);
    return {
      value: formatVnd(Math.round(total)),
      sub: bills.length ? d.bills(bills.length) : d.noBills,
      chart: bills.length
        ? { title: d.cards["kpi-upcoming"].title, kind: "bar", labels: bills.slice(0, 10).map((b) => `${b.name} · ${b.due.slice(8)}/${b.due.slice(5, 7)}`), series: [{ name: at.cal.amount, values: bills.slice(0, 10).map((b) => Math.round(b.amount)) }], unit: "vnd" }
        : null,
      list: bills.map((b) => ({ name: b.name, note: at.cal.day(b.due), amount: `-${formatVnd(Math.round(b.amount))}` })),
    };
  }
  if (id === "radar") {
    const list = runningOut(rows, 30);
    const soon = list.filter((x) => (x.days_left ?? 99) <= 7).length;
    return { value: String(soon), sub: d.lowSoon(soon), chart: chartFor("mimi_running_out", JSON.stringify(list)) };
  }
  if (id === "saved") {
    const s = savings(rows);
    return { value: formatVnd(s.saved_with_discounts), sub: s.could_save_more > 0 ? d.more(formatVnd(s.could_save_more)) : undefined, chart: chartFor("mimi_savings", JSON.stringify(s)) };
  }
  // kpi-avg: trung bình mỗi hóa đơn theo tháng, 6 tháng gần nhất.
  const by = new Map<string, { t: number; n: number }>();
  for (const r of rows.receipts) {
    const e = by.get(r.month) ?? { t: 0, n: 0 };
    e.t += r.total;
    e.n += 1;
    by.set(r.month, e);
  }
  const months = [...by.keys()].sort().slice(-6);
  const vals = months.map((m) => Math.round(by.get(m)!.t / by.get(m)!.n));
  return {
    value: formatVnd(vals.at(-1) ?? 0),
    sub: months.at(-1),
    chart: months.length ? { title: d.avgBy, kind: "column", labels: months, series: [{ name: at.r.avgBill, values: vals }], unit: "vnd" } : null,
  };
}

export function CardDetail({ rows, ids, index, onMove, onClose }: { rows: AnalyticsRows; ids: string[]; index: number; onMove: (i: number) => void; onClose: () => void }) {
  const id = ids[index];
  const info = at.detail.cards[id];
  const detail = useMemo(() => buildDetail(id, rows), [id, rows]);
  const closeRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    closeRef.current?.focus();
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if (e.key === "ArrowLeft" && index > 0) onMove(index - 1);
      if (e.key === "ArrowRight" && index < ids.length - 1) onMove(index + 1);
    };
    window.addEventListener("keydown", key);
    return () => {
      window.removeEventListener("keydown", key);
      opener?.focus?.();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [index]);
  if (!info) return null;
  return (
    <div className="cd" role="dialog" aria-modal="true" aria-labelledby="cd-title">
      <header className="cd__top">
        <button ref={closeRef} type="button" className="cd__round" onClick={onClose} aria-label={at.detail.close}><X size={22} aria-hidden /></button>
        <div className="cd__head">
          <h2 id="cd-title">{info.title}</h2>
        </div>
        <span className="cd__round cd__round--ghost" aria-hidden />
      </header>
      <div className="cd__body">
        <p className="cd__value">{detail.value}</p>
        {detail.sub && <p className="cd__sub">{detail.sub}</p>}
        {detail.chart && (
          <div className="cd__chart">
            <ChatChartView chart={{ ...detail.chart, title: "" }} />
          </div>
        )}
        {detail.list && detail.list.length > 0 && (
          <ul className="cd__list">
            {detail.list.map((x, i) => (
              <li key={`${x.name}-${i}`}>
                <span><b>{x.name}</b><small>{x.note}</small></span>
                <strong>{x.amount}</strong>
              </li>
            ))}
          </ul>
        )}
        <h3 className="cd__what">{at.detail.what}</h3>
        <p className="cd__text">{info.text}</p>
      </div>
      <nav className="cd__nav" aria-label={info.title}>
        <button type="button" className="cd__round" disabled={index === 0} onClick={() => onMove(index - 1)} aria-label={at.detail.prev}><ChevronLeft size={22} aria-hidden /></button>
        <span className="cd__dots" aria-hidden>
          {ids.map((x, i) => <i key={x} className={i === index ? "on" : ""} />)}
        </span>
        <button type="button" className="cd__round" disabled={index === ids.length - 1} onClick={() => onMove(index + 1)} aria-label={at.detail.next}><ChevronRight size={22} aria-hidden /></button>
      </nav>
    </div>
  );
}
