import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Check, Minus, Plus, ShoppingCart } from "lucide-react";
import { GroupInput } from "../components/GroupInput";
import { ApiError } from "../api/client";
import { useCorrectItem, useItem, useItemPriceHistory, useSetQuantity, useTaxonomyLabels } from "../api/hooks";
import { PlanPicker, useKeysInPlans } from "../components/PlanPicker";
import type { ItemDetail as ItemDetailData } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { ALL_UNITS, formatNum, rememberUnit, savedUnit, stockInDisplay, unitLabel } from "../lib/displayUnit";
import { formatDayMonth, formatVnd } from "../lib/text";
import { LoadState } from "../components/LoadState";

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
          </>
        )}
      </div>
    </MainFrame>
  );
}

function QuantityCard({ item }: { item: ItemDetailData }) {
  const set = useSetQuantity();
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
  const inPlan = useKeysInPlans().has(item.key);
  const [picker, setPicker] = useState(false);
  const fix = useCorrectItem();
  const nav = useNavigate();
  const groupLabel = useTaxonomyLabels();
  const [name, setName] = useState(item.name);
  useEffect(() => setName(item.name), [item.name]);
  const step = unit === "g" || unit === "ml" ? 100 : unit === "kg" || unit === "l" ? 0.5 : 1;

  return (
    <section className="box15" aria-labelledby="qty-title">
      <label className="field15">
        <span>{m15.detail.name}</span>
        <input value={name} maxLength={60} onChange={(e) => setName(e.target.value)} />
      </label>
      <GroupInput
        label={m15.detail.group}
        value={groupLabel(item.key) || item.category_label}
        busy={fix.isPending && fix.variables?.group_key != null}
        onPick={(key) =>
          fix.mutate(
            { key: item.key, group_key: key },
            { onSuccess: (it) => it.key !== item.key && nav(`/items/${encodeURIComponent(it.key)}`, { replace: true }) },
          )
        }
      />
      {fix.isError && <p className="error" role="alert">{fix.error instanceof ApiError ? fix.error.message : t.common.genericError}</p>}
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
          const newName = name.trim();
          if (newName && newName !== item.name) fix.mutate({ key: item.key, name: newName });
          set.mutate(
            { key: item.key, qty, unit, action: "set" },
            { onSuccess: () => rememberUnit(item.key, unit) },
          );
        }}
      >
        <Check size={20} aria-hidden /> {m15.detail.update}
      </button>
      {set.isSuccess && <p className="hint" role="status">{m15.detail.updated}</p>}
      <button
        type="button"
        className="btn btn--outline btn--block"
        aria-pressed={inPlan}
        aria-haspopup="dialog"
        onClick={() => setPicker(true)}
      >
        <ShoppingCart size={20} aria-hidden /> {m15.detail.shop}
      </button>
      {picker && <PlanPicker planKey={item.key} name={item.name} onClose={() => setPicker(false)} />}
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
        h.isPending || h.isError ? <LoadState q={h} hasData={false} /> : <p className="hint">{m15.detail.noHistory}</p>
      ) : (
        <>
          <dl className="stats15">
            <div><dt>{m15.detail.last}</dt><dd className="num">{h.data?.last_purchase ? formatVnd(h.data.last_purchase.paid_price, h.data.last_purchase.purchased_at) : "–"}</dd></div>
            <div><dt>{m15.detail.lowest}</dt><dd className="num">{h.data?.lowest_historical ? formatVnd(h.data.lowest_historical.paid_price, h.data.lowest_historical.purchased_at) : "–"}</dd></div>
            <div><dt>{m15.detail.avg}</dt><dd className="num">{avg != null ? formatVnd(avg) : "–"}</dd></div>
          </dl>
          <ul className="hist15">
            {rows.map((r, i) => (
              <li key={i}>
                <span>{formatDayMonth(r.purchased_at.slice(0, 10))}</span>
                <span>{r.store_name}</span>
                <span className="num">{formatVnd(r.paid_price, r.purchased_at)}{per}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}
