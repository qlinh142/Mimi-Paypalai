import { useMemo, useState } from "react";
import { CalendarClock, Droplet, GraduationCap, House, Pencil, Plus, Smartphone, Tv, Wifi, Zap, type LucideIcon } from "lucide-react";
import { t } from "../i18n";
import { m1 } from "../i18n/m1";
import { m15 } from "../i18n/m15";
import { ApiError } from "../api/client";
import { usePlan, useRecurring, useRecurringActions } from "../api/hooks";
import type { Recurring, RecurringCycle } from "../api/types";
import { daysUntil } from "../lib/displayUnit";
import { formatDayMonth, formatVnd, normalizeVi } from "../lib/text";
import { CartButton } from "./CartButton";

const CYCLES: RecurringCycle[] = ["monthly", "weekly", "quarterly", "yearly"];
type Tier = "due" | "upcoming" | "later";
const TIERS: { key: Tier; label: string }[] = [
  { key: "due", label: m15.rec.due },
  { key: "upcoming", label: m15.rec.upcoming },
  { key: "later", label: m15.rec.later },
];

/** Cùng ngưỡng với Hàng tiêu dùng (chốt 31/10): Đến hạn ≤ 3 ngày (kể cả quá hạn) · Sắp tới ≤ 7 ngày. */
export function recurringTier(r: Recurring): Tier {
  if (!r.active) return "later";
  const d = daysUntil(r.next_due_at);
  if (d <= 3) return "due";
  if (d <= 7) return "upcoming";
  return "later";
}

/** Khóa dòng Kế hoạch cho một khoản định kỳ (món tự nhập "custom-...", 3.1 mục 3A.4). */
export const recurringPlanKey = (r: Recurring) => `custom-rec-${r.id}`;

function iconFor(name: string): LucideIcon {
  const n = normalizeVi(name);
  if (/dien thoai|phone|mobile/.test(n)) return Smartphone;
  if (/dien|electric/.test(n)) return Zap;
  if (/internet|wifi|mang|fpt|viettel/.test(n)) return Wifi;
  if (/nuoc|water/.test(n)) return Droplet;
  if (/netflix|youtube|spotify|truyen hinh|tv/.test(n)) return Tv;
  if (/hoc|school|tuition/.test(n)) return GraduationCap;
  if (/nha|rent|chung cu/.test(n)) return House;
  return CalendarClock;
}

function dueText(r: Recurring): string {
  const d = daysUntil(r.next_due_at);
  if (d === 0) return m15.rec.today;
  if (d < 0) return m15.rec.overdue(formatDayMonth(r.next_due_at));
  return m15.rec.dueOn(formatDayMonth(r.next_due_at));
}

/** Tab Khoản chi định kỳ ở màn Đồ dùng (ảnh mẫu 3-khoan-chi-dinh-ky.png). */
export function RecurringList({ term }: { term: string }) {
  const list = useRecurring();
  const plan = usePlan();
  const [editing, setEditing] = useState<string | null>(null);
  const inPlan = useMemo(() => new Set((plan.data?.items ?? []).map((i) => i.key)), [plan.data]);

  const groups = useMemo(() => {
    const q = normalizeVi(term);
    const rows = (list.data?.items ?? []).filter((r) => !q || normalizeVi(r.name).includes(q));
    const byDue = (a: Recurring, b: Recurring) => a.next_due_at.localeCompare(b.next_due_at);
    return TIERS.map((tier) => ({ ...tier, rows: rows.filter((r) => recurringTier(r) === tier.key).sort(byDue) })).filter((g) => g.rows.length > 0);
  }, [list.data, term]);

  if (list.isPending) return <div className="skeleton" aria-busy="true" aria-label={t.common.loading} />;

  return (
    <>
      {list.data && groups.length === 0 && editing !== "new" && <div className="empty"><p>{term ? m15.items.noMatch(term) : m15.rec.empty}</p></div>}
      {groups.map((g) => (
        <section key={g.key} className="g15" aria-label={g.label}>
          <h2 className={`g15__head g15__head--${g.key === "due" ? "now" : g.key === "upcoming" ? "soon" : "plenty"}`}>{g.label}</h2>
          <ul className="g15__list">
            {g.rows.map((r) => {
              const Icon = iconFor(r.name);
              return (
                <li key={r.id} className="card15 card15--wrap">
                  <button type="button" className="card15__main" aria-expanded={editing === r.id} onClick={() => setEditing(editing === r.id ? null : r.id)}>
                    <span className="card15__icon"><span className="row__icon" aria-hidden><Icon size={22} strokeWidth={1.8} /></span></span>
                    <span className="card15__text">
                      <span className="card15__name">{r.name}</span>
                      <span className="card15__sub">{formatVnd(r.amount)} · {dueText(r)} · {m1.recurring.cycles[r.cycle]}</span>
                      <span className="card15__edit"><Pencil size={14} aria-hidden /> {m15.items.edit}</span>
                    </span>
                  </button>
                  <CartButton planKey={recurringPlanKey(r)} name={r.name} added={inPlan.has(recurringPlanKey(r))} body={{ name: r.name }} />
                  {editing === r.id && <RecurringEdit r={r} onDone={() => setEditing(null)} />}
                </li>
              );
            })}
          </ul>
        </section>
      ))}
      {editing === "new" ? (
        <RecurringEdit onDone={() => setEditing(null)} />
      ) : (
        <button type="button" className="btn btn--primary btn--block btn--lg" onClick={() => setEditing("new")}>
          <Plus size={22} aria-hidden /> {m15.rec.add}
        </button>
      )}
    </>
  );
}

function todayIso(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** Sửa (hoặc thêm) một khoản: tên, số tiền, chu kỳ, hạn kế tiếp; kèm Đã trả kỳ này và Xóa. */
function RecurringEdit({ r, onDone }: { r?: Recurring; onDone: () => void }) {
  const act = useRecurringActions();
  const [name, setName] = useState(r?.name ?? "");
  const [amount, setAmount] = useState(r ? String(r.amount) : "");
  const [cycle, setCycle] = useState<RecurringCycle>(r?.cycle ?? "monthly");
  const [due, setDue] = useState(r?.next_due_at ?? todayIso());
  const value = Number(amount.replace(/[.,\s]/g, ""));
  const valid = name.trim() !== "" && Number.isInteger(value) && value > 0 && due !== "";
  const busy = act.create.isPending || act.update.isPending || act.pay.isPending || act.remove.isPending;
  const err = act.create.error ?? act.update.error ?? act.pay.error ?? act.remove.error;
  return (
    <form
      className="plan-form edit15"
      onSubmit={(e) => {
        e.preventDefault();
        if (!valid || busy) return;
        const patch = { name: name.trim(), amount: value, cycle, next_due_at: due };
        if (r) act.update.mutate({ id: r.id, patch }, { onSuccess: onDone });
        else act.create.mutate(patch, { onSuccess: onDone });
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
      {err && <p className="error" role="alert">{err instanceof ApiError ? err.message : t.common.genericError}</p>}
      <div className="plan-form__row">
        <button type="submit" className="btn btn--primary" aria-disabled={!valid || busy || undefined}>{m1.recurring.save}</button>
        <button type="button" className="btn btn--quiet" onClick={onDone}>{m1.recurring.cancel}</button>
      </div>
      {r && (
        <div className="plan-form__row">
          {r.current && r.active && (
            <button type="button" className="btn btn--quiet" aria-disabled={busy || undefined} onClick={() => !busy && act.pay.mutate(r.current!.id, { onSuccess: onDone })}>
              {m15.rec.paid}
            </button>
          )}
          <button type="button" className="btn btn--quiet btn--danger" aria-disabled={busy || undefined} onClick={() => !busy && act.remove.mutate(r.id, { onSuccess: onDone })}>
            {m15.rec.remove}
          </button>
        </div>
      )}
    </form>
  );
}
