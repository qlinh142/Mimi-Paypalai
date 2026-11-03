import { t } from "../i18n";
import { m1 } from "../i18n/m1";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { ApiError } from "../api/client";
import { useCorrectItem, useItemPriceHistory, useTaxonomyLabels } from "../api/hooks";
import { GroupPicker } from "./ReceiptLines";
import type { ItemDetail } from "../api/types";
import { formatDayMonth, formatQty, formatUnitPrice, formatVnd } from "../lib/text";

/** Các lần mua còn lại (0.1 mục 11 đến 13): phần còn lại chia FIFO, lot mới nhất còn nhiều nhất. */
export function LotsSection({ item }: { item: ItemDetail }) {
  const lots = item.lots ?? [];
  if (lots.length === 0) return null;
  const pc = t.stock.units.pc;
  return (
    <section className="plan-sec" aria-labelledby="lots-title">
      <h2 id="lots-title">{m1.lots.title}</h2>
      <ul className="list">
        {lots.map((l) => (
          <li key={l.lot_id} className="plan-row">
            <span className="plan-row__text">
              <span className="plan-row__name">{l.detail_name || item.name}</span>
              <span className="plan-row__sub">
                {formatDayMonth(l.purchased_at)}
                {l.store && ` · ${l.store}`}
              </span>
              <span className="plan-row__sub">
                {l.remaining_quantity > 0
                  ? m1.lots.remaining(formatQty(l.remaining_quantity, l.canonical_unit, pc), formatQty(l.initial_quantity, l.canonical_unit, pc))
                  : m1.lots.used}
              </span>
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Lịch sử giá (8.1 mục 9): so theo đơn giá chuẩn hóa, chỉ cùng nhóm và cùng chiều đo. */
export function PriceHistorySection({ itemKey }: { itemKey: string }) {
  const q = useItemPriceHistory(itemKey);
  const d = q.data;
  if (!d) return null;
  const pc = t.stock.units.pc;
  const unit = d.canonical_unit ?? "item";
  return (
    <section className="plan-sec" aria-labelledby="prices-title">
      <h2 id="prices-title">{m1.priceHistory.title}</h2>
      {!d.last_purchase ? (
        <p className="hint">{m1.priceHistory.none}</p>
      ) : (
        <>
          <dl className="facts">
            <dt>{m1.priceHistory.last}</dt>
            <dd>
              {d.last_purchase.store_name} · {formatVnd(d.last_purchase.paid_price, d.last_purchase.purchased_at)} · {formatDayMonth(d.last_purchase.purchased_at)}
              <br />
              <span className="hint">{formatUnitPrice(d.last_purchase.unit_price, unit, pc, d.last_purchase.purchased_at)}</span>
            </dd>
            {d.lowest_historical && (
              <>
                <dt>{m1.priceHistory.lowest}</dt>
                <dd>
                  {d.lowest_historical.store_name} · {formatUnitPrice(d.lowest_historical.unit_price, unit, pc, d.lowest_historical.purchased_at)}
                </dd>
              </>
            )}
            {d.average_unit_price != null && (
              <>
                <dt>{m1.priceHistory.average}</dt>
                <dd>{formatUnitPrice(d.average_unit_price, unit, pc)}</dd>
              </>
            )}
          </dl>
          <ul className="list">
            {d.history.slice(0, 6).map((o, i) => (
              <li key={i} className="plan-row">
                <span className="plan-row__text">
                  <span className="plan-row__sub">
                    {formatDayMonth(o.purchased_at)} · {o.store_name || "–"} · {formatVnd(o.paid_price, o.purchased_at)} ({formatUnitPrice(o.unit_price, unit, pc, o.purchased_at)})
                  </span>
                </span>
              </li>
            ))}
          </ul>
        </>
      )}
      {d.other_dimension_count > 0 && <p className="hint">{m1.priceHistory.otherDimension(d.other_dimension_count)}</p>}
    </section>
  );
}

/**
 * Sửa tên chuẩn hóa và sửa nhóm khi AI phân loại sai (0.1 mục 3.2). Đổi nhóm thì lịch sử
 * mua và tồn kho chuyển sang nhóm mới; màn chuyển sang món ở nhóm mới.
 */
export function FixItemSection({ item }: { item: ItemDetail }) {
  const fix = useCorrectItem();
  const nav = useNavigate();
  const groupLabel = useTaxonomyLabels();
  const [open, setOpen] = useState(false);
  const [picking, setPicking] = useState(false);
  const [name, setName] = useState(item.name);
  // Sau khi đổi nhóm trang chuyển sang key mới; câu báo đi theo state của điều hướng.
  const moved = (useLocation().state as { moved?: string } | null)?.moved ?? "";
  const group = groupLabel(item.key);
  return (
    <section className="plan-sec" aria-labelledby="fix-title">
      <button id="fix-title" type="button" className="btn btn--quiet btn--block" aria-expanded={open} onClick={() => setOpen(!open)}>
        {m1.itemFix.cta}
      </button>
      {moved && <p className="hint" role="status">{moved}</p>}
      {open && (
        <div className="plan-form">
          <form
            className="plan-form__row"
            onSubmit={(e) => {
              e.preventDefault();
              if (!name.trim() || name.trim() === item.name || fix.isPending) return;
              fix.mutate({ key: item.key, name: name.trim() });
            }}
          >
            <label style={{ flex: 1 }}>
              {m1.itemFix.name}
              <input value={name} maxLength={60} onChange={(e) => setName(e.target.value)} />
            </label>
            <button type="submit" className="btn btn--primary" aria-disabled={!name.trim() || name.trim() === item.name || undefined}>
              {m1.itemFix.saveName}
            </button>
          </form>
          {group && <p className="hint">{m1.itemFix.group(group)}</p>}
          {!picking ? (
            <button type="button" className="btn btn--quiet" onClick={() => setPicking(true)}>{m1.itemFix.changeGroup}</button>
          ) : (
            <GroupPicker
              name={item.name}
              onCancel={() => setPicking(false)}
              onPick={(key) =>
                fix.mutate(
                  { key: item.key, group_key: key },
                  {
                    onSuccess: (it) => {
                      setPicking(false);
                      setOpen(false);
                      nav(`/items/${encodeURIComponent(it.key)}`, { replace: true, state: { moved: m1.itemFix.moved(groupLabel(it.key) || it.name) } });
                    },
                  },
                )
              }
            />
          )}
          {fix.isError && <p className="error" role="alert">{fix.error instanceof ApiError ? fix.error.message : t.common.genericError}</p>}
        </div>
      )}
    </section>
  );
}
