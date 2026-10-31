import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { m1 } from "../i18n/m1";
import { useEffect, useLayoutEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight, CirclePlus, House, MapPin, Minus, Pencil, Plus, Trash2, X } from "lucide-react";
import { ApiError } from "../api/client";
import {
  useAssignments, useDelegated, useHousehold, useItems, useMe, usePlan, usePlanItem, usePlanLine, usePlanMeta, usePlanPrices, usePlans, useSavePlanState,
} from "../api/hooks";
import type { Cart, CartItem, Item } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { CategoryIcon } from "../components/ItemRow";
import { AddAnything, BasketPricesPanel } from "../components/PlanExtras";
import { goodsTier, savedUnit, unitLabel } from "../lib/displayUnit";
import { setCurrentPlanId, useCurrentPlanId } from "../lib/currentPlan";
import { PLAN_ICONS, iconOf, planIcon, setPlanIcon } from "../lib/planIcon";
import { formatVnd } from "../lib/text";
import { planName } from "./PlanList";
import { relationOf } from "../lib/relation";

const pad = (n: number) => String(n).padStart(2, "0");
const errText = (e: unknown) => (e instanceof ApiError ? e.message : t.common.genericError);

/**
 * Chi tiết Kế hoạch (Mimi 1.5, ảnh mẫu 5b, 5c): icon và tên, thời gian và nơi mua, danh sách dự kiến mua,
 * gợi ý mua, dự kiến chi, người mua (Tôi / Gửi cho thành viên / Mimi sắp ra mắt), Lưu Plan.
 */
export function PlanDetail() {
  const { id = "default" } = useParams();
  const current = useCurrentPlanId();
  // Các hook Kế hoạch đọc "Kế hoạch đang xem": chuyển sang Kế hoạch trên đường dẫn trước khi vẽ.
  useLayoutEffect(() => {
    if (current !== id) setCurrentPlanId(id);
  }, [current, id]);
  if (current !== id) return null;
  return <Detail id={id} />;
}

function Detail({ id }: { id: string }) {
  const plans = usePlans();
  const meta = plans.data?.find((p) => p.id === id);
  const plan = usePlan();
  const cart = plan.data;
  const nav = useNavigate();
  const metaAct = usePlanMeta();
  const saveState = useSavePlanState();

  const [name, setName] = useState("");
  const [editName, setEditName] = useState(false);
  const [icon, setIcon] = useState(planIcon(id));
  const [iconSheet, setIconSheet] = useState(false);
  const [editTrip, setEditTrip] = useState(false);
  const [date, setDate] = useState("");
  const [time, setTime] = useState("");
  const [store, setStore] = useState("");
  const [memberSheet, setMemberSheet] = useState(false);

  useEffect(() => {
    if (meta) setName(planName(meta));
  }, [meta]);
  useEffect(() => {
    if (!cart) return;
    setDate(cart.state.buy_date || "");
    setTime(cart.state.buy_hour != null ? `${pad(cart.state.buy_hour)}:${pad(cart.state.buy_minute ?? 0)}` : "");
    setStore(cart.state.trip_note || "");
  }, [cart?.state]); // eslint-disable-line react-hooks/exhaustive-deps

  if (plans.data && !meta) {
    return (
      <MainFrame>
        <TopBar title={m15.plan.title} variant="sub" />
        <div className="page empty"><p>{t.common.genericError}</p></div>
      </MainFrame>
    );
  }

  const Icon = iconOf(icon);
  const busy = saveState.isPending || metaAct.update.isPending;
  const save = () => {
    if (busy || !cart) return;
    const [h, m] = time ? time.split(":").map(Number) : [null, null];
    const newName = name.trim();
    if (meta && !meta.default && newName && newName !== meta.name) metaAct.update.mutate({ id, name: newName });
    saveState.mutate(
      { buy_date: date, buy_hour: h, buy_minute: m, trip_note: store.trim() },
      { onSuccess: () => nav("/plan", { state: { saved: true } }) },
    );
  };

  return (
    <MainFrame>
      <TopBar title={meta ? planName(meta) : m15.plan.title} />
      <div className="page p15 pd15">
        {plan.isPending && <p className="hint">{t.common.loading}</p>}
        {plan.isError && !cart && <p className="error" role="alert">{errText(plan.error)}</p>}

        {/* Icon và tên */}
        <div className="pd15__head">
          <button type="button" className="pd15__back" aria-label={m15.detail.back} onClick={() => nav("/plan")}>
            <ChevronLeft size={28} aria-hidden />
          </button>
          <button type="button" className="pd15__icon" aria-label={m15.plan.pickIcon} onClick={() => setIconSheet(true)}>
            <Icon size={28} aria-hidden />
          </button>
          {editName && meta && !meta.default ? (
            <input className="pd15__name" value={name} maxLength={40} aria-label={m15.plan.name} autoFocus onChange={(e) => setName(e.target.value)} onBlur={() => setEditName(false)} />
          ) : (
            <button
              type="button"
              className="pd15__name"
              aria-label={m15.plan.editName}
              aria-disabled={meta?.default || undefined}
              onClick={() => meta && !meta.default && setEditName(true)}
            >
              <span>{name}</span>
              {meta && !meta.default && <Pencil size={20} aria-hidden />}
            </button>
          )}
        </div>

        {/* Thời gian và nơi mua */}
        <section className="box15 trip15" aria-label={m15.plan.editTrip}>
          {editTrip ? (
            <div className="plan-form">
              <div className="plan-form__row">
                <label>{m15.plan.date}<input type="date" value={date} onChange={(e) => setDate(e.target.value)} /></label>
                <label>{m15.plan.time}<input type="time" value={time} onChange={(e) => setTime(e.target.value)} /></label>
              </div>
              <label>{m15.plan.store}<input value={store} maxLength={80} placeholder={m15.plan.storePlaceholder} onChange={(e) => setStore(e.target.value)} /></label>
              <button type="button" className="btn btn--quiet" onClick={() => setEditTrip(false)}>{m15.plan.close}</button>
            </div>
          ) : (
            <button type="button" className="trip15__view" aria-label={m15.plan.editTrip} onClick={() => setEditTrip(true)}>
              <span className="trip15__lines">
                <span><CalendarDays size={18} aria-hidden /> {date ? `${date.split("-").reverse().join("/")}${time ? ` · ${time}` : ""}` : m15.plan.noDate}</span>
                <span><MapPin size={18} aria-hidden /> {store ? m15.plan.buyAt(store) : m15.plan.noStore}</span>
              </span>
              <Pencil size={20} aria-hidden />
            </button>
          )}
        </section>

        {cart && <Lines cart={cart} />}
        {cart && <Suggestions cart={cart} />}
        {cart && <Forecast cart={cart} />}
        {cart && <Buyer cart={cart} planId={id} onSend={() => setMemberSheet(true)} />}

        {(saveState.isError || metaAct.update.isError) && <p className="error" role="alert">{errText(saveState.error ?? metaAct.update.error)}</p>}
        <button type="button" className="btn btn--primary btn--block btn--lg" aria-disabled={busy || undefined} onClick={save}>
          {m15.plan.save}
        </button>
        {meta && !meta.default && (
          <button
            type="button"
            className="btn btn--quiet btn--block btn--danger"
            onClick={() => metaAct.remove.mutate(id, { onSuccess: () => { setCurrentPlanId("default"); nav("/plan", { replace: true }); } })}
          >
            <Trash2 size={18} aria-hidden /> {m15.plan.delete}
          </button>
        )}
      </div>

      {iconSheet && (
        <IconSheet
          value={icon}
          onCancel={() => setIconSheet(false)}
          onSave={(k) => {
            setIcon(k);
            setPlanIcon(id, k);
            setIconSheet(false);
          }}
        />
      )}
      {memberSheet && cart && <MemberSheet cart={cart} onClose={() => setMemberSheet(false)} />}
    </MainFrame>
  );
}

/* ---------- Danh sách dự kiến mua ---------- */
function Lines({ cart }: { cart: Cart }) {
  const [editing, setEditing] = useState<string | null>(null);
  const h = useHousehold();
  const who = (uid?: string) => h.data?.members.find((m) => m.uid === uid)?.display_name ?? "";
  return (
    <section aria-labelledby="pd-list">
      <h2 id="pd-list" className="h15sec">{m15.plan.list(cart.item_count)}</h2>
      {cart.items.length === 0 && <p className="hint">{m15.plan.listEmpty}</p>}
      <ul className="lines15">
        {cart.items.map((i) => (
          <LineRow key={i.key} line={i} open={editing === i.key} onToggle={() => setEditing(editing === i.key ? null : i.key)} assignee={who(i.assigned_to_uid)} />
        ))}
      </ul>
    </section>
  );
}

function LineRow({ line, open, onToggle, assignee }: { line: CartItem; open: boolean; onToggle: () => void; assignee: string }) {
  const patch = usePlanLine();
  const edit = usePlanItem();
  const unit = line.unit || savedUnit({ key: line.key, name: line.name });
  const price = line.unit_price != null ? formatVnd(line.unit_price * line.qty) : m15.plan.noPrice;
  const setQty = (q: number) => q >= 1 && !patch.isPending && patch.mutate({ key: line.key, patch: { qty: q } });
  return (
    <li className="line15">
      <div className="line15__row">
        <CategoryIcon category={line.category_key} />
        <span className="line15__text">
          <span className="line15__name">{line.name}</span>
          <span className="line15__sub">
            {line.qty} {unitLabel(unit)} · {price}
            {line.line_status === "assigned" && ` · ${m15.plan.assignedTo(assignee || m1.assign.someone)}`}
          </span>
        </span>
        <button type="button" className="icon-btn" aria-expanded={open} aria-label={m15.plan.editLine(line.name)} onClick={onToggle}>
          <Pencil size={20} aria-hidden />
        </button>
      </div>
      {open && (
        <div className="line15__edit">
          <div className="stepper15">
            <button type="button" aria-label={m15.detail.minus} onClick={() => setQty(line.qty - 1)}><Minus size={18} aria-hidden /></button>
            <input readOnly value={line.qty} aria-label={m15.detail.qtyTitle} />
            <button type="button" aria-label={m15.detail.plus} onClick={() => setQty(line.qty + 1)}><Plus size={18} aria-hidden /></button>
          </div>
          <button type="button" className="btn btn--quiet btn--danger" onClick={() => edit.mutate({ key: line.key, add: false })}>
            <Trash2 size={18} aria-hidden /> {m15.plan.removeLine}
          </button>
        </div>
      )}
    </li>
  );
}

/* ---------- Gợi ý mua, Kiểm tra đồ dùng, Thêm sản phẩm ---------- */
function daysText(i: Item) {
  if (i.status === "out" || (i.stock && i.stock.estimated <= 0)) return m15.plan.outNow;
  if (i.status === "waiting" || i.days_left == null) return m15.plan.learning;
  return m15.plan.leftDays(Math.max(0, i.days_left));
}

function Suggestions({ cart }: { cart: Cart }) {
  const all = useItems({ tab: "active", horizon: 7 });
  const edit = usePlanItem();
  const [checkAll, setCheckAll] = useState(false);
  const [adding, setAdding] = useState(false);
  const inCart = useMemo(() => new Set(cart.items.map((i) => i.key)), [cart.items]);
  const rest = (all.data ?? []).filter((i) => !inCart.has(i.key));
  const sugg = rest.filter((i) => goodsTier(i) !== "plenty").sort((a, b) => (a.days_left ?? 999) - (b.days_left ?? 999));
  const others = rest.filter((i) => goodsTier(i) === "plenty");
  const busy = edit.isPending ? edit.variables?.key : undefined;
  const row = (i: Item) => (
    <li key={i.key} className="line15">
      <div className="line15__row">
        <CategoryIcon category={i.category_key} />
        <span className="line15__text">
          <span className="line15__name">{i.name}</span>
          <span className="line15__sub">{daysText(i)}</span>
        </span>
        <button type="button" className="btn btn--outline add15" aria-disabled={busy === i.key || undefined} onClick={() => busy !== i.key && edit.mutate({ key: i.key, add: true })}>
          <Plus size={18} aria-hidden /> {m15.plan.add}
        </button>
      </div>
    </li>
  );
  return (
    <>
      <section aria-labelledby="pd-sugg">
        <h2 id="pd-sugg" className="h15sec">{m15.plan.suggest}</h2>
        {all.data && sugg.length === 0 && <p className="hint">{m15.plan.suggestEmpty}</p>}
        <ul className="lines15">{sugg.map(row)}</ul>
      </section>
      <div className="pd15__two">
        <button type="button" className="btn btn--outline" aria-expanded={checkAll} onClick={() => setCheckAll(!checkAll)}>
          <House size={20} aria-hidden /> <span>{m15.plan.checkHome}</span>
          {checkAll ? <ChevronDown size={18} aria-hidden /> : <ChevronRight size={18} aria-hidden />}
        </button>
        <button type="button" className="btn btn--outline" aria-expanded={adding} onClick={() => setAdding(!adding)}>
          <CirclePlus size={20} aria-hidden /> <span>{m15.plan.addProduct}</span>
        </button>
      </div>
      {checkAll && <ul className="lines15">{others.map(row)}</ul>}
      {adding && <AddAnything cart={cart} />}
    </>
  );
}

/* ---------- Dự kiến chi ---------- */
function Forecast({ cart }: { cart: Cart }) {
  const prices = usePlanPrices(cart.item_count > 0);
  const [compare, setCompare] = useState(false);
  const p = prices.data;
  const sub =
    p?.recommend === "split" && p.split.length > 1
      ? m15.plan.split(p.split.length)
      : p?.single_store
        ? m15.plan.cover(p.single_store.store, p.single_store.groups.length, cart.item_count)
        : "";
  return (
    <section className="box15 fc15b" aria-labelledby="pd-fc">
      <div className="fc15b__top">
        <h2 id="pd-fc">{m15.plan.forecast}</h2>
        <span className="num fc15b__total">{cart.estimate != null ? formatVnd(cart.estimate) : "–"}</span>
      </div>
      {sub && <p className="hint">{sub}</p>}
      {cart.item_count > 0 && (
        <button type="button" className="link15 fc15b__link" aria-expanded={compare} onClick={() => setCompare(!compare)}>
          {compare ? m15.plan.hideCompare : m15.plan.compare} <ChevronRight size={16} aria-hidden />
        </button>
      )}
      {compare && <BasketPricesPanel cart={cart} />}
    </section>
  );
}

/* ---------- Người mua ---------- */
function Buyer({ cart, planId, onSend }: { cart: Cart; planId: string; onSend: () => void }) {
  const d = useDelegated();
  const sentTo = (d.data?.from_me ?? []).find((a) => a.plan_id === planId && (a.status === "assigned" || a.status === "shopping"));
  const assigned = !!sentTo || cart.items.some((i) => i.line_status === "assigned");
  return (
    <section aria-labelledby="pd-buyer">
      <h2 id="pd-buyer" className="h15sec">{m15.plan.buyerTitle}</h2>
      <div className="buyer15" role="group" aria-labelledby="pd-buyer">
        <button type="button" aria-pressed={!assigned}>{m15.plan.me}</button>
        <button type="button" aria-pressed={assigned} onClick={onSend}>
          {sentTo ? sentTo.buyer_display_name : m15.plan.sendMember}
        </button>
        <button type="button" disabled aria-disabled="true" className="buyer15__soon">
          {m15.plan.mimi}<small>{m15.plan.soon}</small>
        </button>
      </div>
    </section>
  );
}

/* ---------- Bảng chọn icon ---------- */
function IconSheet({ value, onCancel, onSave }: { value: string; onCancel: () => void; onSave: (k: string) => void }) {
  const [pick, setPick] = useState(value);
  return (
    <Sheet title={m15.plan.pickIcon} sub={m15.plan.pickIconSub} onClose={onCancel}>
      <div className="icons15" role="radiogroup" aria-label={m15.plan.pickIcon}>
        {PLAN_ICONS.map(({ key, Icon }) => (
          <label key={key} className="icons15__opt">
            <span className="icons15__ico"><Icon size={28} aria-hidden /></span>
            <span>{m15.plan.icons[key]}</span>
            <input type="radio" name="plan-icon" checked={pick === key} onChange={() => setPick(key)} />
          </label>
        ))}
      </div>
      <div className="sheet15__btns">
        <button type="button" className="btn btn--outline" onClick={onCancel}>{m15.plan.cancel}</button>
        <button type="button" className="btn btn--primary" onClick={() => onSave(pick)}>{m15.plan.saveIcon}</button>
      </div>
    </Sheet>
  );
}

/* ---------- Bảng chọn thành viên (Gửi Plan) ---------- */
function MemberSheet({ cart, onClose }: { cart: Cart; onClose: () => void }) {
  const me = useMe();
  const h = useHousehold();
  const { assign } = useAssignments();
  const others = (h.data?.members ?? []).filter((m) => m.uid !== me.data?.uid);
  const [pick, setPick] = useState(others[0]?.uid ?? "");
  const [sent, setSent] = useState("");
  const keys = cart.items.filter((i) => i.line_status !== "assigned").map((i) => i.key);
  // Danh sách người nhà tải sau /me: luôn chọn sẵn người đầu tiên còn hợp lệ.
  useEffect(() => {
    if (others.length && !others.some((m) => m.uid === pick)) setPick(others[0].uid);
  }, [others, pick]);
  return (
    <Sheet title={m15.plan.memberTitle} sub={m15.plan.memberSub} onClose={onClose}>
      {sent ? (
        <p className="sent15" role="status">{sent}</p>
      ) : others.length === 0 ? (
        <p className="hint">{m15.plan.noMember}</p>
      ) : (
        <ul className="members15" role="radiogroup" aria-label={m15.plan.memberTitle}>
          {others.map((m) => (
            <li key={m.uid}>
              <label>
                <span className="members15__avatar" aria-hidden>{(m.display_name || "?").slice(0, 1).toUpperCase()}</span>
                <span className="members15__name">
                  {m.display_name}
                  {relationOf(m.uid) && <span className="members15__rel">{m15.settings.relations[relationOf(m.uid)]}</span>}
                </span>
                <input type="radio" name="buyer" checked={pick === m.uid} onChange={() => setPick(m.uid)} />
              </label>
            </li>
          ))}
        </ul>
      )}
      {!sent && keys.length === 0 && others.length > 0 && <p className="hint">{m15.plan.nothingToSend}</p>}
      {assign.isError && <p className="error" role="alert">{errText(assign.error)}</p>}
      <div className="sheet15__btns">
        <button type="button" className="btn btn--outline" onClick={onClose}>{sent ? m15.plan.close : m15.plan.cancel}</button>
        {!sent && (
          <button
            type="button"
            className="btn btn--primary"
            aria-disabled={!pick || keys.length === 0 || assign.isPending || undefined}
            onClick={() => {
              if (!pick || keys.length === 0 || assign.isPending) return;
              const who = others.find((m) => m.uid === pick)?.display_name ?? "";
              assign.mutate({ buyer_uid: pick, keys }, { onSuccess: () => setSent(m15.plan.sent(who)) });
            }}
          >
            {m15.plan.send}
          </button>
        )}
      </div>
    </Sheet>
  );
}

function Sheet({ title, sub, onClose, children }: { title: string; sub: string; onClose: () => void; children: React.ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div className="sheet15" role="presentation" onClick={onClose}>
      <div className="sheet15__panel" role="dialog" aria-modal="true" aria-label={title} onClick={(e) => e.stopPropagation()}>
        <span className="sheet15__grip" aria-hidden />
        <div className="sheet15__head">
          <div>
            <h2>{title}</h2>
            <p>{sub}</p>
          </div>
          <button type="button" className="icon-btn" aria-label={m15.plan.close} onClick={onClose}><X size={24} aria-hidden /></button>
        </div>
        {children}
      </div>
    </div>
  );
}
