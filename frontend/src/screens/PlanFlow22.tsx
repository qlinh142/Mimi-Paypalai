import { useLayoutEffect, useMemo, useState, type ReactNode } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { Check, ChevronLeft, ChevronRight, CircleCheckBig, CirclePlus, Globe, House, Info, Plus, ReceiptText, Search, TrendingDown, TrendingUp, RefreshCw, Store, UserRound } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { m1 } from "../i18n/m1";
import { apiSend } from "../api/client";
import { planQuery } from "../lib/currentPlan";
import { iconOf, planIcon } from "../lib/planIcon";
import { Sheet } from "../components/Sheet15";
import { AddToCalendar } from "../components/AddToCalendar";
import { IconSheet, Lines } from "./PlanDetail";
import { p22 } from "../i18n/p22";
import { m15 } from "../i18n/m15";
import { t } from "../i18n";
import { ApiError } from "../api/client";
import {
  useDelegated, useHousehold, useMarketPrices, useMe, usePlan, usePlanItem, usePlanMeta, usePlans, useRefreshMarketPrices,
} from "../api/hooks";
import type { Cart, CartItem, MarketPrice, PlanMeta } from "../api/types";
import { MainFrame } from "../components/AppShell";
import { AddAnything } from "../components/PlanExtras";
import { Stepper, TripRows, useLowItems } from "../components/PlanDraft22";
import { LoadState } from "../components/LoadState";
import { setCurrentPlanId, useCurrentPlanId } from "../lib/currentPlan";
import { buyerOf, clearDraft, draftName, markDraft, setBuyer, setDraftName, usePlanDraftVersion } from "../lib/planDraft";
import { useRelations } from "../lib/relation";
import { formatDayMonth, formatVnd } from "../lib/text";
import { planName } from "./PlanList";

const errText = (e: unknown) => (e instanceof ApiError ? e.message : t.common.genericError);

/* ---------- Giá đã chọn cho từng món (T3): lưu trên máy như bản nháp ---------- */
const PICK = (id: string) => `mimi.planPick.${id}`;
function readPicks(id: string): Record<string, string> {
  try {
    return JSON.parse(localStorage.getItem(PICK(id)) ?? "{}") as Record<string, string>;
  } catch {
    return {};
  }
}
function writePicks(id: string, v: Record<string, string>) {
  try {
    localStorage.setItem(PICK(id), JSON.stringify(v));
  } catch {
    // không lưu được thì chỉ nhớ trên màn này
  }
}

export type PriceOption = { id: string; store: string; sub: string; total: number; url?: string; receipt: boolean; online: boolean; price?: MarketPrice };

/** Channel3 là nguồn duy nhất trả online (PayPal, thẻ); hóa đơn cũ và Gemini là giá mua tại cửa hàng (anh chốt 09/11). */
export const isOnlineSource = (source?: string) => source === "channel3";

/** Các lựa chọn giá của một món: giá rẻ nhất của từng nguồn (hóa đơn nhà mình, Gemini, Channel3), rẻ nhất trước. */
export function optionsOf(item: CartItem, prices: MarketPrice[]): PriceOption[] {
  const out: PriceOption[] = [];
  if (item.unit_price != null)
    out.push({ id: "receipt", store: item.best_store || p22.pastReceipt(""), sub: `${p22.pastReceipt("")} · ${p22.atStore}`, total: item.unit_price * item.qty, receipt: true, online: false });
  const best = new Map<string, MarketPrice>();
  for (const p of prices) {
    const old = best.get(p.source);
    if (!old || p.price_vnd < old.price_vnd) best.set(p.source, p);
  }
  for (const p of best.values()) {
    const online = isOnlineSource(p.source);
    out.push({
      id: `${p.source}:${p.store}`,
      store: p.store,
      sub: [p22.src(p.source), online ? p22.onlineBuy : p22.atStore, formatDayMonth(p.on)].filter(Boolean).join(" · "),
      total: p.price_vnd * item.qty,
      url: p.url,
      receipt: false,
      online,
      price: p,
    });
  }
  return out.sort((a, b) => a.total - b.total);
}

function chosen(item: CartItem, opts: PriceOption[], picks: Record<string, string>): PriceOption | undefined {
  return opts.find((o) => o.id === picks[item.key]) ?? opts[0];
}

/** Khung chung của luồng 3 bước: đặt Kế hoạch đang xem theo đường dẫn, ẩn thanh tab, nút quay lại. */
function Flow({ step, title, back, children, bar }: { step: 1 | 2 | 3; title: string; back: string; children: (id: string, cart: Cart) => ReactNode; bar?: (id: string, cart: Cart) => ReactNode }) {
  const { id = "default" } = useParams();
  const current = useCurrentPlanId();
  useLayoutEffect(() => {
    if (current !== id) setCurrentPlanId(id);
  }, [current, id]);
  const plan = usePlan();
  const nav = useNavigate();
  usePlanDraftVersion();
  if (current !== id) return null;
  const cart = plan.data;
  return (
    <MainFrame nav={false}>
      <div className="page p15 flow22">
        <div className="flow22__head">
          <button type="button" className="pd15__back" aria-label={m15.detail.back} onClick={() => nav(back)}>
            <ChevronLeft size={28} aria-hidden />
          </button>
          <h1>{title}</h1>
        </div>
        <Stepper step={step} />
        <LoadState q={plan} hasData={!!cart} />
        {cart && children(id, cart)}
      </div>
      {cart && bar && <div className="flow22__bar">{bar(id, cart)}</div>}
    </MainFrame>
  );
}

/* ---------- Bước 1: Thêm món và thông tin kế hoạch (anh chốt 09/11) ---------- */
export function PlanAdd() {
  const nav = useNavigate();
  return (
    <Flow
      step={1}
      title={p22.addTitle}
      back="/plan"
      bar={(id, cart) => <NextButton id={id} cart={cart} onNext={() => nav(`/plan/${encodeURIComponent(id)}/compare`)} />}
    >
      {(id, cart) => <Step1Body id={id} cart={cart} />}
    </Flow>
  );
}

/** Nút Tiếp tục của bước 1 (chưa có món thì chưa bấm được). */
export function NextButton({ id, cart, onNext }: { id: string; cart: Cart; onNext?: () => void }) {
  const nav = useNavigate();
  const go = onNext ?? (() => nav(`/plan/${encodeURIComponent(id)}/compare`));
  return (
    <button
      type="button"
      className="btn btn--primary btn--block btn--lg"
      aria-disabled={cart.item_count === 0 || undefined}
      onClick={() => cart.item_count > 0 && go()}
    >
      {p22.next} <ChevronRight size={18} aria-hidden />
    </button>
  );
}

/** Nội dung bước 1 (Thêm món): dùng ở màn bước 1 và ở thẻ kế hoạch chưa lưu đầu tab Của tôi. */
export function Step1Body({ id, cart }: { id: string; cart: Cart }) {
  const nav = useNavigate();
  const plans = usePlans();
  const metaAct = usePlanMeta();
  const [sheet, setSheet] = useState<null | "menu" | "product">(null);
  const [iconSheet, setIconSheet] = useState(false);
  const [name, setName] = useState<string | null>(null);
  const meta = plans.data?.find((p) => p.id === id);
  const Icon = iconOf(planIcon(meta));
  const cur = name ?? (meta ? planName(meta) : "");
  const st = cart.state;
  const time = st.buy_hour != null ? `${String(st.buy_hour).padStart(2, "0")}:${String(st.buy_minute ?? 0).padStart(2, "0")}` : "";
  return (
    <>
      <section className="box15 info22" aria-label={p22.planInfo}>
        <div className="info22__name">
          <button type="button" className="pd15__icon" aria-label={m15.plan.pickIcon} onClick={() => setIconSheet(true)}>
            <Icon size={26} aria-hidden />
          </button>
          <input
            className="draft22__input"
            value={cur}
            maxLength={40}
            aria-label={m15.plan.name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
            onBlur={() => {
              const n = cur.trim();
              // Giỏ mua sắm (mặc định) không đổi tên trên máy chủ: tên giữ ở nháp, lưu ở bước 3 thì thành tên kế hoạch mới.
              if (meta?.default) {
                setDraftName(id, n && n !== m15.plan.main ? n : null);
                markDraft(id);
              } else if (meta && n && n !== meta.name) metaAct.update.mutate({ id, name: n }, { onSuccess: () => markDraft(id) });
              setName(null);
            }}
          />
          <span className="tag15 tag15--draft">{p22.draft}</span>
        </div>
        <TripRows planId={id} cart={cart} />
        {st.buy_date && (
          <AddToCalendar
            ev={{
              title: m15.plan.calTitle(cur),
              date: st.buy_date,
              time,
              location: st.trip_note,
              details: m15.plan.calDetails(cart.items.slice(0, 20).map((i) => i.name)),
            }}
          />
        )}
      </section>

      {cart.items.length > 0 ? (
        <Lines cart={cart} onChange={() => markDraft(id)} done={new Set()} />
      ) : (
        <>
          <p className="hint">{p22.noItems}</p>
          <LowQuick planId={id} cart={cart} />
        </>
      )}

      <button type="button" className="btn btn--outline btn--block btn--lg" onClick={() => setSheet("menu")}>
        <Plus size={20} aria-hidden /> {p22.addSheet}
      </button>

      {sheet === "menu" && (
        <Sheet title={p22.addSheet} sub={p22.addSheetSub} onClose={() => setSheet(null)}>
          <div className="menu22">
            <button type="button" className="menu22__row" onClick={() => nav("/items")}>
              <House size={22} aria-hidden />
              <span><b>{p22.checkItems}</b><small>{p22.checkItemsSub}</small></span>
              <ChevronRight size={18} aria-hidden />
            </button>
            <button type="button" className="menu22__row" onClick={() => setSheet("product")}>
              <CirclePlus size={22} aria-hidden />
              <span><b>{p22.addProduct}</b><small>{p22.addProductSub}</small></span>
              <ChevronRight size={18} aria-hidden />
            </button>
          </div>
        </Sheet>
      )}
      {sheet === "product" && (
        <Sheet title={p22.addProduct} sub={p22.addProductSub} onClose={() => setSheet(null)}>
          <AddAnything cart={cart} chips onAdded={() => markDraft(id)} />
          <LowQuick planId={id} cart={cart} />
          <div className="done22">
            <button type="button" className="btn btn--primary btn--block btn--lg" onClick={() => setSheet(null)}>
              <Check size={20} aria-hidden /> {p22.done}
            </button>
          </div>
        </Sheet>
      )}
      {iconSheet && meta && (
        <IconSheet
          value={planIcon(meta)}
          onCancel={() => setIconSheet(false)}
          onSave={(k) => {
            metaAct.update.mutate({ id, icon: k });
            setIconSheet(false);
            markDraft(id);
          }}
        />
      )}
    </>
  );
}

/** Thêm nhanh món sắp hết: danh sách món Cần mua ngay và Sắp hết, thêm từng món hoặc cả danh sách. */
function LowQuick({ planId, cart }: { planId: string; cart: Cart }) {
  const low = useLowItems(cart);
  const edit = usePlanItem();
  const [busy, setBusy] = useState(false);
  if (low.length === 0) return null;
  const add = (key: string) => edit.mutateAsync({ key, add: true, planId }).then(() => markDraft(planId));
  return (
    <section className="lowq22" aria-labelledby="lowq22-h">
      <div className="sg22__head">
        <h3 id="lowq22-h" className="h15sec">{p22.lowList}</h3>
        <button
          type="button"
          className="link15"
          aria-disabled={busy || undefined}
          onClick={async () => {
            if (busy) return;
            setBusy(true);
            try {
              for (const i of low) await add(i.key);
            } finally {
              setBusy(false);
            }
          }}
        >
          {p22.addAll(low.length)}
        </button>
      </div>
      <div className="chips22" role="group" aria-labelledby="lowq22-h">
        {low.map((i) => (
          <button
            key={i.key}
            type="button"
            className="chip22"
            aria-label={`${p22.add} ${i.name}`}
            aria-disabled={busy || (edit.isPending && edit.variables?.key === i.key) || undefined}
            onClick={() => !busy && add(i.key)}
          >
            {i.name}
            {i.days_left != null && <small>{p22.daysLeft(i.days_left)}</small>}
          </button>
        ))}
      </div>
    </section>
  );
}

/* ---------- Bước 2: So sánh giá ---------- */
export function PlanCompare() {
  const nav = useNavigate();
  const market = useMarketPrices(true);
  const refresh = useRefreshMarketPrices();
  const { id: routeId = "default" } = useParams();
  const [picks, setPicks] = useState<Record<string, string>>(() => readPicks(routeId));
  const [detail, setDetail] = useState<{ item: CartItem; opt: PriceOption; opts: PriceOption[]; pick: (o: PriceOption) => void } | null>(null);
  const byKey = useMemo(() => new Map((market.data?.items ?? []).map((m) => [m.key, m.prices])), [market.data]);
  const totalOf = (cart: Cart) => cart.items.reduce((s, i) => s + (chosen(i, optionsOf(i, byKey.get(i.key) ?? []), picks)?.total ?? 0), 0);
  const st = refresh.data?.refresh ?? market.data?.refresh;
  return (
    <>
    <Flow
      step={2}
      title={p22.compareTitle}
      back="/plan"
      bar={(id, cart) => (
        <>
          <div className="flow22__sum">
            <span>
              <small>{p22.total}</small>
              <b className="num">{formatVnd(totalOf(cart))}</b>
            </span>
          </div>
          <div className="flow22__two">
            <button type="button" className="btn btn--outline btn--lg" onClick={() => nav(`/plan/${encodeURIComponent(id)}/add`)}>
              <ChevronLeft size={18} aria-hidden /> {p22.back}
            </button>
            <button type="button" className="btn btn--primary btn--lg" onClick={() => nav(`/plan/${encodeURIComponent(id)}/confirm`)}>
              {p22.next} <ChevronRight size={18} aria-hidden />
            </button>
          </div>
        </>
      )}
    >
      {(id, cart) => (
        <>
          {/* Anh chốt 09/11: bỏ hai tab Theo từng món / Theo cửa hàng, chỉ còn chọn giá từng món. */}
          {(
            <>
              <p className="hint">{p22.compareHint}</p>
              {/* Nút Cập nhật giá của 2.1 (anh chốt 09/11): tra lại giá ngay, giới hạn số lần mỗi ngày. */}
              <button type="button" className="btn btn--outline btn--block" aria-disabled={refresh.isPending || undefined} onClick={() => !refresh.isPending && refresh.mutate()}>
                <RefreshCw size={18} aria-hidden className={refresh.isPending ? "spin" : undefined} /> {refresh.isPending ? m1.marketPrices.refreshing : m1.marketPrices.refresh}
              </button>
              <p className="hint" role="status">
                {refresh.isError ? errText(refresh.error) : refresh.data ? (refresh.data.looked > 0 ? m1.marketPrices.refreshed(refresh.data.looked, refresh.data.found) : m1.marketPrices.upToDate) : null}
                {st && <> {m1.marketPrices.refreshHint(st.left, st.daily)}</>}
              </p>
              {cart.items.map((item) => {
                const opts = optionsOf(item, byKey.get(item.key) ?? []);
                const cur = chosen(item, opts, picks);
                const pick = (o: PriceOption) => {
                  const next = { ...picks, [item.key]: o.id };
                  setPicks(next);
                  writePicks(id, next);
                  markDraft(id);
                };
                return (
                  <section key={item.key} className="box15 cmpi22" aria-label={item.name}>
                    <h2>{item.name}</h2>
                    {opts.length === 0 ? (
                      <p className="hint">{p22.noPrice}</p>
                    ) : (
                      <ul role="radiogroup" aria-label={item.name} className="cmpi22__list">
                        {opts.map((o, i) => (
                          <li key={o.id} className={cur?.id === o.id ? "cmpi22__opt cmpi22__opt--on" : "cmpi22__opt"}>
                            <button
                              type="button"
                              role="radio"
                              aria-checked={cur?.id === o.id}
                              className="cmpi22__pick"
                              onClick={() => pick(o)}
                            >
                              <span className="cmpi22__rad" aria-hidden />
                              <span className="cmpi22__text">
                                <b>
                                  {o.store}
                                  {i === 0 && opts.length > 1 && <span className="cmp22__tag">{p22.lowest}</span>}
                                </b>
                                <small>{o.sub}</small>
                              </span>
                              <span className="num cmpi22__price">{formatVnd(o.total)}</span>
                            </button>
                            <button type="button" className="cmpi22__more" aria-label={p22.detailOf(o.store)} onClick={() => setDetail({ item, opt: o, opts, pick })}>
                              <Info size={20} aria-hidden />
                            </button>
                          </li>
                        ))}
                      </ul>
                    )}
                  </section>
                );
              })}
            </>
          )}
        </>
      )}
    </Flow>
    {detail && (
      <PriceDetail
        item={detail.item}
        opt={detail.opt}
        opts={detail.opts}
        onClose={() => setDetail(null)}
        onChoose={() => {
          detail.pick(detail.opt);
          setDetail(null);
        }}
      />
    )}
    </>
  );
}


/** Chi tiết một lựa chọn giá, theo màn Product details của Redesign (nút Details tách khỏi việc chọn dòng). */
function PriceDetail({ item, opt, opts, onClose, onChoose }: { item: CartItem; opt: PriceOption; opts: PriceOption[]; onClose: () => void; onChoose: () => void }) {
  const receipt = opts.find((o) => o.receipt);
  const diff = receipt && !opt.receipt ? opt.total - receipt.total : null;
  const lowest = opts.length > 1 && opts[0].id === opt.id;
  const p = opt.price;
  const srcLabel = opt.receipt ? p22.pastReceipt("") : p22.src(p?.source ?? "");
  let domain = "";
  try {
    domain = opt.url ? new URL(opt.url).hostname.replace(/^www\./, "") : "";
  } catch {
    domain = "";
  }
  const rows: [string, ReactNode][] = [
    [p22.det.seller, opt.store],
    [p22.det.source, srcLabel],
    ...(domain ? [[p22.det.link, <a key="l" href={opt.url} target="_blank" rel="noopener noreferrer">{domain}</a>] as [string, ReactNode]] : []),
    ...(p?.on ? [[p22.det.checked, formatDayMonth(p.on)] as [string, ReactNode]] : []),
    ...(p?.local != null && p.currency && p.currency !== "VND" ? [[p22.det.local, `${p.local} ${p.currency}`] as [string, ReactNode]] : []),
  ];
  return (
    <Sheet title={p22.det.title} onClose={onClose}>
      <div className="det22">
        <span className="det22__src">
          {opt.receipt ? <ReceiptText size={18} aria-hidden /> : <Search size={18} aria-hidden />} {srcLabel}
        </span>
        <div>
          <h3 className="det22__name">{item.name}</h3>
          <p className="det22__size">{[p?.size, `${p22.det.qty}: ${item.qty}${item.unit ? " " + item.unit : ""}`].filter(Boolean).join(" · ")}</p>
        </div>
        <div className="det22__card det22__hero">
          <p className="det22__price num">{formatVnd(opt.total)}</p>
          <p className="det22__where">{lowest ? p22.det.whereLowest(opt.store) : opt.store}</p>
          {diff != null && diff !== 0 && (
            <span className={diff < 0 ? "det22__diff det22__diff--down" : "det22__diff det22__diff--up"}>
              {diff < 0 ? <TrendingDown size={18} aria-hidden /> : <TrendingUp size={18} aria-hidden />}
              {diff < 0 ? p22.det.cheaper(formatVnd(-diff)) : p22.det.dearer(formatVnd(diff))}
            </span>
          )}
          {diff === 0 && <span className="det22__diff">{p22.det.same}</span>}
        </div>
        <dl className="det22__card det22__table">
          {rows.map(([k, v]) => (
            <div key={k}>
              <dt>{k}</dt>
              <dd>{v}</dd>
            </div>
          ))}
        </dl>
        {!opt.receipt && <p className="hint">{p22.det.note}</p>}
        {opt.url && (
          <a className="det22__card det22__open" href={opt.url} target="_blank" rel="noopener noreferrer">
            <span className="det22__globe"><Globe size={22} aria-hidden /></span>
            <b>{p22.det.open}</b>
            <ChevronRight size={20} aria-hidden />
          </a>
        )}
        {opt.receipt && (
          <Link className="det22__card det22__open" to={`/items/${encodeURIComponent(item.key)}`}>
            <span className="det22__globe"><ReceiptText size={22} aria-hidden /></span>
            <b>{p22.det.history}</b>
            <ChevronRight size={20} aria-hidden />
          </Link>
        )}
        <button type="button" className="btn btn--primary btn--block btn--lg" onClick={onChoose}>
          {p22.det.choose}
        </button>
      </div>
    </Sheet>
  );
}

/* ---------- Bước 3: Mua. Có cả giá online và giá cửa hàng thì tách 2 kế hoạch (anh chốt 09/11) ---------- */

/** Chuyển các món sang một kế hoạch đặt tên mới (giữ số lượng, đơn vị, ngày giờ, nơi mua). */
async function moveToNewPlan(from: string, name: string, items: CartItem[], state: Cart["state"]): Promise<string> {
  const p = await apiSend<PlanMeta>("POST", "/plans", { name: name.slice(0, 40) });
  for (const i of items) {
    await apiSend("PUT", `/plan/items/${encodeURIComponent(i.key)}${planQuery(p.id)}`, { name: i.name, qty: i.qty, unit: i.unit, category_key: i.category_key });
    await apiSend("DELETE", `/plan/items/${encodeURIComponent(i.key)}${planQuery(from)}`);
  }
  if (state.buy_date || state.trip_note)
    await apiSend("PUT", `/plan/state${planQuery(p.id)}`, { buy_date: state.buy_date, buy_hour: state.buy_hour, buy_minute: state.buy_minute, trip_note: state.trip_note, base_rev: 0 });
  return p.id;
}

export function PlanConfirm() {
  const nav = useNavigate();
  const qc = useQueryClient();
  const me = useMe();
  const h = useHousehold();
  const { relationOf } = useRelations();
  const market = useMarketPrices(true);
  const plans = usePlans();
  const delegated = useDelegated();
  const { id: routeId = "default" } = useParams();
  const sentTo = (delegated.data?.from_me ?? []).find((a) => a.plan_id === routeId && (a.status === "assigned" || a.status === "shopping"));
  const saved = buyerOf(routeId) ?? (sentTo ? sentTo.buyer_uid : null);
  const [pick, setPick] = useState<string | null>(saved === me.data?.uid ? "me" : saved);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const picks = readPicks(routeId);
  const byKey = new Map((market.data?.items ?? []).map((m) => [m.key, m.prices]));
  const others = (h.data?.members ?? []).filter((m) => m.uid !== me.data?.uid);
  const memberName = (uid: string) => others.find((m) => m.uid === uid)?.display_name ?? sentTo?.buyer_display_name ?? "";
  const meta = plans.data?.find((p) => p.id === routeId);

  const split = (cart: Cart) => {
    const online: { item: CartItem; o: PriceOption }[] = [];
    const store: { item: CartItem; o?: PriceOption }[] = [];
    for (const item of cart.items) {
      const o = chosen(item, optionsOf(item, byKey.get(item.key) ?? []), picks);
      if (o && o.online) online.push({ item, o });
      else store.push({ item, o });
    }
    return { online, store };
  };

  const submit = async (id: string, cart: Cart) => {
    const { online, store } = split(cart);
    if (busy || (store.length > 0 && !pick)) return;
    setBusy(true);
    setErr(null);
    try {
      const base = meta ? planName(meta) : m15.plan.main;
      const stamp = formatDayMonth(new Date().toISOString().slice(0, 10));
      // Đã tự đặt tên cho Giỏ mua sắm thì dùng đúng tên đó, không thêm ngày.
      const named = (n: string) => (meta?.default && draftName(id) ? n : `${n} ${stamp}`);
      // Giỏ mua sắm (mặc định) là chỗ nháp: lưu thì chuyển món sang một kế hoạch đặt tên để giữ lại trong Tất cả đơn.
      let storeId = id;
      let onlineId: string | null = null;
      if (online.length > 0 && store.length > 0) {
        onlineId = await moveToNewPlan(id, p22.onlineName(base), online.map((x) => x.item), cart.state);
        if (meta?.default) storeId = await moveToNewPlan(id, named(base), store.map((x) => x.item), cart.state);
      } else if (online.length > 0) {
        onlineId = meta?.default ? await moveToNewPlan(id, named(base), online.map((x) => x.item), cart.state) : id;
      } else if (meta?.default) {
        storeId = await moveToNewPlan(id, named(base), store.map((x) => x.item), cart.state);
      }
      clearDraft(id);
      if (meta?.default) setDraftName(id, null);
      let who = "";
      if (store.length > 0 && pick) {
        setBuyer(storeId, pick);
        clearDraft(storeId);
        if (pick !== "me") {
          await apiSend("POST", "/households/shopping-assignments", { buyer_uid: pick, keys: store.map((x) => x.item.key), plan_id: storeId });
          who = memberName(pick);
        }
      }
      if (onlineId) {
        setBuyer(onlineId, "me");
        clearDraft(onlineId);
      }
      await qc.invalidateQueries({ queryKey: ["plan"] });
      await qc.invalidateQueries({ queryKey: ["plans"] });
      const sum = (xs: { o?: PriceOption }[]) => xs.reduce((s, x) => s + (x.o?.total ?? 0), 0);
      if (store.length === 0 && onlineId) {
        setCurrentPlanId(onlineId);
        return nav("/plan/online");
      }
      nav(`/plan/${encodeURIComponent(storeId)}/done`, {
        replace: true,
        state: { who, count: store.length, total: sum(store), name: base, onlineId, onlineCount: online.length, onlineTotal: sum(online) },
      });
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Flow
      step={3}
      title={p22.confirmTitle}
      back="/plan"
      bar={(id, cart) => {
        const { online, store } = split(cart);
        const label =
          online.length > 0 && store.length > 0 ? p22.splitSave : online.length > 0 ? p22.payOnline : pick && pick !== "me" ? p22.sendTo(memberName(pick)) : p22.savePlan;
        return (
          <div className="flow22__two">
            <button type="button" className="btn btn--outline btn--lg" onClick={() => nav(`/plan/${encodeURIComponent(id)}/compare`)}>
              <ChevronLeft size={18} aria-hidden /> {p22.back}
            </button>
            <button type="button" className="btn btn--primary btn--lg" aria-disabled={(store.length > 0 && !pick) || busy || undefined} onClick={() => submit(id, cart)}>
              {busy ? t.confirm.saving : label}
            </button>
          </div>
        );
      }}
    >
      {(_id, cart) => {
        const { online, store } = split(cart);
        const group = (title: string, sub: string, rows: { item: CartItem; o?: PriceOption }[]) => (
          <>
            <h2 className="h15sec">{title} <small className="conf22__sub">{sub}</small></h2>
            <ul className="box15 conf22">
              {rows.map(({ item, o }) => (
                <li key={item.key}>
                  <span>
                    <b>{item.name}</b>
                    <small>{o ? o.store : p22.noPrice}</small>
                  </span>
                  <span className="num">{o ? formatVnd(o.total) : "–"}</span>
                </li>
              ))}
              <li className="conf22__total">
                <span><b>{p22.total}</b></span>
                <span className="num">{formatVnd(rows.reduce((s, r) => s + (r.o?.total ?? 0), 0))}</span>
              </li>
            </ul>
          </>
        );
        return (
          <>
            {online.length > 0 && store.length > 0 && <p className="sent15" role="status">{p22.splitNote}</p>}
            {online.length > 0 && group(p22.payOnline, p22.payOnlineSub, online)}
            {store.length > 0 && (
              <>
                {group(p22.inStore, p22.inStoreSub, store)}
                <h2 className="h15sec">{p22.whoBuys}</h2>
                <div className="box15 who22" role="radiogroup" aria-label={p22.whoBuys}>
                  <WhoRow on={pick === "me"} Icon={UserRound} title={p22.me} sub={p22.meSub} onPick={() => setPick("me")} />
                  {others.map((m) => {
                    const rel = relationOf(m.uid);
                    return (
                      <WhoRow
                        key={m.uid}
                        on={pick === m.uid}
                        Icon={UserRound}
                        title={m.display_name || m15.plan.sendMember}
                        sub={p22.member(rel ? m15.settings.relations[rel as keyof typeof m15.settings.relations] : "")}
                        onPick={() => setPick(m.uid)}
                      />
                    );
                  })}
                </div>
                {!pick && <p className="hint" role="status">{p22.pickBuyer}</p>}
              </>
            )}
            {err != null && <p className="error" role="alert">{errText(err)}</p>}
          </>
        );
      }}
    </Flow>
  );
}

function WhoRow({ on, Icon, title, sub, onPick }: { on: boolean; Icon: typeof Store; title: string; sub: string; onPick: () => void }) {
  return (
    <button type="button" role="radio" aria-checked={on} className={on ? "who22__row who22__row--on" : "who22__row"} onClick={onPick}>
      <span className="cmpi22__rad" aria-hidden />
      <Icon size={20} aria-hidden />
      <span className="who22__text">
        <b>{title}</b>
        <small>{sub}</small>
      </span>
    </button>
  );
}

/* ---------- Màn kết quả (B7) ---------- */
export function PlanDone() {
  const { id = "default" } = useParams();
  const st = (useLocation().state ?? {}) as { who?: string; count?: number; total?: number; name?: string; onlineId?: string | null; onlineCount?: number; onlineTotal?: number };
  const nav = useNavigate();
  return (
    <MainFrame nav={false}>
      <div className="page p15 done22">
        <span className="done22__icon" aria-hidden><CircleCheckBig size={44} /></span>
        <h1>{st.onlineId ? p22.doneSplit : st.who ? p22.doneSent(st.who) : p22.doneSaved}</h1>
        {st.onlineId && <p className="hint">{st.who ? p22.doneSent(st.who) : p22.doneSaved}</p>}
        {st.name && <p className="done22__name">{st.name}</p>}
        {st.count != null && <p className="hint">{p22.doneSub(st.count, formatVnd(st.total ?? 0))}</p>}
        {st.onlineId && <p className="hint">{p22.doneOnline(st.onlineCount ?? 0, formatVnd(st.onlineTotal ?? 0))}</p>}
        <p className="hint">{p22.doneNote}</p>
        <div className="done22__btns">
          {st.onlineId && (
            <button type="button" className="btn btn--primary btn--block btn--lg" onClick={() => { setCurrentPlanId(st.onlineId!); nav("/plan/online"); }}>{p22.payNow}</button>
          )}
          <Link to="/plan" className={`btn btn--block btn--lg ${st.onlineId ? "btn--outline" : "btn--primary"}`}>{p22.backPlans}</Link>
          <Link to={`/plan/${encodeURIComponent(id)}`} className="btn btn--outline btn--block btn--lg">{p22.openPlan}</Link>
        </div>
      </div>
    </MainFrame>
  );
}
