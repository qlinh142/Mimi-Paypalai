import { t } from "../i18n";
import { m1 } from "../i18n/m1";
import { m15 } from "../i18n/m15";
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ChevronLeft, ChevronRight, Trash2 } from "lucide-react";
import { ApiError } from "../api/client";
import { useRecurring, useRecurringActions } from "../api/hooks";
import type { Occurrence, Recurring, RecurringCycle } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { recurringIcon } from "../components/RecurringList";
import { formatDayMonth, formatVnd } from "../lib/text";
import { LoadState } from "../components/LoadState";

const CYCLES: RecurringCycle[] = ["monthly", "weekly", "quarterly", "yearly"];
const todayIso = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

/**
 * Chi tiết khoản chi định kỳ (góp ý 01/11 06:34, Phần E góp ý 4): màn riêng như chi tiết món ở Đồ dùng.
 * Sửa tên, số tiền, chu kỳ, hạn kế tiếp; thẻ kỳ này với Đánh dấu đã trả; lịch sử các kỳ; Tạm dừng, Xóa, Lưu.
 * `/items/recurring/new` mở màn trống để thêm khoản mới.
 */
export function RecurringDetail() {
  const { id = "" } = useParams();
  const list = useRecurring();
  const r = id === "new" ? undefined : list.data?.items.find((x) => x.id === id);
  if (id !== "new" && !r) {
    return (
      <MainFrame>
        <TopBar title={m1.recurring.title} variant="sub" />
        <div className="page p15">
          {list.data ? <p className="hint">{m15.history.gone}</p> : <LoadState q={list} hasData={false} />}
        </div>
      </MainFrame>
    );
  }
  return <Detail key={r?.id ?? "new"} r={r} />;
}

function Detail({ r }: { r?: Recurring }) {
  const nav = useNavigate();
  const act = useRecurringActions();
  const [name, setName] = useState(r?.name ?? "");
  const [amount, setAmount] = useState(r ? String(r.amount) : "");
  const [cycle, setCycle] = useState<RecurringCycle>(r?.cycle ?? "monthly");
  const [due, setDue] = useState(r?.next_due_at ?? todayIso());
  const [confirmDel, setConfirmDel] = useState(false);
  useEffect(() => setConfirmDel(false), [r?.id]);
  const value = Number(amount.replace(/[.,\s]/g, ""));
  const valid = name.trim() !== "" && Number.isInteger(value) && value > 0 && due !== "";
  const busy = act.create.isPending || act.update.isPending || act.pay.isPending || act.remove.isPending;
  const err = act.create.error ?? act.update.error ?? act.pay.error ?? act.remove.error;
  const back = () => nav("/items?tab=recurring");
  const Icon = recurringIcon(name || r?.name || "");

  const save = () => {
    if (!valid || busy) return;
    const patch = { name: name.trim(), amount: value, cycle, next_due_at: due };
    if (r) act.update.mutate({ id: r.id, patch }, { onSuccess: back });
    else act.create.mutate(patch, { onSuccess: back });
  };

  return (
    <MainFrame>
      <TopBar title={r?.name ?? m15.rec.add} />
      <div className="page p15 rd15">
        <div className="pd15__head rd15__head">
          <button type="button" className="pd15__back" aria-label={m15.detail.back} onClick={back}>
            <ChevronLeft size={28} aria-hidden />
          </button>
          <span className="pd15__icon" aria-hidden><Icon size={26} /></span>
          <h1 className="run15__title">{r?.name || m15.rec.add}</h1>
        </div>

        <section className="box15 rd15__form" aria-label={m15.recd.info}>
          <label className="field15">
            <span>{m1.recurring.name}</span>
            <input value={name} maxLength={60} onChange={(e) => setName(e.target.value)} />
          </label>
          <label className="field15">
            <span>{m1.recurring.amount}</span>
            <input inputMode="numeric" value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d]/g, ""))} />
          </label>
          <div className="rd15__row">
            <label className="field15">
              <span>{m1.recurring.cycle}</span>
              <select value={cycle} onChange={(e) => setCycle(e.target.value as RecurringCycle)}>
                {CYCLES.map((c) => <option key={c} value={c}>{m1.recurring.cycles[c]}</option>)}
              </select>
            </label>
            <label className="field15">
              <span>{m1.recurring.nextDue}</span>
              <input type="date" value={due} onChange={(e) => setDue(e.target.value)} />
            </label>
          </div>
          <p className="hint">{m15.recd.group}</p>
        </section>

        {r && <Current r={r} busy={busy} onPay={(occ) => act.pay.mutate(occ)} />}
        {r && <History items={r.history} />}

        {err && <p className="error" role="alert">{err instanceof ApiError ? err.message : t.common.genericError}</p>}
        <div className="rd15__actions">
          {r && (
            <button
              type="button"
              className="btn btn--outline"
              aria-disabled={busy || undefined}
              onClick={() => !busy && act.update.mutate({ id: r.id, patch: { active: !r.active } })}
            >
              {r.active ? m1.recurring.pause : m1.recurring.resume}
            </button>
          )}
          {r && (
            <button
              type="button"
              className={`btn pd15__del${confirmDel ? " pd15__del--confirm" : ""}`}
              aria-disabled={busy || undefined}
              onClick={() => {
                if (busy) return;
                if (!confirmDel) return setConfirmDel(true);
                act.remove.mutate(r.id, { onSuccess: back });
              }}
            >
              <Trash2 size={18} aria-hidden /> {confirmDel ? m15.plan.deleteConfirm : m15.plan.deleteShort}
            </button>
          )}
          <button type="button" className="btn btn--primary btn--lg rd15__save" aria-disabled={!valid || busy || undefined} onClick={save}>
            {m1.recurring.save}
          </button>
        </div>
        {confirmDel && <p className="hint" role="status">{m15.recd.deleteAsk}</p>}
      </div>
    </MainFrame>
  );
}

function Current({ r, busy, onPay }: { r: Recurring; busy: boolean; onPay: (occId: string) => void }) {
  if (!r.active) return <section className="box15"><p className="hint">{m15.recd.paused}</p></section>;
  const c = r.current;
  return (
    <section className="box15 rd15__now" aria-label={m15.recd.now}>
      <h2>{m15.recd.now}</h2>
      <p className="rd15__line"><span>{m15.recd.dueAt(formatDayMonth(r.next_due_at))}</span><span className="num">{formatVnd(r.amount)}</span></p>
      <p className="rd15__line"><span>{m15.recd.status}</span><span className={`tag15 ${c?.status === "pending" ? "tag15--buyer" : "tag15--bought"}`}>{m15.recd.statuses[c?.status ?? "pending"]}</span></p>
      {c?.status === "pending" && (
        <button type="button" className="btn btn--primary" aria-disabled={busy || undefined} onClick={() => !busy && onPay(c.id)}>{m1.recurring.markPaid}</button>
      )}
    </section>
  );
}

function History({ items }: { items: Occurrence[] }) {
  return (
    <section aria-labelledby="rd-hist">
      <h2 id="rd-hist" className="h15sec">{m15.recd.history}</h2>
      {items.length === 0 ? (
        <p className="hint">{m15.recd.noHistory}</p>
      ) : (
        <div className="box15"><ul className="ph15__list">
          {items.map((o) => {
            const viaBill = o.source_type === "receipt" && o.source_id;
            const body = (
              <>
                <span className="rows15__text">
                  <span>{formatDayMonth(o.paid_at ?? o.due_at)}</span>
                  <small>{m15.recd.statuses[o.status]}{viaBill ? ` · ${m15.recd.viaBill}` : ""}</small>
                </span>
                <span className="num">{o.status === "skipped" ? "–" : formatVnd(o.amount)}</span>
              </>
            );
            return (
              <li key={o.id}>
                {viaBill ? (
                  <Link className="ph15__row" to={`/spending/receipt/${encodeURIComponent(o.source_id!)}`}>{body}<ChevronRight size={18} aria-hidden /></Link>
                ) : (
                  <div className="ph15__row">{body}</div>
                )}
              </li>
            );
          })}
        </ul></div>
      )}
    </section>
  );
}
