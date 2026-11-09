import { expect, test } from "@playwright/test";

// Mimi 2.2: màn Đồ dùng (đề xuất chốt 09/11): chip lọc, dòng Rẻ nhất, So sánh giá kèm link nguồn.
test("Đồ dùng 2.2: lọc, giá rẻ nhất, so sánh giá có link nguồn", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/?ui=vi");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await expect(page.getByText("Chào Lan!")).toBeVisible();

  await page.goto("/items");
  await expect(page.locator(".cheap22").first()).toContainText(/Rẻ nhất .* · /);
  // Cần chú ý: ẩn nhóm Còn nhiều, đường dẫn giữ bộ lọc.
  await page.getByRole("button", { name: /^Cần chú ý \(\d+\)$/ }).click();
  await expect(page).toHaveURL(/f=attention/);
  await expect(page.getByText("CÒN NHIỀU", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Tất cả" }).click();
  await page.locator("input[type=search]").fill("khong co mon nay");
  await expect(page.getByText("Không có món nào khớp")).toBeVisible();

  // Chi tiết món: So sánh giá, giá thấp nhất đứng đầu, mỗi giá có link nguồn mở tab mới.
  await page.goto("/items/sua-tuoi");
  await expect(page.getByRole("heading", { name: "So sánh giá" })).toBeVisible();
  await expect(page.locator(".cmp22__row").first()).toContainText("Thấp nhất");
  // Giá nhà mình từng mua (từ hóa đơn) đứng trên giá đang bán.
  await expect(page.locator(".cmp22__ours")).toContainText("Lần trước");
  await expect(page.locator(".cmp22__ours")).toContainText("Thấp nhất từng mua");
  const link = page.locator(".cmp22__src").first();
  await expect(link).toHaveAttribute("href", /^https:\/\//);
  await expect(link).toHaveAttribute("target", "_blank");
  // Món chưa được tra lần nào.
  await page.goto("/items/ca-phe");
  await expect(page.getByText("Mimi đang tra giá món này")).toBeVisible();

  // Tab Định kỳ: nhãn chu kỳ và dòng gợi ý.
  await page.goto("/items?tab=recurring");
  await expect(page.getByText("Hằng tháng").first()).toBeVisible();
  await expect(page.getByText("Mimi liệt kê các khoản chi lặp lại.")).toBeVisible();
  expect(errors).toEqual([]);
});
