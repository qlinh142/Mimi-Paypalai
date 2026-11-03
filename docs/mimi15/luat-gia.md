# Luật giá của Mimi

Nguồn gốc: ADR-004, mẹ chốt các mục dưới đây ngày 02/11/2026. Sửa luật nào thì ghi ngày và lý do ở mục "Lịch sử thay đổi" cuối file, để sau này cải tiến có căn cứ.

## 1. Hai loại giá

1. **Giá chung** lấy từ nguồn công khai trên mạng (tra Google bằng Gemini, đọc trang web siêu thị, sau này thêm đối tác). Mọi nhà trong cùng một nước đều thấy.
2. **Giá riêng** là giá trên hóa đơn của từng tài khoản. Chỉ nhà đó thấy.
3. Giá riêng không bao giờ vào bảng giá chung, không gửi lên để trộn với nhà khác. Cho người dùng tự bật "chia sẻ giá ẩn danh" để sau, khi có luật sư duyệt.

## 2. Chọn giá hiện cho từng món

1. Lấy giá online rẻ nhất của nước người dùng.
2. Chưa có giá online, hoặc giá online cao hơn giá nhà mình từng mua: hiện giá nhà mình, ghi "chỉ nhà mình thấy", kèm giá online để so.
3. Không có giá nào: để trống, nhắc "có giá vào sáng hôm sau hoặc bấm Cập nhật giá".
4. Việc so làm ngay trên máy người dùng.
5. Lưu ý: giá online theo quy cách nơi bán ghi (ví dụ lốc 4x180ml), giá nhà mình theo đơn vị đã mua, nên so chỉ tương đối. Mã món chuẩn sau này sẽ làm chính xác hơn.

## 3. Theo nước

1. Cài đặt có ô **Quốc gia**. Chưa chọn thì đoán theo múi giờ của máy (Asia/Singapore là Singapore, còn lại là Việt Nam).
2. Bảng giá chung tách theo nước. Nhà ở nước nào chỉ thấy giá nước đó.
3. Mỗi nước tính giá bằng tiền của nước đó: Việt Nam VND, Singapore SGD.
4. Chia theo thành phố để sau, khi đủ người dùng.

## 4. Sổ nguồn của mỗi nước

1. Mỗi nước có một sổ nguồn: tên cửa hàng, trang web, **link ngành hàng** (ví dụ trang "Sữa các loại") và **mẫu link tìm kiếm** của web. Không lưu link từng sản phẩm, vì link sản phẩm hay đổi và có nhiều bản trùng.
2. Sổ ban đầu: Việt Nam có WinMart, Co.opmart, Bách Hóa Xanh, GO!, AEON, Lotte Mart, MM Mega Market. Singapore có FairPrice, Cold Storage, Sheng Siong, RedMart, Giant.
3. Web mới chỉ vào sổ khi ra giá hợp lệ ít nhất 2 lần.
4. Trang nào đọc 2 lần liền không ra giá (thường do web hiện giá bằng mã chạy trên trình duyệt) thì đánh dấu, web đó quay về tìm Google.
5. Shopee, Lazada: chỉ lấy giá từ gian hàng chính hãng (Mall), bỏ người bán lẻ.
6. Giá ở cửa hàng ngoài sổ vẫn nhận, nhưng phải có link nguồn.

## 5. Cách tra để tiết kiệm

1. Chỉ tra món đang nằm trong kế hoạch của các nhà ở nước đó. Món nhiều nhà cần chỉ tra một lần, món nhiều nhà cần được tra trước.
2. Gom món cùng ngành hàng, hỏi AI tối đa 5 món một lần.
3. Ngành hàng đã có link trong sổ: đưa link cho AI đọc trước (công cụ đọc trang của Gemini). Không ra đủ giá thì mới tìm Google cho các món còn thiếu.
4. Câu hỏi gửi AI chỉ có tên món và nước, không có thông tin người dùng. AI chỉ được báo giá thấy trên trang, không đoán.

## 6. Job đêm

1. Chạy 2 giờ sáng giờ Việt Nam, tối đa 200 món mỗi đêm (biến `PRICE_SYNC_LIMIT`).
2. Món đã tra trong 3 ngày thì bỏ qua. Tra không ra giá vẫn ghi dòng rỗng để món đó nghỉ 3 ngày.
3. Nguồn lỗi 5 lần liền thì dừng lượt đó.
4. Mỗi món giữ tối đa 5 giá, mỗi nơi bán một giá rẻ nhất. Bỏ giá vô lý (VN: dưới 1.000đ hoặc trên 50 triệu; SG: dưới 0,1 hoặc trên 20.000 SGD) và link lạ.
5. Lịch sử giá chung mỗi món giữ 30 ngày, Firestore tự xóa dòng cũ hơn.

## 7. Nút "Cập nhật giá"

1. Nằm dưới khối "Giá tốt nhất" trong kế hoạch. Chỉ tra các món của kế hoạch đó, tối đa 12 món mỗi lần.
2. Món đã tra trong 24 giờ thì giữ nguyên.
3. Mỗi người 3 lần mỗi ngày. Không có món nào cần tra thì không tính lượt.
4. Dưới nút hiện số lần còn lại hôm nay và lần cập nhật gần nhất (lưu trên máy chủ).

## Lịch sử thay đổi

- 02/11/2026: chốt hai loại giá, chọn giá thấp hơn giữa online và giá nhà mình, nút Cập nhật giá 3 lần mỗi ngày và giữ 24 giờ (ban đầu 5 lần và 12 giờ), lịch sử 30 ngày, quét theo nước, sổ nguồn lưu link ngành hàng thay vì link sản phẩm, Shopee và Lazada chỉ lấy gian hàng chính hãng.
