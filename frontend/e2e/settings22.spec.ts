import { expect, test } from "@playwright/test";

// Cài đặt 2.2 (anh chốt 09/11): bỏ Ủy quyền mua online (T1); nhắc trong app theo 2 ngưỡng cố định,
// chưa nhắc qua email (T2, T5); ngôn ngữ, tiền tệ và mời thành viên giữ như 2.1 (T3, T4).
test("Cài đặt 2.2: không còn Ủy quyền mua online, Thông báo theo ngưỡng 3 và 7 ngày", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.setViewportSize({ width: 393, height: 852 });
  await page.goto("/?ui=vi&uiv2=1");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await page.getByText("Chào Lan!").waitFor();
  const go = (path: string) =>
    page.evaluate((p) => {
      history.pushState({}, "", p);
      dispatchEvent(new PopStateEvent("popstate"));
    }, path);

  await go("/settings");
  await expect(page.getByRole("link", { name: /Thành viên/ })).toBeVisible();
  await expect(page.getByText(/Ủy quyền/)).toHaveCount(0);
  await expect(page.getByText("Cần mua ngay ≤ 3 ngày · Sắp hết ≤ 7 ngày")).toBeVisible();
  await page.getByRole("link", { name: /Thông báo & nhắc mua/ }).click();
  await expect(page.getByText("Cần mua ngay: còn 3 ngày trở xuống hoặc đã hết")).toBeVisible();
  await expect(page.getByText("Sắp hết: còn 4 đến 7 ngày")).toBeVisible();
  await expect(page.getByRole("switch")).toHaveCount(0);

  // B1 đến B4 theo Redesign: vai trò dưới avatar, Ẩn số tiền làm mờ thẻ tổng Sổ chi tiêu, chạm để xem.
  await go("/settings/profile");
  await expect(page.locator(".set15__who")).toContainText("lan@gmail.com");
  await go("/settings/privacy");
  await page.getByRole("switch", { name: /Ẩn số tiền/ }).check();
  await go("/spending");
  const hidden = page.getByRole("button", { name: "Hiện số tiền" });
  await expect(hidden).toBeVisible();
  await hidden.click();
  await expect(hidden).toHaveCount(0);
  await go("/settings/privacy");
  await page.getByRole("switch", { name: /Ẩn số tiền/ }).uncheck();

  // Người mua của kế hoạch đã lưu: chỉ Tôi và người nhà, không còn Mimi mua online.
  await go("/plan/p-be");
  await expect(page.getByRole("heading", { name: "Người mua" })).toBeVisible();
  await expect(page.getByRole("button", { name: /Mua online/ })).toHaveCount(0);
  expect(errors).toEqual([]);
});
