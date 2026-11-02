import { t } from "../i18n";
import { Link } from "react-router-dom";
import { CalendarClock, ChevronRight, Users } from "lucide-react";
import { useMarkNotificationRead, useNotifications } from "../api/hooks";
import type { HouseEvent, Notification } from "../api/types";
import { m15 } from "../i18n/m15";
import { MainFrame, TopBar } from "../components/AppShell";

function todayIso(offset = 0): string {
  const d = new Date();
  d.setDate(d.getDate() + offset);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

const hhmm = (iso: string) => {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
};

function eventText(e: HouseEvent, who: string): string {
  const plan = e.plan_name || m15.plan.main;
  const ev = m15.events;
  switch (e.kind) {
    case "plan_sent": return ev.sent(who, e.items ?? 0, plan);
    case "plan_updated": return ev.updated(who, hhmm(e.at), plan);
    case "plan_cancelled": return ev.cancelled(who, hhmm(e.at), plan);
    case "run_done": return ev.done(who, plan, e.items ?? 0, e.returned ?? 0);
    case "run_declined": return ev.declined(who, plan);
    case "plan_split": return ev.split(e.items ?? 0, plan);
  }
}

/** Bấm vào sự kiện thì mở nơi cần làm tiếp: người được nhờ mở lần đi mua, người nhờ mở kế hoạch. */
function eventLink(e: HouseEvent): string {
  if ((e.kind === "plan_sent" || e.kind === "plan_updated") && e.assignment_id) return `/plan/run/${encodeURIComponent(e.assignment_id)}`;
  if (e.kind === "plan_cancelled") return "/plan";
  return e.plan_id ? `/plan/${encodeURIComponent(e.plan_id)}` : "/plan";
}

function text(n: Exclude<Notification, { kind: "event" }>): string {
  if (n.kind === "trip") {
    const base = n.date === todayIso() ? t.notifications.tripToday(n.item_count) : t.notifications.tripTomorrow(n.item_count);
    return n.hour != null ? `${base} ${t.notifications.at(`${String(n.hour).padStart(2, "0")}:00`)}` : base;
  }
  if (n.status === "out") return t.notifications.itemOut(n.name);
  if (n.status === "buy_now" || n.days_left === 0) return t.notifications.itemNow(n.name);
  return n.days_left != null ? t.notifications.itemDays(n.name, n.days_left) : n.name;
}

/** Thông báo: sự kiện trong nhà (B9), món sắp hết và chuyến mua đã hẹn (GET /notifications). */
export function Notifications() {
  const q = useNotifications();
  const read = useMarkNotificationRead();

  return (
    <MainFrame>
      <TopBar title={t.notifications.title} variant="sub" />
      <div className="page">
        {q.isPending && <p className="hint">{t.common.loading}</p>}
        {q.isError && !q.data && (
          <div className="empty">
            <h2>{t.notifications.loadFailed}</h2>
            <button className="btn btn--quiet" onClick={() => q.refetch()}>{t.common.retry}</button>
          </div>
        )}
        {q.data && q.data.length === 0 && (
          <div className="empty">
            <h2>{t.notifications.empty}</h2>
            <p>{t.notifications.emptyBody}</p>
          </div>
        )}
        {q.data && q.data.length > 0 && (
          <ul className="list">
            {q.data.map((n) =>
              n.kind === "event" ? (
                <li key={n.id}>
                  <Link to={eventLink(n.event)} className="link-row" onClick={() => !n.event.read && read.mutate(n.id)}>
                    <Users size={20} aria-hidden />
                    <span className={`note ${n.event.read ? "" : "note--now"}`}>{eventText(n.event, n.event.actor_name || m15.events.someone)}</span>
                    <ChevronRight size={20} aria-hidden />
                  </Link>
                </li>
              ) : (
              <li key={n.id}>
                <Link to={n.kind === "trip" ? "/plan" : `/items/${encodeURIComponent(n.item_key)}`} className="link-row">
                  {n.kind === "trip" && <CalendarClock size={20} aria-hidden />}
                  <span className={`note ${n.kind === "item" && (n.status === "out" || n.status === "buy_now") ? "note--now" : ""}`}>{text(n)}</span>
                  <ChevronRight size={20} aria-hidden />
                </Link>
              </li>
              ),
            )}
          </ul>
        )}
      </div>
    </MainFrame>
  );
}
