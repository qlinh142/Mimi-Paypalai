import { m15 } from "../i18n/m15";
import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { CalendarDays, ChevronRight, Info, MapPin, Plus, UsersRound } from "lucide-react";
import { useDelegated, usePlanCarts, usePlanMeta, usePlans } from "../api/hooks";
import type { Cart, PlanMeta } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { setCurrentPlanId } from "../lib/currentPlan";
import { iconOf, planIcon } from "../lib/planIcon";
import { formatDayMonth, formatVnd } from "../lib/text";
import { t } from "../i18n";

const pad = (n: number) => String(n).padStart(2, "0");

export function planName(p: Pick<PlanMeta, "default" | "name">) {
  return p.default ? m15.plan.main : p.name;
}

export function whenText(st: Cart["state"] | undefined): string {
  if (!st?.buy_date) return m15.plan.noDate;
  const time = st.buy_hour != null ? ` · ${pad(st.buy_hour)}:${pad(st.buy_minute ?? 0)}` : "";
  return formatDayMonth(st.buy_date) + time;
}

/**
 * Kế hoạch mua sắm (Mimi 1.5, ảnh mẫu 5a): danh sách các Kế hoạch dạng thẻ, hai nút
 * Kế hoạch của tôi / Được nhờ mua, nút + Kế hoạch mới. Bấm thẻ để mở chi tiết.
 */
export function PlanList() {
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") === "to-me" ? "to-me" : "mine";
  return (
    <MainFrame>
      <TopBar title={m15.plan.title} showTitle />
      <div className="page p15">
        <div className="seg seg15" role="group" aria-label={m15.plan.title}>
          <button type="button" aria-pressed={tab === "mine"} onClick={() => setParams({}, { replace: true })}>{m15.plan.mine}</button>
          <button type="button" aria-pressed={tab === "to-me"} onClick={() => setParams({ tab: "to-me" }, { replace: true })}>{m15.plan.toMe}</button>
        </div>
        {tab === "mine" ? <Mine /> : <ToMe />}
      </div>
    </MainFrame>
  );
}

function Mine() {
  const plans = usePlans();
  const meta = usePlanMeta();
  const nav = useNavigate();
  const delegated = useDelegated();
  const list = (plans.data ?? []).filter((p) => p.status === "active");
  const carts = usePlanCarts(list.map((p) => p.id));
  const [err, setErr] = useState(false);

  const open = (id: string) => {
    setCurrentPlanId(id);
    nav(`/plan/${encodeURIComponent(id)}`);
  };

  if (plans.isPending) return <p className="hint">{t.common.loading}</p>;

  return (
    <>
      {list.length === 0 && <p className="hint">{m15.plan.mineEmpty}</p>}
      <ul className="plans15">
        {list.map((p, i) => {
          const c = carts[i]?.data;
          const Icon = iconOf(planIcon(p.id));
          const buyer = (delegated.data?.from_me ?? []).find(
            (a) => a.plan_id === p.id && (a.status === "assigned" || a.status === "shopping"),
          )?.buyer_display_name;
          const count = c?.item_count ?? p.item_count;
          return (
            <li key={p.id}>
              <button type="button" className="plancard15" onClick={() => open(p.id)} aria-label={m15.plan.open(planName(p))}>
                <span className="plancard15__icon"><Icon size={26} aria-hidden /></span>
                <span className="plancard15__body">
                  <span className="plancard15__name">{planName(p)}</span>
                  <span className="plancard15__sub">
                    {m15.plan.count(count)}
                    {c?.estimate != null && ` · ${m15.plan.budget(formatVnd(c.estimate))}`}
                  </span>
                  <span className="plancard15__sub"><MapPin size={15} aria-hidden /> {c?.state.trip_note || m15.plan.noStore}</span>
                  <span className="plancard15__sub"><CalendarDays size={15} aria-hidden /> {whenText(c?.state)}</span>
                  <span className="plancard15__tags">
                    <span className="tag15">{m15.plan.phase[c?.phase ?? "planning"]}</span>
                    {buyer && <span className="tag15 tag15--buyer">{m15.plan.buyer(buyer)}</span>}
                  </span>
                </span>
                <ChevronRight size={22} aria-hidden className="plancard15__chev" />
              </button>
            </li>
          );
        })}
      </ul>
      {err && <p className="error" role="alert">{t.common.genericError}</p>}
      <button
        type="button"
        className="btn btn--outline btn--block btn--lg"
        aria-disabled={meta.create.isPending || undefined}
        onClick={() => {
          if (meta.create.isPending) return;
          const d = new Date();
          const name = m15.plan.newName(`${pad(d.getDate())}/${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`);
          meta.create.mutate(name, { onSuccess: (p) => open(p.id), onError: () => setErr(true) });
        }}
      >
        <Plus size={22} aria-hidden /> {m15.plan.newPlan}
      </button>
      <p className="hint hint15"><Info size={16} aria-hidden /> {m15.plan.note}</p>
    </>
  );
}

/** Được nhờ mua: tạm hiện thẻ (chốt 31/10); màn đi mua chi tiết làm sau. */
function ToMe() {
  const d = useDelegated();
  const list = d.data?.to_me ?? [];
  if (d.isPending) return <p className="hint">{t.common.loading}</p>;
  if (list.length === 0) return <p className="hint">{m15.plan.toMeEmpty}</p>;
  return (
    <ul className="plans15">
      {list.map((a) => {
        const n = a.items.filter((i) => i.status === "assigned").length;
        const st = a.trip ? { buy_date: a.trip.buy_date ?? "", buy_hour: a.trip.buy_hour ?? null, buy_minute: a.trip.buy_minute ?? 0, trip_note: a.trip.trip_note ?? "" } : undefined;
        return (
          <li key={a.assignment_id}>
            <div className="plancard15">
              <span className="plancard15__icon"><UsersRound size={26} aria-hidden /></span>
              <span className="plancard15__body">
                <span className="plancard15__name">{a.trip?.plan_name && a.trip.plan_name !== "Kế hoạch chính" ? a.trip.plan_name : m15.plan.main}</span>
                <span className="plancard15__sub">{m15.plan.from(a.owner_display_name)} · {m15.plan.count(n)}</span>
                <span className="plancard15__sub"><MapPin size={15} aria-hidden /> {st?.trip_note || m15.plan.noStore}</span>
                <span className="plancard15__sub"><CalendarDays size={15} aria-hidden /> {whenText(st)}</span>
                <span className="plancard15__sub">{a.items.map((i) => i.name_display).join(", ")}</span>
              </span>
            </div>
          </li>
        );
      })}
    </ul>
  );
}
