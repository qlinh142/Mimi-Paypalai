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
  const card = page.locator('.ins22[data-insight^="pi:"]');
  await expect(card).toContainText("Giá Nước rửa chén đang cao hơn 12% so với lần mua gần nhất.");
  // Món không tăng giá: không có card.
  await go(page, "/items/sua-tuoi");
  await expect(page.getByRole("heading", { name: "So sánh giá" })).toBeVisible();
  await expect(page.locator('.ins22[data-insight^="pi:"]')).toHaveCount(0);

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
  await expect(card).toHaveCount(0);
  // Điểm 7 vẫn ở đó: mỗi card ẩn riêng.
  await expect(page.locator('.ins22[data-insight^="fq:"]')).toContainText("3 tháng qua nhà mình mua Nước rửa chén 3 lần");

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

test("Mimi nhận thấy: các điểm chạm ở Sổ chi tiêu, định kỳ, kế hoạch, đơn online, thông báo, quét hóa đơn", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.setViewportSize({ width: 393, height: 852 });
  await signIn(page);
  const open = async (title: string) => {
    const sheet = page.getByRole("dialog", { name: title });
    await expect(sheet.locator(".ag-studio-layout")).toBeVisible({ timeout: 30_000 });
    await page.keyboard.press("Escape");
    await expect(sheet).toHaveCount(0);
  };

  // Sổ chi tiêu không đặt nút Phân tích trên từng thẻ (anh chốt 12/11); phân tích nằm ở màn chi tiết.
  await go(page, "/spending");
  await expect(page.locator(".noticed22__card")).toHaveCount(4);
  await expect(page.locator(".ana22")).toHaveCount(0);
  // 2: bấm Xem hóa đơn ở thẻ cửa hàng, Chi tiết hóa đơn có phân tích cửa hàng.
  await page.locator(".noticed22__card").filter({ hasText: "Bách Hóa Xanh chiếm" }).getByRole("button", { name: /Xem hóa đơn/ }).click();
  await expect(page.locator(".ins22")).toContainText("Tháng này Bách Hóa Xanh chiếm 46% số tiền nhà mình chi.");
  await page.locator(".ins22").getByRole("button", { name: "Xem phân tích" }).click();
  await open("Chi ở Bách Hóa Xanh");
  // 5 và 3: dòng phụ của Còn tiêu được dẫn sang Sắp chi, ở đó có hai card phân tích.
  await go(page, "/spending");
  await page.locator(".safe22").getByRole("link", { name: /Sau các khoản định kỳ/ }).click();
  await expect(page.locator('.ins22[data-insight^="sf:"]')).toContainText("Tháng này còn tiêu được");
  await expect(page.locator('.ins22[data-insight^="bl:"]')).toContainText("Tiền điện đến hạn hôm nay, 760.000đ.");
  await page.locator('.ins22[data-insight^="sf:"]').getByRole("button", { name: "Xem phân tích" }).click();
  await open("Còn tiêu được");
  await go(page, "/spending");
  // 16: đặt ngân sách thấy trung bình 3 tháng.
  await page.getByRole("button", { name: "Sửa ngân sách" }).click();
  await expect(page.locator(".ins22")).toContainText("3 tháng qua nhà mình chi trung bình 1.070.000đ mỗi tháng.");
  await page.keyboard.press("Escape");

  // 8: Tiền điện kỳ này cao hơn trung bình.
  await go(page, "/items/recurring/dien");
  await expect(page.locator(".ins22")).toContainText("Tiền điện kỳ này cao hơn trung bình các kỳ trước 26%.");

  // 10 và 11: so sánh giá; đổi sang giá đắt hơn thì card cả kế hoạch hiện.
  await go(page, "/plan/default/compare");
  await expect(page.locator('.ins22[data-insight^="cp:"]')).toContainText("Nước rửa chén: nơi bán rẻ nhất vẫn đắt hơn lần mua trước 12%.");
  await page.locator(".cmpi22").nth(1).getByRole("radio").nth(1).click();
  await expect(page.locator('.ins22[data-insight^="pk:"]')).toContainText("đắt hơn lần mua trước 3.700đ");
  await page.locator(".cmpi22").nth(1).getByRole("radio").nth(0).click();
  // 12: bước 3 nhắc ngân sách theo giá đã chọn.
  await go(page, "/plan/default/confirm");
  await expect(page.locator(".ins22")).toContainText("Sau kế hoạch này, tháng này còn");

  // 15: đơn online đã hoàn tiền.
  await go(page, "/orders/o-mau");
  await expect(page.locator(".ins22")).toContainText("Đã hoàn 37.000đ, Sổ chi tiêu đã trừ.");
  await page.locator(".ins22").getByRole("button", { name: "Xem phân tích" }).click();
  await open("Tiền của đơn này");

  // 18: tóm tắt tuần ở Thông báo.
  await go(page, "/notifications");
  await expect(page.locator(".ins22")).toContainText("Tuần trước nhà mình chi");

  // 13 và 14: lưu hóa đơn xong thấy chuyến mua so với dự kiến và món đắt hơn lần trước.
  await go(page, "/");
  await page.getByText("QUÉT HÓA ĐƠN").click();
  const shot = await page.screenshot({ type: "jpeg" });
  await page.locator("input[type=file]").first().setInputFiles({ name: "bill.jpg", mimeType: "image/jpeg", buffer: shot });
  await page.getByRole("button", { name: /^Xong 1 hóa đơn/ }).click();
  await expect(page.getByText("Sữa tươi không đường 1L")).toBeVisible();
  await expect(page.getByText("→ Giỏ mua sắm").first()).toBeVisible();
  await page.getByRole("button", { name: "Lưu hóa đơn" }).click();
  await expect(page.locator('.ins22[data-insight^="tp:"]')).toContainText("các món trong kế hoạch chi 142.000đ, dự kiến 107.000đ");
  await expect(page.locator('.ins22[data-insight^="pr:"]')).toContainText("1 món đắt hơn lần mua trước");
  expect(errors).toEqual([]);
});
