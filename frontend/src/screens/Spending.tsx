import { categoryLabel, t } from "../i18n";
import { useState } from "react";
import { Link } from "react-router-dom";
import { ChevronLeft, ChevronRight, ScanLine } from "lucide-react";
import { ApiError } from "../api/client";
import { useSpending } from "../api/hooks";
import type { ReceiptSummary, SpendingReport } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { ForecastCard, RecurringPanel } from "../components/Recurring";
import { addMonths, currentMonth, formatDayMonth, formatMonth, formatVnd } from "../lib/text";

/**
 * Sổ chi tiêu (spec mục 3). Mọi con số do API tính từ hóa đơn đã lưu; màn này chỉ hiển thị.
 * Tổng tháng là con số lớn (không cần biểu đồ); chi theo nhóm là thanh ngang một màu, mỗi thanh có
 * nhãn và số tiền nên đọc được cả khi không phân biệt màu.
 */
export function Spending() {
  const [month, setMonth] = useState(currentMonth());
  const q = useSpending(month);
  const r = q.data;
  const isCurrent = month >= currentMonth();

  if (q.error instanceof ApiError && q.error.status === 403) {
    return (
      <MainFrame>
        <TopBar title={t.spending.title} />
        <div className="page empty"><p>{t.spending.ownerOnly}</p></div>
      </MainFrame>
    );
  }

  return (
    <MainFrame>
      <TopBar title={t.spending.title} />
      <div className="page spend">
        <div className="month-switch" role="group" aria-label={t.spending.title}>
          <button type="button" className="icon-btn" aria-label={t.spending.prevMonth} onClick={() => setMonth(addMonths(month, -1))}>
            <ChevronLeft size={22} aria-hidden />
          </button>
          <span className="month-switch__label" aria-live="polite">{formatMonth(month)}</span>
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
        {r && (r.receipt_count === 0 && !r.online_orders ? <EmptyMonth month={month} /> : <Report r={r} month={month} />)}
        {/* 2.1: Dự báo 30 ngày và khoản định kỳ luôn theo hôm nay, không theo tháng đang xem. */}
        {isCurrent && <ForecastCard />}
        <RecurringPanel />
      </div>
    </MainFrame>
  );
}

function EmptyMonth({ month }: { month: string }) {
  return (
    <div className="empty">
      <h2>{t.spending.emptyTitle(formatMonth(month))}</h2>
      <p>{t.spending.emptyBody}</p>
      <Link to="/scan" className="btn btn--primary">
        <ScanLine size={20} aria-hidden /> {t.spending.scan}
      </Link>
    </div>
  );
}

function Report({ r, month }: { r: SpendingReport; month: string }) {
  const prev = formatMonth(addMonths(month, -1));
  const maxCat = Math.max(1, ...r.by_category.map((c) => c.amount));
  return (
    <>
      <section className="spend-hero" aria-label={t.spending.spentIn(formatMonth(month))}>
        <p className="spend-hero__label">{t.spending.spentIn(formatMonth(month))}</p>
        <p className="spend-hero__num num">{formatVnd(r.spent)}</p>
        <p className="spend-hero__sub">
          {r.spent_prev_month_pct != null ? t.spending.vsPrev(Math.round(r.spent_prev_month_pct), prev) : t.spending.noPrev(prev)}
          {" · "}
          {t.spending.receipts(r.receipt_count)}
        </p>
        {(r.online_spent ?? 0) > 0 && (
          <p className="spend-hero__sub">
            <Link to="/orders">{t.spending.online(formatVnd(r.online_spent ?? 0), r.online_orders ?? 0)}</Link>
            {(r.refunded ?? 0) > 0 && ` · ${t.spending.refunded(formatVnd(r.refunded ?? 0))}`}
          </p>
        )}
        {r.saved > 0 && (
          <p className="spend-hero__saved">
            {t.spending.saved(formatVnd(r.saved))}
            {r.saved_ratio != null && ` · ${t.spending.savedRatio(Math.round(r.saved_ratio * 100))}`}
          </p>
        )}
      </section>

      {r.by_category.length > 0 && (
        <section className="plan-sec" aria-labelledby="spend-cat">
          <h2 id="spend-cat">{t.spending.byCategory}</h2>
          <ul className="bars">
            {r.by_category.map((c) => (
              <li key={c.key} className="bar">
                <span className="bar__label">{categoryLabel(c.key)}</span>
                <span className="bar__value num">
                  {formatVnd(c.amount)} <span className="bar__pct">{Math.round(c.pct)}%</span>
                </span>
                {/* Độ dài thanh so với nhóm lớn nhất; số tiền luôn ghi bên cạnh. */}
                <span className="bar__track" aria-hidden>
                  <span className="bar__fill" style={{ width: `${Math.max(2, (c.amount / maxCat) * 100)}%` }} />
                </span>
              </li>
            ))}
          </ul>
          <p className="hint">{t.spending.tableNote}</p>
        </section>
      )}

      {r.top_saving_receipts.length > 0 && (
        <section className="plan-sec" aria-labelledby="spend-save">
          <h2 id="spend-save">{t.spending.topSaving}</h2>
          <ReceiptList rows={r.top_saving_receipts} showDiscount />
        </section>
      )}

      {r.recent_receipts.length > 0 && (
        <section className="plan-sec" aria-labelledby="spend-recent">
          <h2 id="spend-recent">{t.spending.recent}</h2>
          <ReceiptList rows={r.recent_receipts} />
        </section>
      )}
    </>
  );
}

function ReceiptList({ rows, showDiscount = false }: { rows: ReceiptSummary[]; showDiscount?: boolean }) {
  return (
    <ul className="lines">
      {rows.map((x) => (
        <li key={x.receipt_id}>
          <span>{x.store || t.confirm.unknown}</span>
          <span className="num">{formatVnd(x.total_vnd)}</span>
          <span className="sub">
            {formatDayMonth(x.purchased_on)}
            {showDiscount && x.discount_vnd > 0 && ` · ${t.spending.discount(formatVnd(x.discount_vnd))}`}
          </span>
        </li>
      ))}
    </ul>
  );
}
