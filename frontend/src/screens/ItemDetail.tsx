import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Minus, Plus, Save, ShoppingCart } from "lucide-react";
import { ApiError } from "../api/client";
import { useItem, useItemPriceHistory, usePlan, usePlanItem, useSetQuantity } from "../api/hooks";
import type { ItemDetail as ItemDetailData } from "../api/types";
import { MainFrame, TopBar, useUndo } from "../components/AppShell";
import { FixItemSection } from "../components/ItemExtras";
import { ALL_UNITS, formatNum, rememberUnit, savedUnit, stockInDisplay, unitLabel } from "../lib/displayUnit";
import { formatDayMonth, formatVnd } from "../lib/text";

/**
 * Chi tiết món (Mimi 1.5, ảnh mẫu 3-do-dung-va-chi-tiet.png): số lượng hiện có với − / + và đơn vị,
 * Cập nhật sản phẩm, Mua sắm (thêm vào Kế hoạch), Lịch sử mua hàng.
 */
export function ItemDetail() {
  const { key = "" } = useParams();
  const q = useItem(key);
  const item = q.data;

  return (
    <MainFrame>
      <TopBar title={item?.name ?? t.detail.fallbackTitle} variant="sub" />
      <div className="page p15">
        {!item && q.isPending && <p className="hint">{t.common.loading}</p>}
        {!item && q.isError && (
          <div className="empty">
            <p>{q.error instanceof ApiError && q.error.status === 404 ? t.detail.notFound : t.common.genericError}</p>
            <Link className="btn btn--quiet" to="/items">{t.items.title}</Link>
          </div>
        )}
        {item && (
          <>
            <QuantityCard item={item} />
            <HistoryCard itemKey={item.key} unit={stockInDisplay(item)?.[1] ?? savedUnit(item)} />
            <FixItemSection key={item.key} item={item} />
          </>
        )}
      </div>
    </MainFrame>
  );
}

function QuantityCard({ item }: { item: ItemDetailData }) {
  const set = useSetQuantity();
  const plan = usePlan();
  const add = usePlanItem();
  const showUndo = useUndo();
  const current = stockInDisplay(item);
  const [qty, setQty] = useState(current?.[0] ?? 0);
  const [unit, setUnit] = useState(current?.[1] ?? savedUnit(item));
  useEffect(() => {
    const c = stockInDisplay(item);
    if (c) {
      setQty(c[0]);
      setUnit(c[1]);
    }
  }, [item]);
  const inPlan = (plan.data?.items ?? []).some((i) => i.key === item.key) || add.isSuccess;
  const step = unit === "g" || unit === "ml" ? 100 : unit === "kg" || unit === "l" ? 0.5 : 1;

  return (
    <section className="box15" aria-labelledby="qty-title">
      <h2 id="qty-title">{m15.detail.qtyTitle}</h2>
      <div className="qty15">
        <div className="stepper15">
          <button type="button" aria-label={m15.detail.minus} onClick={() => setQty((v) => Math.max(0, +(v - step).toFixed(2)))}>
            <Minus size={20} aria-hidden />
          </button>
          <input
            inputMode="decimal"
            aria-label={m15.detail.qtyTitle}
            value={formatNum(qty)}
            onChange={(e) => {
              const v = Number(e.target.value.replace(/\./g, "").replace(",", "."));
              if (Number.isFinite(v) && v >= 0) setQty(v);
            }}
          />
          <button type="button" aria-label={m15.detail.plus} onClick={() => setQty((v) => +(v + step).toFixed(2))}>
            <Plus size={20} aria-hidden />
          </button>
        </div>
        <label className="unit15">
          <span className="visually-hidden">{m15.detail.unit}</span>
          <select value={unit} onChange={(e) => setUnit(e.target.value)}>
            {ALL_UNITS.map((u) => (
              <option key={u} value={u}>{unitLabel(u)}</option>
            ))}
          </select>
        </label>
      </div>
      {set.isError && <p className="error" role="alert">{set.error instanceof ApiError ? set.error.message : t.common.genericError}</p>}
      <button
        type="button"
        className="btn btn--primary btn--block"
        aria-disabled={set.isPending || undefined}
        onClick={() => {
          if (set.isPending) return;
          set.mutate(
            { key: item.key, qty, unit, action: "set" },
            { onSuccess: () => { rememberUnit(item.key, unit); showUndo({ message: m15.detail.updated, onUndo: () => {} }); } },
          );
        }}
      >
        <Save size={20} aria-hidden /> {m15.detail.update}
      </button>
      <button
        type="button"
        className="btn btn--outline btn--block"
        aria-pressed={inPlan}
        aria-disabled={add.isPending || undefined}
        onClick={() => {
          if (inPlan || add.isPending) return;
          add.mutate({ key: item.key, add: true }, { onSuccess: () => showUndo({ message: m15.items.added(item.name), onUndo: () => add.mutate({ key: item.key, add: false }, { onSuccess: () => add.reset() }) }) });
        }}
      >
        <ShoppingCart size={20} aria-hidden /> {inPlan ? m15.detail.inPlan : m15.detail.shop}
      </button>
    </section>
  );
}

function HistoryCard({ itemKey, unit }: { itemKey: string; unit: string }) {
  const h = useItemPriceHistory(itemKey);
  const rows = h.data?.history ?? [];
  const prices = rows.map((r) => r.paid_price);
  const avg = prices.length ? Math.round(prices.reduce((a, b) => a + b, 0) / prices.length) : null;
  const per = `/${unitLabel(unit)}`;
  return (
    <section className="box15" aria-labelledby="hist-title">
      <h2 id="hist-title">{m15.detail.history}</h2>
      {rows.length === 0 ? (
        <p className="hint">{h.isPending ? t.common.loading : m15.detail.noHistory}</p>
      ) : (
        <>
          <dl className="stats15">
            <div><dt>{m15.detail.last}</dt><dd className="num">{h.data?.last_purchase ? formatVnd(h.data.last_purchase.paid_price) : "–"}</dd></div>
            <div><dt>{m15.detail.lowest}</dt><dd className="num">{h.data?.lowest_historical ? formatVnd(h.data.lowest_historical.paid_price) : "–"}</dd></div>
            <div><dt>{m15.detail.avg}</dt><dd className="num">{avg != null ? formatVnd(avg) : "–"}</dd></div>
          </dl>
          <ul className="hist15">
            {rows.map((r, i) => (
              <li key={i}>
                <span>{formatDayMonth(r.purchased_at.slice(0, 10))}</span>
                <span>{r.store_name}</span>
                <span className="num">{formatVnd(r.paid_price)}{per}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}
