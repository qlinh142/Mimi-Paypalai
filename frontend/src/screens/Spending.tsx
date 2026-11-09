import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { useHideAmounts } from "../lib/hideAmounts";
import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { CalendarDays, ChartPie, ChevronDown, ChevronLeft, ChevronRight, Clock, Minus, ScanLine, ShoppingCart, TrendingDown, TrendingUp, Trophy } from "lucide-react";
import { at as ana } from "../analytics/text";
import { ApiError } from "../api/client";
import { useForecast, useItems, useReceiptHistory, useRecurring, useSpending } from "../api/hooks";
import { ReceiptRow } from "./PurchaseHistory";
import type { ReceiptSummary, SpendingReport } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { recurringTier } from "../components/RecurringList";
import { goodsTier } from "../lib/displayUnit";
import { addMonths, currentMonth, formatDayMonth, formatVnd } from "../lib/text";
import { LoadState } from "../components/LoadState";
import { s22 } from "../i18n/s22";
import { daysUntil } from "../lib/displayUnit";
import { BudgetBar, BudgetInvite, BudgetSheet, CategoryDonut, Noticed, SafeToSpend, Sparkline, billsLeft, monthClock, safeToSpend, useBudget } from "../components/SpendingPlus";

const ym = (s: string) => s.split("-").map(Number) as [number, number];

/**
 * Sổ chi tiêu (Mimi 1.5, ảnh mẫu 4-so-chi-tieu.png): hai nút Đã chi / Sắp chi.
 * Mọi con số tiền của Đã chi do API tính; Sắp chi cộng các khoản đã có giá theo luật chốt 31/10.
 */
export function Spending() {
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") === "upcoming" ? "upcoming" : "spent";
  return (
    <MainFrame>
      <TopBar title={t.spending.title} sub={s22.subtitle} showTitle />
      <div className="page p15">
        <div className="seg seg15" role="group" aria-label={t.spending.title}>
          <button type="button" aria-pressed={tab === "spent"} onClick={() => setParams({}, { replace: true })}>{m15.spending.spent}</button>
          <button type="button" aria-pressed={tab === "upcoming"} onClick={() => setParams({ tab: "upcoming" }, { replace: true })}>{m15.spending.upcoming}</button>
        </div>
        {tab === "spent" ? <Spent /> : <Upcoming />}
      </div>
    </MainFrame>
  );
}

function Spent() {
  const [month, setMonth] = useState(currentMonth());
  const q = useSpending(month);
  const r = q.data;
  const isCurrent = month >= currentMonth();
  const [y, m] = ym(month);

  if (q.error instanceof ApiError && q.error.status === 403) return <div className="empty"><p>{t.spending.ownerOnly}</p></div>;

  return (
    <>
      <div className="month15" role="group" aria-label={t.spending.title}>
        <button type="button" className="icon-btn" aria-label={t.spending.prevMonth} onClick={() => setMonth(addMonths(month, -1))}>
          <ChevronLeft size={22} aria-hidden />
        </button>
        <span aria-live="polite">{m15.spending.month(m, y)}</span>
        <button
          type="button"
          className="icon-btn"
          aria-label={t.spending.nextMonth}
          aria-disabled={isCurrent || undefined}
          onClick={() => !isCurrent && setMonth(addMonths(month, 1))}
        >
          <ChevronRight size={22} aria-hidden />
        </button>
      </div>

      {q.isPending && <p className="hint">{t.common.loading}</p>}
      {q.isError && !r && (
        <div className="empty">
          <h2>{t.spending.loadFailed}</h2>
          <button className="btn btn--quiet" onClick={() => q.refetch()}>{t.common.retry}</button>
        </div>
      )}
      {r && (r.receipt_count === 0 && r.spent === 0 ? <EmptyMonth /> : <Report r={r} month={month} />)}
    </>
  );
}

function EmptyMonth() {
  return (
    <div className="empty">
      <p>{m15.spending.empty}</p>
      <Link to="/scan" className="btn btn--primary">
        <ScanLine size={20} aria-hidden /> {t.spending.scan}
      </Link>
    </div>
  );
}

function compare(r: SpendingReport, month: string): { dir: "up" | "down" | "flat"; text: string } {
  const [, pm] = ym(addMonths(month, -1));
  // Mimi 2.2 (C3): tháng hiện tại so với cùng kỳ tháng trước; tháng cũ vẫn so cả tháng.
  // Nhãn gọn như Redesign: "Nhiều hơn 34% so với 1–9/10", "Ít hơn 12% so với tháng 9".
  const same = r.same_range_day != null && r.spent_prev_same_range_pct !== undefined;
  const prev = s22.prevShort(pm, same ? r.same_range_day : undefined);
  const pct = same ? r.spent_prev_same_range_pct ?? null : r.spent_prev_month_pct;
  if (pct == null) return { dir: "flat", text: m15.spending.noPrev(prev) };
  const n = Math.round(Math.abs(pct));
  if (n === 0) return { dir: "flat", text: m15.spending.same(prev) };
  return pct > 0 ? { dir: "up", text: m15.spending.more(n, prev) } : { dir: "down", text: m15.spending.less(n, prev) };
}

/** Gộp hóa đơn theo cửa hàng: tổng tiền và lần mua gần nhất. */
function byStore(rows: ReceiptSummary[]) {
  const map = new Map<string, { store: string; total: number; last: string }>();
  for (const x of rows) {
    const store = x.store || t.confirm.unknown;
    const cur = map.get(store) ?? { store, total: 0, last: x.purchased_on };
    cur.total += x.total_vnd;
    if (x.purchased_on > cur.last) cur.last = x.purchased_on;
    map.set(store, cur);
  }
  return [...map.values()].sort((a, b) => b.last.localeCompare(a.last));
}

function Report({ r, month }: { r: SpendingReport; month: string }) {
  // Khoản chi định kỳ luôn đứng đầu danh mục; các nhóm còn lại xếp theo số tiền.
  const REC = "hoa-don-dinh-ky";
  const cats = [...r.by_category].sort((a, b) => (a.key === REC ? -1 : b.key === REC ? 1 : b.amount - a.amount));
  // Mimi 2.2: ngân sách, Còn tiêu được, Mimi nhận thấy chỉ ở tháng hiện tại.
  const isCurrent = month >= currentMonth();
  const clock = monthClock();
  const budget = useBudget();
  const rec = useRecurring();
  const prevQ = useSpending(addMonths(month, -1));
  const [editBudget, setEditBudget] = useState(false);
  const bills = billsLeft(rec.data?.items, clock.end).total;
  const safe = budget != null ? safeToSpend(budget, r.spent, bills, clock.left) : null;
  const best = r.top_saving_receipts[0];
  const stores = byStore(r.recent_receipts);
  // Tổng của tháng đã qua quy đổi theo tỷ giá ngày cuối tháng (cố định); tháng này theo hôm nay.
  const [y, m] = ym(month);
  const rateDay = month >= currentMonth() ? undefined : `${month}-${String(new Date(Date.UTC(y, m, 0)).getUTCDate()).padStart(2, "0")}`;
  return (
    <>
      <section className="hero15 hero22" aria-label={m15.spending.spent}>
        <p className="hero22__label">{isCurrent ? s22.spentIn(m, clock.day, clock.days) : s22.spentIn(m)}</p>
        <div className="hero22__top">
          <HeroAmount text={formatVnd(r.spent, rateDay)} />
          {isCurrent && <Sparkline rows={r.recent_receipts} month={month} day={clock.day} />}
        </div>
        {(r.delegated_spent ?? 0) > 0 && <p className="hero22__note">{m15.spending.delegated(formatVnd(r.delegated_spent ?? 0, rateDay))}</p>}
        {isCurrent && budget != null && <BudgetBar budget={budget} spent={r.spent} bills={bills} onEdit={() => setEditBudget(true)} />}
        <div className="hero22__pills">
          {(() => {
            const c = compare(r, month);
            const Icon = c.dir === "up" ? TrendingUp : c.dir === "down" ? TrendingDown : Minus;
            return <p className="hero15__sub"><Icon size={16} aria-hidden />{c.text}</p>;
          })()}
          {!isCurrent && <p className="hero15__sub">{s22.monthDone}</p>}
        </div>
      </section>

      {isCurrent && (safe ? <SafeToSpend free={safe.free} perDay={safe.perDay} daysLeft={clock.left} /> : <BudgetInvite onSet={() => setEditBudget(true)} />)}
      {isCurrent && <Noticed r={r} prev={prevQ.data} />}
      {editBudget && <BudgetSheet current={budget} onClose={() => setEditBudget(false)} />}

      {/* Phân tích chi tiêu (AG Studio): bảng phân tích, radar sắp hết, Hỏi Mimi. */}
      <Link to="/spending/analytics" className="box15 rowlink15 ana-entry">
        <ChartPie size={20} aria-hidden />
        <span>{ana.open}<small>{ana.openSub}</small></span>
        <ChevronRight size={20} aria-hidden />
      </Link>

      {cats.length > 0 && (
        <section aria-labelledby="cat15">
          <h2 id="cat15" className="h15sec">{m15.spending.byCategory}</h2>
          <CategoryDonut cats={cats} total={r.spent} rateDay={rateDay} prev={isCurrent ? prevQ.data : undefined} />
        </section>
      )}

      {best && best.discount_vnd > 0 && (
        <section className="save15" aria-label={m15.spending.topSaving}>
          <span className="save15__icon"><Trophy size={26} aria-hidden /></span>
          <span>
            <strong>{m15.spending.topSaving}</strong>
            <span>{best.store || t.confirm.unknown}</span>
            <span className="save15__amt">{m15.spending.saved(formatVnd(best.discount_vnd, rateDay))}</span>
          </span>
        </section>
      )}

      {stores.length > 0 && (
        <section aria-labelledby="store15">
          <h2 id="store15" className="h15sec">{m15.spending.stores}</h2>
          <ul className="box15 rows15">
            {stores.map((s, i) => (
              <li key={s.store}>
                <span className={`rows15__icon rows15__icon--${i % 2 ? "red" : "green"}`}><ShoppingCart size={18} aria-hidden /></span>
                <span className="rows15__text">
                  <span>{s.store}</span>
                  <small>{formatDayMonth(s.last)}</small>
                </span>
                <span className="num">{formatVnd(s.total, rateDay)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {/* Spec 1.5 mục 4: tóm tắt 3 hóa đơn mới nhất; bấm tiêu đề mở màn 4c, bấm hóa đơn mở 4d. */}
      <HistorySummary />
    </>
  );
}

/**
 * Sắp chi (chốt 31/10): Khoản chi định kỳ Đến hạn + Sắp tới (≤ 7 ngày) và Hàng tiêu dùng
 * Cần mua ngay + Sắp hết, tính theo giá đã trả gần nhất (lấy từ Dự báo của API).
 */
function Upcoming() {
  const rec = useRecurring();
  const items = useItems({ tab: "active", horizon: 7 });
  const fc = useForecast();
  const [open, setOpen] = useState<"rec" | "goods" | null>(null);

  const recRows = useMemo(
    () => (rec.data?.items ?? []).filter((r) => recurringTier(r) !== "later").map((r) => ({ key: r.id, name: r.name, sub: `${formatDayMonth(r.next_due_at)} · ${s22.due(daysUntil(r.next_due_at))}`, amount: r.amount as number | null })),
    [rec.data],
  );
  const goodsRows = useMemo(() => {
    const price = new Map((fc.data?.shopping ?? []).map((s) => [s.key, s.amount]));
    return (items.data ?? []).filter((i) => goodsTier(i) !== "plenty").map((i) => ({ key: i.key, name: i.name, sub: "", amount: price.get(i.key) ?? null }));
  }, [items.data, fc.data]);

  const sum = (rows: { amount: number | null }[]) => rows.reduce((a, x) => a + (x.amount ?? 0), 0);
  const recTotal = sum(recRows);
  const goodsTotal = sum(goodsRows);

  // A10: một trong hai nguồn lỗi thì báo kèm Thử lại, không hiện 0đ như thể chưa có khoản nào.
  if (rec.isError && !rec.data) return <LoadState q={rec} hasData={false} />;
  if (items.isError && !items.data) return <LoadState q={items} hasData={false} />;
  if (rec.isPending || items.isPending) return <p className="hint">{t.common.loading}</p>;

  const lines = [
    { id: "rec" as const, Icon: CalendarDays, label: m15.spending.recurring, total: recTotal, rows: recRows },
    { id: "goods" as const, Icon: ShoppingCart, label: m15.spending.goods, total: goodsTotal, rows: goodsRows },
  ];

  return (
    <section className="hero15 fc15" aria-labelledby="fc15">
      <p id="fc15" className="fc15__label">{m15.spending.forecast}</p>
      <p className="hero15__num num">{formatVnd(recTotal + goodsTotal)}</p>
      <ul className="fc15__lines">
        {lines.map(({ id, Icon, label, total, rows }) => (
          <li key={id}>
            <button type="button" className="fc15__line" aria-expanded={open === id} onClick={() => setOpen(open === id ? null : id)}>
              <Icon size={22} aria-hidden />
              <span>{label}</span>
              <span className="num">{formatVnd(total)}</span>
              {open === id ? <ChevronDown size={20} aria-hidden /> : <ChevronRight size={20} aria-hidden />}
            </button>
            {open === id && (
              <ul className="fc15__rows">
                {rows.map((x) => (
                  <li key={x.key}>
                    <span>{x.name}{x.sub && <small> · {x.sub}</small>}</span>
                    <span className="num">{x.amount != null ? formatVnd(x.amount) : m15.spending.noPrice}</span>
                  </li>
                ))}
              </ul>
            )}
          </li>
        ))}
      </ul>
      <p className="hint">{m15.spending.forecastNote}</p>
    </section>
  );
}

function HistorySummary() {
  const h = useReceiptHistory();
  const rows = [...(h.data ?? [])].sort((a, b) => b.purchased_at.localeCompare(a.purchased_at)).slice(0, 3);
  if (rows.length === 0) return null;
  return (
    <section>
      {/* Anh chốt 09/11: Lịch sử đơn hàng ở một chỗ, tab thứ ba của Kế hoạch. */}
      <Link to="/plan?tab=history" className="box15 rowlink15">
        <Clock size={20} aria-hidden />
        <span>{m15.spending.history}</span>
        <ChevronRight size={20} aria-hidden />
      </Link>
      {/* Mimi 2.2: tiêu đề ngày cho 3 hóa đơn mới nhất (ngày theo giờ máy). */}
      {groupByDay(rows).map(([day, list]) => (
        <div key={day} className="day22">
          <h3 className="day22__head"><span>{formatDayMonth(day)}</span><span className="num">{formatVnd(list.reduce((a, x) => a + x.total_vnd, 0))}</span></h3>
          <ul className="box15 ph15__list">{list.map((x) => <ReceiptRow key={x.receipt_id} r={x} />)}</ul>
        </div>
      ))}
    </section>
  );
}

function groupByDay<T extends { purchased_at: string }>(rows: T[]): [string, T[]][] {
  const out = new Map<string, T[]>();
  for (const r of rows) {
    const d = new Date(r.purchased_at);
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    out.set(key, [...(out.get(key) ?? []), r]);
  }
  return [...out.entries()];
}

/** Số tiền ở thẻ tổng; khi bật Ẩn số tiền (Cài đặt) thì làm mờ, chạm để xem. */
function HeroAmount({ text }: { text: string }) {
  const hidden = useHideAmounts();
  const [peek, setPeek] = useState(false);
  if (!hidden || peek) return <p className="hero15__num num">{text}</p>;
  return (
    <p className="hero15__num num">
      <button type="button" className="amt22--hidden" aria-label={m15.settings.showAmount} onClick={() => setPeek(true)}>{text}</button>
    </p>
  );
}
