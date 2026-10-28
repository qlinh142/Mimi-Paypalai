import { t } from "../i18n";
import { useState } from "react";
import { Link } from "react-router-dom";
import { Pencil, Plus, ShoppingBag, X } from "lucide-react";
import { ApiError } from "../api/client";
import { useItems, usePlan, usePlanItem, usePlanLine, usePlanPrices, useSavePlanState } from "../api/hooks";
import type { CartItem, Item, PlanPriority, PlanPrices, PlanState } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { CategoryIcon, STATUS_TEXT } from "../components/ItemRow";
import { formatDayMonth, formatVnd } from "../lib/text";

/** Dòng phụ dưới tên món: tồn hiện tại, cùng cách nói với màn Đồ dùng. */
function stockText(i: Pick<Item, "status" | "days_left">): string {
  if (i.status === "out" || i.status === "buy_now") return t.row.buyNow;
  if (i.status === "waiting" || i.days_left === null) return t.row.waiting;
  return `${i.days_left} ${t.row.days(i.days_left, STATUS_TEXT[i.status])}`;
}

/**
 * Kế hoạch mua sắm (spec mục 5), bốn phần từ trên xuống:
 * Sắp hết (thêm vào giỏ) → Giỏ (giá, dự kiến chi) → Nơi mua tối ưu → Thời gian mua.
 * Mọi con số do API tính; màn này chỉ hiển thị.
 */
export function Plan() {
  const plan = usePlan();
  const needs = useItems({ tab: "soon", horizon: 7 });
  const cart = plan.data;
  const prices = usePlanPrices((cart?.item_count ?? 0) > 0);
  const edit = usePlanItem();
  const busyKey = edit.isPending ? edit.variables?.key : undefined;

  if (plan.error instanceof ApiError && plan.error.status === 403) {
    return (
      <MainFrame>
        <TopBar title={t.plan.title} />
        <div className="page empty"><p>{t.plan.ownerOnly}</p></div>
      </MainFrame>
    );
  }

  const inCart = new Set(cart?.items.map((i) => i.key));
  const suggestions = (needs.data ?? []).filter((i) => !inCart.has(i.key));
  const names = new Map(cart?.items.map((i) => [i.key, i.name]));

  return (
    <MainFrame>
      <TopBar title={t.plan.title} />
      <div className="page plan">
        {plan.isError && !cart && (
          <div className="empty">
            <h2>{t.plan.loadFailed}</h2>
            <button className="btn btn--quiet" onClick={() => plan.refetch()}>{t.common.retry}</button>
          </div>
        )}
        {plan.isPending && <p className="hint">{t.common.loading}</p>}

        {/* 1. Sắp hết: món chưa có trong giỏ */}
        <section className="plan-sec" aria-labelledby="plan-needs">
          <h2 id="plan-needs">{t.plan.needsTitle}</h2>
          {needs.data && suggestions.length === 0 && <p className="hint">{t.plan.needsEmpty}</p>}
          <ul className="list">
            {suggestions.map((i) => (
              <li key={i.key} className="plan-row">
                <CategoryIcon category={i.category_key} />
                <span className="plan-row__text">
                  <span className="plan-row__name">{i.name}</span>
                  <span className="plan-row__sub">{stockText(i)}</span>
                </span>
                <button
                  type="button"
                  className="btn btn--quiet"
                  aria-label={t.plan.addLabel(i.name)}
                  aria-disabled={busyKey === i.key || undefined}
                  onClick={() => busyKey !== i.key && edit.mutate({ key: i.key, add: true })}
                >
                  <Plus size={18} aria-hidden /> {t.plan.add}
                </button>
              </li>
            ))}
          </ul>
        </section>

        {/* 2. Giỏ */}
        {cart && (
          <section className="plan-sec" aria-labelledby="plan-cart">
            <h2 id="plan-cart">{t.plan.cartTitle(cart.item_count)}</h2>
            {cart.item_count === 0 && <p className="hint">{t.plan.cartEmpty}</p>}
            <ul className="list">
              {cart.items.map((i) => (
                <CartRow key={i.key} item={i} busy={busyKey === i.key} onRemove={() => edit.mutate({ key: i.key, add: false })} />
              ))}
            </ul>
            {cart.item_count > 0 && (
              <>
                <p className="plan-total">
                  <span>{t.plan.estimate}</span>
                  <span className="num">{cart.estimate != null ? formatVnd(cart.estimate) : "–"}</span>
                </p>
                {cart.missing_price > 0 && <p className="hint">{t.plan.missing(cart.missing_price)}</p>}
                <Link to="/plan/online" className="btn btn--primary btn--block">
                  <ShoppingBag size={18} aria-hidden /> {t.online.cta}
                </Link>
                <p className="hint">{t.online.ctaHint}</p>
              </>
            )}
          </section>
        )}

        {/* 3. Nơi mua tối ưu */}
        {cart && cart.item_count > 0 && prices.data && <WhereToBuy prices={prices.data} names={names} />}

        {/* 4. Thời gian mua. Form dựng một lần khi có dữ liệu; sau đó trạng thái nằm trong form. */}
        {cart && <TripForm initial={cart.state} />}

        <p className="hint plan-tip">{t.plan.tip}</p>
      </div>
    </MainFrame>
  );
}

function CartRow({ item, busy, onRemove }: { item: CartItem; busy: boolean; onRemove: () => void }) {
  const [editing, setEditing] = useState(false);
  const qtyText = `${item.qty || 1}${item.unit ? ` ${item.unit}` : ""}`;
  return (
    <li className="plan-row" style={{ flexWrap: "wrap" }}>
      <CategoryIcon category={item.category_key} />
      <span className="plan-row__text">
        <span className="plan-row__name">{item.name}</span>
        <span className="plan-row__sub">
          {t.planLine.qty} {qtyText}
          {item.need_by && ` · ${t.planLine.needBy} ${formatDayMonth(item.need_by)}`}
          {item.priority === "high" && ` · ${t.planLine.priorities.high}`}
          {` · ${t.planLine.sources[item.source]}`}
        </span>
        {item.bought_qty ? <span className="plan-row__sub">{t.planLine.bought(item.bought_qty, item.qty)}</span> : null}
        <span className="plan-row__sub">
          {stockText(item)} ·{" "}
          {item.unit_price != null && item.best_store
            ? t.plan.priceAt(formatVnd(item.unit_price), item.best_store)
            : t.plan.noPrice}
        </span>
      </span>
      <button type="button" className="icon-btn" aria-label={t.planLine.edit(item.name)} aria-expanded={editing} onClick={() => setEditing(!editing)}>
        <Pencil size={18} aria-hidden />
      </button>
      <button
        type="button"
        className="icon-btn"
        aria-label={t.plan.remove(item.name)}
        aria-disabled={busy || undefined}
        onClick={() => !busy && onRemove()}
      >
        <X size={20} aria-hidden />
      </button>
      {editing && <LineEditor item={item} onDone={() => setEditing(false)} />}
    </li>
  );
}

const PRIORITIES: PlanPriority[] = ["high", "normal", "low"];

/** Sửa số lượng, đơn vị, need-by, ưu tiên của một dòng. Chưa tạo giao dịch, không đổi tồn kho. */
function LineEditor({ item, onDone }: { item: CartItem; onDone: () => void }) {
  const edit = usePlanLine();
  const [qty, setQty] = useState(String(item.qty || 1));
  const [unit, setUnit] = useState(item.unit ?? "");
  const [needBy, setNeedBy] = useState(item.need_by ?? "");
  const [priority, setPriority] = useState<PlanPriority>(item.priority);
  const value = Number(qty.replace(",", "."));
  const valid = Number.isFinite(value) && value > 0;
  return (
    <form
      className="plan-form"
      style={{ flexBasis: "100%" }}
      onSubmit={(e) => {
        e.preventDefault();
        if (!valid || edit.isPending) return;
        edit.mutate({ key: item.key, patch: { qty: value, unit, need_by: needBy, priority } }, { onSuccess: onDone });
      }}
    >
      <div className="plan-form__row">
        <label>
          {t.planLine.qty}
          <input inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value)} />
        </label>
        <label>
          {t.stock.unit}
          <select value={unit} onChange={(e) => setUnit(e.target.value)}>
            {["", "kg", "g", "l", "ml", "hop", "goi", "chai", "qua", "loc"].map((u) => (
              <option key={u} value={u}>{u || t.stock.units.pc}</option>
            ))}
          </select>
        </label>
      </div>
      <div className="plan-form__row">
        <label>
          {t.planLine.needBy}
          <input type="date" value={needBy} onChange={(e) => setNeedBy(e.target.value)} />
        </label>
        <label>
          {t.planLine.priority}
          <select value={priority} onChange={(e) => setPriority(e.target.value as PlanPriority)}>
            {PRIORITIES.map((p) => (
              <option key={p} value={p}>{t.planLine.priorities[p]}</option>
            ))}
          </select>
        </label>
      </div>
      {edit.isError && <p className="error" role="alert">{edit.error instanceof ApiError ? edit.error.message : t.common.genericError}</p>}
      <button type="submit" className="btn btn--quiet btn--block" aria-disabled={!valid || edit.isPending || undefined}>
        {t.planLine.done}
      </button>
    </form>
  );
}

function WhereToBuy({ prices, names }: { prices: PlanPrices; names: Map<string, string> }) {
  const label = (keys: string[]) => keys.map((k) => names.get(k) ?? k).join(", ");
  const nothingPriced = prices.split.length === 0;
  return (
    <section className="plan-sec" aria-labelledby="plan-where">
      <h2 id="plan-where">{t.plan.whereTitle}</h2>
      {nothingPriced ? (
        <p className="hint">{t.plan.whereNone}</p>
      ) : prices.recommend === "single" && prices.single_store ? (
        <div className="plan-stop">
          <div className="plan-stop__head">
            <span>{t.plan.oneStore(prices.single_store.store)}</span>
            <span className="num">{formatVnd(prices.single_store.total)}</span>
          </div>
          <div className="plan-stop__items">{label(prices.single_store.groups)}</div>
        </div>
      ) : (
        <>
          <p className="plan-row__sub" style={{ marginBottom: "var(--s2)" }}>{t.plan.split}</p>
          {prices.split.map((s) => (
            <div key={s.store} className="plan-stop">
              <div className="plan-stop__head">
                <span>{s.store}</span>
                <span className="num">{formatVnd(s.total)}</span>
              </div>
              <div className="plan-stop__items">{label(s.groups)}</div>
            </div>
          ))}
          {prices.single_store && prices.saving > 0 && <p className="hint">{t.plan.saving(formatVnd(prices.saving))}</p>}
        </>
      )}
      {prices.single_source.length > 0 && <p className="hint">{t.plan.singleSource(label(prices.single_source))}</p>}
    </section>
  );
}

const HOURS = Array.from({ length: 24 }, (_, h) => h);

function TripForm({ initial }: { initial: PlanState }) {
  const [st, setSt] = useState<PlanState>(initial);
  const [saved, setSaved] = useState<PlanState>(initial); // bản đã lưu gần nhất, để biết có thay đổi chưa
  const save = useSavePlanState();
  const dirty = JSON.stringify(st) !== JSON.stringify(saved);
  return (
    <section className="plan-sec" aria-labelledby="plan-when">
      <h2 id="plan-when">{t.plan.whenTitle}</h2>
      <form
        className="plan-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (save.isPending || !dirty) return;
          save.mutate({ ...st, trip_note: st.trip_note.trim() }, {
            onSuccess: (s) => {
              setSt(s);
              setSaved(s);
            },
          });
        }}
      >
        <div className="plan-form__row">
          <label>
            {t.plan.date}
            <input type="date" value={st.buy_date} onChange={(e) => setSt({ ...st, buy_date: e.target.value })} />
          </label>
          <label>
            {t.plan.hour}
            <select
              value={st.buy_hour ?? ""}
              onChange={(e) => setSt({ ...st, buy_hour: e.target.value === "" ? null : Number(e.target.value) })}
            >
              <option value="">{t.plan.anyTime}</option>
              {HOURS.map((h) => (
                <option key={h} value={h}>{`${String(h).padStart(2, "0")}:00`}</option>
              ))}
            </select>
          </label>
        </div>
        <label>
          {t.plan.note}
          <textarea
            maxLength={120}
            placeholder={t.plan.notePlaceholder}
            value={st.trip_note}
            onChange={(e) => setSt({ ...st, trip_note: e.target.value })}
          />
        </label>
        {save.isError && <p className="error" role="alert">{t.common.genericError}</p>}
        <button type="submit" className="btn btn--primary btn--block" aria-disabled={save.isPending || !dirty || undefined}>
          {save.isSuccess && !dirty ? t.plan.saved : t.plan.save}
        </button>
      </form>
    </section>
  );
}
