import { t } from "../i18n";
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { Link, NavLink, useNavigate } from "react-router-dom";
import { Bell, BookOpen, ChevronLeft, ClipboardList, Package, ScanLine, Settings } from "lucide-react";
import { m15 } from "../i18n/m15";
import { useHome } from "../api/hooks";
import { applyUpdate, useUpdateReady } from "../app/pwaUpdate";
import { useOnline } from "../lib/useOnline";
import { useQueue } from "../lib/offlineQueue";
import { MimiMark } from "./MimiMark";

/* ---------- Thanh trên ---------- */

/**
 * Mimi 1.5 (ảnh mẫu 31/10): không còn biểu tượng tài khoản (Đăng xuất nằm trong Cài đặt).
 * - main: logo Mimi bên trái (bấm về Home), chuông bên phải; tên màn chỉ đọc cho trình đọc màn hình.
 * - sub: nút quay lại, tên ở giữa, chuông bên phải (chi tiết món, thông báo...).
 */
export function TopBar({ title, variant = "main", showTitle = false }: { title?: string; variant?: "main" | "sub"; brand?: boolean; showTitle?: boolean }) {
  const home = useHome();
  const nav = useNavigate();
  const unread = home.data?.unread_notifications ?? 0;
  const bell = (
    <Link to="/notifications" className="icon-btn" aria-label={t.nav.notifications(unread)}>
      <Bell size={24} aria-hidden />
      {unread > 0 && <span className="badge" aria-hidden>{unread}</span>}
    </Link>
  );
  if (variant === "sub") {
    return (
      <header className="topbar topbar--sub">
        <button type="button" className="icon-btn" aria-label={m15.detail.back} onClick={() => (history.length > 1 ? nav(-1) : nav("/"))}>
          <ChevronLeft size={28} aria-hidden />
        </button>
        <h1>{title}</h1>
        {bell}
      </header>
    );
  }
  return (
    <header className="topbar">
      <Link to="/" className="topbar__logo" aria-label={t.nav.home}>
        <MimiMark size={44} label={null} />
      </Link>
      {title && <h1 className={showTitle ? "topbar__title" : "visually-hidden"}>{title}</h1>}
      <span className="topbar__spacer" />
      {bell}
    </header>
  );
}

/* ---------- Thanh dưới: năm mục theo spec ---------- */

const NAV = [
  { to: "/spending", label: m15.home.tiles.spending, Icon: BookOpen },
  { to: "/items", label: m15.home.tiles.items, Icon: Package },
  { to: "/scan", label: t.nav.scan, Icon: ScanLine, scan: true },
  { to: "/plan", label: m15.home.tiles.plan, Icon: ClipboardList },
  { to: "/settings", label: t.nav.settings, Icon: Settings },
];

export function BottomNav() {
  // Spec 1.5 mục 8: chấm đỏ ở Cài đặt khi còn việc chưa gửi.
  const waiting = useQueue().length > 0;
  return (
    <nav className="nav" aria-label={t.nav.label}>
      <ul>
        {NAV.map(({ to, label, Icon, scan }) => (
          <li key={to}>
            <NavLink to={to} className={scan ? "nav__scan" : undefined}>
              <span className="nav__ico"><Icon size={scan ? 24 : 22} aria-hidden />{to === "/settings" && waiting && <i className="nav__dot" aria-label={m15.sync.dot} />}</span>
              <span>{label}</span>
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}

/* ---------- Dải "Đang xem bản đã lưu" (spec mục 11) ---------- */

export function OfflineBanner() {
  const online = useOnline();
  if (online) return null;
  return (
    <p className="offline" role="status">
      {t.common.offline}
    </p>
  );
}

/* ---------- Có phiên bản mới ---------- */

export function UpdateBanner() {
  const ready = useUpdateReady();
  if (!ready) return null;
  return (
    <div className="update" role="status">
      <p>{t.common.updateReady}</p>
      <button type="button" onClick={applyUpdate}>{t.common.updateReload}</button>
    </div>
  );
}

/* ---------- Hoàn tác ---------- */

type UndoState = { message: string; onUndo: () => void } | null;
const UndoCtx = createContext<(s: NonNullable<UndoState>) => void>(() => {});
export const useUndo = () => useContext(UndoCtx);

export function UndoProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<UndoState>(null);
  const timer = useRef<number | undefined>(undefined);
  const show = useCallback((s: NonNullable<UndoState>) => {
    window.clearTimeout(timer.current);
    setState(s);
    timer.current = window.setTimeout(() => setState(null), 6000);
  }, []);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  return (
    <UndoCtx.Provider value={show}>
      {children}
      <div aria-live="polite">
        {state && (
          <div className="undo">
            <p>{state.message}</p>
            <button
              type="button"
              onClick={() => {
                state.onUndo();
                setState(null);
              }}
            >
              {t.common.undo}
            </button>
          </div>
        )}
      </div>
    </UndoCtx.Provider>
  );
}

/* ---------- Khung cho màn có thanh dưới ---------- */

export function MainFrame({ children, nav = true }: { children: ReactNode; nav?: boolean }) {
  return (
    <div className={nav ? "frame frame--nav" : "frame"}>
      <OfflineBanner />
      {children}
      {nav && <BottomNav />}
    </div>
  );
}
