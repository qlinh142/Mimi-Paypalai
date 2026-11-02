# Kịch bản demo Mimi 1.5 (vòng cơ bản)

Chốt bởi Quang Linh, 31/10/2026 03:53. Phần chi tiết màn và dữ liệu mẫu do Claude đề xuất, chờ bạn duyệt.

**Mimi 1.5** = hoàn thiện nốt 1.0 và chuẩn bị nền để nâng lên 2.0.
Hình user flow: `assets/quy-trinh/user-flow-demo.png`

**Ý chính:** Mimi là **một thành viên trong gia đình**. Chị Lan đi siêu thị về, Mimi giúp ghi lại, biết nhà còn gì, cùng lên kế hoạch cho tuần sau. Mua về thì vòng lặp bắt đầu lại.

## Các cảnh

| Cảnh | Câu chuyện | Màn trong app | Mimi nói / hiện gì | Hiện trạng |
|---|---|---|---|---|
| **1. Về nhà, quét bill** | Chị Lan đi siêu thị về, hóa đơn để trên bàn. Mimi quét bill. | Màn chính → nút **Quét hóa đơn** → camera | "Để Mimi ghi lại giúp mẹ nhé!" | ✅ có |
| **2a. Mimi đọc bill** | Mimi đọc ra từng món, giá, cửa hàng. Chị Lan xem lại, sửa nếu cần, bấm Lưu. | Màn **xác nhận hóa đơn** | Danh sách món; 1 món Mimi chưa chắc nhóm nên hỏi lại | ✅ có |
| **2b. Vào Đồ dùng** | Các món tự cộng vào Đồ dùng. | Tab **Đồ dùng** | "Đã thêm 8 món vào tủ nhà mình" | ✅ có, cần lời thoại |
| **2c. Vào Sổ chi tiêu** | Số tiền tự cộng vào Sổ chi tiêu. | Tab **Sổ chi tiêu** | Tổng tháng tăng; hóa đơn mới ở đầu danh sách | ✅ có |
| **3a. Kiểm tra tủ lạnh** | Chị Lan xem nhà còn gì, món nào sắp hết. | Tab **Đồ dùng** → Sắp hết / Đang dùng | "Mẹ ơi, sữa còn khoảng 2 ngày, trứng còn 3 quả" | ✅ có ("Sắp hết" đơn giản) |
| **3b. Lên kế hoạch tuần sau** | Tạo Kế hoạch "Đi chợ thứ Bảy", thêm món sắp hết và món tự nhập. | Tab **Kế hoạch** | Ước tính chi theo lịch sử giá, nơi mua rẻ nhất | ✅ có, cần chia tab con cho gọn |
| **3c. Chọn ai đi mua** | Ba lựa chọn: **Tự đi mua** · **Nhờ người nhà** · **Mimi mua** | Kế hoạch → **Ai đi mua?** | Mimi mua: nhãn "Sắp ra mắt" | 🟡 cần làm màn chọn 3 cách |
| **4. Mua về, quay lại vòng** | Mua về có hóa đơn mới → quay lại Cảnh 1. Món đã mua tự gạch khỏi Kế hoạch. | Quét → Kế hoạch | "Đã mua đủ 5/6 món, còn Trứng chưa mua" | ✅ có (đối chiếu Plan) |

## Quyết định đi kèm

- **Mimi là thành viên gia đình:** dùng robot Mimi đã có (logo) xuất hiện ở các thời điểm chính, kèm một câu nói ngắn. Đây là phần mới (cần brief ở bước Mockup).
- **Nhờ người nhà mua:** demo chỉ cần thấy **gửi đi thành công**. Màn của người nhận thiết kế chi tiết sau, đúng như bạn nói.
- **Mimi mua:** nút có mặt nhưng khóa, ghi "Sắp ra mắt". Giữ nguyên câu chuyện mà không phải làm luồng PayPal.
- **Quét bill khi demo:** dùng Vertex AI (Gemini) thật với hóa đơn thật. Khi tập trên máy thì dùng hóa đơn mẫu.

## Dữ liệu mẫu đề xuất (bạn sửa nếu muốn)

- **Nhà:** chị Lan, anh Minh, 2 con. Mimi đã có lịch sử 3 tuần mua sắm.
- **Hóa đơn cảnh 1:** Bách Hóa Xanh, 8 món, khoảng 412.000đ (sữa tươi, trứng gà 10 quả, thịt ba chỉ, rau cải, gạo ST25, nước mắm, nước rửa chén, bánh quy).
- **Cảnh 3a:** sắp hết gồm Sữa tươi (2 ngày), Dầu gội (5 ngày), Trứng (3 quả).
- **Ngày "hôm nay":** lấy theo ngày thật khi mở app, để không còn lỗi "Mua ngay" (V5).

## Đã chốt (31/10 04:21)

1. **Lời thoại:** Mimi xưng **"Mimi"**, gọi chị Lan là **"mẹ"**. Mimi như đứa con trong nhà.
2. **Ngày trong app:** theo ngày thật.
3. **Quét bill:** dùng **Vertex AI (Gemini) thật** với hóa đơn thật, vì bạn đã có sẵn Vertex. Dữ liệu mẫu chỉ dùng khi chạy thử trên máy.
4. **Dữ liệu mẫu:** dùng như đề xuất ở trên.

## Đã chốt (31/10 15:17)

Bạn chốt user flow demo theo 5 cảnh trên. Cảnh 3c "Chọn ai đi mua" đã có trong chi tiết Kế hoạch (Tôi · Gửi cho thành viên · Mimi khóa), PR #2. Mô tả từng màn: [mimi-1-5-tong-the.md](mimi-1-5-tong-the.md).
