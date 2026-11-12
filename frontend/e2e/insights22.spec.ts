import { expect, test, type Page } from "@playwright/test";

// "Mimi nhận thấy" giai đoạn 1 (đặc tả Contextual Insights 12/11): card đúng chỗ, mở bảng AG Studio nhỏ, quay lại đúng chỗ.
async function signIn(page: Page) {
  await page.goto("/?ui=vi&uiv2=1");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await page.getByText("Chào Lan!").waitFor();
}
const go = (page: Page, path: string) =>
  page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);

test("Mimi nhận thấy: giá món tăng ở Chi tiết món, ngân sách ở Kế hoạch bước 1, bảng phân tích vừa màn 320", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.setViewportSize({ width: 320, height: 700 });
  await signIn(page);

  // Điểm 6: giá đang bán rẻ nhất vẫn cao hơn lần mua gần nhất từ 10%.
  await go(page, "/items/nuoc-rua-chen");
  const card = page.locator(".ins22");
  await expect(card).toContainText("Giá Nước rửa chén đang cao hơn 12% so với lần mua gần nhất.");
  // Món không tăng giá: không có card.
  await go(page, "/items/sua-tuoi");
  await expect(page.getByRole("heading", { name: "So sánh giá" })).toBeVisible();
  await expect(page.locator(".ins22")).toHaveCount(0);

  await go(page, "/items/nuoc-rua-chen");
  await card.getByRole("button", { name: "Xem phân tích" }).click();
  const sheet = page.getByRole("dialog", { name: "Giá Nước rửa chén" });
  await expect(sheet).toBeVisible();
  await expect(sheet.locator(".ag-studio-value-widget").first()).toContainText("50.670đ", { timeout: 30_000 });
  expect(await sheet.evaluate((e) => e.scrollWidth <= e.clientWidth)).toBe(true);
  await sheet.locator(".insr22__foot").getByRole("button", { name: "Quay lại" }).click();
  await expect(sheet).toHaveCount(0);
  await expect(page).toHaveURL(/\/items\/nuoc-rua-chen/);

  // Ẩn rồi thì không hiện lại khi số liệu chưa đổi.
  await card.getByRole("button", { name: "Ẩn nhận xét" }).click();
  await expect(page.locator(".ins22")).toHaveCount(0);

  // Điểm 9: ngân sách tháng còn lại sau kế hoạch; hạ ngân sách thì thành cảnh báo vượt.
  await go(page, "/plan");
  await expect(page.locator(".ins22")).toContainText("Sau kế hoạch này, tháng này còn");
  await go(page, "/spending");
  await page.getByRole("button", { name: "Sửa ngân sách" }).click();
  await page.locator(".bud22__form input").fill("2500000");
  await page.getByRole("button", { name: "Lưu ngân sách" }).click();
  await go(page, "/plan");
  const over = page.locator(".ins22--warning, .ins22--info").filter({ hasText: "vượt ngân sách" });
  await expect(over).toBeVisible();
  await over.getByRole("button", { name: "Xem phân tích" }).click();
  await expect(page.getByRole("dialog", { name: "Ngân sách tháng này" }).locator(".ag-studio-value-widget").first()).toContainText("2.500.000đ", { timeout: 30_000 });
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(errors).toEqual([]);
});
