import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { m1 } from "../i18n/m1";
import { useEffect, useLayoutEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { CalendarDays, Check, ChevronLeft, ChevronRight, CirclePlus, House, MapPin, Minus, Pencil, Plus, Send, Trash2 } from "lucide-react";
import { ApiError } from "../api/client";
import { LoadState } from "../components/LoadState";
import {
  useAssignments, useDelegated, useHousehold, useItems, useMe, usePlan, usePlanItem, usePlanLine, usePlanMeta, usePlanPrices, usePlans, useSavePlanState, useClearDefaultPlan,
} from "../api/hooks";
import type { Assignment, Cart, CartItem, Item } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { CategoryIcon } from "../components/ItemRow";
import { AddAnything, BasketPricesPanel, MarketPricesPanel } from "../components/PlanExtras";
import { goodsTier, savedUnit, unitLabel } from "../lib/displayUnit";
import { setCurrentPlanId, useCurrentPlanId } from "../lib/currentPlan";
import { PLAN_ICONS, iconOf, planIcon } from "../lib/planIcon";
import { formatVnd } from "../lib/text";
import { planName } from "./PlanList";
import { Sheet } from "../components/Sheet15";
import { useRelations } from "../lib/relation";
import { buyerOf, clearDraft, isDraft, markDraft, setBuyer, usePlanDraftVersion, type BuyerPick } from "../lib/planDraft";
import { p22 } from "../i18n/p22";
import { AddToCalendar } from "../components/AddToCalendar";

const pad = (n: number) => String(n).padStart(2, "0");
const errText = (e: unknown) => (e instanceof ApiError ? e.message : t.common.genericError);

/**
 * Chi tiết Kế hoạch (Mimi 1.5, ảnh mẫu 5b, 5c): icon và tên, thời gian và nơi mua, danh sách dự kiến mua,
 * gợi ý mua, dự kiến chi, người mua (Tôi / Gửi cho thành viên / Mimi sắp ra mắt), Lưu Plan.
 * Spec 1.5 mục 1 (chốt 01/11): chọn người mua chưa gửi ngay; nút Lưu kế hoạch (máy bay giấy) mới gửi,
 * và phải chọn người mua thì mới bấm được. Sửa mà chưa Lưu thì kế hoạch "Chưa hoàn thành".
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
  const [confirmDel, setConfirmDel] = useState(false);
  const [confirmDone, setConfirmDone] = useState(false);
  const clearDefault = useClearDefaultPlan();
  const deleting = metaAct.remove.isPending || clearDefault.isPending;
  const saveState = useSavePlanState();
  usePlanDraftVersion();
  const me = useMe();
  const h = useHousehold();
  const { assign, cancel } = useAssignments();
  const [mimiAsk, setMimiAsk] = useState(false);
  const delegated = useDelegated();
  const sentTo = (delegated.data?.from_me ?? []).find((a) => a.plan_id === id && (a.status === "assigned" || a.status === "shopping"));
  const buyer: BuyerPick | null = buyerOf(id) ?? (sentTo ? sentTo.buyer_uid : null);
  const buyerName = buyer && buyer !== "me" && buyer !== me.data?.uid ? (h.data?.members.find((m) => m.uid === buyer)?.display_name ?? sentTo?.buyer_display_name ?? "") : "";

  const [name, setName] = useState("");
  const [editName, setEditName] = useState(false);
  const icon = planIcon(meta);
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
  const busy = saveState.isPending || metaAct.update.isPending || assign.isPending;
  const changed = () => markDraft(id);
  const save = () => {
    if (busy || !cart || !buyer) return;
    const [h, m] = time ? time.split(":").map(Number) : [null, null];
    const newName = name.trim();
    if (meta && !meta.default && newName && newName !== meta.name) metaAct.update.mutate({ id, name: newName });
    saveState.mutate(
      { buy_date: date, buy_hour: h, buy_minute: m, trip_note: store.trim(), base_rev: cart.state.rev },
      {
        onSuccess: () => {
          const done = (msg: string) => {
            clearDraft(id);
            nav("/plan", { state: { saved: msg } });
          };
          if (!buyerName) return done(m15.plan.saved);
          // Gửi cho người mua: lần đầu là gửi, các lần sau là cập nhật (gửi tuần tự, Spec 1.5 mục 1).
          // B5: gửi cả danh sách hiện tại; máy chủ ghi đè bản đã gửi, món đã bỏ khỏi kế hoạch trả về.
          const keys = cart.items.map((i) => i.key);
          if (keys.length === 0) return done(sentTo ? m15.plan.updated(buyerName) : m15.plan.saved);
          assign.mutate({ buyer_uid: buyer, keys }, { onSuccess: () => done(sentTo ? m15.plan.updated(buyerName) : m15.plan.sent(buyerName)) });
        },
      },
    );
  };

  return (
    <MainFrame>
      <TopBar title={meta ? planName(meta) : m15.plan.title} />
      <div className="page p15 pd15">
        <LoadState q={plan} hasData={!!cart} />

        {/* Icon và tên */}
        <div className="pd15__head">
          <button type="button" className="pd15__back" aria-label={m15.detail.back} onClick={() => nav("/plan")}>
            <ChevronLeft size={28} aria-hidden />
          </button>
          <button type="button" className="pd15__icon" aria-label={m15.plan.pickIcon} onClick={() => setIconSheet(true)}>
            <Icon size={28} aria-hidden />
          </button>
          {editName && meta && !meta.default ? (
            <input className="pd15__name" value={name} maxLength={40} aria-label={m15.plan.name} autoFocus onChange={(e) => { setName(e.target.value); changed(); }} onBlur={() => setEditName(false)} />
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
                <label>{m15.plan.date}<input type="date" value={date} onChange={(e) => { setDate(e.target.value); changed(); }} /></label>
                <label>{m15.plan.time}<input type="time" value={time} onChange={(e) => { setTime(e.target.value); changed(); }} /></label>
              </div>
              <label>{m15.plan.store}<input value={store} maxLength={80} placeholder={m15.plan.storePlaceholder} onChange={(e) => { setStore(e.target.value); changed(); }} /></label>
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
          {!editTrip && date && (
            <AddToCalendar
              ev={{
                title: m15.plan.calTitle(name),
                date,
                time,
                location: store,
                details: m15.plan.calDetails((cart?.items ?? []).slice(0, 20).map((i) => i.name)),
              }}
            />
          )}
        </section>

        {sentTo && <Progress a={sentTo} />}
        {cart && <Lines cart={cart} onChange={changed} done={new Set((sentTo?.items ?? []).filter((i) => i.status === "done").map((i) => i.consumption_group_key))} />}
        {cart && <Suggestions cart={cart} onChange={changed} />}
        {cart && <Forecast cart={cart} />}
        {cart && (
          <Buyer
            pick={buyer === me.data?.uid ? "me" : buyer}
            name={buyerName}
            onMe={() => { setBuyer(id, "me"); changed(); }}
            onMember={() => setMemberSheet(true)}
            onMimi={() => (sentTo ? setMimiAsk(true) : nav("/plan/online"))}
          />
        )}

        {/* Mimi 2.2 (anh chốt 09/11): kế hoạch chỉ hoàn thành khi quét hóa đơn hoặc người dùng tự xác nhận. */}
        {meta?.status === "archived" ? (
          <p className="sent15" role="status"><Check size={18} aria-hidden /> {p22.confirmedDone}</p>
        ) : (
          meta && !meta.default && !isDraft(id) && buyer && cart?.phase !== "reconciled" && (
            <div className="pd22__more">
              <button type="button" className="btn btn--outline btn--block btn--lg" onClick={() => { markDraft(id); nav(`/plan/${encodeURIComponent(id)}/add`); }}>
                <Pencil size={18} aria-hidden /> {p22.editPlan}
              </button>
              <button
                type="button"
                className={`btn btn--outline btn--block btn--lg${confirmDone ? " pd15__del--confirm" : ""}`}
                aria-disabled={metaAct.update.isPending || undefined}
                onClick={() => {
                  if (metaAct.update.isPending) return;
                  if (!confirmDone) return setConfirmDone(true);
                  metaAct.update.mutate({ id, status: "archived" }, { onSuccess: () => { setCurrentPlanId("default"); nav("/plan", { replace: true }); } });
                }}
              >
                <Check size={18} aria-hidden /> {confirmDone ? p22.confirmDoneAsk : p22.confirmDone}
              </button>
              <p className="hint">{p22.confirmDoneHint}</p>
            </div>
          )
        )}
        {(saveState.isError || metaAct.update.isError) && <p className="error" role="alert">{errText(saveState.error ?? metaAct.update.error)}</p>}
        {/* Góp ý 31/10 15:43: nút Xóa nằm cạnh Lưu kế hoạch. Xóa không hoàn tác được nên bấm hai lần. */}
        {meta?.status !== "archived" && <div className="pd15__actions">
          <button
            type="button"
            className={`btn btn--lg pd15__del${confirmDel ? " pd15__del--confirm" : ""}`}
            aria-disabled={!meta || deleting || undefined}
            onClick={() => {
              if (!meta || deleting) return;
              if (!confirmDel) return setConfirmDel(true);
              const back = () => { setCurrentPlanId("default"); nav("/plan", { replace: true }); };
              // Giỏ mua sắm (mặc định) không xóa hẳn được ở máy chủ: làm trống thì nó tự ẩn (góp ý 31/10 16:04).
              if (meta.default) clearDefault.mutate((cart?.items ?? []).map((i) => i.key), { onSuccess: back });
              else metaAct.remove.mutate(id, { onSuccess: back });
            }}
          >
            <Trash2 size={18} aria-hidden /> {confirmDel ? m15.plan.deleteConfirm : m15.plan.deleteShort}
          </button>
          <button type="button" className="btn btn--primary btn--lg" aria-disabled={busy || !buyer || undefined} onClick={save}>
            <Send size={18} aria-hidden /> {sentTo ? m15.plan.saveUpdate : m15.plan.save}
          </button>
        </div>}
        {!buyer && meta?.status !== "archived" && <p className="hint" role="status">{m15.plan.needBuyer}</p>}
        {assign.isError && <p className="error" role="alert">{errText(assign.error)}</p>}
        {mimiAsk && sentTo && (
          <Sheet title={m15.plan.mimiAskTitle} sub={m15.plan.mimiTakeOver(sentTo.buyer_display_name)} onClose={() => setMimiAsk(false)}>
            {cancel.isError && <p className="error" role="alert">{errText(cancel.error)}</p>}
            <div className="sheet15__btns">
              <button type="button" className="btn btn--outline" onClick={() => setMimiAsk(false)}>{m15.plan.mimiKeep}</button>
              <button
                type="button"
                className="btn btn--primary"
                aria-disabled={cancel.isPending || undefined}
                onClick={() => !cancel.isPending && cancel.mutate(sentTo.assignment_id, { onSuccess: () => { setMimiAsk(false); nav("/plan/online"); } })}
              >
                {m15.plan.mimiConfirm}
              </button>
            </div>
          </Sheet>
        )}
        {confirmDel && <p className="hint" role="status">{sentTo ? m15.plan.deleteAskSent(sentTo.buyer_display_name) : m15.plan.deleteAsk}</p>}
        {(metaAct.remove.isError || clearDefault.isError) && <p className="error" role="alert">{errText(metaAct.remove.error ?? clearDefault.error)}</p>}
      </div>

      {iconSheet && (
        <IconSheet
          value={icon}
          onCancel={() => setIconSheet(false)}
          onSave={(k) => {
            metaAct.update.mutate({ id, icon: k });
            setIconSheet(false);
            changed();
          }}
        />
      )}
      {memberSheet && (
        <MemberSheet
          value={buyer && buyer !== "me" ? buyer : ""}
          onClose={() => setMemberSheet(false)}
          onPick={(uid) => {
            setBuyer(id, uid);
            changed();
            setMemberSheet(false);
          }}
        />
      )}
    </MainFrame>
  );
}

/* ---------- Danh sách dự kiến mua ---------- */
export function Lines({ cart, onChange, done }: { cart: Cart; onChange: () => void; done: Set<string> }) {
  const [editing, setEditing] = useState<string | null>(null);
  const h = useHousehold();
  const who = (uid?: string) => h.data?.members.find((m) => m.uid === uid)?.display_name ?? "";
  return (
    <section aria-labelledby="pd-list">
      <h2 id="pd-list" className="h15sec">{m15.plan.list(cart.item_count)}</h2>
      {cart.items.length === 0 && <p className="hint">{m15.plan.listEmpty}</p>}
      <ul className="lines15">
        {cart.items.map((i) => (
          <LineRow key={i.key} line={i} open={editing === i.key} onToggle={() => setEditing(editing === i.key ? null : i.key)} assignee={who(i.assigned_to_uid)} onChange={onChange} bought={done.has(i.key)} />
        ))}
      </ul>
    </section>
  );
}

function LineRow({ line, open, onToggle, assignee, onChange, bought }: { line: CartItem; open: boolean; onToggle: () => void; assignee: string; onChange: () => void; bought: boolean }) {
  const patch = usePlanLine();
  const edit = usePlanItem();
  const unit = line.unit || savedUnit({ key: line.key, name: line.name });
  const price = line.unit_price != null ? formatVnd(line.unit_price * line.qty) : m15.plan.noPrice;
  // Góp ý 31/10 16:28: −/+ chỉ đổi số trên màn; bấm dấu tích mới lưu. Bỏ món bằng biểu tượng thùng rác.
  const [qty, setQty] = useState(line.qty);
  useEffect(() => setQty(line.qty), [line.qty, open]);
  const saveQty = () => {
    if (patch.isPending) return;
    if (qty === line.qty) return onToggle();
    patch.mutate({ key: line.key, patch: { qty } }, { onSuccess: () => { onChange(); onToggle(); } });
  };
  return (
    <li className="line15">
      <div className="line15__row">
        <CategoryIcon category={line.category_key} />
        <span className="line15__text">
          <span className="line15__name">{line.name}{bought && <span className="tag15 tag15--bought">{m15.run.bought}</span>}</span>
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
            <button type="button" aria-label={m15.detail.minus} onClick={() => qty > 1 && setQty(qty - 1)}><Minus size={18} aria-hidden /></button>
            <input readOnly value={qty} aria-label={m15.detail.qtyTitle} />
            <button type="button" aria-label={m15.detail.plus} onClick={() => setQty(qty + 1)}><Plus size={18} aria-hidden /></button>
          </div>
          <button type="button" className="icon15 icon15--ok" aria-label={m15.plan.saveLine(line.name)} aria-disabled={patch.isPending || undefined} onClick={saveQty}>
            <Check size={22} aria-hidden />
          </button>
          <button type="button" className="icon15 icon15--del" aria-label={m15.plan.removeLineOf(line.name)} aria-disabled={edit.isPending || undefined} onClick={() => !edit.isPending && edit.mutate({ key: line.key, add: false }, { onSuccess: onChange })}>
            <Trash2 size={22} aria-hidden />
          </button>
        </div>
      )}
    </li>
  );
}

/* ---------- Mẹ xem kết quả mua hộ (Spec 1.5 mục 2): tiến độ và ghi chú, không có hóa đơn của người mua ---------- */
function Progress({ a }: { a: Assignment }) {
  const [openNote, setOpenNote] = useState(false);
  const total = a.items.length;
  const done = a.items.filter((i) => i.status === "done").length;
  return (
    <section className="box15 prog15" aria-label={m15.run.progress(done, total)}>
      <div className="prog15__top">
        <span>{m15.run.progress(done, total)}</span>
        <span className="hint">{m15.plan.buyer(a.buyer_display_name)}</span>
      </div>
      <div className="prog15__bar" role="progressbar" aria-valuemin={0} aria-valuemax={total} aria-valuenow={done}>
        <i style={{ width: `${total ? (done / total) * 100 : 0}%` }} />
      </div>
      {a.note && (
        <button type="button" className="prog15__note" aria-expanded={openNote} onClick={() => setOpenNote(!openNote)}>
          <span>{m15.run.noteOf(a.buyer_display_name)}</span> <ChevronRight size={16} aria-hidden />
        </button>
      )}
      {openNote && a.note && <p className="prog15__notetext">{a.note}</p>}
    </section>
  );
}

/* ---------- Gợi ý mua, Kiểm tra đồ dùng, Thêm sản phẩm ---------- */
function daysText(i: Item) {
  if (i.status === "out" || (i.stock && i.stock.estimated <= 0)) return m15.plan.outNow;
  if (i.status === "waiting" || i.days_left == null) return m15.plan.learning;
  return m15.plan.leftDays(Math.max(0, i.days_left));
}

function Suggestions({ cart, onChange }: { cart: Cart; onChange: () => void }) {
  const all = useItems({ tab: "active", horizon: 7 });
  const edit = usePlanItem();
  const nav = useNavigate();
  const [adding, setAdding] = useState(false);
  const inCart = useMemo(() => new Set(cart.items.map((i) => i.key)), [cart.items]);
  const rest = (all.data ?? []).filter((i) => !inCart.has(i.key));
  const sugg = rest.filter((i) => goodsTier(i) !== "plenty").sort((a, b) => (a.days_left ?? 999) - (b.days_left ?? 999));
  const busy = edit.isPending ? edit.variables?.key : undefined;
  const row = (i: Item) => (
    <li key={i.key} className="line15">
      <div className="line15__row">
        <CategoryIcon category={i.category_key} />
        <span className="line15__text">
          <span className="line15__name">{i.name}</span>
          <span className="line15__sub">{daysText(i)}</span>
        </span>
        <button type="button" className="btn btn--outline add15" aria-disabled={busy === i.key || undefined} onClick={() => busy !== i.key && edit.mutate({ key: i.key, add: true }, { onSuccess: onChange })}>
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
        {/* Góp ý 31/10 16:22: bấm thì mở thẳng tab Đồ dùng. */}
        <button type="button" className="btn btn--outline" onClick={() => nav("/items")}>
          <House size={20} aria-hidden /> <span>{m15.plan.checkHome}</span>
          <ChevronRight size={18} aria-hidden />
        </button>
        <button type="button" className="btn btn--outline" aria-expanded={adding} onClick={() => setAdding(!adding)}>
          <CirclePlus size={20} aria-hidden /> <span>{m15.plan.addProduct}</span>
        </button>
      </div>
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
      {compare && <MarketPricesPanel cart={cart} />}
      {compare && <BasketPricesPanel cart={cart} />}
    </section>
  );
}

/* ---------- Người mua ---------- */
const SHOW_MIMI_BUYER = false;
function Buyer({ pick, name, onMe, onMember, onMimi }: { pick: BuyerPick | null; name: string; onMe: () => void; onMember: () => void; onMimi: () => void }) {
  const member = !!pick && pick !== "me";
  return (
    <section aria-labelledby="pd-buyer">
      <h2 id="pd-buyer" className="h15sec">{m15.plan.buyerTitle}</h2>
      <div className="buyer15" role="group" aria-labelledby="pd-buyer">
        <button type="button" aria-pressed={pick === "me"} onClick={onMe}>{m15.plan.me}</button>
        <button type="button" aria-pressed={member} onClick={onMember}>
          {member && name ? name : m15.plan.sendMember}
        </button>
        {/* Anh chốt 09/11 (Cài đặt T1): bỏ Mimi mua online theo Ủy quyền; mua online chỉ qua giá Channel3 ở bước So sánh giá. */}
        {SHOW_MIMI_BUYER && <button type="button" onClick={onMimi}>{m15.plan.mimi}<small className="buyer15__sub">{m15.plan.mimiOnline}</small></button>}
      </div>
    </section>
  );
}

/* ---------- Bảng chọn icon ---------- */
export function IconSheet({ value, onCancel, onSave }: { value: string; onCancel: () => void; onSave: (k: string) => void }) {
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

/* ---------- Bảng chọn thành viên: chỉ chọn người mua, chưa gửi (Spec 1.5 mục 1) ---------- */
function MemberSheet({ value, onClose, onPick }: { value: string; onClose: () => void; onPick: (uid: string) => void }) {
  const me = useMe();
  const h = useHousehold();
  const { relationOf } = useRelations();
  const others = (h.data?.members ?? []).filter((m) => m.uid !== me.data?.uid);
  const [pick, setPick] = useState(value || others[0]?.uid || "");
  // Danh sách người nhà tải sau /me: luôn chọn sẵn người đầu tiên còn hợp lệ.
  useEffect(() => {
    if (others.length && !others.some((m) => m.uid === pick)) setPick(others[0].uid);
  }, [others, pick]);
  return (
    <Sheet title={m15.plan.memberTitle} sub={m15.plan.memberSub} onClose={onClose}>
      {others.length === 0 ? (
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
      <div className="sheet15__btns">
        <button type="button" className="btn btn--outline" onClick={onClose}>{m15.plan.cancel}</button>
        <button type="button" className="btn btn--primary" aria-disabled={!pick || undefined} onClick={() => pick && onPick(pick)}>
          {m15.plan.choose}
        </button>
      </div>
    </Sheet>
  );
}
