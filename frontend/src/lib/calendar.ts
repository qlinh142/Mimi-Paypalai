/**
 * Thêm lịch đi chợ một lần (ADR-004, mẹ chốt 02/11): mở Google Calendar với sự kiện điền sẵn,
 * hoặc tải file .ics cho Lịch trên iPhone. Không xin quyền vào lịch, không sửa lịch về sau.
 */
export type CalendarEvent = {
  title: string;
  date: string; // YYYY-MM-DD
  time?: string; // HH:MM, trống: cả ngày
  location?: string;
  details?: string;
};

const TZ = "Asia/Ho_Chi_Minh";
const DURATION_MIN = 60;

const compact = (d: string) => d.replace(/-/g, "");

function addDays(date: string, n: number): string {
  const [y, m, d] = date.split("-").map(Number);
  const t = new Date(Date.UTC(y, m - 1, d + n));
  return t.toISOString().slice(0, 10);
}

/** Giờ bắt đầu và kết thúc dạng YYYYMMDDTHHMMSS (giờ địa phương, kèm múi giờ riêng). */
function range(ev: CalendarEvent): [string, string] | null {
  if (!ev.time) return null;
  const [h, mi] = ev.time.split(":").map(Number);
  const end = h * 60 + mi + DURATION_MIN;
  const endDate = end >= 24 * 60 ? addDays(ev.date, 1) : ev.date;
  const eh = Math.floor((end % (24 * 60)) / 60);
  const em = end % 60;
  const p = (n: number) => String(n).padStart(2, "0");
  return [`${compact(ev.date)}T${p(h)}${p(mi)}00`, `${compact(endDate)}T${p(eh)}${p(em)}00`];
}

export function googleCalendarUrl(ev: CalendarEvent): string {
  const r = range(ev);
  const dates = r ? `${r[0]}/${r[1]}` : `${compact(ev.date)}/${compact(addDays(ev.date, 1))}`;
  const q = new URLSearchParams({ action: "TEMPLATE", text: ev.title, dates, ctz: TZ });
  if (ev.location) q.set("location", ev.location);
  if (ev.details) q.set("details", ev.details);
  return `https://calendar.google.com/calendar/render?${q.toString()}`;
}

/** Chuỗi trong .ics: thoát \ ; , và xuống dòng (RFC 5545 mục 3.3.11). */
const esc = (s: string) => s.replace(/\\/g, "\\\\").replace(/;/g, "\;").replace(/,/g, "\\,").replace(/\r?\n/g, "\\n");

export function icsText(ev: CalendarEvent, uid: string, now = new Date()): string {
  const r = range(ev);
  const stamp = now.toISOString().replace(/[-:]/g, "").replace(/\.\d{3}/, "");
  const lines = [
    "BEGIN:VCALENDAR",
    "VERSION:2.0",
    "PRODID:-//Mimi//Ke hoach//VI",
    "BEGIN:VEVENT",
    `UID:${uid}@mimi`,
    `DTSTAMP:${stamp}`,
    ...(r
      ? [`DTSTART;TZID=${TZ}:${r[0]}`, `DTEND;TZID=${TZ}:${r[1]}`]
      : [`DTSTART;VALUE=DATE:${compact(ev.date)}`, `DTEND;VALUE=DATE:${compact(addDays(ev.date, 1))}`]),
    `SUMMARY:${esc(ev.title)}`,
    ...(ev.location ? [`LOCATION:${esc(ev.location)}`] : []),
    ...(ev.details ? [`DESCRIPTION:${esc(ev.details)}`] : []),
    "END:VEVENT",
    "END:VCALENDAR",
  ];
  return lines.join("\r\n") + "\r\n";
}

export function downloadIcs(ev: CalendarEvent, uid: string) {
  const url = URL.createObjectURL(new Blob([icsText(ev, uid)], { type: "text/calendar;charset=utf-8" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = "mimi-di-cho.ics";
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}
