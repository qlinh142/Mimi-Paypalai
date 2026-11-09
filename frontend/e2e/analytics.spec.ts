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
  test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

  test("Phân tích chi tiêu: thẻ hai cột, không cuộn ngang, Hỏi Mimi chiếm cả màn", async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await signIn(page);
    await page.goto("/spending/analytics");
    await expect(page.locator(".radar")).toBeVisible({ timeout: 20_000 });

    // Câu gợi ý: bấm "Mua ở đâu rẻ hơn?" là sang trang So sánh giá.
    await page.getByRole("button", { name: "Mua ở đâu rẻ hơn?" }).click();
    await expect(page.getByText("Giá đang bán của từng món, rẻ nhất trước")).toBeVisible();
    // Lịch chi tiêu: chạm vào ngày có chi thì hiện các khoản của ngày đó.
    await page.getByRole("button", { name: "Lịch chi tiêu theo ngày" }).click();
    await page.locator(".cal__cell:has(.cal__amt)").first().click();
    await expect(page.locator(".cal__list li").first()).toBeVisible();
    await page.getByRole("button", { name: "Tổng quan" }).click();

    // Thẻ hai cột: chạm thẻ "Sắp phải trả" thì mở trang chi tiết, qua thẻ sau được, Esc để đóng.
    await page.locator(".ag-studio-layout-widget", { hasText: "Sắp phải trả" }).click({ position: { x: 40, y: 40 } });
    const sheet = page.getByRole("dialog", { name: "Sắp phải trả" });
    await expect(sheet.getByText("Đây là gì?")).toBeVisible();
    await sheet.getByRole("button", { name: "Thẻ sau" }).click();
    await expect(page.getByRole("dialog", { name: "Sắp hết" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);

    // Widget nằm gọn trong bề ngang màn hình.
    const box = await page.locator(".radar").boundingBox();
    expect(box && box.x + box.width).toBeLessThanOrEqual(390);

    await page.getByRole("button", { name: "Hỏi Mimi", exact: true }).click();
    await page.getByText("Tuần này cần mua gì?").click();
    await expect(page.getByText("Các món sắp hết trong 7 ngày:")).toBeVisible({ timeout: 20_000 });
    const chat = await page.getByText("Các món sắp hết trong 7 ngày:").boundingBox();
    expect(chat && chat.x + chat.width).toBeLessThanOrEqual(390);

    expect(errors).toEqual([]);
  });
});
