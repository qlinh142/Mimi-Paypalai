import { useMemo, useState } from "react";
import { CalendarDays, Check, Plus } from "lucide-react";
import { m15 } from "../i18n/m15";
import { t } from "../i18n";
import { ApiError } from "../api/client";
import { usePlanCarts, usePlanItem, usePlanMeta, usePlans } from "../api/hooks";
import type { PlanMeta } from "../api/types";
import { iconOf, planIcon } from "../lib/planIcon";
import { planName, whenText } from "../screens/PlanList";
import { Sheet } from "./Sheet15";
import { useUndo } from "./AppShell";

const pad = (n: number) => String(n).padStart(2, "0");

/** Kế hoạch đang hiện cho người dùng: đang mở; Giỏ mua sắm (mặc định) chỉ hiện khi còn món (chốt 31/10 15:58). */
export function visiblePlans(list: PlanMeta[] | undefined): PlanMeta[] {
  return (list ?? []).filter((p) => p.status === "active" && (!p.default || p.item_count > 0));
}

/** Các khóa món đã nằm trong bất kỳ Kế hoạch nào đang hiện: để tô đậm nút giỏ hàng. */
export function useKeysInPlans(): Set<string> {
  const plans = usePlans();
  const list = visiblePlans(plans.data);
  const carts = usePlanCarts(list.map((p) => p.id));
  const sig = carts.map((c) => c.dataUpdatedAt).join(",");
  // eslint-disable-next-line react-hooks/exhaustive-deps
  return useMemo(() => new Set(carts.flatMap((c) => (c.data?.items ?? []).map((i) => i.key))), [sig]);
}

/**
 * Bảng "Thêm vào kế hoạch nào?" (góp ý 31/10 15:58): bấm Mua sắm ở một sản phẩm thì chọn một
 * Kế hoạch sẵn có, hoặc Tạo kế hoạch mới rồi thêm món vào đó.
 */
export function PlanPicker({ planKey, name, body, onClose }: { planKey: string; name: string; body?: { name?: string }; onClose: () => void }) {
  const plans = usePlans();
  const list = visiblePlans(plans.data);
  const carts = usePlanCarts(list.map((p) => p.id));
  const add = usePlanItem();
  const meta = usePlanMeta();
  const showUndo = useUndo();
  const [err, setErr] = useState<unknown>(null);
  const busy = add.isPending || meta.create.isPending;

  const put = (p: Pick<PlanMeta, "id" | "default" | "name">) =>
    add.mutate(
      { key: planKey, add: true, body, planId: p.id },
      {
        onSuccess: () => {
          onClose();
          showUndo({ message: m15.plan.addedTo(name, planName(p)), onUndo: () => add.mutate({ key: planKey, add: false, planId: p.id }) });
        },
        onError: setErr,
      },
    );

  const createNew = () => {
    const d = new Date();
    const n = m15.plan.newName(`${pad(d.getDate())}/${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`);
    meta.create.mutate(n, { onSuccess: (p) => put(p), onError: setErr });
  };

  return (
    <Sheet title={m15.plan.pickTitle} sub={m15.plan.pickSub(name)} onClose={onClose}>
      {plans.isPending && <p className="hint">{t.common.loading}</p>}
      <ul className="pick15list">
        {list.map((p, i) => {
          const c = carts[i]?.data;
          const has = (c?.items ?? []).some((x) => x.key === planKey);
          const Icon = iconOf(planIcon(p));
          return (
            <li key={p.id}>
              <button type="button" className="pick15list__row" aria-disabled={has || busy || undefined} onClick={() => !has && !busy && put(p)}>
                <span className="plancard15__icon"><Icon size={22} aria-hidden /></span>
                <span className="pick15list__text">
                  <strong>{planName(p)}</strong>
                  <small>
                    {m15.plan.count(c?.item_count ?? p.item_count)} · <CalendarDays size={13} aria-hidden /> {whenText(c?.state)}
                  </small>
                </span>
                {has && <span className="tag15"><Check size={14} aria-hidden /> {m15.plan.pickHas}</span>}
              </button>
            </li>
          );
        })}
        <li>
          <button type="button" className="pick15list__row pick15list__row--new" aria-disabled={busy || undefined} onClick={() => !busy && createNew()}>
            <span className="plancard15__icon"><Plus size={22} aria-hidden /></span>
            <span className="pick15list__text"><strong>{m15.plan.pickNew}</strong></span>
          </button>
        </li>
      </ul>
      {err != null && <p className="error" role="alert">{err instanceof ApiError ? err.message : t.common.genericError}</p>}
    </Sheet>
  );
}
