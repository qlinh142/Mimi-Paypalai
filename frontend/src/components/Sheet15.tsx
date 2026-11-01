import { useEffect, type ReactNode } from "react";
import { X } from "lucide-react";
import { m15 } from "../i18n/m15";

/** Bảng trượt từ dưới lên (Mimi 1.5): tiêu đề, dòng phụ, nút đóng; bấm nền hoặc Esc để đóng. */
export function Sheet({ title, sub, onClose, children }: { title: string; sub: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div className="sheet15" role="presentation" onClick={onClose}>
      <div className="sheet15__panel" role="dialog" aria-modal="true" aria-label={title} onClick={(e) => e.stopPropagation()}>
        <span className="sheet15__grip" aria-hidden />
        <div className="sheet15__head">
          <div>
            <h2>{title}</h2>
            <p>{sub}</p>
          </div>
          <button type="button" className="icon-btn" aria-label={m15.plan.close} onClick={onClose}><X size={24} aria-hidden /></button>
        </div>
        {children}
      </div>
    </div>
  );
}
