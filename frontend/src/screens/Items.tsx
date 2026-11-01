import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Pencil, Search } from "lucide-react";
import { OfflineError } from "../api/client";
import { useItems } from "../api/hooks";
import type { Item } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { CartButton } from "../components/CartButton";
import { useKeysInPlans } from "../components/PlanPicker";
import { Group15 } from "../components/Group15";
import { CategoryIcon } from "../components/ItemRow";
import { RecurringList } from "../components/RecurringList";
import { formatNum, goodsTier, stockInDisplay, unitLabel, type GoodsTier } from "../lib/displayUnit";
import { normalizeVi } from "../lib/text";

const TIERS: { key: GoodsTier; label: string }[] = [
  { key: "now", label: m15.items.needNow },
  { key: "soon", label: m15.items.soon },
  { key: "plenty", label: m15.items.plenty },
];

/**
 * Đồ dùng (Mimi 1.5, ảnh mẫu 3-do-dung-va-chi-tiet.png và 3-khoan-chi-dinh-ky.png):
 * hai nút Hàng tiêu dùng / Khoản chi định kỳ, ô tìm kiếm, ba nhóm màu. Nút giỏ hàng tròn
 * thêm món vào Kế hoạch (giống "thêm vào giỏ").
 */
export function Items() {
  const [params, setParams] = useSearchParams();
  const tier1 = params.get("tab") === "recurring" ? "recurring" : "goods";
  const [term, setTerm] = useState("");

  return (
    <MainFrame>
      <TopBar title={t.items.title} />
      <div className="page p15">
        <div className="seg seg15" role="group" aria-label={t.items.typeGroup}>
          <button type="button" aria-pressed={tier1 === "goods"} onClick={() => { setParams({}, { replace: true }); setTerm(""); }}>
            {m15.items.goods}
          </button>
          <button type="button" aria-pressed={tier1 === "recurring"} onClick={() => { setParams({ tab: "recurring" }, { replace: true }); setTerm(""); }}>
            {m15.items.recurring}
          </button>
        </div>

        <label className="search15">
          <Search size={22} aria-hidden />
          <span className="visually-hidden">{t.items.search}</span>
          <input
            type="search"
            enterKeyHint="search"
            autoComplete="off"
            spellCheck={false}
            placeholder={tier1 === "goods" ? m15.items.searchGoods : m15.items.searchRecurring}
            value={term}
            onChange={(e) => setTerm(e.target.value)}
          />
        </label>

        {tier1 === "goods" ? <Goods term={term} /> : <RecurringList term={term} />}
      </div>
    </MainFrame>
  );
}

function Goods({ term }: { term: string }) {
  const items = useItems({ tab: "active", horizon: 7 });
  const inPlan = useKeysInPlans();

  const groups = useMemo(() => {
    const q = normalizeVi(term);
    const list = (items.data ?? []).filter((i) => !q || normalizeVi(i.name).includes(q));
    const byDays = (a: Item, b: Item) => (a.days_left ?? 999) - (b.days_left ?? 999);
    return TIERS.map((tier) => ({ ...tier, items: list.filter((i) => goodsTier(i) === tier.key).sort(byDays) })).filter((g) => g.items.length > 0);
  }, [items.data, term]);

  if (items.isPending) {
    return (
      <div aria-busy="true" aria-label={t.common.loading}>
        {Array.from({ length: 4 }, (_, i) => <div key={i} className="skeleton" />)}
      </div>
    );
  }
  if (items.isError && !items.data) {
    return (
      <div className="empty">
        <h2>{t.items.loadFailedTitle}</h2>
        <p>{items.error instanceof OfflineError ? t.items.loadFailedBody : t.common.genericError}</p>
        <button className="btn btn--quiet" onClick={() => items.refetch()}>{t.common.retry}</button>
      </div>
    );
  }
  if (groups.length === 0) {
    return term ? (
      <div className="empty"><h2>{m15.items.noMatch(term)}</h2></div>
    ) : (
      <div className="empty">
        <p>{m15.items.empty}</p>
        <Link className="btn btn--primary" to="/scan">{t.home.scan}</Link>
      </div>
    );
  }
  return (
    <>
      {groups.map((g) => (
        <Group15 key={g.key} id={`goods-${g.key}`} label={g.label} tone={g.key} count={g.items.length}>
          {g.items.map((item) => (
            <GoodsCard key={item.key} item={item} added={inPlan.has(item.key)} />
          ))}
        </Group15>
      ))}
    </>
  );
}

function GoodsCard({ item, added }: { item: Item; added: boolean }) {
  const nav = useNavigate();
  const qty = stockInDisplay(item);
  const days = item.status === "waiting" || item.days_left == null ? null : Math.max(0, item.days_left);
  const sub = [
    qty ? `${formatNum(qty[0])} ${unitLabel(qty[1])}` : null,
    days == null ? m15.items.learning : m15.items.left(days),
  ].filter(Boolean).join(" · ");
  return (
    <li className="card15">
      <button type="button" className="card15__main" onClick={() => nav(`/items/${encodeURIComponent(item.key)}`)}>
        <span className="card15__icon"><CategoryIcon category={item.category_key} /></span>
        <span className="card15__text">
          <span className="card15__name">{item.name}</span>
          <span className="card15__sub">{sub}</span>
          <span className="card15__edit"><Pencil size={14} aria-hidden /> {m15.items.edit}</span>
        </span>
      </button>
      <CartButton planKey={item.key} name={item.name} added={added} />
    </li>
  );
}
