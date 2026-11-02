import { CalendarPlus } from "lucide-react";
import { m15 } from "../i18n/m15";
import { type CalendarEvent, googleCalendarUrl } from "../lib/calendar";

/** Thêm vào Google Calendar (ADR-004: tạo một lần, không nối tài khoản lịch). Dùng ở kế hoạch của mẹ và màn đi mua hộ. */
export function AddToCalendar({ ev }: { ev: CalendarEvent }) {
  return (
    <div className="cal15">
      <div className="cal15__row">
        <a className="btn btn--quiet" href={googleCalendarUrl(ev)} target="_blank" rel="noopener noreferrer">
          <CalendarPlus size={18} aria-hidden /> {m15.plan.calGoogle}
        </a>
      </div>
      <p className="hint">{m15.plan.calHint}</p>
    </div>
  );
}
