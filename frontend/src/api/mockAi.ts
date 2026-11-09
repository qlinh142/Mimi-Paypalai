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
    const first = (r.series as { values: number[] }[] | undefined)?.[0]?.values ?? [];
    if (!labels.length) return vi ? "Chưa có số liệu để vẽ, mẹ quét thêm hóa đơn nhé." : "Nothing to chart yet. Scan a few more receipts.";
    return vi
      ? `Mimi vẽ biểu đồ **${r.title}** ở trên rồi. Đứng đầu là **${labels[0]}**${first[0] != null ? ` với ${vnd(first[0])}` : ""}.`
      : `Here is **${r.title}** above. **${labels[0]}** leads${first[0] != null ? ` at ${vnd(first[0])}` : ""}.`;
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
  if (/(them|add|widget)/.test(q)) {
    return { id, model: "mock", parts: [{ text: vi ? "Bản thử chưa nối Gemini nên Mimi chưa sửa bảng được. Mẹ bấm **Chỉnh bảng** rồi kéo trường Cửa hàng vào trang nhé." : "Without Gemini this demo can't edit the dashboard yet. Tap **Edit dashboard** and drag the Store field onto the page." }] };
  }
  const month = new Date().toISOString().slice(0, 7);
  const chart = /(cua hang|store|shop)/.test(q)
    ? { topic: "spending_by_store", month }
    : /(theo thang|by month|month by month|xu huong|trend)/.test(q)
      ? { topic: "spending_by_month", month: "" }
      : /(gia online|online price|so sanh gia|price)/.test(q)
        ? { topic: "price_compare", month: "" }
        : /(mon nao chi|top item|mua nhieu)/.test(q)
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
