import { useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { m15 } from "../i18n/m15";

const KEY = (id: string) => `mimi.group.${id}`;
function readCollapsed(id: string): boolean {
  try {
    return localStorage.getItem(KEY(id)) === "1";
  } catch {
    return false;
  }
}

/**
 * Nhóm có dải màu tiêu đề (Cần mua ngay, Sắp hết...). Bấm dải để rút gọn / mở rộng;
 * mũi tên bên phải chỉ trạng thái, số món hiện cạnh tên. Nhớ trên máy theo từng nhóm.
 */
export function Group15({ id, label, tone, count, children }: { id: string; label: string; tone: "now" | "soon" | "plenty"; count: number; children: ReactNode }) {
  const [collapsed, setCollapsed] = useState(() => readCollapsed(id));
  const listId = `g15-${id}`;
  return (
    <section className="g15" aria-label={label}>
      <h2 className={`g15__head g15__head--${tone}`}>
        <button
          type="button"
          className="g15__toggle"
          aria-expanded={!collapsed}
          aria-controls={listId}
          aria-label={`${label} (${count}), ${collapsed ? m15.items.expand : m15.items.collapse}`}
          onClick={() => {
            const next = !collapsed;
            setCollapsed(next);
            try {
              localStorage.setItem(KEY(id), next ? "1" : "0");
            } catch {
              // bỏ qua
            }
          }}
        >
          <span>{label}</span>
          <span className="g15__count">{count}</span>
          <ChevronDown className="g15__chev" size={22} aria-hidden />
        </button>
      </h2>
      {!collapsed && <ul id={listId} className="g15__list">{children}</ul>}
    </section>
  );
}
