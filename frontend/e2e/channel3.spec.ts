import { expect, test } from "@playwright/test";

// Channel3: chỉ ở Mỹ, chỉ đồ dùng nhà, thuốc, đồ trẻ em; giá hiện cạnh giá Gemini ở bước So sánh giá.
test("So sánh giá: Mỹ có giá Channel3 cho đồ dùng nhà, Việt Nam không có", async ({ page }) => {
  await page.setViewportSize({ width: 393, height: 852 });
  await page.goto("/?ui=vi&uiv2=1");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await page.getByText("Chào Lan!").waitFor();

  // Bản thử giữ dữ liệu trong bộ nhớ: chuyển màn trong app, không tải lại trang.
  const go = (path: string) =>
    page.evaluate((p) => {
      history.pushState({}, "", p);
      dispatchEvent(new PopStateEvent("popstate"));
    }, path);
  await go("/plan/default/compare");
  await expect(page.locator(".cmpi22").first()).toBeVisible();
  await expect(page.locator(".cmpi22").getByText(/Channel3/)).toHaveCount(0);

  await go("/settings/language");
  await page.getByRole("combobox", { name: /Quốc gia/ }).selectOption("US");
  await page.waitForTimeout(500);
  await go("/plan/default/compare");
  const soap = page.locator(".cmpi22", { hasText: "Nước rửa chén" });
  await expect(soap.getByText(/Channel3/)).toBeVisible();
  await soap.getByRole("button", { name: "Chi tiết giá ở Walmart" }).first().click();
  const open = page.getByRole("link", { name: "Mở trang sản phẩm" });
  await expect(open).toHaveAttribute("rel", "noopener noreferrer");
  await expect(page.getByText("buy.trychannel3.com")).toBeVisible();
  await page.getByRole("button", { name: "Đóng" }).first().click();
  await expect(page.locator(".cmpi22", { hasText: "Sữa tươi" }).getByText(/Channel3/)).toHaveCount(0);
});
