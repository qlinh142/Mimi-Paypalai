import { useState } from "react";
import { Check, Plus, RefreshCw } from "lucide-react";
import { t } from "../i18n";
import { m1 } from "../i18n/m1";
import { ApiError } from "../api/client";
import {
  useAssignments, useBasketPrices, useDelegated, useHousehold, useMarketPrices, useMe, usePlanItem, useRefreshMarketPrices, usePlanMeta, usePlans, useTaxonomySearch,
} from "../api/hooks";
import type { Assignment, Cart, MarketPrice } from "../api/types";
import { DEFAULT_PLAN, setCurrentPlanId, useCurrentPlanId } from "../lib/currentPlan";
import { formatDateTime, formatDayMonth, formatVnd, normalizeVi } from "../lib/text";

/** Giá tiền gốc ngoài Việt Nam, ví dụ "4.50 USD". */
function formatLocal(amount: number, currency: string) {
  return `${amount.toFixed(2)} ${currency}`;
}

function errText(e: unknown) {
  return e instanceof ApiError ? e.message : t.common.genericError;
}

/** Chọn Kế hoạch đang xem, tạo Kế hoạch mới, lưu trữ / xóa Kế hoạch đặt tên (3.1 mục 3A). */
export function PlanSwitcher() {
  const plans = usePlans();
  const meta = usePlanMeta();
  const id = useCurrentPlanId();
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [renaming, setRenaming] = useState<string | null>(null); // tên đang sửa của Kế hoạch đặt tên
  const list = plans.data ?? [];
  const current = list.find((p) => p.id === id);
  // Kế hoạch đã xóa ở máy khác: quay về Kế hoạch chính.
  // Chờ danh sách tải xong: ngay sau khi tạo Kế hoạch mới, danh sách cũ chưa có nó.
  if (plans.data && !plans.isFetching && !current && id !== DEFAULT_PLAN) setCurrentPlanId(DEFAULT_PLAN);
  return (
    <section className="plan-sec" aria-label={m1.plans.label}>
      <div className="chips" role="tablist" aria-label={m1.plans.label}>
        {list.map((p) => (
          <button
            key={p.id}
            type="button"
            role="tab"
            className="chip"
            aria-selected={p.id === id}
            onClick={() => setCurrentPlanId(p.id)}
          >
            {p.default ? m1.plans.main : p.name}
            {p.item_count > 0 && ` · ${p.item_count}`}
            {p.status === "archived" && ` · ${m1.plans.archived}`}
          </button>
        ))}
        <button type="button" className="chip" onClick={() => setCreating(!creating)} aria-expanded={creating}>
          <Plus size={14} aria-hidden /> {m1.plans.new}
        </button>
      </div>
      {creating && (
        <form
          className="plan-form"
          onSubmit={(e) => {
            e.preventDefault();
            if (!name.trim() || meta.create.isPending) return;
            meta.create.mutate(name.trim(), {
              onSuccess: (p) => {
                setCurrentPlanId(p.id);
                setName("");
                setCreating(false);
              },
            });
          }}
        >
          <div className="plan-form__row">
            <input value={name} maxLength={40} placeholder={m1.plans.namePlaceholder} onChange={(e) => setName(e.target.value)} aria-label={m1.plans.new} />
            <button type="submit" className="btn btn--primary" aria-disabled={!name.trim() || undefined}>{m1.plans.create}</button>
          </div>
          {meta.create.isError && <p className="error" role="alert">{errText(meta.create.error)}</p>}
        </form>
      )}
      {current && !current.default && renaming !== null && (
        <form
          className="plan-form__row"
          onSubmit={(e) => {
            e.preventDefault();
            if (!renaming.trim() || meta.update.isPending) return;
            meta.update.mutate({ id: current.id, name: renaming.trim() }, { onSuccess: () => setRenaming(null) });
          }}
        >
          <input value={renaming} maxLength={40} aria-label={m1.planMore.rename} onChange={(e) => setRenaming(e.target.value)} />
          <button type="submit" className="btn btn--primary" aria-disabled={!renaming.trim() || undefined}>{m1.planMore.saveName}</button>
        </form>
      )}
      {meta.update.isError && <p className="error" role="alert">{errText(meta.update.error)}</p>}
      {current && !current.default && (
        <div className="plan-form__row">
          <button type="button" className="btn btn--quiet" onClick={() => setRenaming(renaming === null ? current.name : null)}>
            {m1.planMore.rename}
          </button>
          <button
            type="button"
            className="btn btn--quiet"
            onClick={() => meta.update.mutate({ id: current.id, status: current.status === "archived" ? "active" : "archived" })}
          >
            {current.status === "archived" ? m1.plans.restore : m1.plans.archive}
          </button>
          <button
            type="button"
            className="btn btn--quiet"
            onClick={() => meta.remove.mutate(current.id, { onSuccess: () => setCurrentPlanId(DEFAULT_PLAN) })}
          >
            {m1.plans.remove}
          </button>
        </div>
      )}
      {meta.remove.isError && <p className="error" role="alert">{errText(meta.remove.error)}</p>}
    </section>
  );
}

/** Thêm món chưa từng mua: chọn nhóm từ taxonomy 0.2 hoặc nhập tên tự do (3.1 mục 3A.4). */
export function AddAnything({ cart, chips, onAdded }: { cart: Cart; chips?: boolean; onAdded?: () => void }) {
  const [q, setQ] = useState("");
  const found = useTaxonomySearch(q);
  const edit = usePlanItem();
  const inCart = new Set(cart.items.map((i) => i.key));
  const slug = normalizeVi(q).replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 50);
  const add = (key: string, name?: string) =>
    edit.mutate({ key, add: true, body: name ? { name } : undefined }, { onSuccess: () => { if (!chips) setQ(""); onAdded?.(); } });
  if (chips)
    // Mimi 2.2: kết quả là hàng tên món nằm ngang, chạm tên là thêm, không có nút Thêm riêng.
    return (
      <section className="plan-sec" aria-labelledby="plan-add">
        <h2 id="plan-add">{m1.add.title}</h2>
        <input className="input" value={q} placeholder={m1.add.placeholder} onChange={(e) => setQ(e.target.value)} aria-label={m1.add.placeholder} />
        <div className="chips22" role="group" aria-label={m1.add.title}>
          {(found.data ?? []).map((g) => {
            const has = inCart.has(g.key);
            return (
              <button
                key={g.key}
                type="button"
                className={`chip22${has ? " chip22--on" : ""}`}
                aria-pressed={has}
                aria-disabled={has || edit.isPending || undefined}
                onClick={() => !has && !edit.isPending && add(g.key)}
              >
                {has && <Check size={16} aria-hidden />} {g.label}
              </button>
            );
          })}
          {slug.length >= 2 && (
            <button type="button" className="chip22 chip22--new" aria-disabled={edit.isPending || undefined} onClick={() => !edit.isPending && add(`custom-${slug}`, q.trim())}>
              <Plus size={16} aria-hidden /> {m1.add.custom(q.trim())}
            </button>
          )}
        </div>
        {edit.isError && <p className="error" role="alert">{errText(edit.error)}</p>}
      </section>
    );
  return (
    <section className="plan-sec" aria-labelledby="plan-add">
      <h2 id="plan-add">{m1.add.title}</h2>
      <input className="input" value={q} placeholder={m1.add.placeholder} onChange={(e) => setQ(e.target.value)} aria-label={m1.add.placeholder} />
      <ul className="list">
        {(found.data ?? []).map((g) => (
          <li key={g.key} className="plan-row add15__row">
            <span className="plan-row__text">
              <span className="plan-row__name">{g.label}</span>
            </span>
            <button
              type="button"
              className="btn btn--quiet add15__btn"
              aria-disabled={inCart.has(g.key) || edit.isPending || undefined}
              onClick={() => !inCart.has(g.key) && add(g.key)}
            >
              {inCart.has(g.key) ? m1.add.added : <><Plus size={18} aria-hidden /> {t.plan.add}</>}
            </button>
          </li>
        ))}
      </ul>
      {slug.length >= 2 && (
        <button type="button" className="btn btn--quiet btn--block" onClick={() => add(`custom-${slug}`, q.trim())}>
          {m1.add.custom(q.trim())}
        </button>
      )}
      {edit.isError && <p className="error" role="alert">{errText(edit.error)}</p>}
    </section>
  );
}

/** Gửi các món đang mở cho một thành viên cùng nhà (4.1 mục 9.1). */
export function AssignPanel({ cart }: { cart: Cart }) {
  const me = useMe();
  const h = useHousehold();
  const { assign } = useAssignments();
  const others = (h.data?.members ?? []).filter((m) => m.uid !== me.data?.uid);
  const open = cart.items.filter((i) => i.line_status !== "assigned");
  const [buyer, setBuyer] = useState("");
  const [picked, setPicked] = useState<Set<string>>(new Set());
  if (!h.data) return null;
  if (others.length === 0) return <p className="hint">{m1.assign.noHousehold}</p>;
  if (open.length === 0) return null;
  const toggle = (k: string) => {
    const n = new Set(picked);
    if (n.has(k)) n.delete(k);
    else n.add(k);
    setPicked(n);
  };
  return (
    <section className="plan-sec" aria-labelledby="plan-assign">
      <h2 id="plan-assign">{m1.assign.title}</h2>
      <form
        className="plan-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (!buyer || picked.size === 0 || assign.isPending) return;
          assign.mutate({ buyer_uid: buyer, keys: [...picked] }, { onSuccess: () => setPicked(new Set()) });
        }}
      >
        <label>
          {m1.assign.pick}
          <select value={buyer} onChange={(e) => setBuyer(e.target.value)}>
            <option value="">–</option>
            {others.map((m) => (
              <option key={m.uid} value={m.uid}>{m.display_name || m1.assign.someone}</option>
            ))}
          </select>
        </label>
        <fieldset>
          <legend>{m1.assign.choose}</legend>
          {open.map((i) => (
            <label key={i.key} className="switch-row">
              {i.name}
              <input type="checkbox" checked={picked.has(i.key)} onChange={() => toggle(i.key)} />
            </label>
          ))}
        </fieldset>
        {assign.isError && <p className="error" role="alert">{errText(assign.error)}</p>}
        {assign.isSuccess && <p className="hint">{m1.assign.sent}</p>}
        <button type="submit" className="btn btn--primary btn--block" aria-disabled={!buyer || picked.size === 0 || assign.isPending || undefined}>
          {m1.assign.send(picked.size)}
        </button>
      </form>
    </section>
  );
}

/** Vòng mua hộ được giao cho tôi và vòng tôi đã gửi (4.1 mục 6.2, 9.2). */
export function DelegatedPanel() {
  const d = useDelegated();
  const { cancel } = useAssignments();
  if (!d.data || (d.data.to_me.length === 0 && d.data.from_me.length === 0)) return null;
  const active = d.data.from_me.filter((a) => a.status === "assigned" || a.status === "shopping");
  return (
    <>
      {d.data.to_me.length > 0 && (
        <section className="plan-sec" aria-labelledby="plan-delegated">
          <h2 id="plan-delegated">{m1.delegated.title}</h2>
          {d.data.to_me.map((a) => (
            <div key={a.assignment_id} className="plan-stop">
              <div className="plan-stop__head">
                <span>{m1.delegated.from(a.owner_display_name || m1.assign.someone)}</span>
                <span>{m1.assign.status[a.status]}</span>
              </div>
              {a.trip && <TripLine trip={a.trip} />}
              <div className="plan-stop__items">
                {a.items.filter((i) => i.status === "assigned").map((i) => i.name_display).join(", ")}
              </div>
              {a.note && <p className="hint">{a.note}</p>}
            </div>
          ))}
          <p className="hint">{m1.delegated.scanHint}</p>
        </section>
      )}
      {active.length > 0 && (
        <section className="plan-sec" aria-labelledby="plan-from-me">
          <h2 id="plan-from-me">{m1.assign.fromMe}</h2>
          {active.map((a) => (
            <div key={a.assignment_id} className="plan-stop">
              <div className="plan-stop__head">
                <span>{a.buyer_display_name || m1.assign.someone}</span>
                <span>{m1.assign.status[a.status]}</span>
              </div>
              <div className="plan-stop__items">{a.items.map((i) => i.name_display).join(", ")}</div>
              <button type="button" className="btn btn--quiet" aria-disabled={cancel.isPending || undefined} onClick={() => cancel.mutate(a.assignment_id)}>
                {m1.assign.cancel}
              </button>
            </div>
          ))}
        </section>
      )}
    </>
  );
}

/** Ước tính giỏ theo lịch sử giá (8.1 mục 10 đến 13): độ phủ từng nơi, phương án chia nơi. */
export function BasketPricesPanel({ cart }: { cart: Cart }) {
  const open = cart.items.filter((i) => i.line_status !== "assigned");
  const bp = useBasketPrices(open.length > 0);
  if (!bp.data || open.length === 0) return null;
  const d = bp.data;
  const names = new Map(cart.items.map((i) => [i.key, i.name]));
  const label = (keys: string[]) => keys.map((k) => names.get(k) ?? k).join(", ");
  if (d.store_options.length === 0) return null;
  return (
    <section className="plan-sec" aria-labelledby="plan-history">
      <h2 id="plan-history">{m1.basketPrices.title}</h2>
      {d.store_options.slice(0, 4).map((o) => (
        <div key={o.store_key} className="plan-stop">
          <div className="plan-stop__head">
            <span>{o.store_name}</span>
            <span className="num">{o.estimated_total != null ? `~${formatVnd(o.estimated_total)}` : "–"}</span>
          </div>
          <div className="plan-stop__items">
            {m1.basketPrices.coverage(o.covered_items, o.total_items)}
            {o.estimated_total == null && ` · ${m1.basketPrices.notEnough}`}
          </div>
        </div>
      ))}
      {d.split_store_option && d.split_store_option.stores.length > 1 && (
        <div className="plan-stop">
          <div className="plan-stop__head">
            <span>{m1.basketPrices.split(d.split_store_option.stores.length)}</span>
            <span className="num">~{formatVnd(d.split_store_option.estimated_total)}</span>
          </div>
          {d.split_store_option.stores.map((s) => (
            <div key={s.store_name} className="plan-stop__items">{s.store_name}: {label(s.keys)}</div>
          ))}
        </div>
      )}
      {d.no_history.length > 0 && <p className="hint">{m1.basketPrices.noHistory(label(d.no_history))}</p>}
      <p className="hint">{m1.basketPrices.disclaimer}</p>
    </section>
  );
}

/** ADR-004: giá đang bán (tra trên mạng mỗi đêm), cạnh giá nhà mình đã trả. */
export function MarketPricesPanel({ cart }: { cart: Cart }) {
  const mp = useMarketPrices(cart.item_count > 0);
  const refresh = useRefreshMarketPrices();
  if (!mp.data || cart.item_count === 0) return null;
  const done = refresh.data;
  const st = done?.refresh ?? mp.data.refresh;
  // Mỗi món lấy giá thấp hơn giữa giá online và giá nhà mình từng mua (mẹ chốt 02/11): chưa có
  // giá online, hoặc online đắt hơn, thì hiện giá nhà mình. Giá nhà mình đọc từ lịch sử của chính
  // tài khoản, chỉ hiện ở đây, không ghi vào bảng giá chung.
  const market = new Map(mp.data.items.filter((i) => i.prices.length > 0).map((i) => [i.key, i]));
  type Row = { key: string; name: string; own: boolean; price: number; store?: string; online?: MarketPrice; more?: number };
  const rows = cart.items.flatMap((i): Row[] => {
    const m = market.get(i.key);
    const best = m?.prices[0];
    if (i.unit_price != null && (!best || i.unit_price < best.price_vnd)) {
      return [{ key: i.key, name: i.name, own: true, price: i.unit_price, store: i.best_store, online: best }];
    }
    return m && best ? [{ key: i.key, name: i.name, own: false, price: best.price_vnd, store: best.store, online: best, more: m.prices.length - 1 }] : [];
  });
  // Lần cập nhật giá gần nhất: mới nhất giữa lần tra đêm (updated_at của từng món) và lần bấm nút.
  const lastAt = [st?.last_at, ...mp.data.items.map((i) => i.updated_at)]
    .filter((t): t is string => !!t)
    .reduce<string | undefined>((a, t) => (!a || Date.parse(t) > Date.parse(a) ? t : a), undefined);
  return (
    <section className="plan-sec" aria-labelledby="plan-market">
      <h2 id="plan-market">{m1.marketPrices.title}</h2>
      {rows.map((r) => (
        <div key={r.key} className="plan-stop">
          <div className="plan-stop__head">
            <span>{r.name}</span>
            <span className="num">{formatVnd(r.price)}</span>
          </div>
          <div className="plan-stop__items">
            {r.own ? (
              <>
                {m1.marketPrices.ownPrice(r.store)}
                {` · ${r.online ? m1.marketPrices.onlineHigher(formatVnd(r.online.price_vnd), r.online.store) : m1.marketPrices.noOnline}`}
              </>
            ) : (
              <>
                {m1.marketPrices.cheapest(r.store ?? "")}
                {r.online?.local != null && r.online.currency && r.online.currency !== "VND" && ` · ${formatLocal(r.online.local, r.online.currency)}`}
                {r.online?.size && ` · ${r.online.size}`}
                {r.online && ` · ${formatDayMonth(r.online.on)}`}
                {!!r.more && ` · ${m1.marketPrices.more(r.more)}`}
              </>
            )}
          </div>
        </div>
      ))}
      {rows.length < cart.items.length && <p className="hint">{m1.marketPrices.noneYet}</p>}
      <button
        type="button"
        className="btn btn--quiet btn--block"
        aria-disabled={refresh.isPending || undefined}
        onClick={() => !refresh.isPending && refresh.mutate()}
      >
        <RefreshCw size={18} aria-hidden className={refresh.isPending ? "spin" : undefined} />{" "}
        {refresh.isPending ? m1.marketPrices.refreshing : m1.marketPrices.refresh}
      </button>
      <p className="hint" role="status">
        {refresh.isError
          ? errText(refresh.error)
          : done
            ? done.looked > 0 ? m1.marketPrices.refreshed(done.looked, done.found) : m1.marketPrices.upToDate
            : null}
        {st && <>{(refresh.isError || done) && <br />}{m1.marketPrices.refreshHint(st.left, st.daily)}</>}
      </p>
      <p className="hint">
        {lastAt && <>{m1.marketPrices.lastRefresh(formatDateTime(lastAt))}<br /></>}
        {m1.marketPrices.disclaimer}
      </p>
    </section>
  );
}

/** 3B.4: người được nhờ thấy tên, thời gian, ghi chú của Plan gốc (luôn là bản mới nhất). */
function TripLine({ trip }: { trip: NonNullable<Assignment["trip"]> }) {
  const time = trip.buy_hour != null ? `${String(trip.buy_hour).padStart(2, "0")}:${String(trip.buy_minute ?? 0).padStart(2, "0")}` : "";
  const when = [trip.buy_date ? formatDayMonth(trip.buy_date) : "", time].filter(Boolean).join(" · ");
  return (
    <p className="hint">
      {m1.planMore.tripFor(trip.plan_name || m1.plans.main)}
      {when && ` · ${when}`}
      {trip.trip_note && <><br />{trip.trip_note}</>}
    </p>
  );
}

/**
 * Nút "Thêm vào Kế hoạch" ở chi tiết món (3.1 mục 13.1 cách 2). Có nhiều Kế hoạch đang dùng
 * thì hỏi Kế hoạch đích (mục 3A.4); một Kế hoạch thì thêm thẳng.
 */
export function AddToPlanButton({ itemKey }: { itemKey: string }) {
  const plans = usePlans();
  const edit = usePlanItem();
  const [picking, setPicking] = useState(false);
  const [done, setDone] = useState("");
  const active = (plans.data ?? []).filter((p) => p.status === "active");
  const label = (p: { default: boolean; name: string }) => (p.default ? m1.plans.main : p.name);
  const add = (planId: string, name: string) =>
    edit.mutate({ key: itemKey, add: true, planId }, { onSuccess: () => { setDone(m1.planMore.addedTo(name)); setPicking(false); } });
  return (
    <div className="plan-form">
      <button
        type="button"
        className="btn btn--primary btn--block"
        aria-expanded={active.length > 1 ? picking : undefined}
        aria-disabled={edit.isPending || undefined}
        onClick={() => {
          if (edit.isPending) return;
          if (active.length > 1) setPicking(!picking);
          else add(active[0]?.id ?? DEFAULT_PLAN, active[0] ? label(active[0]) : m1.plans.main);
        }}
      >
        <Plus size={18} aria-hidden /> {m1.planMore.addToPlan}
      </button>
      {picking && (
        <fieldset>
          <legend>{m1.planMore.pickPlan}</legend>
          {active.map((p) => (
            <button key={p.id} type="button" className="btn btn--quiet btn--block" onClick={() => add(p.id, label(p))}>
              {label(p)}
            </button>
          ))}
        </fieldset>
      )}
      {done && <p className="hint" role="status">{done}</p>}
      {edit.isError && <p className="error" role="alert">{errText(edit.error)}</p>}
    </div>
  );
}
