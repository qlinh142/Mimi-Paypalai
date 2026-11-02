import { expect, test, type Page } from "@playwright/test";

// Kịch bản demo đã chốt (notes/kich-ban-demo.md), chạy trên bản thử với dữ liệu mẫu.
// Mỗi bước kiểm tra đúng điều người xem demo sẽ thấy.

async function signIn(page: Page) {
  await page.goto("/?ui=vi");
  await page.locator("input[type=email]").fill("lan@gmail.com");
  await page.locator("input").nth(1).fill("Lan");
  await page.getByRole("button", { name: "Nhận link đăng nhập" }).click();
  await page.getByText("Mở link (bản thử)").click();
  await expect(page.getByText("Chào Lan!")).toBeVisible();
}

async function home(page: Page) {
  await page.getByRole("link", { name: "Về Trang chủ" }).or(page.getByRole("button", { name: "Về Trang chủ" })).first().click();
  await expect(page.getByText("Chào Lan!")).toBeVisible();
}

test("kịch bản demo: quét bill, Đồ dùng, Sổ chi tiêu, Kế hoạch", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));

  await signIn(page);

  // Cảnh 1 và 2a: quét bill, Mimi đọc ra từng món, một món hỏi lại nhóm, bấm Lưu.
  await page.getByText("QUÉT HÓA ĐƠN").click();
  const shot = await page.screenshot({ type: "jpeg" });
  await page.locator("input[type=file]").first().setInputFiles({ name: "bill.jpg", mimeType: "image/jpeg", buffer: shot });
  await page.getByRole("button", { name: /^Xong 1 hóa đơn/ }).click();
  await expect(page.getByText("Sữa tươi không đường 1L")).toBeVisible();
  await expect(page.getByText("Chưa có nhóm hàng")).toBeVisible();
  await page.getByRole("button", { name: "Lưu hóa đơn" }).click();
  await expect(page.getByText("Đã lưu hóa đơn")).toBeVisible();
  await expect(page.getByText("212.000đ")).toBeVisible();

  // Cảnh 2b và 3a: Đồ dùng có nhóm Cần mua ngay / Sắp hết; Dầu gội còn khoảng 5 ngày.
  await page.getByRole("button", { name: "Về Trang chủ" }).click();
  await page.getByRole("link", { name: /^Đồ dùng/ }).first().click();
  await expect(page.getByText("CẦN MUA NGAY")).toBeVisible();
  await expect(page.getByText("SẮP HẾT")).toBeVisible();
  await expect(page.getByText("Sữa tươi không đường").first()).toBeVisible();
  await expect(page.getByText(/Dầu gội/).first()).toBeVisible();
  await expect(page.getByText("Dự kiến còn khoảng: 5 ngày")).toBeVisible();

  // Cảnh 2c: Sổ chi tiêu có tổng tháng và nhãn danh mục (lấy từ categories.json).
  await home(page);
  await page.getByRole("link", { name: /^Sổ chi tiêu/ }).first().click();
  await expect(page.getByText(/^Tháng \d+\/\d{4}$/)).toBeVisible();
  await expect(page.getByText("Sữa và chế phẩm sữa")).toBeVisible();
  await expect(page.getByText("Khoản chi định kỳ").first()).toBeVisible();

  // Cảnh 3b và 3c: Kế hoạch có gợi ý món sắp hết, ước tính chi, ba cách mua và Mimi đang khóa.
  await home(page);
  await page.getByRole("link", { name: /^Kế hoạch/ }).first().click();
  await page.getByText("Giỏ mua sắm", { exact: true }).first().click();
  await expect(page.getByText("Gợi ý mua")).toBeVisible();
  await expect(page.getByText(/Dự kiến chi/)).toBeVisible();
  await expect(page.getByRole("button", { name: "Tôi", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Gửi cho thành viên" })).toBeVisible();
  const mimi = page.getByRole("button", { name: /Mimi.*Sắp ra mắt/s });
  await expect(mimi).toBeVisible();
  await expect(mimi).toBeDisabled();

  expect(errors).toEqual([]);
});
