/**
 * Bản thử của POST /ai/turn (chế độ mock, không có Gemini): một "mô hình" viết tay, đủ để bấm
 * thử trọn luồng Hỏi Mimi trong Studio. Lượt đầu chọn một tool mimi_* theo từ khóa trong câu hỏi;
 * lượt sau đọc kết quả tool và viết câu trả lời ngắn. Không dùng ngoài chế độ mock.
 */
import { uiLang } from "../i18n";

type Json = Record<string, unknown>;
type Part = { text?: string; functionCall?: { id?: string; name: string; args?: Json }; functionResponse?: { name: string; response: Json } };
type Body = { contents: { role: string; parts: Part[] }[]; tools?: { name: string }[] };

const vi = uiLang === "vi";
const vnd = (n: unknown) => {
  const v = typeof n === "number" ? n : 0;
  return vi ? `${new Intl.NumberFormat("vi-VN").format(v)}đ` : `₫${new Intl.NumberFormat("en-US").format(v)}`;
};
const fold = (s: string) => s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").toLowerCase();

function parseResult(r: Json): Json {
  // Studio gửi kết quả tool dạng chuỗi; adapter bọc lại thành { result: "..." } khi không phải JSON.
  for (const v of [r.response, r.result, r.output]) {
    if (typeof v === "string") {
      try {
        return JSON.parse(v) as Json;
      } catch {
        return { text: v };
      }
    }
  }
  return r;
}

function answer(name: string, r: Json): string {
  if (name === "mimi_spending_summary") {
    const cats = (r.by_category as { name: string; amount: number; previous_month: number }[]) ?? [];
    const top = cats[0];
    const head = vi
      ? `Tháng ${String(r.month).slice(5)} nhà mình đã chi ${vnd(r.spent)} qua ${r.receipts} hóa đơn (tháng trước ${vnd(r.previous_spent)}).`
      : `In ${r.month} the family spent ${vnd(r.spent)} across ${r.receipts} receipts (last month: ${vnd(r.previous_spent)}).`;
    if (!top) return head;
    const diff = top.amount - top.previous_month;
    return vi
      ? `${head} Nhiều nhất là **${top.name}**: ${vnd(top.amount)}, ${diff >= 0 ? "tăng" : "giảm"} ${vnd(Math.abs(diff))} so với tháng trước. Mẹ xem biểu đồ Chi theo nhóm hàng ở trang Tổng quan nhé.`
      : `${head} The biggest category is **${top.name}** at ${vnd(top.amount)}, ${diff >= 0 ? "up" : "down"} ${vnd(Math.abs(diff))} on last month. See Spending by category on the Overview page.`;
  }
  if (name === "mimi_running_out") {
    const list = (Array.isArray(r) ? r : (r.result as unknown[]) ?? Object.values(r)) as { item: string; days_left: number; cheapest_online: { store: string; price: number } | null }[];
    if (!list.length) return vi ? "Tuần này chưa có món nào sắp hết, mẹ yên tâm." : "Nothing runs out this week.";
    const lines = list.slice(0, 5).map((x) => {
      const where = x.cheapest_online ? (vi ? ` · rẻ nhất ở ${x.cheapest_online.store} (${vnd(x.cheapest_online.price)})` : ` · cheapest at ${x.cheapest_online.store} (${vnd(x.cheapest_online.price)})`) : "";
      return `- **${x.item}**: ${x.days_left <= 0 ? (vi ? "đã hết" : "out now") : vi ? `còn ${x.days_left} ngày` : `${x.days_left} days left`}${where}`;
    });
    return `${vi ? "Các món sắp hết trong 7 ngày:" : "Running out in the next 7 days:"}\n${lines.join("\n")}`;
  }
  if (name === "mimi_savings") {
    const more = (r.cheaper_elsewhere as { item: string; best_store: string; saving: number }[]) ?? [];
    const head = vi ? `Nhờ khuyến mãi trên hóa đơn, nhà mình đã tiết kiệm ${vnd(r.saved_with_discounts)}.` : `Receipt discounts have saved the family ${vnd(r.saved_with_discounts)}.`;
    if (!more.length) return head;
    const tip = more[0];
    return vi
      ? `${head} Còn có thể bớt thêm ${vnd(r.could_save_more)}: ví dụ **${tip.item}** ở ${tip.best_store} rẻ hơn ${vnd(tip.saving)}. Chi tiết ở trang So sánh giá.`
      : `${head} You could save another ${vnd(r.could_save_more)}: for example **${tip.item}** is ${vnd(tip.saving)} cheaper at ${tip.best_store}. Details on the Price check page.`;
  }
  if (name === "mimi_chart") {
    const labels = (r.labels as string[]) ?? [];
    const vals = (r.series as { values: number[] }[] | undefined)?.[0]?.values ?? [];
    const own = (r.series as { values: number[] }[] | undefined)?.[1]?.values ?? [];
    const topic = String(r.topic ?? "");
    if (!labels.length) {
      if (topic === "upcoming_bills") return vi ? "7 ngày tới chưa có khoản định kỳ nào đến hạn." : "No recurring bills are due in the next 7 days.";
      return vi ? "Chưa có số liệu để vẽ, mẹ quét thêm hóa đơn nhé." : "Nothing to chart yet. Scan a few more receipts.";
    }
    const total = vals.reduce((a, x) => a + x, 0);
    const maxI = vals.indexOf(Math.max(...vals));
    if (topic === "spending_by_month") {
      const last = vals.at(-1) ?? 0;
      const prev = vals.at(-2) ?? 0;
      return vi
        ? `Mimi vẽ chi tiêu ${labels.length} tháng gần đây ở trên. Tháng cao nhất là **${labels[maxI]}** (${vnd(vals[maxI])}). Tháng này ${vnd(last)}${prev ? `, tháng trước ${vnd(prev)}` : ""}.`
        : `Here are the last ${labels.length} months. The highest was **${labels[maxI]}** (${vnd(vals[maxI])}). This month: ${vnd(last)}${prev ? `, last month: ${vnd(prev)}` : ""}.`;
    }
    if (topic === "avg_bill") {
      return vi
        ? `Tháng này trung bình mỗi hóa đơn ${vnd(vals.at(-1))}. Cao nhất là tháng **${labels[maxI]}** với ${vnd(vals[maxI])} mỗi hóa đơn.`
        : `This month the average receipt is ${vnd(vals.at(-1))}. The highest was **${labels[maxI]}** at ${vnd(vals[maxI])}.`;
    }
    if (topic === "spending_by_day") {
      return vi
        ? `Tháng này nhà mình chi vào ${labels.length} ngày, tổng ${vnd(total)}. Ngày chi nhiều nhất là **${labels[maxI]}**: ${vnd(vals[maxI])}.`
        : `This month there was spending on ${labels.length} days, ${vnd(total)} in total. The biggest day was **${labels[maxI]}**: ${vnd(vals[maxI])}.`;
    }
    if (topic === "upcoming_bills") {
      return vi
        ? `7 ngày tới có ${labels.length} khoản định kỳ đến hạn, tổng ${vnd(total)}. Sớm nhất là **${labels[0]}** (${vnd(vals[0])}).`
        : `${labels.length} recurring bills are due in the next 7 days, ${vnd(total)} in total. First up: **${labels[0]}** (${vnd(vals[0])}).`;
    }
    if (topic === "price_compare") {
      const gaps = labels.map((l, i) => ({ l, d: (vals[i] ?? 0) - (own[i] ?? 0) })).sort((a, b) => b.d - a.d);
      const g = gaps[0];
      return vi
        ? g && g.d > 0
          ? `Mimi so giá ${labels.length} món ở trên. **${g.l}** đang rẻ hơn ${vnd(g.d)} nếu mua online ở nơi rẻ nhất.`
          : `Mimi so giá ${labels.length} món ở trên. Nhà mình đang mua rẻ bằng hoặc hơn giá online rồi.`
        : g && g.d > 0
          ? `Compared ${labels.length} items above. **${g.l}** is ${vnd(g.d)} cheaper at the cheapest online store.`
          : `Compared ${labels.length} items above. We already pay the online price or less.`;
    }
    return vi
      ? `Mimi vẽ biểu đồ **${r.title}** ở trên rồi. Đứng đầu là **${labels[0]}**${vals[0] != null ? ` với ${vnd(vals[0])}` : ""}.`
      : `Here is **${r.title}** above. **${labels[0]}** leads${vals[0] != null ? ` at ${vnd(vals[0])}` : ""}.`;
  }
  return vi ? "Mimi đã làm xong." : "Done.";
}

export function mockAiTurn(body: Body): Json {
  const last = body.contents.at(-1);
  const tools = new Set((body.tools ?? []).map((t) => t.name));
  const fr = last?.parts.find((p) => p.functionResponse)?.functionResponse;
  const id = `mock_${Date.now().toString(36)}`;
  if (fr) return { id, model: "mock", parts: [{ text: answer(fr.name, parseResult(fr.response)) }] };

  const q = fold(last?.parts.map((p) => p.text ?? "").join(" ") ?? "");
  if (!tools.has("mimi_spending_summary")) {
    return { id, model: "mock", parts: [{ text: vi ? "Bản thử chưa nối Gemini nên agent này chỉ trả lời mẫu." : "This demo has no Gemini connection, so this agent only gives a sample answer." }] };
  }
  // Chỉ coi là "sửa bảng" khi hỏi thêm/sửa biểu đồ, widget; "tiết kiệm thêm" không phải.
  if (/(them (bieu do|widget|bang)|add (a )?(chart|widget)|widget)/.test(q)) {
    return { id, model: "mock", parts: [{ text: vi ? "Bản thử chưa nối Gemini nên Mimi chưa vẽ thêm vào bảng được. Mẹ chọn một câu gợi ý ở trên, Mimi trả lời kèm biểu đồ nhé." : "Without Gemini this demo can't add to the dashboard yet. Pick one of the suggestions above and Mimi will answer with a chart." }] };
  }
  const month = new Date().toISOString().slice(0, 7);
  const chart = /(cua hang|store|shop)/.test(q)
    ? { topic: "spending_by_store", month }
    : /(theo thang|6 thang|by month|month by month|xu huong|trend|6 months)/.test(q)
      ? { topic: "spending_by_month", month: "" }
      : /(trung binh|average)/.test(q)
        ? { topic: "avg_bill", month: "" }
        : /(ngay nao|by day|which day|spending day)/.test(q)
          ? { topic: "spending_by_day", month }
          : /(dinh ky|phai tra|bills? due|recurring)/.test(q)
            ? { topic: "upcoming_bills", month: "" }
            : /(gia online|online price|so sanh gia|re hon|cheapest online|where is it cheaper)/.test(q) && !/(tiet kiem|saved)/.test(q)
              ? { topic: "price_compare", month: "" }
              : /(top item|mua nhieu|ton tien|cost us the most|most expensive)/.test(q)
                ? { topic: "top_items", month }
                : null;
  if (chart && tools.has("mimi_chart")) return { id, model: "mock", parts: [{ functionCall: { id: `${id}_call`, name: "mimi_chart", args: chart } }] };
  const call = /(het|run out|running|need|can mua|tuan nay|this week)/.test(q)
    ? { name: "mimi_running_out", args: { days: 7 } }
    : /(tiet kiem|save|saved|re hon|cheaper)/.test(q)
      ? { name: "mimi_savings", args: {} }
      : { name: "mimi_spending_summary", args: { month: new Date().toISOString().slice(0, 7) } };
  return { id, model: "mock", parts: [{ functionCall: { id: `${id}_call`, ...call } }] };
}
