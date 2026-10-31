import { ShoppingCart } from "lucide-react";
import { m15 } from "../i18n/m15";
import { usePlanItem } from "../api/hooks";
import { useUndo } from "./AppShell";

/** Nút giỏ hàng tròn: thêm vào Kế hoạch. Đã có trong Kế hoạch thì tô đặc. */
export function CartButton({ planKey, name, added, body }: { planKey: string; name: string; added: boolean; body?: { name?: string } }) {
  const edit = usePlanItem();
  const showUndo = useUndo();
  const on = added || edit.isSuccess;
  return (
    <button
      type="button"
      className="cart15"
      aria-pressed={on}
      aria-label={on ? m15.items.inPlan(name) : m15.items.addToPlan(name)}
      aria-disabled={edit.isPending || undefined}
      onClick={() => {
        if (on || edit.isPending) return;
        edit.mutate(
          { key: planKey, add: true, body },
          { onSuccess: () => showUndo({ message: m15.items.added(name), onUndo: () => edit.mutate({ key: planKey, add: false }, { onSuccess: () => edit.reset() }) }) },
        );
      }}
    >
      <ShoppingCart size={22} aria-hidden />
    </button>
  );
}
