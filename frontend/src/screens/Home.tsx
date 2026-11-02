import { t } from "../i18n";
import { m15 } from "../i18n/m15";
import { API_MODE } from "../lib/firebase";
import { Link } from "react-router-dom";
import { Bell } from "lucide-react";
import { useHome, useMe } from "../api/hooks";
import { MainFrame } from "../components/AppShell";
import { MimiMark } from "../components/MimiMark";
import { formatVnd } from "../lib/text";
import { loadErrorText } from "../components/LoadState";

/**
 * Home (Mimi 1.5, ảnh mẫu 1-home.png): logo giữa, chuông phải, lời chào, 4 ô màu có hình,
 * nút tròn QUÉT HÓA ĐƠN. Không có thanh tab dưới; các màn khác bấm logo Mimi để về đây.
 */
export function Home({ entering = false }: { entering?: boolean }) {
  const me = useMe();
  const home = useHome();
  const h = home.data;
  const unread = h?.unread_notifications ?? 0;

  const tiles = [
    {
      to: "/spending", tone: "blue", Art: WalletArt, title: m15.home.tiles.spending,
      meta: h ? (h.spent_this_month != null ? m15.home.spentMonth(formatVnd(h.spent_this_month)) : m15.home.noSpend) : "",
    },
    { to: "/items", tone: "green", Art: BasketArt, title: m15.home.tiles.items, meta: h ? m15.home.low(h.needs_count) : "" },
    { to: "/plan", tone: "orange", Art: CartArt, title: m15.home.tiles.plan, meta: h ? m15.home.toBuy(h.plan.item_count) : "" },
    { to: "/settings", tone: "purple", Art: GearArt, title: m15.home.tiles.settings, meta: m15.home.settingsMeta },
  ];

  return (
    <MainFrame nav={false}>
      <div className="h15">
        <header className="h15__top">
          <span />
          <h1 className="h15__brand">
            <span>Mimi AI</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="#FF8A3D" aria-hidden="true">
              <path d="M12 2l2.2 6.3L21 10l-6.8 1.7L12 18l-2.2-6.3L3 10l6.8-1.7z" />
            </svg>
            <MimiMark size={48} animated label={null} />
          </h1>
          <Link to="/notifications" className="h15__bell" aria-label={t.nav.notifications(unread)}>
            <Bell size={22} aria-hidden />
            {unread > 0 && <span className="h15__dot" aria-hidden />}
          </Link>
        </header>

        <Link to="/today" className="h15__bubble">
          {t.home.bubble} <small>{t.home.comingSoon}</small>
        </Link>

        <div className="h15__hello">
          <p className="h15__greet">{t.home.greet(me.data?.display_name)}</p>
          <p className="h15__tagline">{m15.home.tagline}</p>
        </div>

        {/* A10: không tải được số liệu thì báo, ô vẫn bấm được. */}
        {home.isError && !h && (
          <p className="hint" role="alert">
            {loadErrorText(home.error)}{" "}
            <button type="button" className="link15" onClick={() => home.refetch()}>{t.common.retry}</button>
          </p>
        )}
        <ul className={entering ? "h15__tiles tiles--enter" : "h15__tiles"}>
          {tiles.map(({ to, tone, Art, title, meta }) => (
            <li key={to}>
              <Link to={to} className={`h15__tile h15__tile--${tone} tile`}>
                <Art />
                <span className="h15__title">{title}</span>
                <span className="h15__meta">{meta}</span>
              </Link>
            </li>
          ))}
        </ul>

        <Link to="/scan" className="h15__scan" aria-label={m15.home.scan}>
          <svg viewBox="0 0 200 200" className="h15__ring" aria-hidden="true">
            <defs>
              <path id="h15-arc" d="M 30 100 A 70 70 0 0 1 170 100" />
            </defs>
            <text>
              <textPath href="#h15-arc" startOffset="50%" textAnchor="middle">{m15.home.scan}</textPath>
            </text>
          </svg>
          <span className="h15__btn">
            <svg width="44" height="44" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2" strokeLinecap="round" aria-hidden="true">
              <path d="M4 8V6a2 2 0 0 1 2-2h2M16 4h2a2 2 0 0 1 2 2v2M20 16v2a2 2 0 0 1-2 2h-2M8 20H6a2 2 0 0 1-2-2v-2M8 12h8" />
            </svg>
          </span>
        </Link>
        {API_MODE === "mock" && <p className="h15__proto"><span className="proto">{t.common.prototype}</span></p>}
      </div>
    </MainFrame>
  );
}

/* Hình minh họa nhỏ cho 4 ô (vẽ tay bằng SVG, không dùng ảnh ngoài). */
function WalletArt() {
  return (
    <svg className="h15__art" viewBox="0 0 64 56" aria-hidden="true">
      <rect x="4" y="12" width="46" height="34" rx="6" fill="#3B6CF6" />
      <rect x="4" y="12" width="46" height="9" rx="4" fill="#6E93FF" />
      <rect x="34" y="27" width="24" height="12" rx="6" fill="#fff" />
      <circle cx="41" cy="33" r="3" fill="#FF8A3D" />
      <circle cx="54" cy="10" r="7" fill="#FFC542" stroke="#FF8A3D" strokeWidth="2" />
    </svg>
  );
}
function BasketArt() {
  return (
    <svg className="h15__art" viewBox="0 0 64 56" aria-hidden="true">
      <circle cx="20" cy="16" r="8" fill="#F26B4F" />
      <circle cx="32" cy="11" r="8" fill="#FFC542" />
      <circle cx="44" cy="16" r="8" fill="#7FD07A" />
      <rect x="6" y="20" width="52" height="7" rx="3" fill="#2E9D57" />
      <path d="M10 27h44l-5 22H15z" fill="#3DB36A" />
    </svg>
  );
}
function CartArt() {
  return (
    <svg className="h15__art" viewBox="0 0 64 56" aria-hidden="true">
      <path d="M4 8h9l7 30h30l6-22H17" fill="none" stroke="#FF7A2F" strokeWidth="4" strokeLinejoin="round" strokeLinecap="round" />
      <rect x="24" y="12" width="10" height="16" rx="2" fill="#FFC542" />
      <rect x="36" y="9" width="10" height="19" rx="2" fill="#3B6CF6" />
      <circle cx="24" cy="47" r="5" fill="#FF7A2F" />
      <circle cx="46" cy="47" r="5" fill="#FF7A2F" />
    </svg>
  );
}
function GearArt() {
  return (
    <svg className="h15__art" viewBox="0 0 64 56" aria-hidden="true">
      <g transform="translate(32 28)" fill="#8B5CF6">
        {Array.from({ length: 8 }, (_, i) => (
          <rect key={i} x="-4" y="-26" width="8" height="12" rx="2" transform={`rotate(${i * 45})`} />
        ))}
        <circle r="17" />
        <circle r="8" fill="#EDE7FF" />
      </g>
    </svg>
  );
}
