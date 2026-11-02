import { m15 } from "../i18n/m15";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { CalendarDays, Camera, Check, ChevronLeft, MapPin, Minus, Pencil, Plus, ReceiptText, Trash2, UserRound } from "lucide-react";
import { qk, useAssignments, useDelegated } from "../api/hooks";
import { apiSendQueued } from "../api/client";
import { useQueryClient } from "@tanstack/react-query";
import type { Assignment } from "../api/types";
import { MainFrame, TopBar } from "../components/AppShell";
import { formatVnd } from "../lib/text";
import { useRelations } from "../lib/relation";
import { LoadState } from "../components/LoadState";
import { whenText } from "./PlanList";
import { AddToCalendar } from "../components/AddToCalendar";

/** Một dòng trên danh sách đi mua. `mine` = món khác trên hóa đơn (khối "Của tôi"). */
type RunLine = { key: string; name: string; qty: number; unit: string; price: number | null; ticked: boolean; fixedName: boolean; mine: boolean; category: string };
type Run = { lines: RunLine[]; note: string; scanned: boolean; version?: string };

const hhmm = (iso: string) => {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
};

/** Lưu tạm danh sách đang mua trên máy (Spec 1.5 mục 2 và 8): thoát app mở lại vẫn còn, chưa ghi gì trước Mua hàng xong. */
const KEY = (id: string) => `mimi.run.${id}`;
function freshLines(a: Assignment): RunLine[] {
  return a.items
    .filter((i) => i.status === "assigned")
    .map((i) => ({ key: i.consumption_group_key, name: i.name_display, qty: i.qty ?? 1, unit: i.unit ?? "", price: i.unit_price ?? null, ticked: false, fixedName: true, mine: false, category: i.category_key ?? "" }));
}
/** B5: người nhờ gửi bản cập nhật thì lấy danh sách theo bản mới. Món còn trong bản mới giữ dấu tick
 *  và chỗ đã sửa, món bị bỏ thì mất, món mới thêm vào. Khối "Của tôi" giữ nguyên. */
function loadRun(a: Assignment): Run {
  const version = a.updated_at ?? "";
  try {
    const v = localStorage.getItem(KEY(a.assignment_id));
    if (v) {
      const saved = JSON.parse(v) as Run;
      if ((saved.version ?? "") === version) return saved;
      const old = new Map(saved.lines.filter((l) => !l.mine).map((l) => [l.key, l]));
      const lines = freshLines(a).map((l) => {
        const o = old.get(l.key);
        return o ? { ...l, ticked: o.ticked, price: o.price ?? l.price, qty: o.ticked ? o.qty : l.qty } : l;
      });
      return { ...saved, version, lines: [...lines, ...saved.lines.filter((l) => l.mine)] };
    }
  } catch {
    // bỏ qua
  }
  return { note: "", scanned: false, version, lines: freshLines(a) };
}
function saveRun(id: string, r: Run | null) {
  try {
    if (r) localStorage.setItem(KEY(id), JSON.stringify(r));
    else localStorage.removeItem(KEY(id));
  } catch {
    // bỏ qua
  }
}

/** Hóa đơn mẫu cho prototype: khớp phần lớn món, đổi giá vài món, thêm món của người mua và một dòng đọc thừa. */
function mockScan(r: Run): Run {
  const bill: Record<string, number> = { "sua-tuoi": 31_500, "trung-ga": 36_000, "banh-mi": 5_000, "nuoc-rua-chen": 37_900 };
  const lines = r.lines.filter((l) => !l.mine).map((l) => (l.key in bill ? { ...l, ticked: true, price: bill[l.key] } : l));
  lines.push(
    { key: "x-nuoc-ngot", name: m15.run.billMine, qty: 1, unit: "", price: 15_000, ticked: true, fixedName: false, mine: true, category: "sua" },
    { key: "x-tui", name: m15.run.billBag, qty: 1, unit: "", price: 0, ticked: false, fixedName: false, mine: true, category: "" },
  );
  return { ...r, lines, scanned: true };
}

const lineTotal = (l: RunLine) => (l.price ?? 0) * l.qty;
const sum = (ls: RunLine[]) => ls.filter((l) => l.ticked).reduce((s, l) => s + lineTotal(l), 0);

/**
 * Màn đi mua cho người được nhờ (Spec 1.5 mục 2, ảnh `viec-3-di-mua-ho-chot.png`): danh sách mua có tick,
 * sửa số lượng và đơn giá, quét hóa đơn chia 2 khối "Mua hộ" / "Của tôi", ghi chú, Mua hàng xong, Từ chối.
 */
export function ShoppingRun() {
  const { id = "" } = useParams();
  const d = useDelegated();
  const a = d.data?.to_me.find((x) => x.assignment_id === id);
  const [doneView, setDoneView] = useState<DoneInfo | null>(null);
  if (doneView) return <Done {...doneView} />;
  // A10: lỗi tải (mất mạng, máy chủ lỗi) khác với lần nhờ mua không còn nữa.
  if (!a && (d.isPending || d.isError)) return <MainFrame><TopBar title={m15.plan.title} variant="sub" /><div className="page p15"><LoadState q={d} hasData={false} /></div></MainFrame>;
  if (!a) return <MainFrame><TopBar title={m15.plan.title} variant="sub" /><div className="page p15"><p className="hint">{m15.run.gone}</p></div></MainFrame>;
  return <RunList a={a} onDone={setDoneView} />;
}

type DoneInfo = { scanned: boolean; ask: number; mine: number; plan: string; owner: string; queued: boolean; left: number };

function RunList({ a, onDone }: { a: Assignment; onDone: (v: DoneInfo) => void }) {
  const nav = useNavigate();
  const { decline } = useAssignments();
  const qc = useQueryClient();
  const [finishing, setFinishing] = useState(false);
  const [run, setRun] = useState<Run>(() => loadRun(a));
  const [editing, setEditing] = useState<string | null>(null);
  const [scanState, setScanState] = useState<"" | "reading" | "offline">("");
  const [confirmDecline, setConfirmDecline] = useState(false);
  useEffect(() => saveRun(a.assignment_id, run), [a.assignment_id, run]);

  const { relationOf } = useRelations();
  const rel = relationOf(a.owner_uid);
  const owner = rel ? m15.settings.relations[rel] : a.owner_display_name;
  const planName = a.trip?.plan_name || m15.plan.main;
  const st = a.trip ? { buy_date: a.trip.buy_date ?? "", buy_hour: a.trip.buy_hour ?? null, buy_minute: a.trip.buy_minute ?? 0, trip_note: a.trip.trip_note ?? "" } : undefined;
  const ask = run.lines.filter((l) => !l.mine);
  const mine = run.lines.filter((l) => l.mine);
  const set = (key: string, patch: Partial<RunLine> | null) =>
    setRun((r) => ({ ...r, lines: patch ? r.lines.map((l) => (l.key === key ? { ...l, ...patch } : l)) : r.lines.filter((l) => l.key !== key) }));

  const scan = () => {
    if (!navigator.onLine) return setScanState("offline");
    setScanState("reading");
    window.setTimeout(() => {
      setRun((r) => mockScan(r));
      setScanState("");
    }, 900);
  };
  // Mua hàng xong: ghi một lần. Mất mạng thì lưu vào hộp thư đi, có mạng tự gửi (Spec 1.5 mục 2, 8).
  const finish = async () => {
    if (finishing) return;
    setFinishing(true);
    // Gửi đủ danh sách (cả món chưa tick) để máy chủ biết món nào trả về kế hoạch của người nhờ.
    const result = {
      scanned: run.scanned, note: run.note, store: st?.trip_note || undefined,
      lines: run.lines.map(({ key, name, qty, unit, price, ticked, mine, category }) => ({ key, name, qty, unit, price, ticked, mine, category })),
    };
    try {
      const r = await apiSendQueued("POST", `/households/shopping-assignments/${encodeURIComponent(a.assignment_id)}/complete`, result, m15.run.queueLabel(owner));
      saveRun(a.assignment_id, null);
      qc.invalidateQueries({ queryKey: qk.plan });
      // Câu 3: món trong kế hoạch chưa tick thì sang kế hoạch "· còn lại" của người nhờ.
      onDone({ scanned: run.scanned, ask: sum(ask), mine: sum(mine), plan: planName, owner, queued: r === "queued", left: ask.filter((l) => !l.ticked).length });
    } finally {
      setFinishing(false);
    }
  };

  const block = (title: string, ls: RunLine[]) => (
    <section aria-label={title}>
      {run.scanned && (
        <div className="run15__blockhead">
          <h2 className="h15sec">{title}</h2>
          <span className="num">{formatVnd(sum(ls))}</span>
        </div>
      )}
      <ul className="lines15">
        {ls.map((l) => (
          <Line key={l.key} l={l} open={editing === l.key} onToggle={() => setEditing(editing === l.key ? null : l.key)} onSet={(p) => set(l.key, p)} />
        ))}
      </ul>
    </section>
  );

  return (
    <MainFrame>
      <TopBar title={planName} />
      <div className="page p15 run15">
        <div className="pd15__head">
          <button type="button" className="pd15__back" aria-label={m15.detail.back} onClick={() => nav("/plan?tab=to-me")}>
            <ChevronLeft size={28} aria-hidden />
          </button>
          <h1 className="run15__title">{planName}</h1>
          <button
            type="button"
            className={`btn run15__decline${confirmDecline ? " pd15__del--confirm" : ""}`}
            onClick={() => {
              if (!confirmDecline) return setConfirmDecline(true);
              decline.mutate(a.assignment_id, { onSuccess: () => { saveRun(a.assignment_id, null); nav("/plan?tab=to-me", { replace: true }); } });
            }}
          >
            {confirmDecline ? m15.run.declineConfirm : m15.run.decline}
          </button>
        </div>
        {confirmDecline && <p className="hint" role="status">{m15.run.declineAsk(owner)}</p>}

        {a.updated_at && <p className="hint" role="status">{m15.run.updatedAt(owner, hhmm(a.updated_at))}</p>}

        <section className="box15 run15__sum">
          <span><ReceiptText size={18} aria-hidden /> {m15.plan.count(ask.length)}</span>
          <span><MapPin size={18} aria-hidden /> {st?.trip_note || m15.plan.noStore}</span>
          <span><CalendarDays size={18} aria-hidden /> {whenText(st)}</span>
          <span><UserRound size={18} aria-hidden /> {m15.run.sender(owner)}</span>
        </section>
        {st?.buy_date && (
          <AddToCalendar
            ev={{
              title: m15.plan.calTitle(planName),
              date: st.buy_date,
              time: st.buy_hour != null ? `${String(st.buy_hour).padStart(2, "0")}:${String(st.buy_minute).padStart(2, "0")}` : undefined,
              location: st.trip_note || undefined,
              details: m15.plan.calDetails(ask.slice(0, 20).map((l) => l.name)),
            }}
          />
        )}

        {block(m15.run.forOwner(owner, ask.length), ask)}
        {run.scanned && mine.length > 0 && block(m15.run.mine(mine.length), mine)}

        <label className="run15__note">
          {m15.run.note}
          <textarea rows={2} value={run.note} maxLength={200} placeholder={m15.run.notePh} onChange={(e) => setRun((r) => ({ ...r, note: e.target.value }))} />
        </label>

        {scanState === "offline" && (
          <div className="run15__offline" role="alert">
            <p>{m15.run.offline}</p>
            <button type="button" className="btn btn--outline" onClick={() => setScanState("")}>{m15.run.manual}</button>
          </div>
        )}
        {scanState === "reading" && <p className="hint" role="status">{m15.run.reading}</p>}

        <div className="pd15__actions">
          <button type="button" className="btn btn--outline btn--lg" aria-disabled={scanState === "reading" || undefined} onClick={scan}>
            <Camera size={18} aria-hidden /> {m15.run.scan}
          </button>
          <button type="button" className="btn btn--primary btn--lg" aria-disabled={finishing || undefined} onClick={finish}>
            <Check size={18} aria-hidden /> {m15.run.finish}
          </button>
        </div>
        <p className="hint">{m15.run.hint}</p>
        <p className="hint run15__saved" role="status">{m15.run.savedLocal}</p>
      </div>
    </MainFrame>
  );
}

function Line({ l, open, onToggle, onSet }: { l: RunLine; open: boolean; onToggle: () => void; onSet: (p: Partial<RunLine> | null) => void }) {
  const [name, setName] = useState(l.name);
  const [qty, setQty] = useState(l.qty);
  const [price, setPrice] = useState(l.price != null ? String(l.price) : "");
  useEffect(() => {
    setName(l.name);
    setQty(l.qty);
    setPrice(l.price != null ? String(l.price) : "");
  }, [l, open]);
  const sub = l.price != null ? `${l.qty} × ${formatVnd(l.price)}` : `${l.qty}${l.unit ? ` ${l.unit}` : ""} · ${m15.plan.noPrice}`;
  return (
    <li className={`line15 run15__line${l.ticked ? " run15__line--on" : ""}`}>
      <div className="line15__row">
        <input type="checkbox" className="run15__tick" checked={l.ticked} aria-label={m15.run.tick(l.name)} onChange={(e) => onSet({ ticked: e.target.checked })} />
        <span className="line15__text">
          <span className="line15__name">{l.name}</span>
          <span className="line15__sub">{sub}</span>
        </span>
        <span className="num run15__total">{l.price != null ? formatVnd(lineTotal(l)) : "–"}</span>
        <button type="button" className="icon-btn" aria-expanded={open} aria-label={m15.plan.editLine(l.name)} onClick={onToggle}>
          <Pencil size={20} aria-hidden />
        </button>
      </div>
      {open && (
        <div className="run15__edit">
          {/* Món đã chuẩn hóa không sửa tên (chốt 01/11). */}
          <div className="run15__editrow">
            <input className="run15__name" value={name} disabled={l.fixedName} aria-label={m15.run.name} onChange={(e) => setName(e.target.value)} />
            <button type="button" className="icon15 icon15--ok" aria-label={m15.plan.saveLine(l.name)} onClick={() => { onSet({ name: name.trim() || l.name, qty, price: price ? Number(price) : null }); onToggle(); }}>
              <Check size={22} aria-hidden />
            </button>
            <button type="button" className="icon15 icon15--del" aria-label={m15.plan.removeLineOf(l.name)} onClick={() => onSet(null)}>
              <Trash2 size={22} aria-hidden />
            </button>
          </div>
          <div className="run15__editrow">
            <div className="stepper15">
              <button type="button" aria-label={m15.detail.minus} onClick={() => qty > 1 && setQty(qty - 1)}><Minus size={18} aria-hidden /></button>
              <input readOnly value={qty} aria-label={m15.detail.qtyTitle} />
              <button type="button" aria-label={m15.detail.plus} onClick={() => setQty(qty + 1)}><Plus size={18} aria-hidden /></button>
            </div>
            <label className="run15__pricewrap">
              <span>{m15.run.unitPrice}</span>
              <input className="run15__price" inputMode="numeric" value={price} placeholder="0" onChange={(e) => setPrice(e.target.value.replace(/\D/g, ""))} />
            </label>
          </div>
          {l.fixedName && <p className="hint">{m15.run.fixedName}</p>}
        </div>
      )}
    </li>
  );
}

function Done({ scanned, ask, mine, plan, owner, queued, left }: DoneInfo) {
  const nav = useNavigate();
  const total = useMemo(() => ask + mine, [ask, mine]);
  return (
    <MainFrame>
      <TopBar title={plan} />
      <div className="page p15 run15">
        <section className="box15 run15__done">
          <Check size={40} aria-hidden className="run15__doneicon" />
          <h1>{m15.run.doneTitle}</h1>
          <span className="tag15">{scanned ? m15.run.withBill : m15.run.manualTag}</span>
          <p>{m15.run.linked(plan)}</p>
          <p className="num run15__big">{formatVnd(total)}</p>
          {scanned && (
            <>
              <p className="run15__split"><span>{m15.run.forOwner(owner)}</span><span className="num">{formatVnd(ask)}</span></p>
              <p className="run15__split"><span>{m15.run.mineShort}</span><span className="num">{formatVnd(mine)}</span></p>
            </>
          )}
          {queued ? (
            <p className="run15__offline" role="status">{m15.run.queued(owner)}</p>
          ) : (
            <p className="sent15" role="status">{m15.run.sentTo(owner)}</p>
          )}
          {left > 0 && <p className="hint">{m15.run.leftover(left, plan, owner)}</p>}
        </section>
        <button type="button" className="btn btn--primary btn--lg btn--block" onClick={() => nav("/plan?tab=to-me")}>{m15.run.backToPlans}</button>
      </div>
    </MainFrame>
  );
}
