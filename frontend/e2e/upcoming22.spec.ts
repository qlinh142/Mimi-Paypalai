import { expect, test } from "@playwright/test";

// Lỗi 10/11: tab Sắp chi không ghi khoảng thời gian và dòng ghi chú gần như không đọc được trên nền xanh.
test("Sắp chi: tiêu đề 7 ngày tới, tổng khớp các dòng, ghi chú đọc được", async ({ page }) => {
  await page.setViewportSize({ width: 393, height: 852 });
  await page.goto("/?ui=vi&uiv2=1");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await page.getByText("Chào Lan!").waitFor();
  await page.evaluate(() => {
    history.pushState({}, "", "/spending?tab=upcoming");
    dispatchEvent(new PopStateEvent("popstate"));
  });
  const card = page.locator(".fc15");
  await expect(card.getByText("Sắp chi 7 ngày tới")).toBeVisible();

  // Tổng bằng hai dòng cộng lại; mở Hàng tiêu dùng thì món nào cũng có giá hoặc ghi "chưa có giá".
  const vnd = (s: string) => Number(s.replace(/[^\d]/g, ""));
  const total = vnd(await card.locator(".hero15__num").innerText());
  const lines = await card.locator(".fc15__line .num").allInnerTexts();
  expect(total).toBe(lines.reduce((a, x) => a + vnd(x), 0));
  await card.getByRole("button", { name: /Hàng tiêu dùng/ }).click();
  await expect(card.locator(".fc15__rows li").first()).toBeVisible();

  // Ghi chú trên nền xanh phải là chữ sáng.
  const color = await card.locator(".hint").evaluate((e) => getComputedStyle(e).color);
  const [r, g, b] = color.match(/\d+/g)!.map(Number);
  expect(Math.min(r, g, b)).toBeGreaterThan(200);
});
