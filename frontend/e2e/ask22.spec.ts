import { expect, test } from "@playwright/test";

// Anh chốt 10/11: Hỏi Mimi trên điện thoại có hàng gợi ý phủ đủ nội dung của bảng phân tích,
// chạm là hỏi; câu nào cũng có câu trả lời (bản thử), không bảo người dùng đi "Chỉnh bảng".
test.use({ viewport: { width: 393, height: 852 }, isMobile: true, hasTouch: true });

test("Hỏi Mimi: chạm từng gợi ý đều ra câu trả lời đúng chủ đề", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/?ui=vi&uiv2=1");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await page.getByText("Chào Lan!").waitFor();
  await page.goto("/spending/analytics?ui=vi&uiv2=1");
  await page.getByRole("button", { name: "Hỏi Mimi", exact: true }).click();

  // Chat còn trống: Studio hiện đủ 11 gợi ý.
  await expect(page.locator(".ag-studio-ai-prompt-starter")).toHaveCount(11);
  const expected: [string, RegExp][] = [
    ["Tháng này chi bao nhiêu?", /nhà mình đã chi .* qua \d+ hóa đơn/],
    ["Chi nhiều nhất cho nhóm nào?", /Nhiều nhất là/],
    ["Chi 6 tháng gần đây", /Tháng cao nhất là/],
    ["Chi theo cửa hàng", /Chi theo cửa hàng/],
    ["Món nào tốn tiền nhất?", /Món chi nhiều nhất/],
    ["Ngày nào chi nhiều nhất?", /Ngày chi nhiều nhất là/],
    ["Trung bình mỗi hóa đơn", /trung bình mỗi hóa đơn/],
    ["Sắp phải trả khoản nào?", /khoản định kỳ/],
    ["Tuần này cần mua gì?", /Các món sắp hết trong 7 ngày/],
    ["Mua ở đâu rẻ hơn?", /Mimi so giá/],
    ["Mimi giúp tiết kiệm bao nhiêu?", /đã tiết kiệm/],
  ];
  const chat = page.locator(".ana__studio");
  const more = page.locator(".ana__more button");
  for (const [i, [label, answer]] of expected.entries()) {
    // Câu trả lời mới: số đoạn khớp chủ đề tăng thêm một (vài chủ đề có chữ giống câu trước).
    const before = await chat.getByText(answer).count();
    const pick = i === 0 ? page.locator(".ag-studio-ai-prompt-starter", { hasText: label }) : more.filter({ hasText: label });
    await pick.click();
    await expect.poll(() => chat.getByText(answer).count(), { timeout: 20_000, message: label }).toBeGreaterThan(before);
    // Dưới câu trả lời vẫn hiện các câu chưa hỏi, câu vừa hỏi thì không.
    await expect(more).toHaveCount(expected.length - i - 1);
    if (i < expected.length - 1) await expect(more.filter({ hasText: label })).toHaveCount(0);
  }
  await expect(page.getByText("Chỉnh bảng")).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(393);
  expect(errors).toEqual([]);
});
