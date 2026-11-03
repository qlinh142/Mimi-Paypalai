import { useState, type ReactNode } from "react";
import { t } from "../i18n";
import { m1 } from "../i18n/m1";
import { m15 } from "../i18n/m15";
import { useTaxonomyLabels, useTaxonomySearch } from "../api/hooks";
import type { ReceiptDraftLine } from "../api/types";
import { formatQty, formatVnd } from "../lib/text";
import { GroupInput } from "./GroupInput";

/** Dòng trên màn xác nhận: removed = người dùng bỏ, không gửi khi lưu (6.1 mục 10). */
export type EditLine = ReceiptDraftLine & { removed?: boolean };

/** Số tiền in trên hóa đơn ("64.000", "12.50") ra số; null khi không đọc được. */
export function parseAmount(s: string, currency: string): number | null {
  const v = s.trim();
  if (!v) return null;
  const n = currency === "VND" ? Number(v.replace(/[.,\s]/g, "")) : Number(v.replace(/,/g, "."));
  return Number.isFinite(n) ? n : null;
}

/**
 * Các dòng hóa đơn trên màn xác nhận: sửa tên / số lượng / giá, bỏ dòng, chọn hoặc đổi nhóm.
 * Tên hiển thị hai dòng theo 7.1: "Brand Tên riêng" rồi "Nhóm · lượng". Dòng lỗi khi lưu
 * (BUG-06) được tô và mở sẵn để sửa.
 */
export function ReceiptLines({
  lines, currency, badLine, onChange, extra,
}: {
  lines: EditLine[];
  /** Spec 1.5 mục 3: dòng "→ [kế hoạch]" dưới mỗi món. */
  extra?: (l: EditLine) => ReactNode;
  currency: string;
  badLine?: string;
  onChange: (id: string, patch: Partial<EditLine>) => void;
}) {
  const groupLabel = useTaxonomyLabels();
  const [editing, setEditing] = useState<string | null>(null);
  const pc = t.stock.units.pc;
  return (
    <ul className="lines">
      {lines.map((l) => {
        // Tên đã dịch sang ngôn ngữ app thì hiện tên dịch, tên theo bill nhỏ bên dưới.
        const title = l.original_name ? l.name : l.detail_name || l.name;
        const bad = l.line_id === badLine;
        const group = groupLabel(l.group_key);
        const open = editing === l.line_id || bad;
        // A7: ô giá trống là "chưa rõ giá": không bịa số, không tính 0đ, hiện mờ.
        const noPrice = !l.discount && l.amount.trim() === "";
        return (
          <li key={l.line_id} className={bad ? "lines__bad" : undefined} aria-invalid={bad || undefined}
            style={l.removed ? { opacity: 0.55 } : undefined}>
            <span style={l.removed ? { textDecoration: "line-through" } : undefined}>
              {title}
              {l.original_name && <span className="name15__orig">{l.original_name}</span>}
            </span>
            <span className="num" style={noPrice ? { opacity: 0.55 } : undefined}>
              {noPrice ? t.confirm.noPrice : l.amount_vnd != null ? formatVnd(l.amount_vnd) : `${l.amount} ${currency}`}
            </span>
            <span className="sub" style={{ gridColumn: "1 / -1" }}>
              {l.removed
                ? m1.receiptEdit.removed
                : l.discount
                  ? t.confirm.discountLine
                  : [
                      l.canonical_quantity ? formatQty(l.canonical_quantity, l.canonical_unit ?? "item", pc) : t.confirm.qty(l.qty),
                      noPrice ? t.confirm.noPriceNote : l.amount_vnd == null ? t.confirm.notConverted : "",
                      l.needs_review ? (l.review_reasons ?? []).map((r) => m1.scan.reviewReasons[r]).join(", ") || m1.scan.needsReview : "",
                    ].filter(Boolean).join(" · ")}
            </span>
            {l.detail_name && l.detail_name !== l.name && !l.removed && <span className="sub" style={{ gridColumn: "1 / -1" }}>{l.name}</span>}
            {extra && !l.removed && !l.discount && <span style={{ gridColumn: "1 / -1" }}>{extra(l)}</span>}

            {!l.removed && !l.discount && (
              <GroupInput
                label={m15.detail.group}
                value={group}
                placeholder={m1.receiptEdit.noGroup}
                onPick={(key) => {
                  // Người dùng chọn nhóm: server lấy danh mục theo nhóm, dòng hết "cần chọn nhóm".
                  const reasons = (l.review_reasons ?? []).filter((r) => r !== "missing_group" && r !== "missing_category");
                  onChange(l.line_id, { group_key: key, category_key: "", review_reasons: reasons, needs_review: reasons.length > 0 });
                }}
              />
            )}
            <div className="plan-form__row" style={{ gridColumn: "1 / -1" }}>
              {l.removed ? (
                <button type="button" className="btn btn--quiet" onClick={() => onChange(l.line_id, { removed: false })}>{m1.receiptEdit.keep}</button>
              ) : (
                <>
                  <button type="button" className="btn btn--quiet" aria-expanded={open}
                    onClick={() => setEditing(open ? null : l.line_id)}>
                    {open ? m1.receiptEdit.done : m1.receiptEdit.edit}
                  </button>
                  <button type="button" className="btn btn--quiet" onClick={() => onChange(l.line_id, { removed: true })}>{m1.receiptEdit.remove}</button>
                </>
              )}
            </div>

            {open && !l.removed && <LineForm line={l} currency={currency} onChange={(p) => onChange(l.line_id, p)} />}
          </li>
        );
      })}
    </ul>
  );
}

function LineForm({ line, currency, onChange }: { line: EditLine; currency: string; onChange: (p: Partial<EditLine>) => void }) {
  return (
    <div className="plan-form" style={{ gridColumn: "1 / -1" }}>
      <label>
        {m1.receiptEdit.name}
        <input value={line.name} maxLength={80} onChange={(e) => onChange({ name: e.target.value, detail_name: undefined })} />
      </label>
      <div className="plan-form__row">
        <label>
          {m1.receiptEdit.qty}
          <input inputMode="decimal" value={String(line.qty)}
            onChange={(e) => {
              const q = Number(e.target.value.replace(",", "."));
              if (Number.isFinite(q)) onChange({ qty: q });
            }} />
        </label>
        <label>
          {m1.receiptEdit.amount} ({currency})
          <input inputMode="decimal" value={line.amount}
            onChange={(e) => {
              const amount = e.target.value;
              const v = parseAmount(amount, currency);
              const old = parseAmount(line.amount, currency);
              // VND: số nhập là số tiền; ngoại tệ: giữ tỷ giá đã quy đổi của dòng.
              const amount_vnd = v == null ? null
                : currency === "VND" ? Math.round(v)
                : line.amount_vnd != null && old ? Math.round((line.amount_vnd / old) * v) : null;
              onChange({ amount, amount_vnd });
            }} />
        </label>
      </div>
    </div>
  );
}

export function GroupPicker({ name, onPick, onCancel }: { name: string; onPick: (key: string) => void; onCancel: () => void }) {
  const [q, setQ] = useState("");
  const found = useTaxonomySearch(q);
  return (
    <div className="plan-form" style={{ gridColumn: "1 / -1" }}>
      <label>
        {m1.receiptEdit.chooseGroupFor(name)}
        <input autoFocus value={q} placeholder={m1.receiptEdit.searchGroup} onChange={(e) => setQ(e.target.value)} />
      </label>
      <div className="chips">
        {(found.data ?? []).map((g) => (
          <button key={g.key} type="button" className="chip" onClick={() => onPick(g.key)}>{g.label}</button>
        ))}
      </div>
      <button type="button" className="btn btn--quiet" onClick={onCancel}>{m1.receiptEdit.cancel}</button>
    </div>
  );
}
