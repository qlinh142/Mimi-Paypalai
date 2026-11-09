import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CalendarDays, Check, ChevronRight, Pencil, Plus, Scale, ShoppingCart, TrendingDown } from "lucide-react";
import { p22 } from "../i18n/p22";
import { m15 } from "../i18n/m15";
import { t } from "../i18n";
import { useItems, useMarketPrices, usePlan, usePlanItem, usePlanMeta, useSavePlanState } from "../api/hooks";
import type { Cart, CartItem, Item, MarketPrice, PlanMeta } from "../api/types";
import { CategoryIcon } from "./ItemRow";
import { Sheet } from "./Sheet15";
import { useUndo } from "./AppShell";
import { goodsTier, savedUnit, unitLabel } from "../lib/displayUnit";
import { formatDayMonth, formatVnd } from "../lib/text";
import { markDraft } from "../lib/planDraft";
import { planName } from "../screens/PlanList";

const pad = (n: number) => String(n).padStart(2, "0");
const iso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

/** Bước của kế hoạch (B1): chưa có món là 1, có món là 2, đã chọn người mua là 3. */
export function stepOf(cart: Pick<Cart, "item_count"> | undefined, hasBuyer: boolean): 1 | 2 | 3 {
  if (!cart || cart.item_count === 0) return 1;
  return hasBuyer ? 3 : 2;
}

/** Thanh 3 bước: Thêm món, So sánh giá, Mua. */
export function Stepper({ step }: { step: 1 | 2 | 3 }) {
  return (
    <ol className="steps22" aria-label={p22.stepOf(step, p22.steps[step - 1])}>
      {p22.steps.map((label, i) => {
        const n = i + 1;
        const st = n < step ? "done" : n === step ? "now" : "next";
        return (
          <li key={label} className={`steps22__item steps22__item--${st}`} aria-current={st === "now" ? "step" : undefined}>
            <span className="steps22__dot">{st === "done" ? <Check size={14} aria-hidden /> : n}</span>
            <span className="steps22__label">{label}</span>
          </li>
        );
      })}
    </ol>
  );
}

/** Món sắp hết (B2): nhóm Cần mua ngay và Sắp hết của Đồ dùng, chưa nằm trong kế hoạch. */
export function useLowItems(cart: Cart | undefined, horizon = 7): Item[] {
  const all = useItems({ tab: "active", horizon });
  return useMemo(() => {
    const inCart = new Set((cart?.items ?? []).map((i) => i.key));
    return (all.data ?? [])
      .filter((i) => !inCart.has(i.key))
      .filter((i) => goodsTier(i) !== "plenty" || (horizon > 7 && i.days_left != null && i.days_left <= horizon))
      .sort((a, b) => (a.days_left ?? 999) - (b.days_left ?? 999));
  }, [all.data, cart?.items, horizon]);
}

export type PriceLine = { item: CartItem; old: number | null; best: number | null; bestStore: string };

/** B3, B4: giá cũ (hóa đơn nhà mình) và giá thấp nhất đang bán cho từng món. */
export function priceLines(cart: Cart | undefined, market: { key: string; prices: MarketPrice[] }[] | undefined): PriceLine[] {
  const byKey = new Map((market ?? []).map((m) => [m.key, m.prices]));
  return (cart?.items ?? []).map((item) => {
    const old = item.unit_price != null ? item.unit_price * item.qty : null;
    const low = [...(byKey.get(item.key) ?? [])].sort((a, b) => a.price_vnd - b.price_vnd)[0];
    const online = low ? low.price_vnd * item.qty : null;
    // Giá thấp nhất tìm được: rẻ hơn trong hai nguồn.
    if (online != null && (old == null || online < old)) return { item, old, best: online, bestStore: low!.store };
    return { item, old, best: old, bestStore: item.best_store ?? "" };
  });
}

export function summary(lines: PriceLine[]) {
  const total = lines.reduce((s, l) => s + (l.best ?? 0), 0);
  const priced = lines.some((l) => l.best != null);
  // Chỉ cộng món có đủ hai giá (đề xuất B3).
  const save = lines.reduce((s, l) => s + (l.old != null && l.best != null && l.best < l.old ? l.old - l.best : 0), 0);
  return { total: priced ? total : null, save };
}

/** Ngày nhanh trong bảng Chọn ngày (5.3c). */
export function quickDates(now = new Date()) {
  const d = (n: number) => {
    const x = new Date(now);
    x.setDate(x.getDate() + n);
    return iso(x);
  };
  const dow = now.getDay(); // 0 CN, 6 T7
  const toSat = dow === 6 ? 0 : (6 - dow + 7) % 7;
  const toMon = ((1 - dow + 7) % 7) || 7;
  return [
    { k: "today", label: p22.quick.today, date: d(0) },
    { k: "tomorrow", label: p22.quick.tomorrow, date: d(1) },
    { k: "weekend", label: p22.quick.weekend, date: d(toSat) },
    { k: "nextWeek", label: p22.quick.nextWeek, date: d(toMon) },
  ];
}

/** Hai hàng Cửa hàng và Ngày mua, bấm mở bảng chọn. Lưu ngay vào kế hoạch, kế hoạch vẫn là nháp. */
export function TripRows({ planId, cart }: { planId: string; cart: Cart }) {
  const [sheet, setSheet] = useState<"store" | "date" | null>(null);
  const st = cart.state;
  const when = st.buy_date ? formatDayMonth(st.buy_date) + (st.buy_hour != null ? ` · ${pad(st.buy_hour)}:${pad(st.buy_minute ?? 0)}` : "") : "";
  return (
    <>
      <div className="trip22">
        {/* Anh bỏ dòng Cửa hàng ở bước 1 (09/11): nơi mua chọn theo giá ở bước 2. */}
        <button type="button" className="trip22__row" onClick={() => setSheet("date")}>
          <CalendarDays size={20} aria-hidden />
          <span className="trip22__label">{p22.date}</span>
          <span className={when ? "trip22__val" : "trip22__val trip22__val--empty"}>{when || p22.chooseDate}</span>
          <ChevronRight size={18} aria-hidden />
        </button>
      </div>
      {sheet && <TripSheet kind={sheet} planId={planId} cart={cart} onClose={() => setSheet(null)} />}
    </>
  );
}

function TripSheet({ kind, planId, cart, onClose }: { kind: "store" | "date"; planId: string; cart: Cart; onClose: () => void }) {
  const save = useSavePlanState();
  const st = cart.state;
  const [store, setStore] = useState(st.trip_note || "");
  const [date, setDate] = useState(st.buy_date || "");
  const [time, setTime] = useState(st.buy_hour != null ? `${pad(st.buy_hour)}:${pad(st.buy_minute ?? 0)}` : "09:00");
  const stores = useMemo(() => [...new Set(cart.items.map((i) => i.best_store).filter((s): s is string => !!s))].slice(0, 4), [cart.items]);
  const commit = () => {
    if (save.isPending) return;
    const [h, m] = date && time ? time.split(":").map(Number) : [null, null];
    save.mutate(
      { buy_date: date, buy_hour: h, buy_minute: m, trip_note: store.trim(), base_rev: st.rev },
      { onSuccess: () => { markDraft(planId); onClose(); } },
    );
  };
  return (
    <Sheet title={kind === "store" ? p22.storeSheet : p22.dateSheet} sub={kind === "store" ? p22.storeSub : p22.dateSub} onClose={onClose}>
      {kind === "store" ? (
        <div className="tsheet22">
          <label className="field15">
            <span>{p22.store}</span>
            <input value={store} maxLength={80} placeholder={p22.storePlaceholder} onChange={(e) => setStore(e.target.value)} />
          </label>
          {stores.length > 0 && (
            <div className="chips22" role="group" aria-label={p22.store}>
              {stores.map((s) => (
                <button key={s} type="button" aria-pressed={store === s} onClick={() => setStore(s)}>{s}</button>
              ))}
            </div>
          )}
        </div>
      ) : (
        <div className="tsheet22">
          <div className="tsheet22__quick" role="radiogroup" aria-label={p22.dateSheet}>
            {quickDates().map((q) => (
              <button key={q.k} type="button" role="radio" aria-checked={date === q.date} onClick={() => setDate(q.date)}>
                <span>{q.label}</span>
                <small>{formatDayMonth(q.date)}</small>
              </button>
            ))}
          </div>
          <div className="plan-form__row">
            <label className="field15">
              <span>{p22.otherDate}</span>
              <input type="date" value={date} min={quickDates()[0].date} onChange={(e) => setDate(e.target.value)} />
            </label>
            <label className="field15">
              <span>{p22.time}</span>
              <input type="time" value={time} onChange={(e) => e.target.value && setTime(e.target.value)} />
            </label>
          </div>
        </div>
      )}
      {save.isError && <p className="error" role="alert">{t.common.genericError}</p>}
      <button type="button" className="btn btn--primary btn--block btn--lg" aria-disabled={save.isPending || undefined} onClick={commit}>{p22.done}</button>
    </Sheet>
  );
}

/** Thêm một chạm các món sắp hết, có Hoàn tác (B2). */
function useAddLow(planId: string, plan: string) {
  const edit = usePlanItem();
  const showUndo = useUndo();
  const [busy, setBusy] = useState(false);
  const run = async (items: Item[]) => {
    if (busy || items.length === 0) return;
    setBusy(true);
    const added: string[] = [];
    try {
      for (const i of items) {
        await edit.mutateAsync({ key: i.key, add: true, planId });
        added.push(i.key);
      }
      markDraft(planId);
    } finally {
      setBusy(false);
      if (added.length)
        showUndo({ message: p22.addedLow(added.length, plan), onUndo: () => added.forEach((key) => edit.mutate({ key, add: false, planId })) });
    }
  };
  return { run, busy, error: edit.isError };
}

/**
 * Thẻ kế hoạch chưa lưu ở đầu màn Kế hoạch (T1 chốt 09/11): hiện sẵn đầy đủ như Redesign.
 * Cần "Kế hoạch đang xem" là planId (màn cha đặt trước khi vẽ).
 */
export function DraftCard({ meta }: { meta: PlanMeta }) {
  const id = meta.id;
  const plan = usePlan();
  const cart = plan.data;
  const nav = useNavigate();
  const market = useMarketPrices(!!cart && cart.item_count > 0);
  const low = useLowItems(cart);
  const name = planName(meta);
  const addLow = useAddLow(id, name);
  const metaAct = usePlanMeta();
  const [editName, setEditName] = useState(false);
  const [draftName, setDraftName] = useState(name);
  const step = stepOf(cart, false);
  const lines = priceLines(cart, market.data?.items);
  const sum = summary(lines);
  const go = (path: string) => nav(`/plan/${encodeURIComponent(id)}${path}`);
  if (!cart) return <section className="box15 draft22" aria-busy="true"><p className="hint">{t.common.loading}</p></section>;
  return (
    <section className="box15 draft22" aria-labelledby="draft22-name">
      <div className="draft22__head">
        <div className="draft22__title">
          {editName && !meta.default ? (
            <input
              className="draft22__input"
              value={draftName}
              maxLength={40}
              autoFocus
              aria-label={m15.plan.name}
              onChange={(e) => setDraftName(e.target.value)}
              onBlur={() => {
                setEditName(false);
                const n = draftName.trim();
                if (n && n !== meta.name) metaAct.update.mutate({ id, name: n });
                else setDraftName(name);
              }}
            />
          ) : (
            <h2 id="draft22-name">
              {name}
              {!meta.default && (
                <button type="button" className="icon-btn draft22__pen" aria-label={m15.plan.editName} onClick={() => setEditName(true)}>
                  <Pencil size={18} aria-hidden />
                </button>
              )}
            </h2>
          )}
          <span className="tag15 tag15--draft">{p22.draft}</span>
        </div>
        <p className="draft22__step">{p22.stepOf(step, p22.steps[step - 1])}</p>
        <span className="draft22__art" aria-hidden><ShoppingCart size={26} /></span>
      </div>
      <Stepper step={step} />

      {cart.item_count === 0 ? (
        <div className="draft22__start">
          <span className="draft22__startIcon" aria-hidden><ShoppingCart size={22} /></span>
          <span>
            <b>{p22.startTitle}</b>
            <small>{p22.startSub(low.length)}</small>
          </span>
        </div>
      ) : (
        <>
          <div className="sum22">
            <div className="sum22__count"><b className="num">{cart.item_count}</b><small>{p22.items}</small></div>
            <div className="sum22__main">
              <b className="num">{sum.total != null ? `≈ ${formatVnd(sum.total)}` : "–"}</b>
              {sum.total != null && <span className="sum22__tag"><TrendingDown size={14} aria-hidden /> {p22.lowestFound}</span>}
            </div>
            {sum.save > 0 && <span className="sum22__save">{p22.save(formatVnd(sum.save))}</span>}
          </div>
          <ul className="pl22">
            {lines.slice(0, 4).map((l) => {
              const cut = l.old != null && l.best != null && l.best < l.old;
              const pct = cut ? Math.round(((l.old! - l.best!) / l.old!) * 100) : 0;
              return (
                <li key={l.item.key}>
                  <button type="button" className="pl22__row" onClick={() => go("/compare")}>
                    <CategoryIcon category={l.item.category_key} />
                    <span className="pl22__name">
                      <b>{l.item.name}</b>
                      <small>{[`${l.item.qty} ${unitLabel(l.item.unit || savedUnit({ key: l.item.key, name: l.item.name }))}`, l.bestStore].filter(Boolean).join(" · ")}</small>
                    </span>
                    <span className="pl22__price">
                      {cut && <s className="num">{formatVnd(l.old!)}</s>}
                      <b className="num">{l.best != null ? formatVnd(l.best) : "–"}</b>
                    </span>
                    {pct > 0 && <span className="pl22__pct">-{pct}%</span>}
                    <ChevronRight size={18} aria-hidden />
                  </button>
                </li>
              );
            })}
          </ul>
          {lines.length > 4 && (
            <button type="button" className="link15 pl22__all" onClick={() => go("/compare")}>
              {p22.viewAll(lines.length)} <ChevronRight size={16} aria-hidden />
            </button>
          )}
        </>
      )}

      <TripRows planId={id} cart={cart} />

      {cart.item_count === 0 ? (
        <div className="draft22__btns">
          {low.length > 0 && (
            <button type="button" className="btn btn--primary btn--block btn--lg" aria-disabled={addLow.busy || undefined} onClick={() => addLow.run(low)}>
              <Plus size={20} aria-hidden /> {p22.addLow(low.length)}
            </button>
          )}
          <button type="button" className="btn btn--outline btn--block btn--lg" onClick={() => go("/add")}>
            <Pencil size={18} aria-hidden /> {p22.chooseSelf}
          </button>
        </div>
      ) : (
        <div className="draft22__btns">
          <button type="button" className="btn btn--primary btn--block btn--lg" onClick={() => go("/compare")}>
            <Scale size={20} aria-hidden /> {p22.compare} <ChevronRight size={18} aria-hidden />
          </button>
          <button type="button" className="btn btn--outline btn--block btn--lg" onClick={() => go("/add")}>
            <Pencil size={18} aria-hidden /> {p22.editItems}
          </button>
        </div>
      )}
      {addLow.error && <p className="error" role="alert">{t.common.genericError}</p>}
    </section>
  );
}

/** Gợi ý cho kế hoạch (B5): lưới 2 cột, tối đa 2 thẻ, món còn 14 ngày trở xuống. */
export function SuggestGrid({ meta }: { meta: PlanMeta }) {
  const plan = usePlan();
  const cart = plan.data;
  const low = useLowItems(cart, 14);
  const edit = usePlanItem();
  const showUndo = useUndo();
  const nav = useNavigate();
  if (!cart || cart.item_count === 0 || low.length === 0) return null;
  return (
    <section className="sg22" aria-labelledby="sg22-h">
      <div className="sg22__head">
        <h2 id="sg22-h" className="h15sec">{p22.suggested}</h2>
        <button type="button" className="link15" onClick={() => nav(`/plan/${encodeURIComponent(meta.id)}/add`)}>
          {p22.seeAll} <ChevronRight size={16} aria-hidden />
        </button>
      </div>
      <ul className="sg22__grid">
        {low.slice(0, 2).map((i) => (
          <li key={i.key} className="sg22__card" data-cat={i.category_key.split(".")[0]}>
            <CategoryIcon category={i.category_key} />
            <b>{i.name}</b>
            <small><CalendarDays size={14} aria-hidden /> {i.days_left != null ? p22.daysLeft(i.days_left) : m15.plan.learning}</small>
            <button
              type="button"
              className="btn sg22__add"
              aria-disabled={(edit.isPending && edit.variables?.key === i.key) || undefined}
              onClick={() =>
                edit.mutate(
                  { key: i.key, add: true, planId: meta.id },
                  {
                    onSuccess: () => {
                      markDraft(meta.id);
                      showUndo({ message: m15.plan.addedTo(i.name, planName(meta)), onUndo: () => edit.mutate({ key: i.key, add: false, planId: meta.id }) });
                    },
                  },
                )
              }
            >
              <Plus size={18} aria-hidden /> {p22.add}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
