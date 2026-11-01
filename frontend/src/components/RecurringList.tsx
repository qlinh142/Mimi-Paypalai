import { useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { CalendarClock, Droplet, GraduationCap, House, Pencil, Plus, Smartphone, Tv, Wifi, Zap, type LucideIcon } from "lucide-react";
import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { useRecurring } from "../api/hooks";
import { useKeysInPlans } from "./PlanPicker";
import type { Recurring } from "../api/types";
import { daysUntil } from "../lib/displayUnit";
import { formatDayMonth, formatVnd, normalizeVi } from "../lib/text";
import { CartButton } from "./CartButton";
import { Group15 } from "./Group15";

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

export function recurringIcon(name: string): LucideIcon {
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
  const nav = useNavigate();
  const inPlan = useKeysInPlans();

  const groups = useMemo(() => {
    const q = normalizeVi(term);
    const rows = (list.data?.items ?? []).filter((r) => !q || normalizeVi(r.name).includes(q));
    const byDue = (a: Recurring, b: Recurring) => a.next_due_at.localeCompare(b.next_due_at);
    return TIERS.map((tier) => ({ ...tier, rows: rows.filter((r) => recurringTier(r) === tier.key).sort(byDue) })).filter((g) => g.rows.length > 0);
  }, [list.data, term]);

  if (list.isPending) return <div className="skeleton" aria-busy="true" aria-label={t.common.loading} />;

  return (
    <>
      {list.data && groups.length === 0 && <div className="empty"><p>{term ? m15.items.noMatch(term) : m15.rec.empty}</p></div>}
      {groups.map((g) => (
        <Group15 key={g.key} id={`rec-${g.key}`} label={g.label} tone={g.key === "due" ? "now" : g.key === "upcoming" ? "soon" : "plenty"} count={g.rows.length}>
            {g.rows.map((r) => {
              const Icon = recurringIcon(r.name);
              return (
                <li key={r.id} className="card15 card15--wrap">
                  {/* Góp ý 01/11 06:34: bấm mở màn chi tiết riêng, như chi tiết món ở Đồ dùng. */}
                  <button type="button" className="card15__main" onClick={() => nav(`/items/recurring/${encodeURIComponent(r.id)}`)}>
                    <span className="card15__icon"><span className="row__icon" aria-hidden><Icon size={22} strokeWidth={1.8} /></span></span>
                    <span className="card15__text">
                      <span className="card15__name">{r.name}</span>
                      <span className="card15__sub">{formatVnd(r.amount)} · {dueText(r)}</span>
                      <span className="card15__edit"><Pencil size={14} aria-hidden /> {m15.items.edit}</span>
                    </span>
                  </button>
                  <CartButton planKey={recurringPlanKey(r)} name={r.name} added={inPlan.has(recurringPlanKey(r))} body={{ name: r.name }} />
                </li>
              );
            })}
        </Group15>
      ))}
      <button type="button" className="btn btn--primary btn--block btn--lg" onClick={() => nav("/items/recurring/new")}>
        <Plus size={22} aria-hidden /> {m15.rec.add}
      </button>
    </>
  );
}
