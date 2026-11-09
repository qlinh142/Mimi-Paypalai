import { s22 } from "../i18n/s22";
import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Package, Pencil, Repeat, Search, Tag } from "lucide-react";
import { OfflineError } from "../api/client";
import { useItems, useItemsMarketPrices } from "../api/hooks";
import type { Item } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { CartButton } from "../components/CartButton";
import { useKeysInPlans } from "../components/PlanPicker";
import { Group15 } from "../components/Group15";
import { CategoryIcon } from "../components/ItemRow";
import { RecurringList } from "../components/RecurringList";
import { formatNum, goodsTier, stockInDisplay, unitLabel, type GoodsTier } from "../lib/displayUnit";
import { formatVnd, normalizeVi } from "../lib/text";

/** Chip lọc (Mimi 2.2, theo Redesign): Cần chú ý = Cần mua ngay + Sắp hết. Lưu trên đường dẫn ?f=. */
type Filter = "all" | "attention" | "plenty";
const FILTER_TIERS: Record<Filter, GoodsTier[]> = { all: ["now", "soon", "plenty"], attention: ["now", "soon"], plenty: ["plenty"] };

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
  const f = params.get("f");
  const filter: Filter = f === "attention" || f === "plenty" ? f : "all";
  const [term, setTerm] = useState("");

  return (
    <MainFrame>
      <TopBar title={t.items.title} sub={s22.heads.items} showTitle />
      <div className="page p15">
        <div className="seg seg15" role="group" aria-label={t.items.typeGroup}>
          <button type="button" aria-pressed={tier1 === "goods"} onClick={() => { setParams({}, { replace: true }); setTerm(""); }}>
            <Package size={18} aria-hidden /> {m15.items.goods}
          </button>
          <button type="button" aria-pressed={tier1 === "recurring"} onClick={() => { setParams({ tab: "recurring" }, { replace: true }); setTerm(""); }}>
            <Repeat size={18} aria-hidden /> {m15.items.recurring}
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

        {tier1 === "goods" ? (
          <Goods term={term} filter={filter} onFilter={(x) => setParams(x === "all" ? {} : { f: x }, { replace: true })} />
        ) : (
          <RecurringList term={term} />
        )}
      </div>
    </MainFrame>
  );
}

function Goods({ term, filter, onFilter }: { term: string; filter: Filter; onFilter: (f: Filter) => void }) {
  const items = useItems({ tab: "active", horizon: 7 });
  const inPlan = useKeysInPlans();
  const market = useItemsMarketPrices();
  // Giá rẻ nhất đang bán của từng món (bảng giá chung), cho dòng "Rẻ nhất" trên thẻ.
  const cheapest = useMemo(() => {
    const m = new Map<string, { price: number; store: string }>();
    for (const x of market.data?.items ?? []) {
      const best = [...x.prices].sort((a, b) => a.price_vnd - b.price_vnd)[0];
      if (best) m.set(x.key, { price: best.price_vnd, store: best.store });
    }
    return m;
  }, [market.data]);
  const attention = (items.data ?? []).filter((i) => goodsTier(i) !== "plenty").length;

  const groups = useMemo(() => {
    const q = normalizeVi(term);
    const list = (items.data ?? []).filter((i) => !q || normalizeVi(i.name).includes(q));
    const byDays = (a: Item, b: Item) => (a.days_left ?? 999) - (b.days_left ?? 999);
    return TIERS.filter((tier) => FILTER_TIERS[filter].includes(tier.key))
      .map((tier) => ({ ...tier, items: list.filter((i) => goodsTier(i) === tier.key).sort(byDays) }))
      .filter((g) => g.items.length > 0);
  }, [items.data, term, filter]);

  const chips = (items.data?.length ?? 0) > 0 && (
    <div className="chips22" role="group" aria-label={s22.items.filters}>
      {(["all", "attention", "plenty"] as const).map((k) => (
        <button key={k} type="button" aria-pressed={filter === k} onClick={() => onFilter(k)}>
          {k === "all" ? s22.items.all : k === "attention" ? s22.items.attention(attention) : s22.items.plenty}
        </button>
      ))}
    </div>
  );

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
    return term || filter !== "all" ? (
      <>
        {chips}
        <div className="nomatch22" role="status">
          <span className="nomatch22__ill"><Search size={26} aria-hidden /></span>
          <h2>{s22.items.noMatch}</h2>
          {term && <p>{s22.items.noMatchSub}</p>}
        </div>
      </>
    ) : (
      <div className="empty">
        <p>{m15.items.empty}</p>
        <Link className="btn btn--primary" to="/scan">{t.home.scan}</Link>
      </div>
    );
  }
  return (
    <>
      {chips}
      {groups.map((g) => (
        <Group15 key={g.key} id={`goods-${g.key}`} label={g.label} tone={g.key} count={g.items.length}>
          {g.items.map((item) => (
            <GoodsCard key={item.key} item={item} added={inPlan.has(item.key)} cheapest={cheapest.get(item.key)} />
          ))}
        </Group15>
      ))}
    </>
  );
}

function GoodsCard({ item, added, cheapest }: { item: Item; added: boolean; cheapest?: { price: number; store: string } }) {
  const nav = useNavigate();
  const qty = stockInDisplay(item);
  const days = item.status === "waiting" || item.days_left == null ? null : Math.max(0, item.days_left);
  // Mimi 2.2 (theo Redesign): số ngày còn lại tô màu trạng thái, kèm thanh tiến độ (ngày / 30).
  const tier = goodsTier(item);
  const qtyText = qty ? `${formatNum(qty[0])} ${unitLabel(qty[1])}` : null;
  return (
    <li className="card15">
      <button type="button" className="card15__main" onClick={() => nav(`/items/${encodeURIComponent(item.key)}`)}>
        <span className="card15__icon"><CategoryIcon category={item.category_key} /></span>
        <span className="card15__text">
          <span className="card15__name">{item.name}</span>
          <span className="card15__sub">
            {qtyText && <>{qtyText} · </>}
            {days == null ? m15.items.learning : <span className={`left22 left22--${tier}`}>{m15.items.left(days)}</span>}
          </span>
          {days != null && (
            <span className={`bar22 bar22--${tier}`} aria-hidden><i style={{ width: `${Math.max(6, Math.min(100, (days / 30) * 100))}%` }} /></span>
          )}
          {cheapest && <span className="cheap22"><Tag size={13} aria-hidden /> {s22.items.cheapest(formatVnd(cheapest.price), cheapest.store)}</span>}
          <span className="card15__edit"><Pencil size={14} aria-hidden /> {m15.items.edit}</span>
        </span>
      </button>
      <CartButton planKey={item.key} name={item.name} added={added} />
    </li>
  );
}
