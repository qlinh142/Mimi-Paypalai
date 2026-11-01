import { useState } from "react";
import { ShoppingCart } from "lucide-react";
import { m15 } from "../i18n/m15";
import { PlanPicker } from "./PlanPicker";

/**
 * Nút giỏ hàng tròn (góp ý 31/10 15:58): bấm thì mở bảng chọn Kế hoạch để thêm món.
 * Tô đặc khi món đã nằm trong một Kế hoạch nào đó; bấm vẫn mở bảng để thêm vào Kế hoạch khác.
 */
export function CartButton({ planKey, name, added, body }: { planKey: string; name: string; added: boolean; body?: { name?: string } }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        className="cart15"
        aria-pressed={added}
        aria-haspopup="dialog"
        aria-label={added ? m15.items.inPlan(name) : m15.items.addToPlan(name)}
        onClick={() => setOpen(true)}
      >
        <ShoppingCart size={22} aria-hidden />
      </button>
      {open && <PlanPicker planKey={planKey} name={name} body={body} onClose={() => setOpen(false)} />}
    </>
  );
}
