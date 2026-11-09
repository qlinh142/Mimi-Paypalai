/**
 * Các khối Sổ chi tiêu thêm ở Mimi 2.2 (đề xuất tổng hợp chốt 09/11, theo bản Redesign):
 * thanh ngân sách, Còn tiêu được, Mimi nhận thấy và biểu đồ vòng theo nhóm hàng.
 * Mọi số tiền gốc vẫn do API trả về; ở đây chỉ cộng trừ để hiển thị như spec Redesign mục 5.1.
 */
import { useLayoutEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { AlertTriangle, CalendarClock, ChevronRight, Pencil, PiggyBank, Repeat, Store, TrendingDown, TrendingUp, Wallet } from "lucide-react";
import { categoryLabel, t } from "../i18n";
import { s22 } from "../i18n/s22";
import { ApiError } from "../api/client";
import { useForecast, useItems, useMe, useRecurring, useSaveSettings } from "../api/hooks";
import type { ReceiptSummary, Recurring, SpendingReport } from "../api/types";
import { daysUntil, goodsTier } from "../lib/displayUnit";
import { formatVnd } from "../lib/text";
import { Sheet } from "./Sheet15";
import { PlanPicker } from "./PlanPicker";

export const REC = "hoa-don-dinh-ky";
const MAX_BUDGET = 1_000_000_000;

/** Ngân sách tháng đã đặt (đồng), null khi chưa đặt. */
export function useBudget(): number | null {
  const me = useMe();
  const v = me.data?.settings.monthly_budget_vnd;
  return typeof v === "number" && v > 0 ? v : null;
}

/** Hôm nay theo máy, số ngày của tháng và số ngày còn lại (tính cả hôm nay). */
export function monthClock(now = new Date()) {
  const days = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  const day = now.getDate();
  return { day, days, left: days - day + 1, end: `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(days).padStart(2, "0")}` };
}

/** Khoản định kỳ chưa trả, đến hạn trước hoặc đúng ngày cuối tháng (kể cả khoản đã quá hạn). */
export function billsLeft(list: Recurring[] | undefined, end: string): { total: number; rows: Recurring[] } {
  const rows = (list ?? []).filter((r) => r.active && r.next_due_at && r.next_due_at.slice(0, 10) <= end);
  return { total: rows.reduce((a, r) => a + (r.amount ?? 0), 0), rows };
}

/** Spec Redesign 5.1: free = ngân sách − đã chi − định kỳ còn lại; mỗi ngày làm tròn 100. */
export function safeToSpend(budget: number, spent: number, bills: number, daysLeft: number) {
  const free = budget - spent - bills;
  const perDay = free > 0 ? Math.round(free / Math.max(1, daysLeft) / 100) * 100 : 0;
  return { free, perDay };
}

/** Thanh ngân sách 3 phần trong thẻ tổng: đã chi, định kỳ sắp trả, còn lại. */
export function BudgetBar({ budget, spent, bills, onEdit }: { budget: number; spent: number; bills: number; onEdit: () => void }) {
  const total = Math.max(budget, spent + bills, 1);
  const w = (v: number) => `${Math.max(0, Math.min(100, (v / total) * 100))}%`;
  const left = Math.max(0, budget - spent - bills);
  const over = Math.max(0, spent + bills - budget);
  const pct = Math.round((spent / Math.max(1, budget)) * 100);
  return (
    <div className="bud22">
      <div className="bud22__bar" role="img" aria-label={`${s22.budget.spent} ${formatVnd(spent)}, ${s22.budget.bills} ${formatVnd(bills)}, ${over > 0 ? `${s22.budget.over} ${formatVnd(over)}` : `${s22.budget.left} ${formatVnd(left)}`}`}>
        <span className="bud22__seg bud22__seg--spent" style={{ width: w(spent) }} />
        <span className="bud22__seg bud22__seg--bills" style={{ width: w(bills) }} />
      </div>
      <div className="bud22__row">
        <span>{s22.pctSpent(pct)}</span>
        <span>{s22.billsDue(formatVnd(bills))}</span>
        <button type="button" className="bud22__edit" onClick={onEdit} aria-label={s22.budget.edit}>
          {s22.ofBudget(formatVnd(budget))} <Pencil size={14} aria-hidden />
        </button>
      </div>
    </div>
  );
}

/** Bảng sửa ngân sách. Lưu qua PUT /settings như mọi cài đặt (không xếp hàng ngoại tuyến). */
export function BudgetSheet({ current, onClose }: { current: number | null; onClose: () => void }) {
  const save = useSaveSettings();
  const [text, setText] = useState(current ? String(current) : "");
  const [bad, setBad] = useState(false);
  const submit = (value: number) =>
    save.mutate({ monthly_budget_vnd: value }, { onSuccess: onClose });
  const onSave = () => {
    const n = Number(text.replace(/[^\d]/g, ""));
    if (!Number.isFinite(n) || n < 1000 || n > MAX_BUDGET) return setBad(true);
    setBad(false);
    submit(n);
  };
  return (
    <Sheet title={s22.budget.sheet} sub={s22.budget.sheetSub} onClose={onClose}>
      <form className="bud22__form" onSubmit={(e) => { e.preventDefault(); onSave(); }}>
        <label className="field15">
          <span>{s22.budget.amount}</span>
          <input
            className="input num"
            inputMode="numeric"
            autoFocus
            value={text ? Number(text.replace(/[^\d]/g, "") || 0).toLocaleString("vi-VN") : ""}
            onChange={(e) => setText(e.target.value.replace(/[^\d]/g, "").slice(0, 10))}
            aria-invalid={bad || undefined}
            aria-describedby="bud22hint"
          />
        </label>
        <p id="bud22hint" className={bad ? "error" : "hint"} role={bad ? "alert" : undefined}>{bad ? s22.budget.bad : s22.budget.hint}</p>
        {save.isError && <p className="error" role="alert">{save.error instanceof ApiError ? save.error.message : t.common.genericError}</p>}
        <button type="submit" className="btn btn--primary" disabled={save.isPending}>{s22.budget.save}</button>
        {current != null && (
          <button type="button" className="btn btn--quiet" disabled={save.isPending} onClick={() => submit(0)}>{s22.budget.remove}</button>
        )}
      </form>
    </Sheet>
  );
}

/** Lời mời đặt ngân sách khi chưa có (spec Redesign mục 7). */
export function BudgetInvite({ onSet }: { onSet: () => void }) {
  return (
    <button type="button" className="box15 rowlink15 safe22 safe22--invite" onClick={onSet}>
      <span className="safe22__icon"><PiggyBank size={22} aria-hidden /></span>
      <span>{s22.budget.invite}<small>{s22.budget.inviteSub}</small></span>
      <ChevronRight size={20} aria-hidden />
    </button>
  );
}

/** Mở màn Phân tích với khung Hỏi Mimi mở sẵn và câu hỏi điền sẵn (người dùng tự bấm gửi). */
export function askPath(q: string) {
  return `/spending/analytics?ask=${encodeURIComponent(q)}`;
}

/** Thẻ Còn tiêu được gọn như Redesign: số mỗi ngày là số lớn, nút hỏi bên phải. */
export function SafeToSpend({ free, perDay, daysLeft }: { free: number; perDay: number; daysLeft: number }) {
  const over = free < 0;
  return (
    <section className={`box15 safe22${over ? " safe22--over" : ""}`} aria-labelledby="safe22">
      <span className="safe22__icon">{over ? <AlertTriangle size={22} aria-hidden /> : <Wallet size={22} aria-hidden />}</span>
      <div className="safe22__body">
        <h2 id="safe22">{over ? s22.safe.overTitle : s22.safe.title}</h2>
        <p className="safe22__num num">
          {over ? formatVnd(Math.abs(free)) : <>{formatVnd(perDay)}<span>{s22.perDayUnit}</span></>}
        </p>
        <p className="safe22__sub">
          {over ? <Link to="/spending?tab=upcoming">{s22.safe.overSub}</Link> : s22.freeAfter(daysLeft)}
        </p>
      </div>
      {!over && (
        <Link className="pill22 safe22__ask" to={askPath(s22.safe.affordQ(formatVnd(free)))}>{s22.safe.afford}</Link>
      )}
    </section>
  );
}

type Insight = { id: string; c: string; Icon: typeof Store; text: string; sub?: string; cta: string; to?: string; plan?: { key: string; name: string } };

/**
 * Mimi nhận thấy (spec Redesign 5.1, bốn loại), tính ở máy từ API sẵn có:
 * món sắp hết (Thêm vào kế hoạch), khoản định kỳ sắp đến hạn (Xem sắp chi),
 * cửa hàng chiếm phần lớn chi tiêu (Xem hóa đơn), nhóm hàng tăng giảm so tháng trước (Hỏi Mimi vì sao).
 */
/** "**Tiền điện** đến hạn" → Tiền điện in đậm. */
function bold(text: string) {
  return text.split(/\*\*(.+?)\*\*/).map((p, i) => (i % 2 ? <b key={i}>{p}</b> : p));
}

export function Noticed({ r, prev }: { r: SpendingReport; prev?: SpendingReport }) {
  const items = useItems({ tab: "active", horizon: 7 });
  const fc = useForecast();
  const rec = useRecurring();
  const [pick, setPick] = useState<{ key: string; name: string } | null>(null);
  const nav = useNavigate();

  const list = useMemo(() => {
    const out: Insight[] = [];
    // 1. Món sắp hết, ưu tiên món cần mua ngay.
    const price = new Map((fc.data?.shopping ?? []).map((s) => [s.key, s.amount]));
    const low = (items.data ?? []).filter((i) => goodsTier(i) !== "plenty").sort((a, b) => (a.days_left ?? -1) - (b.days_left ?? -1))[0];
    if (low) {
      const p = price.get(low.key);
      out.push({ id: `low:${low.key}`, c: "var(--c-groc, #2f9e6e)", Icon: CalendarClock, text: s22.noticed.low(low.name, low.days_left), sub: p != null ? s22.noticed.lowSub(formatVnd(p)) : undefined, cta: s22.noticed.addToPlan, plan: { key: low.key, name: low.name } });
    }
    // 2. Khoản định kỳ đến hạn sớm nhất trong 7 ngày.
    const bill = (rec.data?.items ?? []).filter((x) => x.active && daysUntil(x.next_due_at) <= 7).sort((a, b) => a.next_due_at.localeCompare(b.next_due_at))[0];
    if (bill) {
      out.push({ id: `bill:${bill.id}`, c: "var(--c-bill, #d4538a)", Icon: Repeat, text: s22.noticed.bill(bill.name, daysUntil(bill.next_due_at)), sub: formatVnd(bill.amount), cta: s22.noticed.seeUpcoming, to: "/spending?tab=upcoming" });
    }
    // 3. Cửa hàng chiếm phần lớn nhất (khi có từ 2 cửa hàng).
    const byStore = new Map<string, { total: number; last: string; id: string }>();
    for (const x of r.recent_receipts) {
      if (x.delegated || !x.store) continue;
      const cur = byStore.get(x.store) ?? { total: 0, last: "", id: x.receipt_id };
      cur.total += x.total_vnd;
      if (x.purchased_on > cur.last) { cur.last = x.purchased_on; cur.id = x.receipt_id; }
      byStore.set(x.store, cur);
    }
    const top = [...byStore.entries()].sort((a, b) => b[1].total - a[1].total)[0];
    if (top && byStore.size > 1 && r.spent > 0) {
      const pct = Math.round((top[1].total / r.spent) * 100);
      if (pct >= 25) out.push({ id: `store:${top[0]}`, c: "var(--c-home, #2a5bd7)", Icon: Store, text: s22.noticed.store(top[0], pct), cta: s22.noticed.viewReceipt, to: `/spending/receipt/${encodeURIComponent(top[1].id)}` });
    }
    // 4. Nhóm hàng đổi nhiều nhất so với tháng trước (từ 20% và 50.000đ trở lên).
    if (prev) {
      const before = new Map(prev.by_category.map((c) => [c.key, c.amount]));
      const moves = r.by_category
        .filter((c) => c.key !== REC && (before.get(c.key) ?? 0) > 0)
        .map((c) => { const b = before.get(c.key)!; return { key: c.key, diff: c.amount - b, pct: Math.round(((c.amount - b) / b) * 100) }; })
        .filter((m) => Math.abs(m.pct) >= 20 && Math.abs(m.diff) >= 50_000)
        .sort((a, b) => Math.abs(b.diff) - Math.abs(a.diff));
      const m = moves[0];
      if (m) {
        const up = m.diff > 0;
        const cat = categoryLabel(m.key);
        out.push({ id: `trend:${m.key}`, c: "var(--c-care, #8b5cf6)", Icon: up ? TrendingUp : TrendingDown, text: s22.noticed.trend(cat, Math.abs(m.pct), up), cta: s22.noticed.askWhy, to: askPath(s22.noticed.whyQ(cat.toLowerCase(), up)) });
      }
    }
    return out;
  }, [items.data, fc.data, rec.data, r, prev]);

  // Thẻ đến dần (đồ dùng, định kỳ tải sau); cuộn bắt dính sẽ giữ thẻ cũ nên đưa hàng về thẻ đầu.
  const row = useRef<HTMLUListElement>(null);
  const ids = list.map((x) => x.id).join("|");
  useLayoutEffect(() => { if (row.current) row.current.scrollLeft = 0; }, [ids]);

  if (list.length === 0) return null;
  return (
    <section className="noticed22wrap" aria-labelledby="noticed22">
      <h2 id="noticed22" className="h15sec">{s22.noticed.title}</h2>
      <ul className="noticed22" ref={row}>
        {list.map((x) => (
          <li key={x.id} className="box15 noticed22__card" style={{ ["--c" as string]: x.c }}>
            <span className="noticed22__icon"><x.Icon size={20} aria-hidden /></span>
            <p className="noticed22__text"><span>{bold(x.text)}</span>{x.sub && <small>{x.sub}</small>}</p>
            <button type="button" className="noticed22__cta" onClick={() => (x.plan ? setPick(x.plan) : x.to && nav(x.to))}>
              {x.cta} <ChevronRight size={16} aria-hidden />
            </button>
          </li>
        ))}
      </ul>
      {pick && <PlanPicker planKey={pick.key} name={pick.name} onClose={() => setPick(null)} />}
    </section>
  );
}

const COLORS = ["var(--c-bill, #d4538a)", "var(--c-groc, #2f9e6e)", "var(--c-home, #2a5bd7)", "var(--c-baby, #e8873a)", "var(--c-care, #8b5cf6)", "#0ea5b7", "#a1a1aa"];

/**
 * Theo nhóm hàng: biểu đồ vòng bên trái, chú giải bên phải như Redesign (anh chốt 09/11).
 * Tháng hiện tại: mỗi nhóm có nhãn tăng giảm so với tháng trước; tháng cũ: phần trăm.
 * Khoản chi định kỳ vẫn đứng đầu chú giải và bấm mở Đồ dùng; nhóm khác chạm để xem số ở giữa vòng.
 */
export function CategoryDonut({ cats, total, rateDay, prev }: { cats: SpendingReport["by_category"]; total: number; rateDay?: string; prev?: SpendingReport }) {
  const [sel, setSel] = useState<string | null>(null);
  const sum = cats.reduce((a, c) => a + c.amount, 0) || 1;
  const R = 56, C = 2 * Math.PI * R; // vòng mảnh như Redesign: nét 10, chọn 13, khe 2
  const before = new Map((prev?.by_category ?? []).map((c) => [c.key, c.amount]));
  let acc = 0;
  const arcs = cats.map((c, i) => {
    const len = (c.amount / sum) * C;
    const a = { key: c.key, color: COLORS[Math.min(i, COLORS.length - 1)], len, off: acc };
    acc += len;
    return a;
  });
  const picked = cats.find((c) => c.key === sel);
  const trend = (key: string, pct: number) => {
    const b = before.get(key);
    if (!prev || !b) return <span className="donut22__pct">{Math.round(pct)}%</span>;
    const cur = cats.find((c) => c.key === key)!.amount;
    const d = Math.round((cur / b - 1) * 100);
    return <span className={`tr22 tr22--${d < 0 ? "down" : d > 0 ? "up" : "flat"}`}>{d < 0 ? "↓" : d > 0 ? "↑" : "–"} {Math.abs(d)}%</span>;
  };
  return (
    <div className="box15 donut22">
      <svg viewBox="0 0 140 140" className="donut22__svg" role="img" aria-label={cats.map((c) => `${categoryLabel(c.key)} ${Math.round(c.pct)}%`).join(", ")}>
        <circle cx="70" cy="70" r={R} fill="none" stroke="var(--line, #e3e8f7)" strokeWidth="10" />
        {arcs.map((a) => (
          <circle key={a.key} cx="70" cy="70" r={R} fill="none" stroke={a.color} strokeWidth={sel === a.key ? 13 : 10}
            strokeDasharray={`${Math.max(1, a.len - 2)} ${C}`} strokeDashoffset={-a.off} transform="rotate(-90 70 70)"
            opacity={sel && sel !== a.key ? 0.3 : 1} />
        ))}
        <text x="70" y="65" textAnchor="middle" className="donut22__k">{picked ? categoryLabel(picked.key) : s22.donut.total}</text>
        <text x="70" y="81" textAnchor="middle" className="donut22__v">{formatVnd(picked ? picked.amount : total, rateDay)}</text>
      </svg>
      <ul className="donut22__legend">
        {cats.map((c, i) => {
          const body = (
            <>
              <i className="donut22__dot" style={{ background: COLORS[Math.min(i, COLORS.length - 1)] }} />
              <span className="donut22__name">{categoryLabel(c.key)}<small className="num">{formatVnd(c.amount, rateDay)}</small></span>
              {c.key === REC ? <ChevronRight size={16} aria-hidden /> : trend(c.key, c.pct)}
            </>
          );
          return (
            <li key={c.key}>
              {c.key === REC ? (
                <Link to="/items?tab=recurring" className="donut22__row">{body}</Link>
              ) : (
                <button type="button" className="donut22__row" aria-pressed={sel === c.key} onClick={() => setSel(sel === c.key ? null : c.key)}>{body}</button>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * Đường chi tiêu cộng dồn từ ngày 1 đến hôm nay, trong thẻ tổng (Redesign, C8).
 * Tính từ hóa đơn và người nhà mua hộ của tháng (recent_receipts); đơn online không có ngày
 * nên không nằm trên đường, vì thế chỉ vẽ hình dáng, không ghi số.
 */
export function Sparkline({ rows, month, day }: { rows: ReceiptSummary[]; month: string; day: number }) {
  if (day < 2) return null;
  const perDay = new Array<number>(day).fill(0);
  for (const x of rows) {
    if (!x.purchased_on.startsWith(month)) continue;
    const d = Number(x.purchased_on.slice(8, 10));
    if (d >= 1 && d <= day) perDay[d - 1] += x.total_vnd;
  }
  let c = 0;
  const pts = perDay.map((v) => (c += v));
  const mx = pts[pts.length - 1];
  if (mx <= 0) return null;
  const W = 112, H = 44;
  const xy = pts.map((v, i) => [(i / (pts.length - 1)) * W, H - 4 - (v / mx) * (H - 10)]);
  const line = xy.map((p, i) => `${i ? "L" : "M"}${p[0].toFixed(1)} ${p[1].toFixed(1)}`).join(" ");
  const [lx, ly] = xy[xy.length - 1];
  return (
    <svg className="spark22" viewBox={`0 0 ${W} ${H}`} role="img" aria-label={s22.spark}>
      <path d={`${line} L${W} ${H} L0 ${H}z`} fill="rgb(255 255 255 / .16)" />
      <path d={line} fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={lx} cy={ly} r="3.4" fill="currentColor" />
    </svg>
  );
}
