/**
 * "Mimi nhận thấy" (đặc tả Contextual Insights 12/11): card nhỏ ngay dưới thông tin liên quan,
 * chạm "Xem phân tích" thì mở một bảng AG Studio nhỏ đúng ngữ cảnh. Không pop-up tự bật,
 * không chặn thao tác; ẩn rồi thì không hiện lại khi số liệu chưa đổi.
 */
import { Component, lazy, Suspense, useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { AlertTriangle, ChevronLeft, ChevronRight, Sparkles, X } from "lucide-react";
import { MimiMark } from "../components/MimiMark";
import { formatDayMonth } from "../lib/text";
import type { ReportSpec } from "./reports";
import type { Severity } from "./rules";
import { it } from "./text";
import "./insights.css";

const StudioView = lazy(() => import("../analytics/AnalyticsScreen").then((m) => ({ default: m.MiniStudio })));

const HIDDEN = "mimi.insights.hidden.v1";
const hidden = (): string[] => {
  try {
    return JSON.parse(localStorage.getItem(HIDDEN) ?? "[]");
  } catch {
    return [];
  }
};
export function useHidden(id: string | undefined) {
  const [, bump] = useState(0);
  const isHidden = !!id && hidden().includes(id);
  const hide = () => {
    if (!id) return;
    try {
      localStorage.setItem(HIDDEN, JSON.stringify([...hidden().slice(-49), id]));
    } catch { /* trình duyệt chặn lưu: chỉ ẩn trong phiên này */ }
    bump((n) => n + 1);
  };
  return { isHidden, hide };
}

/** "**Sữa** tăng **12%**" → in đậm phần trong dấu sao. */
function bold(text: string) {
  return text.split(/\*\*(.+?)\*\*/).map((p, i) => (i % 2 ? <b key={i}>{p}</b> : p));
}

export function InsightCard({ id, severity, text, sub, onOpen }: { id: string; severity: Severity; text: string; sub?: string; onOpen: () => void }) {
  const { isHidden, hide } = useHidden(id);
  if (isHidden) return null;
  return (
    <section className={`ins22 ins22--${severity}`} aria-label={it.header} data-insight={id}>
      <div className="ins22__head">
        <span className="ins22__avatar">
          <MimiMark size={36} animated label={null} />
          <span className="ins22__badge">{severity === "warning" ? <AlertTriangle size={12} aria-hidden /> : <Sparkles size={12} aria-hidden />}</span>
        </span>
        <span className="ins22__label">{it.header}</span>
        <button type="button" className="ins22__x" aria-label={it.dismiss} onClick={hide}><X size={16} aria-hidden /></button>
      </div>
      <p className="ins22__text">{bold(text)}</p>
      {sub && <p className="ins22__sub">{sub}</p>}
      <button type="button" className="ins22__cta" onClick={onOpen}>
        {it.open} <ChevronRight size={18} aria-hidden />
      </button>
    </section>
  );
}

class Boundary extends Component<{ children: ReactNode; fallback: (retry: () => void) => ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? this.props.fallback(() => this.setState({ failed: false })) : this.props.children;
  }
}

/** Bảng phân tích nhỏ, toàn màn trên điện thoại, có nút quay lại đúng chỗ đang làm. */
export function InsightReport({ spec, onClose }: { spec: ReportSpec | null; onClose: () => void }) {
  const [ready, setReady] = useState(false);
  const [attempt, setAttempt] = useState(0);
  // AG Studio cuộn trong khung cố định; đo chiều cao thật của bố cục để cả bảng cuộn một lần, không có thanh cuộn lồng.
  const box = useRef<HTMLDivElement>(null);
  const [fit, setFit] = useState<number | null>(null);
  useEffect(() => {
    if (!ready || !box.current) return;
    const el = box.current;
    let width = -1;
    const measure = () => {
      const l = el.querySelector<HTMLElement>(".ag-studio-layout");
      // Chỉ đo lại khi đổi bề ngang (xoay máy); đo khi đổi chiều cao thì khung tự lớn mãi.
      if (!l || el.clientWidth === width) return;
      width = el.clientWidth;
      if (l.scrollHeight > 0) setFit(l.scrollHeight + 8);
    };
    const t = setTimeout(measure, 150);
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => { clearTimeout(t); ro.disconnect(); };
  }, [ready]);
  useEffect(() => {
    if (!spec) return;
    setReady(false);
    setFit(null);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
    };
  }, [spec, onClose]);
  if (!spec) return null;
  const empty = spec.data.sources.every((s) => { const d = (s as { data?: unknown }).data; return !Array.isArray(d) || d.length === 0; });
  return createPortal(
    <div className="insr22" role="dialog" aria-modal="true" aria-label={spec.title}>
      <header className="insr22__bar">
        <button type="button" className="icon-btn" aria-label={it.back} onClick={onClose}><ChevronLeft size={26} aria-hidden /></button>
        <div className="insr22__titles">
          <h2>{spec.title}</h2>
          <p>{spec.sub}</p>
        </div>
      </header>
      <div className="insr22__body">
        {empty ? (
          <p className="insr22__state">{it.noData}</p>
        ) : (
          <Boundary
            key={attempt}
            fallback={(retry) => (
              <div className="insr22__state" role="alert">
                <p>{it.error}</p>
                <button type="button" className="btn btn--outline" onClick={() => { setAttempt((n) => n + 1); retry(); }}>{it.retry}</button>
              </div>
            )}
          >
            {!ready && <p className="insr22__state" role="status">{it.loading}</p>}
            <div ref={box} className="insr22__studio" style={{ height: fit ?? spec.height, visibility: ready ? "visible" : "hidden" }}>
              <Suspense fallback={null}>
                <StudioView spec={spec} onReady={() => setReady(true)} />
              </Suspense>
            </div>
          </Boundary>
        )}
        {spec.note && <p className="insr22__note">{spec.note}</p>}
        {spec.asOf && <p className="insr22__note">{it.updated(formatDayMonth(spec.asOf))}</p>}
      </div>
      <footer className="insr22__foot">
        <button type="button" className="btn btn--primary btn--block btn--lg" onClick={onClose}>{it.back}</button>
      </footer>
    </div>,
    document.body,
  );
}
