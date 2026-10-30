import { useState } from "react";
import { Trash2 } from "lucide-react";
import { t } from "../i18n";
import { m1 } from "../i18n/m1";
import { ApiError } from "../api/client";
import { useForecast, useRecurring, useRecurringActions } from "../api/hooks";
import type { Occurrence, Recurring, RecurringCycle } from "../api/types";
import { formatDayMonth, formatVnd } from "../lib/text";

const CYCLES: RecurringCycle[] = ["monthly", "weekly", "quarterly", "yearly"];

/**
 * Dự báo 30 ngày (2.1 mục 7): kỳ định kỳ chưa trả + món sắp hết theo giá đã trả.
 * Khoản chưa trả chỉ nằm ở đây; bấm "Đã trả" mới cộng vào Đã chi.
 */
export function ForecastCard() {
  const f = useForecast();
  if (!f.data) return null;
  const d = f.data;
  return (
    <section className="plan-sec" aria-labelledby="forecast-title">
      <h2 id="forecast-title">{m1.forecast.title}</h2>
      <p className="plan-total">
        <span>{m1.forecast.total}</span>
        <span className="num">{formatVnd(d.total)}</span>
      </p>
      <dl className="facts">
        <dt>{m1.forecast.recurring}</dt>
        <dd className="num">{formatVnd(d.recurring_total)}</dd>
        <dt>{m1.forecast.shopping}</dt>
        <dd className="num">{formatVnd(d.shopping_total)}</dd>
      </dl>
      {d.missing_prices > 0 && <p className="hint">{m1.forecast.missing(d.missing_prices)}</p>}
      <p className="hint">{m1.forecast.note}</p>
    </section>
  );
}

function occurrenceText(o: Occurrence): string {
  switch (o.status) {
    case "paid":
      return m1.recurring.paid(formatDayMonth(o.paid_at ?? o.due_at));
    case "matched":
      return m1.recurring.paidViaReceipt;
    case "skipped":
      return m1.recurring.skipped;
  }
  return m1.recurring.due(formatDayMonth(o.due_at));
}

/** Khoản định kỳ (2.1 mục 9): trạng thái từng khoản, Đã trả / Bỏ qua kỳ này, thêm / sửa / xóa. */
export function RecurringPanel() {
  const list = useRecurring();
  const act = useRecurringActions();
  const [adding, setAdding] = useState(false);
  const items = list.data?.items ?? [];
  return (
    <section className="plan-sec" aria-labelledby="recurring-title">
      <h2 id="recurring-title">{m1.recurring.title}</h2>
      {list.data && list.data.monthly_total > 0 && <p className="hint">{m1.recurring.monthly(formatVnd(list.data.monthly_total))}</p>}
      {list.data && items.length === 0 && !adding && <p className="hint">{m1.recurring.empty}</p>}
      <ul className="list">
        {items.map((r) => (
          <RecurringRow key={r.id} r={r} act={act} />
        ))}
      </ul>
      {adding ? (
        <RecurringForm onDone={() => setAdding(false)} />
      ) : (
        <button type="button" className="btn btn--quiet btn--block" onClick={() => setAdding(true)}>
          {m1.recurring.add}
        </button>
      )}
    </section>
  );
}

function RecurringRow({ r, act }: { r: Recurring; act: ReturnType<typeof useRecurringActions> }) {
  const cur = r.current;
  const busy = act.pay.isPending || act.skip.isPending;
  const last = r.history[0];
  return (
    <li className="plan-row" style={{ flexWrap: "wrap" }}>
      <span className="plan-row__text">
        <span className="plan-row__name">{r.name}</span>
        <span className="plan-row__sub">
          {formatVnd(r.amount)} · {m1.recurring.cycles[r.cycle]} · {m1.recurring.display[r.display]}
        </span>
        <span className="plan-row__sub">
          {cur ? occurrenceText(cur) : m1.recurring.due(formatDayMonth(r.next_due_at))}
          {last && ` · ${occurrenceText(last)}`}
        </span>
      </span>
      <button type="button" className="icon-btn" aria-label={m1.recurring.remove(r.name)} onClick={() => act.remove.mutate(r.id)}>
        <Trash2 size={18} aria-hidden />
      </button>
      <div className="actions" style={{ flexBasis: "100%", display: "flex", gap: "var(--s2)" }}>
        {cur && r.active && (
          <>
            <button type="button" className="btn btn--primary" aria-disabled={busy || undefined} onClick={() => !busy && act.pay.mutate(cur.id)}>
              {m1.recurring.markPaid}
            </button>
            <button type="button" className="btn btn--quiet" aria-disabled={busy || undefined} onClick={() => !busy && act.skip.mutate(cur.id)}>
              {m1.recurring.skip}
            </button>
          </>
        )}
        <button
          type="button"
          className="btn btn--quiet"
          onClick={() => act.update.mutate({ id: r.id, patch: { active: !r.active } })}
        >
          {r.active ? m1.recurring.pause : m1.recurring.resume}
        </button>
      </div>
    </li>
  );
}

function todayIso(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function RecurringForm({ onDone }: { onDone: () => void }) {
  const { create } = useRecurringActions();
  const [name, setName] = useState("");
  const [amount, setAmount] = useState("");
  const [cycle, setCycle] = useState<RecurringCycle>("monthly");
  const [due, setDue] = useState(todayIso());
  const value = Number(amount.replace(/[.,\s]/g, ""));
  const valid = name.trim() !== "" && Number.isInteger(value) && value > 0 && due !== "";
  return (
    <form
      className="plan-form"
      onSubmit={(e) => {
        e.preventDefault();
        if (!valid || create.isPending) return;
        create.mutate({ name: name.trim(), amount: value, cycle, next_due_at: due }, { onSuccess: onDone });
      }}
    >
      <label>
        {m1.recurring.name}
        <input value={name} maxLength={60} onChange={(e) => setName(e.target.value)} />
      </label>
      <div className="plan-form__row">
        <label>
          {m1.recurring.amount}
          <input inputMode="numeric" value={amount} onChange={(e) => setAmount(e.target.value)} />
        </label>
        <label>
          {m1.recurring.cycle}
          <select value={cycle} onChange={(e) => setCycle(e.target.value as RecurringCycle)}>
            {CYCLES.map((c) => (
              <option key={c} value={c}>{m1.recurring.cycles[c]}</option>
            ))}
          </select>
        </label>
      </div>
      <label>
        {m1.recurring.nextDue}
        <input type="date" value={due} onChange={(e) => setDue(e.target.value)} />
      </label>
      {create.isError && <p className="error" role="alert">{create.error instanceof ApiError ? create.error.message : t.common.genericError}</p>}
      <div className="plan-form__row">
        <button type="submit" className="btn btn--primary" aria-disabled={!valid || create.isPending || undefined}>{m1.recurring.save}</button>
        <button type="button" className="btn btn--quiet" onClick={onDone}>{m1.recurring.cancel}</button>
      </div>
    </form>
  );
}
