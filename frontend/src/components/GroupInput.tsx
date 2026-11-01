import { useEffect, useId, useState } from "react";
import { m1 } from "../i18n/m1";
import { m15 } from "../i18n/m15";
import { useTaxonomySearch } from "../api/hooks";

const fold = (s: string) => s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLowerCase().trim();

/**
 * Ô "Nhóm sản phẩm" gõ có gợi ý (Phần E góp ý 2, chốt 01/11 06:33 và 06:41): mẹ gõ tên, Mimi gợi ý
 * 4–5 nhóm trong 300 nhóm chuẩn; chọn một gợi ý mới đổi nhóm. Không tạo nhóm mới. Rời ô mà chưa chọn
 * thì ô trở về nhóm đang có. Dùng chung ở chi tiết món (Đồ dùng) và màn xác nhận hóa đơn.
 */
export function GroupInput({
  label, value, onPick, busy, placeholder,
}: {
  /** Nhãn của ô; bỏ trống thì nhãn chỉ dành cho trình đọc màn hình. */
  label?: string;
  /** Tên nhóm đang có ("" khi chưa có nhóm). */
  value: string;
  onPick: (key: string, label: string) => void;
  busy?: boolean;
  placeholder?: string;
}) {
  const [text, setText] = useState(value);
  const [open, setOpen] = useState(false);
  useEffect(() => setText(value), [value]);
  const typed = open && fold(text) !== fold(value) ? text : "";
  const found = useTaxonomySearch(typed);
  const list = (found.data ?? []).slice(0, 5);
  const id = useId();
  const show = typed.trim().length >= 2 && !found.isPending;

  return (
    <div className="gi15">
      <label className="field15">
        <span className={label ? undefined : "visually-hidden"}>{label ?? m15.detail.group}</span>
        <input
          value={text}
          role="combobox"
          aria-expanded={show}
          aria-controls={id}
          aria-autocomplete="list"
          autoComplete="off"
          placeholder={placeholder ?? m1.receiptEdit.searchGroup}
          onFocus={(e) => {
            setOpen(true);
            e.target.select();
          }}
          onChange={(e) => {
            setOpen(true);
            setText(e.target.value);
          }}
          onBlur={() =>
            // Chờ một nhịp để cú chạm vào gợi ý kịp chạy trước khi ô trở về nhóm cũ.
            setTimeout(() => {
              setOpen(false);
              setText(value);
            }, 150)
          }
          onKeyDown={(e) => {
            if (e.key === "Escape") (e.target as HTMLInputElement).blur();
            if (e.key === "Enter" && list.length > 0) {
              e.preventDefault();
              onPick(list[0].key, list[0].label);
              (e.target as HTMLInputElement).blur();
            }
          }}
        />
      </label>
      {show && (
        <ul id={id} className="gi15__list" role="listbox">
          {list.length === 0 ? (
            <li className="gi15__none">{m15.groupInput.noMatch}</li>
          ) : (
            list.map((g) => (
              <li key={g.key} role="option" aria-selected={false}>
                <button
                  type="button"
                  className="gi15__opt"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => {
                    setOpen(false);
                    setText(g.label);
                    onPick(g.key, g.label);
                  }}
                >
                  {g.label}
                </button>
              </li>
            ))
          )}
        </ul>
      )}
      {busy && <p className="hint" role="status">{m15.groupInput.saving}</p>}
    </div>
  );
}
