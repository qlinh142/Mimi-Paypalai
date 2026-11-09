import { s22 } from "../i18n/s22";
import { m15 } from "../i18n/m15";
import { useLayoutEffect, useState } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { CalendarDays, Check, ChevronRight, Info, MapPin, Plus, UsersRound } from "lucide-react";
import { useDelegated, usePlan, usePlanCarts, usePlanMeta, usePlans } from "../api/hooks";
import type { Assignment, Cart, PlanMeta } from "../api/types";
import { OrderHistoryList } from "./PurchaseHistory";
import { MainFrame, TopBar } from "../components/AppShell";
import { setCurrentPlanId, useCurrentPlanId } from "../lib/currentPlan";
import { visiblePlans } from "../components/PlanPicker";
import { iconOf, planIcon } from "../lib/planIcon";
import { buyerOf, draftName, isDraft, usePlanDraftVersion } from "../lib/planDraft";
import { Stepper } from "../components/PlanDraft22";
import { NextButton, Step1Body } from "./PlanFlow22";
import { p22 } from "../i18n/p22";
import { formatDayMonth, formatVnd } from "../lib/text";
import { t } from "../i18n";
import { LoadState } from "../components/LoadState";

const pad = (n: number) => String(n).padStart(2, "0");

export function planName(p: Pick<PlanMeta, "default" | "name">) {
  return p.default ? draftName("default") || m15.plan.main : p.name;
}

export function whenText(st: Cart["state"] | undefined): string {
  if (!st?.buy_date) return m15.plan.noDate;
  const time = st.buy_hour != null ? ` · ${pad(st.buy_hour)}:${pad(st.buy_minute ?? 0)}` : "";
  return formatDayMonth(st.buy_date) + time;
}

/**
 * Kế hoạch mua sắm. Mimi 2.2 (đề xuất chốt 09/11): ba tab Của tôi, Được nhờ mua, Tất cả đơn (T5).
 * Tab Được nhờ mua giữ như 2.1 (anh chốt 09/11): danh sách kế hoạch được nhờ, nhận hoặc từ chối trong màn đi mua.
 * Tab Của tôi: trên cùng là kế hoạch chưa lưu, hiện sẵn đầy đủ như Redesign (T1); bên dưới là kế hoạch
 * đã lưu hoặc đang triển khai dạng thẻ gọn (bấm mới xem chi tiết). Kế hoạch đã mua xong chỉ nằm ở
 * tab Tất cả đơn (anh chốt 09/11: Của tôi chỉ có kế hoạch chưa mua).
 */
export function PlanList() {
  const [params, setParams] = useSearchParams();
  const raw = params.get("tab");
  // "orders" là tên cũ của tab Lịch sử đơn hàng (2.2 trước 09/11), giữ để link cũ vẫn mở đúng.
  const tab = raw === "to-me" ? "to-me" : raw === "history" || raw === "orders" ? "history" : "mine";
  const toMe = (useDelegated().data?.to_me ?? []).filter((a) => a.status === "assigned").length;
  return (
    <MainFrame>
      <TopBar title={m15.plan.title} sub={s22.heads.plans} showTitle />
      <div className="page p15">
        <div className="seg seg15 seg22--3" role="group" aria-label={m15.plan.title}>
          <button type="button" aria-pressed={tab === "mine"} onClick={() => setParams({}, { replace: true })}>{p22.tabs.mine}</button>
          <button type="button" aria-pressed={tab === "to-me"} onClick={() => setParams({ tab: "to-me" }, { replace: true })}>
            {m15.plan.toMe}
            {toMe > 0 && <span className="badge15" aria-label={m15.run.newBadge(toMe)}>{toMe}</span>}
          </button>
          <button type="button" aria-pressed={tab === "history"} onClick={() => setParams({ tab: "history" }, { replace: true })}>{p22.tabs.orders}</button>
        </div>
        {tab === "mine" ? <Mine /> : tab === "to-me" ? <ToMe /> : <div className="ph15"><OrderHistoryList /></div>}
      </div>
    </MainFrame>
  );
}

/** Kế hoạch chưa lưu: đang sửa dở (nháp), giỏ trống, hoặc chưa từng chọn người mua và chưa gửi ai. */
function isUnsaved(p: PlanMeta, c: Cart | undefined, sent: boolean): boolean {
  if (isDraft(p.id)) return true;
  if (sent || (c?.phase ?? "planning") !== "planning") return false;
  return (c?.item_count ?? p.item_count) === 0 || !buyerOf(p.id);
}

function Mine() {
  const plans = usePlans();
  const meta = usePlanMeta();
  const nav = useNavigate();
  const delegated = useDelegated();
  const current = useCurrentPlanId();
  // Của tôi (anh chốt 09/11): Kế hoạch đang lên, Đã lưu và đang triển khai, Đã hoàn thành.
  const mine = (plans.data ?? []).filter((p) => p.status === "active" || p.status === "archived");
  const carts = usePlanCarts(mine.map((p) => p.id));
  const [err, setErr] = useState(false);
  usePlanDraftVersion();
  const saved = (useLocation().state as { saved?: unknown } | null)?.saved;
  const all = mine.map((p, i) => ({ p, c: carts[i]?.data }));
  const rows = all.filter((r) => r.p.status === "active");
  const sentOf = (id: string) => (delegated.data?.from_me ?? []).find((a) => a.plan_id === id && (a.status === "assigned" || a.status === "shopping"));
  const unsaved = rows.filter(({ p, c }) => isUnsaved(p, c, !!sentOf(p.id)));
  const top = unsaved.find((r) => r.p.id === current) ?? unsaved.find((r) => r.p.default) ?? unsaved[0];
  // Các hook của thẻ đầu đọc "Kế hoạch đang xem": chuyển sang kế hoạch chưa lưu trước khi vẽ thẻ.
  useLayoutEffect(() => {
    if (top && current !== top.p.id) setCurrentPlanId(top.p.id);
  }, [top?.p.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const open = (id: string) => {
    setCurrentPlanId(id);
    nav(`/plan/${encodeURIComponent(id)}`);
  };
  const createNew = () => {
    if (meta.create.isPending) return;
    const d = new Date();
    const name = m15.plan.newName(`${pad(d.getDate())}/${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`);
    meta.create.mutate(name, { onSuccess: (p) => setCurrentPlanId(p.id), onError: () => setErr(true) });
  };
  if (!plans.data) return <LoadState q={plans} hasData={false} />;
  const visible = new Set(visiblePlans(plans.data).map((p) => p.id));
  const rest = rows.filter((r) => r !== top && visible.has(r.p.id));
  const going = rest.filter((r) => r.c?.phase !== "reconciled");
  // Hoàn thành khi quét hóa đơn khớp (reconciled) hoặc người dùng tự xác nhận (archived).
  const doneDay = (r: (typeof all)[number]) => (r.c?.state.reconciled_on || r.c?.state.buy_date || "").slice(0, 10);
  const done = all
    .filter((r) => r !== top && (r.p.status === "archived" || (visible.has(r.p.id) && r.c?.phase === "reconciled")))
    .sort((x, y) => doneDay(y).localeCompare(doneDay(x)));
  return (
    <>
      {typeof saved === "string" && <p className="sent15" role="status">{saved}</p>}
      <div className="sec22">
        <h2 className="h15sec">{p22.current}</h2>
        <button type="button" className="pill22 sec22__new" aria-disabled={meta.create.isPending || undefined} onClick={createNew}>
          <Plus size={18} aria-hidden /> {p22.newPlan}
        </button>
      </div>
      {top && current === top.p.id ? (
        <>
          <TopDraft id={top.p.id} />
        </>
      ) : (
        !top && <p className="hint">{m15.plan.mineEmpty}</p>
      )}
      {err && <p className="error" role="alert">{t.common.genericError}</p>}

      <h2 className="h15sec">{p22.saved}</h2>
      {going.length === 0 && <p className="hint">{p22.noSaved}</p>}
      <ul className="plans15">
        {going.map(({ p, c }) => {
          const Icon = iconOf(planIcon(p));
          const buyer = sentOf(p.id)?.buyer_display_name;
          const count = c?.item_count ?? p.item_count;
          // Câu 5: người được nhờ đã từ chối mà chưa gửi cho ai khác.
          const declined = !buyer && (delegated.data?.from_me ?? []).some((x) => x.plan_id === p.id && x.status === "declined");
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
                  <span className="plancard15__sub"><MapPin size={15} aria-hidden /> {c?.state.trip_note || m15.plan.noStore} · <CalendarDays size={15} aria-hidden /> {whenText(c?.state)}</span>
                  <span className="plancard15__tags">
                    {isDraft(p.id) ? (
                      <span className="tag15 tag15--draft">{m15.plan.draft}</span>
                    ) : (
                      <span className="tag15">{m15.plan.phase[buyer ? "shopping" : (c?.phase ?? "planning")]}</span>
                    )}
                    {buyer && <span className="tag15 tag15--buyer">{m15.plan.buyer(buyer)}</span>}
                    {declined && <span className="tag15 tag15--draft">{m15.plan.noBuyer}</span>}
                  </span>
                </span>
                <ChevronRight size={22} aria-hidden className="plancard15__chev" />
              </button>
            </li>
          );
        })}
      </ul>

      <h2 className="h15sec">{p22.doneHead}</h2>
      {done.length === 0 ? (
        <p className="hint">{p22.noDone}</p>
      ) : (
        <ul className="box15 past22">
          {done.map((r) => (
            <li key={r.p.id}>
              <button type="button" className="past22__row" onClick={() => open(r.p.id)}>
                <span className="past22__ok" aria-hidden><Check size={20} /></span>
                <span className="past22__text">
                  <b>{planName(r.p)}</b>
                  <small>{[doneDay(r) ? formatDayMonth(doneDay(r)) : "", m15.plan.count(r.c?.item_count ?? r.p.item_count)].filter(Boolean).join(" · ")}</small>
                </span>
                <span className="past22__end">
                  {r.c?.estimate != null && <span className="num past22__sum">{formatVnd(r.c.estimate)}</span>}
                  <span className="tag15">{p22.completed}</span>
                </span>
                <ChevronRight size={20} aria-hidden />
              </button>
            </li>
          ))}
        </ul>
      )}

      <p className="hint hint15"><Info size={16} aria-hidden /> {m15.plan.note}</p>
    </>
  );
}

/**
 * Được nhờ mua: mọi kế hoạch người nhà gửi cho mình đi mua (anh chốt 09/11), gồm Đang chuẩn bị mua và
 * Đã hoàn thành. Bấm thẻ mở màn đi mua (Spec 1.5 mục 2); kế hoạch đã xong mở bản chỉ xem.
 */
function ToMe() {
  const d = useDelegated();
  const open = d.data?.to_me ?? [];
  const done = [...(d.data?.to_me_done ?? [])].sort((x, y) => (y.updated_at ?? y.created_at).localeCompare(x.updated_at ?? x.created_at));
  if (d.isPending) return <p className="hint">{t.common.loading}</p>;
  if (open.length === 0 && done.length === 0) return <p className="hint">{m15.plan.toMeEmpty}</p>;
  return (
    <>
      <h2 className="h15sec">{p22.askOpen}</h2>
      {open.length === 0 ? <p className="hint">{m15.plan.toMeEmpty}</p> : <ul className="plans15">{open.map((a) => <AskCard key={a.assignment_id} a={a} />)}</ul>}
      {done.length > 0 && (
        <>
          <h2 className="h15sec">{p22.askDone}</h2>
          <ul className="plans15">{done.map((a) => <AskCard key={a.assignment_id} a={a} />)}</ul>
        </>
      )}
    </>
  );
}

function AskCard({ a }: { a: Assignment }) {
  const nav = useNavigate();
  const finished = a.status === "completed" || a.status === "closed_partial";
  const n = finished ? a.items.length : a.items.filter((i) => i.status === "assigned").length;
  const st = a.trip ? { buy_date: a.trip.buy_date ?? "", buy_hour: a.trip.buy_hour ?? null, buy_minute: a.trip.buy_minute ?? 0, trip_note: a.trip.trip_note ?? "" } : undefined;
  return (
    <li>
      <button type="button" className="plancard15" onClick={() => nav(`/plan/run/${encodeURIComponent(a.assignment_id)}`)}>
        <span className="plancard15__icon"><UsersRound size={26} aria-hidden /></span>
        <span className="plancard15__body">
          <span className="plancard15__name">{a.trip?.plan_name && a.trip.plan_name !== "Kế hoạch chính" ? a.trip.plan_name : m15.plan.main}</span>
          <span className="plancard15__sub">{m15.plan.from(a.owner_display_name)} · {m15.plan.count(n)}</span>
          <span className="plancard15__sub"><MapPin size={15} aria-hidden /> {st?.trip_note || m15.plan.noStore}</span>
          <span className="plancard15__sub"><CalendarDays size={15} aria-hidden /> {whenText(st)}</span>
          {finished ? (
            <span className="plancard15__tags"><span className="tag15 tag15--bought">{p22.askStatus[a.status] ?? a.status}</span></span>
          ) : (
            <span className="plancard15__sub">{m15.run.sender(a.owner_display_name)}</span>
          )}
        </span>
        <ChevronRight size={22} aria-hidden className="plancard15__chev" />
      </button>
    </li>
  );
}

/** Kế hoạch chưa lưu ở đầu tab Của tôi: chính là màn bước 1 (Thêm món), cuối có nút Tiếp tục. */
function TopDraft({ id }: { id: string }) {
  const plan = usePlan();
  const cart = plan.data;
  if (!cart) return <LoadState q={plan} hasData={false} />;
  return (
    <section className="top22" aria-label={p22.current}>
      <Stepper step={1} />
      <Step1Body id={id} cart={cart} />
      <NextButton id={id} cart={cart} />
    </section>
  );
}
