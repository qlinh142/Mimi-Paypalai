/**
 * Thêm lịch đi chợ một lần (ADR-004, mẹ chốt 02/11): mở Google Calendar với sự kiện điền sẵn.
 * Không xin quyền vào lịch, không sửa lịch về sau. Nút Lịch iPhone (.ics) đã bỏ: iPhone chỉ tải
 * file về chứ không mở app Lịch (mẹ thử 02/11), nên chỉ giữ Google Calendar.
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
