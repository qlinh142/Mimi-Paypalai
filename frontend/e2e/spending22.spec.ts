import { expect, test, type Page } from "@playwright/test";

// Mimi 2.2: Sổ chi tiêu bản tổng hợp (ngân sách, Còn tiêu được, Mimi nhận thấy, biểu đồ vòng).

async function signIn(page: Page) {
  await page.goto("/?ui=vi");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await expect(page.getByText("Chào Lan!")).toBeVisible();
}

test("Sổ chi tiêu 2.2: ngân sách, Còn tiêu được, Mimi nhận thấy, hỏi Mimi", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await signIn(page);
  await page.goto("/spending");

  await expect(page.getByText("Theo dõi những gì mẹ đã chi")).toBeVisible();
  await expect(page.getByText(/so với 1–\d+\/\d+/)).toBeVisible();
  await expect(page.getByRole("img", { name: "Chi tiêu cộng dồn từ đầu tháng" })).toBeVisible();
  // Thẻ đến dần nhưng hàng vẫn đứng ở thẻ đầu (không bị cuộn bắt dính giữ thẻ cũ).
  await expect(page.locator(".noticed22__card")).toHaveCount(4);
  expect(await page.locator(".noticed22").evaluate((e) => e.scrollLeft)).toBe(0);
  await expect(page.getByRole("button", { name: "Sửa ngân sách" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Còn tiêu được" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Mimi nhận thấy" })).toBeVisible();
  // Khoản chi định kỳ vẫn đứng đầu chú giải và mở Đồ dùng.
  await expect(page.locator(".donut22__legend li").first()).toContainText("Khoản chi định kỳ");

  // Bỏ ngân sách thì hiện lời mời; đặt ngân sách nhỏ thì thẻ chuyển sang vượt ngân sách.
  await page.getByRole("button", { name: "Sửa ngân sách" }).click();
  await page.getByRole("button", { name: "Bỏ ngân sách" }).click();
  await page.getByText("Đặt ngân sách tháng").click();
  await page.locator(".bud22__form input").fill("1000000");
  await page.getByRole("button", { name: "Lưu ngân sách" }).click();
  await expect(page.getByText("Vượt ngân sách sau các khoản định kỳ")).toBeVisible();

  // Hỏi Mimi vì sao: mở màn Phân tích, câu hỏi điền sẵn, chưa tự gửi.
  await page.getByRole("button", { name: "Hỏi Mimi vì sao" }).click();
  await expect(page.locator("textarea")).toHaveValue(/^Vì sao tiền /, { timeout: 15_000 });

  // Tháng cũ: không có ngân sách và Mimi nhận thấy.
  await page.goto("/spending");
  await page.getByRole("button", { name: "Tháng trước" }).click();
  await expect(page.getByText("Đã hết tháng")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Mimi nhận thấy" })).toHaveCount(0);

  // Lịch sử đơn hàng (anh chốt 09/11): bấm ở Sổ chi tiêu thì sang tab Lịch sử đơn hàng của Kế hoạch.
  await page.getByRole("link", { name: "Lịch sử đơn hàng" }).click();
  await expect(page).toHaveURL(/\/plan\?tab=history/);
  await expect(page.getByRole("button", { name: "Lịch sử đơn hàng" })).toHaveAttribute("aria-pressed", "true");
  expect(errors).toEqual([]);
});
