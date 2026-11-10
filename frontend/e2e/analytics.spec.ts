import { expect, test, type Page } from "@playwright/test";

// Màn Phân tích chi tiêu (AG Studio) trên bản thử: bảng dựng sẵn hiện đủ widget của Mimi,
// "Hỏi Mimi" gọi tool của Mimi và trả lời (bản thử dùng mô hình viết tay, xem src/api/mockAi.ts).

test.use({ viewport: { width: 1280, height: 900 }, isMobile: false, hasTouch: false });

async function signIn(page: Page) {
  await page.goto("/?ui=vi");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await expect(page.getByText("Chào Lan!")).toBeVisible();
}

test("Phân tích chi tiêu: bảng, widget Mimi, Hỏi Mimi", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await signIn(page);

  // Vào từ Sổ chi tiêu.
  await page.goto("/spending");
  await page.getByRole("link", { name: /Phân tích chi tiêu/ }).click();
  await expect(page.getByRole("heading", { name: "Phân tích chi tiêu" })).toBeVisible();

  // Widget riêng của Mimi đã có dữ liệu.
  await expect(page.getByText("Sắp hết trước")).toBeVisible({ timeout: 20_000 });
  await expect(page.locator(".radar__name").first()).toBeVisible();
  await expect(page.locator(".saved__num")).toContainText("đ");

  // Trang So sánh giá: bảng AG Grid có giá online.
  await page.getByRole("button", { name: "So sánh giá", exact: true }).click();
  await expect(page.getByText("Giá đang bán của từng món, rẻ nhất trước")).toBeVisible();
  await expect(page.locator(".ag-row").first()).toBeVisible();

  // Hỏi Mimi: chọn câu gợi ý, Mimi gọi tool món sắp hết rồi trả lời.
  await page.getByRole("button", { name: "Hỏi Mimi", exact: true }).click();
  await page.getByText("Tuần này cần mua gì?").click();
  await expect(page.getByText("Mimi xem món sắp hết")).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText("Các món sắp hết trong 7 ngày:")).toBeVisible();
  // Biểu đồ của câu trả lời hiện ngay trong khung chat, mở sẵn.
  await expect(page.locator(".chatchart canvas").first()).toBeVisible();

  // Hỏi một biểu đồ khác: Mimi vẽ chi theo cửa hàng trong khung chat.
  await page.locator("textarea").last().fill("Cho mẹ xem biểu đồ chi tiêu theo cửa hàng tháng này.");
  await page.locator("textarea").last().press("Enter");
  await expect(page.getByText("Mimi vẽ biểu đồ", { exact: true })).toBeVisible({ timeout: 20_000 });
  await expect(page.locator(".chatchart")).toHaveCount(2);

  expect(errors).toEqual([]);
});

test.describe("trên điện thoại", () => {
  test.use({ viewport: { width: 393, height: 852 }, isMobile: true, hasTouch: true });

  // Anh chốt 10/11: bản điện thoại là một cột thẻ như Sổ chi tiêu, số khớp Sổ chi tiêu, không có lưới AG Studio.
  test("Phân tích chi tiêu: một cột, số khớp Sổ chi tiêu, không tràn ngang, Hỏi Mimi chiếm cả màn", async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await signIn(page);
    const vnd = (s: string) => Number(s.replace(/[^\d]/g, ""));

    // Số của Sổ chi tiêu: đã chi tháng này và Sắp chi 7 ngày tới.
    await page.goto("/spending?ui=vi&uiv2=1");
    await expect(page.locator(".hero22 .hero15__num")).toContainText("đ");
    const spent = vnd(await page.locator(".hero22 .hero15__num").innerText());
    await page.getByRole("button", { name: "Sắp chi", exact: true }).click();
    await expect(page.locator(".fc15__line").first()).toBeVisible();
    await page.waitForTimeout(800);
    const upcoming = vnd(await page.locator(".fc15 .hero15__num").innerText());

    await page.goto("/spending/analytics?ui=vi&uiv2=1");
    const pa = page.locator(".pa");
    await expect(pa.getByText("Sắp chi 7 ngày tới")).toBeVisible({ timeout: 20_000 });
    await expect(page.locator(".ag-studio-layout-widget")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Chỉnh bảng" })).toHaveCount(0);
    expect(vnd(await pa.locator(".pa__hero .hero15__num").innerText())).toBe(spent);
    await expect.poll(async () => vnd(await pa.locator(".pa__big").first().innerText())).toBe(upcoming);

    // Món tốn tiền nhất không có khoản định kỳ; các thẻ nằm gọn trong màn.
    await expect(pa.locator("#pa-top + p + ol")).not.toContainText("Tiền điện");
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(393);
    for (const box of await pa.locator(".pa__card").all()) {
      const b = await box.boundingBox();
      expect(b && b.x + b.width).toBeLessThanOrEqual(393);
    }

    // Ba nút trang nằm trên một hàng, cao ít nhất 44px.
    const tabs = pa.locator(".seg15 button");
    const ys = await tabs.evaluateAll((els) => els.map((e) => e.getBoundingClientRect()));
    expect(new Set(ys.map((r) => Math.round(r.top))).size).toBe(1);
    for (const r of ys) expect(r.height).toBeGreaterThanOrEqual(44);

    // So sánh giá: mỗi món một thẻ, nơi rẻ nhất có nhãn.
    await page.getByRole("button", { name: "So sánh giá", exact: true }).click();
    await expect(pa.getByText("Mua ở đâu rẻ hơn?")).toBeVisible();
    await expect(pa.locator(".pa__card", { hasText: "Sữa tươi không đường" }).getByText("Rẻ nhất")).toBeVisible();

    // Hóa đơn: lọc Định kỳ thì cửa hàng không còn; chạm ngày có chi thì hiện các khoản của ngày đó.
    await page.getByRole("button", { name: "Hóa đơn", exact: true }).click();
    await page.locator(".cal__cell:has(.cal__amt)").first().click();
    await expect(page.locator(".cal__list li").first()).toBeVisible();
    await expect(pa.locator(".ph15__list li").first()).toBeVisible();

    // Hỏi Mimi: khung chat chiếm cả màn, Xong thì về màn một cột.
    await page.getByRole("button", { name: "Hỏi Mimi", exact: true }).click();
    await page.getByText("Tuần này cần mua gì?").click();
    await expect(page.getByText("Các món sắp hết trong 7 ngày:")).toBeVisible({ timeout: 20_000 });
    const chat = await page.getByText("Các món sắp hết trong 7 ngày:").boundingBox();
    expect(chat && chat.x + chat.width).toBeLessThanOrEqual(393);
    await page.getByRole("button", { name: "Xong", exact: true }).click();
    await expect(pa.getByText("Sắp chi 7 ngày tới")).toBeVisible();

    expect(errors).toEqual([]);
  });
});
