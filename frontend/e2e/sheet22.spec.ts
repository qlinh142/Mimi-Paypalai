import { expect, test } from "@playwright/test";

// Lỗi 10/11 trên iPhone: bấm giỏ ở món cuối danh sách thì bảng "Thêm vào kế hoạch nào?" nằm tít cuối trang,
// nền tối chỉ phủ nhóm món. Bảng phải nổi ở đáy màn hình và nền tối phủ cả màn.
test("Bảng chọn kế hoạch nổi ở đáy màn hình dù đã cuộn xuống", async ({ page }) => {
  await page.setViewportSize({ width: 393, height: 852 });
  await page.goto("/?ui=vi&uiv2=1");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await page.getByText("Chào Lan!").waitFor();
  await page.evaluate(() => {
    history.pushState({}, "", "/items");
    dispatchEvent(new PopStateEvent("popstate"));
  });
  const carts = page.locator(".cart15");
  await expect(carts.first()).toBeVisible();
  await page.waitForTimeout(600); // chờ hiệu ứng vào màn chạy xong
  const last = carts.last();
  await last.scrollIntoViewIfNeeded();
  await last.click();
  const dialog = page.getByRole("dialog", { name: "Thêm vào kế hoạch nào?" });
  await expect(dialog).toBeVisible();
  // Chờ hiệu ứng trượt lên xong rồi đo đáy bảng.
  await expect.poll(async () => {
    const box = (await dialog.boundingBox())!;
    return Math.round(box.y + box.height);
  }).toBe(852);
  const scrim = (await page.locator(".sheet15").boundingBox())!;
  expect(scrim.y).toBe(0);
  expect(scrim.height).toBe(852);
});
