import { categoryLabel, t } from "../i18n";
import { m15 } from "../i18n/m15";
import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight, Clock, ScanLine, ShoppingCart, Trophy } from "lucide-react";
import { ApiError } from "../api/client";
import { useForecast, useItems, useReceiptHistory, useRecurring, useSpending } from "../api/hooks";
import { ReceiptRow } from "./PurchaseHistory";
import type { ReceiptSummary, SpendingReport } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { recurringTier } from "../components/RecurringList";
import { goodsTier } from "../lib/displayUnit";
import { addMonths, currentMonth, formatDayMonth, formatVnd } from "../lib/text";
import { LoadState } from "../components/LoadState";

const TOP_CATS = 5;
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
      <TopBar title={t.spending.title} />
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

function compare(r: SpendingReport, month: string): string {
  const [py, pm] = ym(addMonths(month, -1));
  const prev = m15.spending.prevMonth(pm, py);
  const pct = r.spent_prev_month_pct;
  if (pct == null) return m15.spending.noPrev(prev);
  const n = Math.round(Math.abs(pct));
  if (n === 0) return m15.spending.same(prev);
  return pct > 0 ? `▲ ${m15.spending.more(n, prev)}` : `▼ ${m15.spending.less(n, prev)}`;
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
  const [allCats, setAllCats] = useState(false);
  // Khoản chi định kỳ luôn đứng đầu danh mục; các nhóm còn lại xếp theo số tiền.
  const REC = "hoa-don-dinh-ky";
  const cats = [...r.by_category].sort((a, b) => (a.key === REC ? -1 : b.key === REC ? 1 : b.amount - a.amount));
  const shown = allCats ? cats : cats.slice(0, TOP_CATS);
  const maxCat = Math.max(1, ...cats.map((c) => c.amount));
  const best = r.top_saving_receipts[0];
  const stores = byStore(r.recent_receipts);
  // Tổng của tháng đã qua quy đổi theo tỷ giá ngày cuối tháng (cố định); tháng này theo hôm nay.
  const [y, m] = ym(month);
  const rateDay = month >= currentMonth() ? undefined : `${month}-${String(new Date(Date.UTC(y, m, 0)).getUTCDate()).padStart(2, "0")}`;
  return (
    <>
      <section className="hero15" aria-label={m15.spending.spent}>
        <p className="hero15__num num">{formatVnd(r.spent, rateDay)}</p>
        <p className="hero15__sub">{compare(r, month)}</p>
        {(r.delegated_spent ?? 0) > 0 && <p className="hero15__sub">{m15.spending.delegated(formatVnd(r.delegated_spent ?? 0, rateDay))}</p>}
      </section>

      {cats.length > 0 && (
        <section aria-labelledby="cat15">
          <h2 id="cat15" className="h15sec">{m15.spending.byCategory}</h2>
          <div className="box15">
            <ul className="bars15">
              {shown.map((c) => {
                const row = (
                  <>
                    <span className="bars15__label">{categoryLabel(c.key)}{c.key === REC && <ChevronRight size={16} aria-hidden />}</span>
                    <span className="bars15__value num">{formatVnd(c.amount, rateDay)}</span>
                    <span className="bars15__pct">{Math.round(c.pct)}%</span>
                    <span className="bars15__track" aria-hidden>
                      <span style={{ width: `${Math.max(2, (c.amount / maxCat) * 100)}%` }} />
                    </span>
                  </>
                );
                // Góp ý 01/11 06:30: dòng Khoản chi định kỳ mở Đồ dùng, phần Khoản chi định kỳ. Các nhóm khác chỉ để xem số.
                return (
                  <li key={c.key}>
                    {c.key === REC ? <Link to="/items?tab=recurring" className="bars15__link">{row}</Link> : row}
                  </li>
                );
              })}
            </ul>
            {cats.length > TOP_CATS && (
              <button type="button" className="link15" aria-expanded={allCats} onClick={() => setAllCats(!allCats)}>
                {allCats ? m15.spending.seeLess : m15.spending.seeAll} <ChevronRight size={16} aria-hidden />
              </button>
            )}
          </div>
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
    () => (rec.data?.items ?? []).filter((r) => recurringTier(r) !== "later").map((r) => ({ key: r.id, name: r.name, sub: formatDayMonth(r.next_due_at), amount: r.amount as number | null })),
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
      <Link to="/spending/history" className="box15 rowlink15">
        <Clock size={20} aria-hidden />
        <span>{m15.spending.history}</span>
        <ChevronRight size={20} aria-hidden />
      </Link>
      <ul className="box15 ph15__list">{rows.map((x) => <ReceiptRow key={x.receipt_id} r={x} />)}</ul>
    </section>
  );
}
