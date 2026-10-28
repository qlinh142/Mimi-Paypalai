import { t } from "../i18n";
import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ApiError } from "../api/client";
import { useItem, useQuickStock, useSetQuantity, useUndoStock } from "../api/hooks";
import type { ItemDetail as ItemDetailData } from "../api/types";
import { TopBar, useUndo, MainFrame } from "../components/AppShell";
import { CategoryIcon, STATUS_TEXT, statusTone } from "../components/ItemRow";
import { formatDayMonth, formatQty } from "../lib/text";

const CONFIDENCE = t.detail.confidenceLevels;

/** Chi tiết món (spec mục 4). Bản khung: số ngày, độ tin cậy, nguồn, ba nút chỉnh nhanh. */
export function ItemDetail() {
  const { key = "" } = useParams();
  const q = useItem(key);
  const quick = useQuickStock();
  const undo = useUndoStock();
  const showUndo = useUndo();
  const item = q.data;

  return (
    <MainFrame>
      <TopBar title={item?.name ?? t.detail.fallbackTitle} variant="sub" />
      <div className="page detail">
        {!item && q.isPending && <p className="hint">{t.common.loading}</p>}
        {!item && q.isError && (
          <div className="empty">
            <p>
              {q.error instanceof ApiError && q.error.status === 404 ? t.detail.notFound : t.common.genericError}
            </p>
            <Link className="btn btn--quiet" to="/items">{t.items.title}</Link>
          </div>
        )}
        {item && (
          <>
            <div className="detail__figure">
              <CategoryIcon category={item.category_key} />
              <span className={`tone-${statusTone(item)}`}>
                <span className="row__big num">{item.days_left ?? "–"}</span>{" "}
                <span className="row__unit">{item.days_left === null ? t.detail.waiting : t.detail.daysLeft(STATUS_TEXT[item.status])}</span>
              </span>
            </div>
            <dl className="facts">
              <dt>{t.detail.expected}</dt>
              <dd>{item.predicted_out_on ? formatDayMonth(item.predicted_out_on) : t.detail.notEnough}</dd>
              <dt>{t.detail.confidence}</dt>
              <dd>{item.confidence ? CONFIDENCE[item.confidence] : t.detail.none} ({t.detail.purchases(item.purchase_count)})</dd>
              <dt>{t.detail.source}</dt>
              <dd>{item.source === "manual" ? t.detail.manual : t.detail.predicted}</dd>
              <dt>{t.detail.category}</dt>
              <dd>{item.category_label}</dd>
            </dl>
            <div className="stock-btns" role="group" aria-label={t.detail.stockGroup}>
              {(
                [
                  ["plenty", t.detail.plenty],
                  ["soon", t.detail.soon],
                  ["out", t.detail.out],
                ] as const
              ).map(([k, label]) => (
                <button
                  key={k}
                  type="button"
                  className="btn btn--quiet"
                  onClick={() =>
                    quick.mutate(
                      { item, quick: k },
                      {
                        onSuccess: () => {
                          q.refetch();
                          showUndo({ message: t.detail.marked(label), onUndo: () => undo.mutate(item, { onSuccess: () => q.refetch() }) });
                        },
                      },
                    )
                  }
                >
                  {label}
                </button>
              ))}
            </div>
            <p className="hint">{t.detail.hint}</p>
            <StockSection item={item} onSaved={() => q.refetch()} />
          </>
        )}
      </div>
    </MainFrame>
  );
}

const SET_UNITS = ["kg", "g", "l", "ml", "cai", "hop", "goi", "chai", "qua"] as const;

/**
 * Tồn theo số lượng (Mimi 2.0 mục 4): số Mimi ước tính, mốc xác nhận gần nhất và nguồn,
 * và form "SET số lượng thực tế". Mọi con số do API tính; màn này chỉ hiển thị.
 */
function StockSection({ item, onSaved }: { item: ItemDetailData; onSaved: () => void }) {
  const set = useSetQuantity();
  const [qty, setQty] = useState("");
  const [unit, setUnit] = useState<string>(item.stock?.unit === "g" ? "kg" : item.stock?.unit === "ml" ? "l" : "cai");
  const st = item.stock;
  const pc = t.stock.units.pc;
  const value = Number(qty.replace(",", "."));
  const valid = qty.trim() !== "" && Number.isFinite(value) && value >= 0;
  return (
    <section className="plan-sec" aria-labelledby="stock-title">
      <h2 id="stock-title">{t.stock.title}</h2>
      {st ? (
        <dl className="facts">
          <dt>{t.stock.estimated}</dt>
          <dd className="num">{formatQty(st.estimated, st.unit, pc)}</dd>
          <dt>{t.detail.source}</dt>
          <dd>{t.stock.confirmed(formatQty(st.confirmed, st.unit, pc), formatDayMonth(st.confirmed_on), t.stock.sources[st.confirmed_source])}</dd>
          {st.days_of_stock != null && (
            <>
              <dt>{t.detail.expected}</dt>
              <dd>{t.stock.lasts(st.days_of_stock)}</dd>
            </>
          )}
        </dl>
      ) : (
        <p className="hint">{t.stock.none}</p>
      )}
      <form
        className="plan-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (!valid || set.isPending) return;
          set.mutate({ key: item.key, qty: value, unit }, { onSuccess: () => { setQty(""); onSaved(); } });
        }}
      >
        <div className="plan-form__row">
          <label>
            {t.stock.qty}
            <input inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value)} placeholder="0" />
          </label>
          <label>
            {t.stock.unit}
            <select value={unit} onChange={(e) => setUnit(e.target.value)}>
              {SET_UNITS.map((u) => (
                <option key={u} value={u}>{u}</option>
              ))}
            </select>
          </label>
        </div>
        {set.isError && <p className="error" role="alert">{t.common.genericError}</p>}
        {set.isSuccess && <p className="hint">{t.stock.saved}</p>}
        <button type="submit" className="btn btn--quiet btn--block" aria-disabled={!valid || set.isPending || undefined}>
          {t.stock.set}
        </button>
      </form>
    </section>
  );
}
