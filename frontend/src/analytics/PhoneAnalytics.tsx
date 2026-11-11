/**
 * Màn Phân tích chi tiêu, bản điện thoại (Mimi 2.2, anh chốt 10/11).
 * Lưới AG Studio thiết kế cho màn rộng: ép xuống 393px thì chữ bị cắt, bảng tràn ngang, biểu đồ
 * thiếu chú thích. Bản điện thoại dựng lại thành một cột thẻ như Sổ chi tiêu, ba trang như cũ
 * (Tổng quan, So sánh giá, Hóa đơn). Bản máy tính vẫn là bảng AG Studio.
 *
 * Quy ước số liệu, để không lệch với các màn khác:
 * - "Tháng này đã chi", chi theo tháng và theo nhóm hàng: lấy từ API Sổ chi tiêu (/spending).
 * - "Sắp chi 7 ngày tới": cùng hook với tab Sắp chi (useUpcoming7).
 * - Cửa hàng và Món tốn tiền nhất: chỉ tính hóa đơn mua hàng, khoản định kỳ tách riêng.
 */
import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { CalendarDays, ChevronLeft, ChevronRight, Minus, ShoppingCart, Sparkles, TrendingDown, TrendingUp } from "lucide-react";
import { useReceiptHistory, useSpending } from "../api/hooks";
import type { SpendingReport } from "../api/types";
import { categoryLabel } from "../i18n";
import { addMonths, currentMonth, formatVnd } from "../lib/text";
import { useUpcoming7 } from "../lib/upcoming7";
import { ReceiptRow } from "../screens/PurchaseHistory";
import { savings } from "./askMimi";
import { ChatChartView } from "./chatCharts";
import type { AnalyticsRows } from "./data";
import { at } from "./text";
import { CalendarView } from "./widgets";

type Page = "overview" | "prices" | "receipts";
const p = at.ph;
const REC_CAT = categoryLabel("hoa-don-dinh-ky");
const monthNum = (m: string) => Number(m.slice(5));

export function PhoneAnalytics({ rows, note, onAsk }: { rows: AnalyticsRows; note?: string; onAsk: () => void }) {
  const [page, setPage] = useState<Page>("overview");
  return (
    <>
      <header className="ana__top">
        <Link to="/spending" className="icon-btn" aria-label={at.back}><ChevronLeft size={26} aria-hidden /></Link>
        <h1>{at.title}</h1>
        <span className="ana__spacer" />
        <button type="button" className="btn btn--outline pa__ask" onClick={onAsk}><Sparkles size={18} aria-hidden /> {p.ask}</button>
      </header>
      <div className="pa">
        <div className="pa__inner p15">
          <div className="seg seg15" role="group" aria-label={at.title}>
            {(["overview", "prices", "receipts"] as const).map((id) => (
              <button key={id} type="button" aria-pressed={page === id} onClick={() => setPage(id)}>{at.pages[id]}</button>
            ))}
          </div>
          {note && <p className="pa__note">{note}</p>}
          {page === "overview" && <Overview rows={rows} onPrices={() => setPage("prices")} />}
          {page === "prices" && <Prices rows={rows} />}
          {page === "receipts" && <Receipts rows={rows} />}
        </div>
      </div>
    </>
  );
}

// ---------------- Tổng quan ----------------

function compare(r: SpendingReport) {
  const same = r.same_range_day != null && r.spent_prev_same_range_pct != null;
  const raw = same ? r.spent_prev_same_range_pct! : r.spent_prev_month_pct;
  if (raw == null) return null;
  const pct = Math.round(raw);
  return { dir: pct > 0 ? "up" : pct < 0 ? "down" : "flat", text: p.vs(pct, monthNum(addMonths(r.month, -1)), same ? r.same_range_day! : undefined) };
}

function Overview({ rows, onPrices }: { rows: AnalyticsRows; onPrices: () => void }) {
  const month = currentMonth();
  const months = [-5, -4, -3, -2, -1, 0].map((n) => addMonths(month, n));
  // Sáu lời gọi cố định (thứ tự hook không đổi); react-query dùng lại cache của Sổ chi tiêu.
  const qs = [useSpending(months[0]), useSpending(months[1]), useSpending(months[2]), useSpending(months[3]), useSpending(months[4]), useSpending(months[5])];
  const r = qs[5].data;
  const prev = qs[4].data;
  const up = useUpcoming7();

  const mine = rows.receipts.filter((x) => x.month === month && x.kindKey !== "recurring");
  const ids = new Set(mine.map((x) => x.id));
  const top = useMemo(() => {
    const m = new Map<string, number>();
    for (const l of rows.lines) if (ids.has(l.receiptId) && l.category !== REC_CAT && l.amount) m.set(l.name, (m.get(l.name) ?? 0) + l.amount);
    return [...m.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, month]);
  const bills = rows.receipts.filter((x) => x.month === month && x.kindKey === "recurring").sort((a, b) => b.date.localeCompare(a.date));
  const soon = rows.items.filter((i) => i.daysLeft != null && i.daysLeft <= 7).sort((a, b) => (a.daysLeft ?? 0) - (b.daysLeft ?? 0));
  const sv = savings(rows);
  const c = r ? compare(r) : null;
  const CIcon = c?.dir === "up" ? TrendingUp : c?.dir === "down" ? TrendingDown : Minus;

  const prevCat = new Map((prev?.by_category ?? []).map((x) => [x.key, x.amount]));
  const cats = [...(r?.by_category ?? [])].sort((a, b) => b.amount - a.amount);
  const catMax = Math.max(1, ...cats.map((x) => x.amount));
  const monthVals = qs.map((q) => q.data?.spent ?? 0);
  const monthChart = useMemo(
    () => ({ title: "", kind: "column" as const, labels: months.map(p.monthLabel), series: [{ name: at.r.spent, values: monthVals }], unit: "vnd" as const }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [monthVals.join()],
  );

  return (
    <>
      <section className="hero15 hero22 pa__hero" aria-label={p.spentIn(monthNum(month))}>
        <p className="hero22__label">{p.spentIn(monthNum(month))}</p>
        <p className="hero15__num num">{r ? formatVnd(r.spent) : "…"}</p>
        {r && (
          <p className="pa__heroSub">
            {p.receipts(r.receipt_count)}
            {r.receipt_count > 0 && ` · ${p.avgBill(formatVnd(Math.round(r.spent / r.receipt_count / 1000) * 1000))}`}
          </p>
        )}
        {c && <p className="hero15__sub"><CIcon size={16} aria-hidden />{c.text}</p>}
      </section>

      <section className="box15 pa__card" aria-labelledby="pa-up">
        <h2 id="pa-up">{p.upcoming}</h2>
        <p className="pa__big num">{up.pending ? "…" : formatVnd(up.total)}</p>
        <ul className="pa__rows">
          <li><CalendarDays size={20} aria-hidden /><span>{p.recurring}</span><b className="num">{formatVnd(up.recTotal)}</b></li>
          <li><ShoppingCart size={20} aria-hidden /><span>{p.goods}</span><b className="num">{formatVnd(up.goodsTotal)}</b></li>
        </ul>
        <p className="pa__hint">{p.upcomingHow}</p>
        <Link to="/spending?tab=upcoming" className="pa__link">{p.seeUpcoming}<ChevronRight size={18} aria-hidden /></Link>
      </section>

      <section className="box15 pa__card" aria-labelledby="pa-months">
        <h2 id="pa-months">{p.months}</h2>
        <ChatChartView chart={monthChart} />
      </section>

      {cats.length > 0 && (
        <section className="box15 pa__card" aria-labelledby="pa-cat">
          <h2 id="pa-cat">{p.categories}</h2>
          <p className="pa__hint">{p.categoriesSub(monthNum(addMonths(month, -1)))}</p>
          <ul className="pa__bars">
            {cats.map((x) => {
              const before = prevCat.get(x.key);
              const d = before ? Math.round(((x.amount - before) / before) * 100) : null;
              return (
                <li key={x.key}>
                  <span className="pa__barName">{categoryLabel(x.key)}</span>
                  <b className="num">{formatVnd(x.amount)}</b>
                  <span className="pa__bar" aria-hidden><i style={{ width: `${(x.amount / catMax) * 100}%` }} /></span>
                  <span className={`pa__chg${d == null ? " pa__chg--new" : d > 0 ? " pa__chg--up" : d < 0 ? " pa__chg--down" : ""}`}>{d == null ? (prev ? p.newCat : "") : `${d > 0 ? "↑" : d < 0 ? "↓" : ""} ${Math.abs(d)}%`}</span>
                </li>
              );
            })}
          </ul>
        </section>
      )}

      <section className="box15 pa__card" aria-labelledby="pa-top">
        <h2 id="pa-top">{p.topItems}</h2>
        <p className="pa__hint">{p.topItemsSub}</p>
        {top.length ? (
          <ol className="pa__list pa__list--num">
            {top.map(([name, v]) => <li key={name}><span>{name}</span><b className="num">{formatVnd(Math.round(v))}</b></li>)}
          </ol>
        ) : <p className="pa__hint">{p.empty}</p>}
      </section>

      <section className="box15 pa__card" aria-labelledby="pa-bills">
        <h2 id="pa-bills">{p.bills}</h2>
        {bills.length ? (
          <ul className="pa__list">
            {bills.map((b) => <li key={b.id}><span>{b.store}<small>{at.cal.day(b.date)}</small></span><b className="num">{formatVnd(Math.round(b.total))}</b></li>)}
          </ul>
        ) : <p className="pa__hint">{p.noBills}</p>}
      </section>

      <section className="box15 pa__card" aria-labelledby="pa-soon">
        <h2 id="pa-soon">{p.runningOut}</h2>
        {soon.length ? (
          <ul className="pa__list">
            {soon.map((i) => (
              <li key={i.key}>
                <span>{i.name}<small>{i.category}</small></span>
                <span className={`pa__days${(i.daysLeft ?? 0) <= 0 ? " pa__days--out" : ""}`}>{p.daysLeft(i.daysLeft ?? 0)}</span>
              </li>
            ))}
          </ul>
        ) : <p className="pa__hint">{p.runningOutNone}</p>}
        <Link to="/items" className="pa__link">{p.seeItems}<ChevronRight size={18} aria-hidden /></Link>
      </section>

      <section className="box15 pa__card" aria-labelledby="pa-saved">
        <h2 id="pa-saved">{p.saved}</h2>
        <p className="pa__big pa__big--green num">{formatVnd(sv.saved_with_discounts)}</p>
        <p className="pa__hint">{p.savedSub}</p>
        {sv.could_save_more > 0 && <p className="pa__more">{p.couldSave(formatVnd(sv.could_save_more))}</p>}
        <button type="button" className="pa__link" onClick={onPrices}>{p.seePrices}<ChevronRight size={18} aria-hidden /></button>
      </section>
    </>
  );
}

// ---------------- So sánh giá ----------------

function Prices({ rows }: { rows: AnalyticsRows }) {
  const sv = savings(rows);
  const groups = useMemo(() => {
    const m = new Map<string, AnalyticsRows["prices"]>();
    for (const x of rows.prices) m.set(x.key, [...(m.get(x.key) ?? []), x]);
    return [...m.values()]
      .map((list) => [...list].sort((a, b) => a.onlinePrice - b.onlinePrice))
      .sort((a, b) => Math.max(...b.map((x) => x.saving)) - Math.max(...a.map((x) => x.saving)) || a[0].name.localeCompare(b[0].name));
  }, [rows.prices]);
  return (
    <>
      <section className="hero15 hero22 pa__hero" aria-label={p.pricesHead}>
        <p className="hero22__label">{p.pricesHead}</p>
        {groups.length ? (
          <>
            <p className="hero15__num num">{formatVnd(sv.could_save_more)}</p>
            <p className="pa__heroSub">{sv.could_save_more > 0 ? p.pricesSave(formatVnd(sv.could_save_more), sv.cheaper_elsewhere.length) : p.pricesSame}</p>
          </>
        ) : <p className="pa__heroSub">{p.noPrices}</p>}
      </section>
      <p className="pa__hint">{p.pricesHow}</p>
      {rows.prices.some((x) => x.listPrice) && <p className="pa__hint">{p.listNote}</p>}
      {groups.map((list) => {
        const first = list[0];
        return (
          <section key={first.key} className="box15 pa__card" aria-label={first.name}>
            <h2>{first.name}</h2>
            {first.ownPrice != null && <p className="pa__hint">{p.ours(formatVnd(first.ownPrice), first.ownStore)}</p>}
            <ul className="pa__list">
              {list.map((x) => (
                <li key={x.id} className={x.cheapest ? "pa__best" : undefined}>
                  <span>
                    {x.store}
                    <small>{[x.size, x.source, x.on ? at.cal.day(x.on).replace(/^\S+ /, "") : ""].filter(Boolean).join(" · ")}</small>
                  </span>
                  <span className="pa__price">
                    {x.listPrice ? (
                      <span className="pa__list-was">
                        <s className="num" aria-label={p.listWas(formatVnd(x.listPrice))}>{formatVnd(x.listPrice)}</s>
                        <span className="pa__off">{p.off(x.offPct)}</span>
                      </span>
                    ) : null}
                    <b className="num">{formatVnd(x.onlinePrice)}</b>
                    {x.cheapest ? <span className="tag15 pa__tag">{p.cheapest}</span> : null}
                    {x.saving > 0 && <small className="pa__save">{p.cheaper(formatVnd(x.saving))}</small>}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        );
      })}
    </>
  );
}

// ---------------- Hóa đơn ----------------

type Kind = "all" | "own" | "delegated" | "recurring";

function Receipts({ rows }: { rows: AnalyticsRows }) {
  const [month, setMonth] = useState(currentMonth());
  const [kind, setKind] = useState<Kind>("all");
  const h = useReceiptHistory();
  const isCurrent = month >= currentMonth();
  const inMonth = rows.receipts.filter((x) => x.month === month && (kind === "all" || x.kindKey === kind));
  const total = inMonth.reduce((a, x) => a + x.total, 0);
  const ids = new Set(inMonth.map((x) => x.id));
  const list = (h.data ?? []).filter((x) => ids.has(x.receipt_id)).sort((a, b) => b.purchased_at.localeCompare(a.purchased_at));
  const stores = useMemo(() => {
    const m = new Map<string, number>();
    for (const x of inMonth) if (x.kindKey !== "recurring") m.set(x.store, (m.get(x.store) ?? 0) + x.total);
    return [...m.entries()].sort((a, b) => b[1] - a[1]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, month, kind]);
  const storeMax = Math.max(1, ...stores.map((s) => s[1]));
  const [y, mm] = month.split("-").map(Number);

  return (
    <>
      <div className="month15" role="group" aria-label={at.cal.month(y, mm)}>
        <button type="button" className="icon-btn" aria-label={at.cal.prev} onClick={() => setMonth(addMonths(month, -1))}><ChevronLeft size={22} aria-hidden /></button>
        <span aria-live="polite">{at.cal.month(y, mm)}</span>
        <button type="button" className="icon-btn" aria-label={at.cal.next} aria-disabled={isCurrent || undefined} onClick={() => !isCurrent && setMonth(addMonths(month, 1))}><ChevronRight size={22} aria-hidden /></button>
      </div>
      <div className="pa__chips" role="group" aria-label={at.f.kind}>
        {(["all", "own", "delegated", "recurring"] as const).map((k) => (
          <button key={k} type="button" aria-pressed={kind === k} onClick={() => setKind(k)}>{k === "all" ? p.kindAll : at.kinds[k]}</button>
        ))}
      </div>

      <section className="box15 pa__card pa__cal" aria-label={at.cal.title}>
        <h2>{at.cal.title}</h2>
        <p className="pa__big num">{formatVnd(Math.round(total))}</p>
        <p className="pa__hint">{p.monthTotal(inMonth.length)}</p>
        <CalendarView fixedMonth={month} entries={inMonth.map((x) => ({ date: x.date, name: x.store, amount: x.total }))} />
      </section>

      {stores.length > 0 && (
        <section className="box15 pa__card" aria-labelledby="pa-stores">
          <h2 id="pa-stores">{p.byStore}</h2>
          <p className="pa__hint">{p.byStoreSub}</p>
          <ul className="pa__bars">
            {stores.map(([name, v]) => (
              <li key={name}>
                <span className="pa__barName">{name}</span>
                <b className="num">{formatVnd(Math.round(v))}</b>
                <span className="pa__bar" aria-hidden><i style={{ width: `${(v / storeMax) * 100}%` }} /></span>
              </li>
            ))}
          </ul>
        </section>
      )}

      <section aria-labelledby="pa-list">
        <h2 id="pa-list" className="h15sec">{p.list}</h2>
        {list.length ? <ul className="box15 ph15__list">{list.map((x) => <ReceiptRow key={x.receipt_id} r={x} />)}</ul> : <p className="pa__hint">{p.noReceipts}</p>}
      </section>
    </>
  );
}
