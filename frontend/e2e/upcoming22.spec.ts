import { expect, test } from "@playwright/test";

// Anh chốt 10/11: Sắp chi 7 ngày = khoản định kỳ đến hạn trong 7 ngày + món sắp hết × mức dùng mỗi ngày × 7; mỗi mục có mô tả cách tính, chữ đọc được trên nền xanh.
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

  // Món có chu kỳ mua: tính mức dùng mỗi ngày × 7 ngày, dòng phụ ghi rõ cách tính.
  await expect(card.locator(".fc15__rows small").first()).toContainText("/ngày × 7 ngày");
  // Sữa: 30.000đ mỗi lần mua, 5 ngày mua một lần → 6.000đ/ngày × 7 = 42.000đ.
  await expect(card.locator(".fc15__rows li", { hasText: "Sữa tươi" })).toContainText("6.000đ/ngày × 7 ngày");
  await expect(card.locator(".fc15__rows li", { hasText: "Sữa tươi" })).toContainText("42.000đ");

  // Mỗi mục có câu mô tả cách tính, chữ sáng trên nền xanh.
  await expect(card.locator(".fc15__how")).toHaveCount(2);
  await expect(card.locator(".fc15__how").first()).toContainText("đến hạn trong 7 ngày tới");
  for (const sel of [".fc15__how", ".fc15__rows small"]) {
    const color = await card.locator(sel).first().evaluate((e) => getComputedStyle(e).color);
    const [r, g, b] = color.match(/\d+/g)!.map(Number);
    expect(Math.min(r, g, b)).toBeGreaterThan(200);
  }
});
