import { m15 } from "../i18n/m15";
import { t } from "../i18n";
import { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { CalendarClock, ChevronLeft, ChevronRight, ClipboardList, Search, ShieldCheck, Store, UsersRound } from "lucide-react";
import { useReceiptDetail, useReceiptHistory } from "../api/hooks";
import { LoadState } from "../components/LoadState";
import type { HistoryReceipt } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { CategoryIcon } from "../components/ItemRow";
import { formatVnd } from "../lib/text";
import { useRelations } from "../lib/relation";

const pad = (n: number) => String(n).padStart(2, "0");
/** Giờ Việt Nam của một mốc ISO: "14/10 · 18:15". */
export function dayTime(iso: string) {
  const d = new Date(new Date(iso).getTime() + 7 * 3_600_000);
  return `${pad(d.getUTCDate())}/${pad(d.getUTCMonth() + 1)} · ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
}
const monthKey = (iso: string) => {
  const d = new Date(new Date(iso).getTime() + 7 * 3_600_000);
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}`;
};
const fold = (s: string) => s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLowerCase();

/** Người mua hộ hiện bằng tên gọi người dùng đã đặt (Anh, Bố...), không có thì tên hiển thị. */
export function receiptTitle(r: HistoryReceipt, relationOf: (uid: string) => string = () => "") {
  const rel = r.buyer_uid ? relationOf(r.buyer_uid) : "";
  const buyer = rel ? m15.settings.relations[rel] : (r.buyer_name ?? "");
  return r.kind === "delegated" ? m15.history.delegated(buyer, r.plan_name ?? "") : r.store;
}

const vndOrNone = (v: number | null, on?: string) => (v == null ? m15.plan.noPrice : formatVnd(v, on));

/** Một dòng hóa đơn (hoặc khoản định kỳ đã trả, góp ý 01/11): dùng ở Sổ chi tiêu (tóm tắt) và màn 4c. */
export function ReceiptRow({ r }: { r: HistoryReceipt }) {
  const rec = r.kind === "recurring";
  const { relationOf } = useRelations();
  const Icon = rec ? CalendarClock : r.kind === "delegated" ? UsersRound : Store;
  const to = rec ? `/items/recurring/${encodeURIComponent(r.recurring_id ?? "")}` : `/spending/receipt/${encodeURIComponent(r.receipt_id)}`;
  return (
    <li>
      <Link to={to} className="ph15__row">
        <span className="ph15__icon"><Icon size={22} aria-hidden /></span>
        <span className="rows15__text">
          <span>{receiptTitle(r, relationOf)}{rec && <span className="tag15 ph15__rectag">{m15.history.recurringTag}</span>}</span>
          <small>{r.kind === "delegated" ? `${r.store} · ` : ""}{rec ? dayTime(r.purchased_at).split(" · ")[0] : dayTime(r.purchased_at)}</small>
        </span>
        <span className="num">{formatVnd(r.total_vnd, r.purchased_at)}</span>
        <ChevronRight size={18} aria-hidden />
      </Link>
    </li>
  );
}

/**
 * Màn 4c · Lịch sử mua hàng (Spec 1.5 mục 4, ảnh `4c-4d-lich-su-chi-tiet-hoa-don.png`):
 * tìm theo nơi mua hoặc món, lọc cửa hàng, chọn tháng; mỗi tháng một khối 3 hóa đơn mới nhất và "Xem thêm".
 */
export function PurchaseHistory() {
  const nav = useNavigate();
  const h = useReceiptHistory();
  const { relationOf } = useRelations();
  const [q, setQ] = useState("");
  const [store, setStore] = useState("");
  const [month, setMonth] = useState("");
  const [showFilter, setShowFilter] = useState(false);
  const [open, setOpen] = useState<Record<string, boolean>>({});
  const all = h.data ?? [];
  const stores = [...new Set(all.map((r) => r.store))];
  const months = [...new Set(all.map((r) => monthKey(r.purchased_at)))];
  const list = useMemo(() => {
    const f = fold(q.trim());
    return all
      .filter((r) => !f || fold(r.store).includes(f) || r.item_names.some((n) => fold(n).includes(f)) || fold(receiptTitle(r, relationOf)).includes(f))
      .filter((r) => !store || r.store === store)
      .filter((r) => !month || monthKey(r.purchased_at) === month)
      .sort((a, b) => b.purchased_at.localeCompare(a.purchased_at));
  }, [all, q, store, month]);
  const groups = months
    .map((m) => ({ m, rows: list.filter((r) => monthKey(r.purchased_at) === m) }))
    .filter((g) => g.rows.length > 0);
  const label = (m: string) => {
    const [y, mm] = m.split("-").map(Number);
    return m15.spending.month(mm, y);
  };

  return (
    <MainFrame>
      <TopBar title={m15.history.title} />
      <div className="page p15 ph15">
        <div className="pd15__head ph15__head">
          <button type="button" className="pd15__back" aria-label={m15.detail.back} onClick={() => nav("/spending")}>
            <ChevronLeft size={28} aria-hidden />
          </button>
          <h1 className="run15__title">{m15.history.title}</h1>
        </div>
        <div className="ph15__search">
          <label className="ph15__q">
            <Search size={18} aria-hidden />
            <input value={q} placeholder={m15.history.search} aria-label={m15.history.search} onChange={(e) => setQ(e.target.value)} />
          </label>
          <button type="button" className="btn btn--outline" aria-expanded={showFilter} onClick={() => setShowFilter(!showFilter)}>{m15.history.filter}</button>
        </div>
        {showFilter && (
          <label className="ph15__sel">
            {m15.history.store}
            <select value={store} onChange={(e) => setStore(e.target.value)}>
              <option value="">{m15.history.allStores}</option>
              {stores.map((s) => <option key={s} value={s}>{s}</option>)}
            </select>
          </label>
        )}
        <label className="ph15__sel">
          <span className="sr-only">{m15.history.month}</span>
          <select value={month} onChange={(e) => setMonth(e.target.value)}>
            <option value="">{m15.history.allMonths}</option>
            {months.map((m) => <option key={m} value={m}>{label(m)}</option>)}
          </select>
        </label>

        <LoadState q={h} hasData={!!h.data} />
        {h.data && groups.length === 0 && <p className="hint">{m15.history.empty}</p>}
        {groups.map(({ m, rows }) => {
          const shown = open[m] ? rows : rows.slice(0, 3);
          return (
            <section key={m} className="box15 ph15__month" aria-label={label(m)}>
              <h2>{m15.history.monthHead(label(m), rows.filter((r) => r.kind !== "recurring").length, rows.filter((r) => r.kind === "recurring").length)}</h2>
              <ul className="ph15__list">{shown.map((r) => <ReceiptRow key={r.receipt_id} r={r} />)}</ul>
              {rows.length > 3 && !open[m] && (
                <button type="button" className="link15 ph15__more" onClick={() => setOpen({ ...open, [m]: true })}>
                  {m15.history.more(rows.length - 3)} <ChevronRight size={16} aria-hidden />
                </button>
              )}
            </section>
          );
        })}
      </div>
    </MainFrame>
  );
}

/** Màn 4d · Chi tiết hóa đơn: chỉ xem, không có ảnh bill gốc, thông tin cá nhân đã lọc. */
export function ReceiptDetailScreen() {
  const { id = "" } = useParams();
  const nav = useNavigate();
  const dq = useReceiptDetail(id);
  const r = dq.data;
  const { relationOf } = useRelations();
  return (
    <MainFrame>
      <TopBar title={m15.history.detail} />
      <div className="page p15 ph15">
        <div className="pd15__head ph15__head">
          <button type="button" className="pd15__back" aria-label={m15.detail.back} onClick={() => nav(-1)}>
            <ChevronLeft size={28} aria-hidden />
          </button>
          <h1 className="run15__title">{m15.history.detail}</h1>
        </div>
        {!r ? (
          <LoadState q={dq} hasData={false} notFound={m15.history.gone} />
        ) : (
          <>
            <section className="box15 ph15__store">
              <span className="ph15__icon">{r.kind === "delegated" ? <UsersRound size={24} aria-hidden /> : <Store size={24} aria-hidden />}</span>
              <span className="rows15__text">
                <strong>{receiptTitle(r, relationOf)}</strong>
                <small>{r.kind === "delegated" ? `${r.store} · ` : ""}{dayTime(r.purchased_at)}</small>
              </span>
            </section>
            {r.plans.length > 0 && (
              <Link to={`/plan/${encodeURIComponent(r.plans[0].id)}`} className="box15 ph15__plans">
                <ClipboardList size={20} aria-hidden />
                <span>{m15.history.plans(r.plans.map((p) => p.name).join(", "))}</span>
                <ChevronRight size={18} aria-hidden />
              </Link>
            )}
            <section aria-labelledby="rd-lines">
              <h2 id="rd-lines" className="h15sec">{m15.history.lines(r.lines.length)}</h2>
              <ul className="lines15">
                {r.lines.map((l, i) => (
                  <li key={i} className="line15">
                    <div className="line15__row">
                      <CategoryIcon category={l.category_key ?? ""} />
                      <span className="line15__text">
                        <span className="line15__name">
                          {l.name}{l.spec ? ` ${l.spec}` : ""}
                          {l.tag && <span className={`tag15 ${l.tag === "mine" ? "tag15--buyer" : "tag15--bought"}`}>{l.tag === "mine" ? m15.run.mineShort : m15.run.forOwner(r.split?.owner_name ?? "")}</span>}
                          {l.original_name && <span className="name15__orig">{l.original_name}</span>}
                        </span>
                        <span className="line15__sub">
                          {l.qty} × {vndOrNone(l.unit_price, r.purchased_at)}
                          {l.price_source === "estimate" && <span className="tag15 tag15--draft">{m15.plan.estimated}</span>}
                        </span>
                      </span>
                      <span className="num run15__total" style={l.amount == null ? { opacity: 0.55 } : undefined}>{vndOrNone(l.amount, r.purchased_at)}</span>
                    </div>
                  </li>
                ))}
              </ul>
            </section>
            <section className="box15 ph15__sum">
              <p><span>{m15.history.subtotal}</span><span className="num">{formatVnd(r.subtotal, r.purchased_at)}</span></p>
              {r.discount > 0 && <p className="ph15__disc"><span>{m15.history.discount}</span><span className="num">−{formatVnd(r.discount, r.purchased_at)}</span></p>}
              <p className="ph15__total"><span>{m15.history.total}</span><span className="num">{formatVnd(r.total_vnd, r.purchased_at)}</span></p>
              {(r.missing_price ?? 0) > 0 && <p className="hint">{t.confirm.noPriceCount(r.missing_price ?? 0)}</p>}
              {r.split && (
                <>
                  <p><span>{m15.run.forOwner(r.split.owner_name)}</span><span className="num">{formatVnd(r.split.for_owner, r.purchased_at)}</span></p>
                  <p><span>{m15.run.mineShort}</span><span className="num">{formatVnd(r.split.mine, r.purchased_at)}</span></p>
                </>
              )}
            </section>
            <p className="hint ph15__privacy"><ShieldCheck size={16} aria-hidden /> {m15.history.privacy}</p>
          </>
        )}
      </div>
    </MainFrame>
  );
}
