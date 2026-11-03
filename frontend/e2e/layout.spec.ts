import { expect, test } from "@playwright/test";

// Quy định hiển thị khi đổi ngôn ngữ (mẹ chốt 03/11): chữ cố định tinh chỉnh riêng cho từng ngôn
// ngữ, tên sản phẩm cắt theo luật. Kiểm tra mọi màn chính ở cả hai ngôn ngữ trên màn nhỏ nhất
// (320px): không màn nào được tràn ngang.

const ROUTES = [
  "/", "/spending", "/spending/history", "/spending/receipt/h1", "/items", "/plan", "/plan/trip-mau",
  "/plan/run/trip-mau", "/settings", "/settings/members", "/settings/language", "/settings/notify", "/notifications",
];

test.use({ viewport: { width: 320, height: 640 }, deviceScaleFactor: 1, isMobile: true, hasTouch: true });

for (const lang of ["vi", "en"] as const) {
  test(`không tràn ngang ở 320px (${lang})`, async ({ page }) => {
    await page.goto(`/?ui=${lang}`);
    await page.locator("input[type=email]").fill("lan@gmail.com");
    await page.locator("input").nth(1).fill("Lan");
    await page.locator("button[type=submit]").first().click();
    await page.getByText(lang === "vi" ? "Mở link (bản thử)" : "Open link (demo)").click();
    await expect(page.getByText(lang === "vi" ? "Chào Lan!" : "Hi, Lan!")).toBeVisible();

    const overflow: string[] = [];
    for (const r of ROUTES) {
      await page.goto(r);
      await page.waitForLoadState("networkidle");
      await page.waitForTimeout(400);
      const wide = await page.evaluate(() => {
        const w = document.documentElement.clientWidth;
        if (document.documentElement.scrollWidth <= w + 1) return null;
        const el = [...document.querySelectorAll("body *")].find((e) => e.getBoundingClientRect().right > w + 1 && getComputedStyle(e).position !== "fixed");
        return el ? `${el.tagName.toLowerCase()}.${el.className}` : "?";
      });
      if (wide) overflow.push(`${r}: ${wide}`);
    }
    expect(overflow, "màn bị tràn ngang").toEqual([]);
  });
}
