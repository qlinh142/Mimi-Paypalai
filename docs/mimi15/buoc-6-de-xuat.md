# Bước 6 · Functional spec · Đề xuất để mẹ chốt (bản 2)

Ngày: 31/10/2026 16:55. **Bản 2 lập lại từ đầu theo prototype mẹ đã duyệt lúc 16:34** (`notes/mimi-1-5-tong-the.md`), từng màn 0 đến 6. Bản 1 (16:45) chỉ gán lại ưu tiên trên danh sách 46 dòng cũ của sáng nay, vốn viết theo spec 2.0 trước khi có prototype, nên đã bỏ.

**Đối chiếu với danh sách cũ:**
- Danh sách cũ có nhiều dòng không còn trên màn (Meal Plan, Delta, PayPal, Mandate, đơn hàng…). Các dòng này nay gom vào mục **Treo** ở cuối.
- Danh sách cũ lại thiếu nhiều thứ mới có trong prototype: bảng chọn kế hoạch, Giỏ mua sắm, xóa kế hoạch, biểu tượng kế hoạch, Sắp chi, Địa điểm mua hàng, mã mời 48 giờ, vai trong nhà…

**Cách đọc:**
- Cột **Máy chủ:**
  - ✅ = chạy được với máy chủ thật.
  - 💾 = chỉ lưu trên máy đang dùng.
  - 🐞 = có lỗi.
  - ❌ = chưa có.
- Cột **Ưu tiên:**
  - **P1** = cần cho bản chạy được.
  - **P2** = nên có.
  - **P3** = để sau.
- Mọi dòng dưới đây đều đã có trên prototype, trừ những dòng ghi ❌.

---

## Phần A · Tính năng theo từng màn (55 dòng, đã có 49)

### Màn 0 · Vào app

| # | Tính năng | Máy chủ | Ưu tiên |
|---|---|---|---|
| 0.1 | Đăng nhập bằng link qua email | ✅ | P1 |
| 0.2 | Đăng xuất thì xóa dữ liệu trên máy (N2) | ✅ | P1 |
| 0.3 | **Offline Outbox & tự đồng bộ** (xem việc 4, gồm lỗi N3) | 🐞 | **P1 · cần làm** |

### Màn 1 · Home

| # | Tính năng | Máy chủ | Ưu tiên |
|---|---|---|---|
| 1.1 | Lời chào "Chào [tên]!" | ✅ | P1 |
| 1.2 | 4 ô: tiền chi tháng này, số món sắp hết, số món cần mua, Cài đặt | ✅ | P1 |
| 1.3 | Nút tròn Quét hóa đơn | ✅ | P1 |
| 1.4 | Chuông thông báo | ✅ | P2 |

### Màn 2 · Quét

| # | Tính năng | Máy chủ | Ưu tiên |
|---|---|---|---|
| 2.1 | Chụp nhiều ảnh, báo lỗi từng ảnh | ✅ | P1 |
| 2.2 | Đọc hóa đơn bằng Gemini (demo dùng thật, chạy mẫu thì giả lập) | ✅ | P1 |
| 2.3 | Màn xác nhận: sửa dòng, chọn nhóm | ✅ | P1 |
| 2.4 | Nhắc không chụp thông tin cá nhân | ✅ | P1 |
| 2.5 | Báo hóa đơn trùng | ✅ | P2 |
| 2.6 | **Hóa đơn trừ vào kế hoạch dự kiến diễn ra trước** (xem việc 1). Hiện app tự lấy kế hoạch mở gần nhất | ❌ | **P1 · cần làm** (câu 1) |

### Màn 3 · Đồ dùng

| # | Tính năng | Máy chủ | Ưu tiên |
|---|---|---|---|
| 3.1 | Hàng tiêu dùng chia 3 nhóm, thu gọn từng nhóm | ✅ (nhớ nhóm thu gọn: 💾) | P1 |
| 3.2 | Tìm kiếm | ✅ | P1 |
| 3.3 | Thẻ món: số lượng, "Dự kiến còn N ngày", nút giỏ hàng | ✅ | P1 |
| 3.4 | Chi tiết: sửa tên, nhóm | ✅ | P1 |
| 3.5 | Chi tiết: số lượng −/+, Cập nhật sản phẩm | ✅ | P1 |
| 3.6 | Chọn đơn vị hiển thị | 💾 | P3 |
| 3.7 | Lịch sử mua hàng của món: giá gần nhất, thấp nhất, trung bình, từng lần mua | ✅ V3 đã sửa 02/11 (PR #9): đọc toàn bộ lịch sử, tối đa 500 lần mua | P1 |
| 3.8 | Khoản chi định kỳ chia 3 nhóm | ✅ | P1 |
| 3.9 | Sửa định kỳ trên thẻ: lưu, hủy, Đã trả kỳ này, Xóa | ✅ | P1 |
| 3.10 | + Thêm khoản chi | ✅ | P1 |
| 3.11 | Biến thể của cùng một món trừ tồn lẫn nhau (V4) | 🐞 | P2 |

### Màn 4 · Sổ chi tiêu

| # | Tính năng | Máy chủ | Ưu tiên |
|---|---|---|---|
| 4.1 | Chọn tháng, tổng tiền, so với tháng trước | ✅ | P1 |
| 4.2 | Theo nhóm hàng: định kỳ đứng đầu, 5 nhóm và Xem tất cả | ✅ | P1 |
| 4.3 | Tiết kiệm nhiều nhất | ✅ | P2 |
| 4.4 | Địa điểm mua hàng | ✅ | P1 |
| 4.5 | Lịch sử mua hàng: mọi hóa đơn (nơi mua, ngày, tiền) | ✅ (từ 16:32) | P1 |
| 4.6 | **Màn 4c Lịch sử mua hàng, 4d Chi tiết hóa đơn** (xem việc 2) | ❌ | **P1 · cần làm** |
| 4.7 | Sắp chi: định kỳ và hàng tiêu dùng, bấm để xem chi tiết | ✅ | P1 |

### Màn 5 · Kế hoạch mua sắm

| # | Tính năng | Máy chủ | Ưu tiên |
|---|---|---|---|
| 5.1 | Danh sách thẻ: tên, số món, ngân sách, nơi, ngày giờ, trạng thái, người mua | ✅ | P1 |
| 5.2 | + Kế hoạch mới | ✅ | P1 |
| 5.3 | Sửa tên, ngày giờ, nơi mua | ✅ | P1 |
| 5.4 | Chọn biểu tượng kế hoạch: chuyển lên lưu ở máy chủ (câu 6) | 💾 | P2 |
| 5.5 | Danh sách món: sửa số lượng bằng −/+ và dấu tích, xóa bằng thùng rác | ✅ | P1 |
| 5.6 | Gợi ý mua (Cần mua ngay, Sắp hết) với + Thêm | ✅ | P1 |
| 5.7 | Thêm sản phẩm theo tên, Kiểm tra đồ dùng | ✅ | P1 |
| 5.8 | Dự kiến chi, cửa hàng rẻ nhất, so sánh giá | ✅ | P1 |
| 5.9 | Gửi cho thành viên: chọn người mua trong kế hoạch, **gửi khi bấm Lưu kế hoạch** (nút máy bay giấy), xem Việc 4 mục 3 | ✅ | P1 |
| 5.10 | Xóa kế hoạch (bấm 2 lần) | ✅ | P1 |
| 5.11 | Bảng "Thêm vào kế hoạch nào?" và Tạo kế hoạch mới, có Hoàn tác | ✅ | P1 |
| 5.12 | Giỏ mua sắm: ẩn khi trống, Xóa là làm trống | ✅ | P1 |
| 5.13 | Trạng thái Đang lên kế hoạch → Đang đi mua → **Đã mua xong** (tự chuyển khi bấm Mua hàng xong hoặc khi hóa đơn trừ hết món, theo việc 1 và 3) | ✅ | P1 |
| 5.14 | Được nhờ mua: thẻ kế hoạch người nhà gửi | ✅ | P1 |
| 5.15 | **Màn đi mua cho người được nhờ** (đã chốt, xem việc 3 ở Phần D) | ❌ | **P1 · cần làm** |

### Màn 6 · Cài đặt

| # | Tính năng | Máy chủ | Ưu tiên |
|---|---|---|---|
| 6.1 | Hồ sơ: sửa tên Mimi gọi | ✅ | P1 |
| 6.2 | Thông báo & nhắc mua: email, giờ, số ngày báo trước | ✅ | P2 |
| 6.3 | Ngôn ngữ (đổi cả hóa đơn), múi giờ | ✅ | P1 |
| 6.4 | Thành viên: mã mời 48 giờ, sao chép mã | ✅ | P1 |
| 6.5 | Vào nhà khác bằng mã, rời khỏi nhà | ✅ | P1 |
| 6.6 | Chủ nhà xóa thành viên | ✅ | P1 |
| 6.7 | Tên gọi, vai trong nhà (Anh, Chị, Bố, Mẹ…): riêng từng tài khoản, lưu trong tài khoản người đặt (câu 6) | 💾 | P2 |
| 6.8 | Quyền riêng tư & bảo mật | Trang chữ | P3 |
| 6.9 | Đăng xuất | ✅ | P1 |

### Treo (giữ code, hiện "Sắp ra mắt" hoặc không hiện)

Đây là các dòng của danh sách cũ, đều là P3:
- Hôm nay ăn gì, Meal Plan, khẩu phần, lượng cần mua (Delta), hạn dùng.
- Tạo kế hoạch cùng Mimi, BUY / WAIT, quét giá.
- Mimi mua online, Ủy quyền mua online, PayPal, đơn hàng, kiểm tra hàng nhận, refund.
- Dự đoán có lý do và độ tin cậy, khách đến chơi.

---

## Phần B · Lỗ hổng logic, rút ra từ chính prototype

12 lỗ hổng cũ được tìm khi kiểm spec 2.0. Trong đó 9 cái thuộc phần Treo (đếm trùng với Meal Plan, làm tròn quy cách, WAIT, giá online, Mandate, refund…) nên dời lại. Khi bấm thử prototype, Mimi thấy có các chỗ cần chốt sau:

| # | Câu hỏi | Đề xuất của Mimi |
|---|---|---|
| 1 | **Quét hóa đơn thì đối chiếu với kế hoạch nào?** (lỗ hổng 9 cũ) | ✅ Đã chốt: trừ dần vào kế hoạch dự kiến diễn ra trước. Xem việc 1. |
| 2 | **Món có trong 2 kế hoạch,** quét hóa đơn thì đánh dấu đã mua ở đâu? | ✅ Đã chốt theo luật việc 1. |
| 3 | **Kế hoạch "Đã mua xong" thì sao?** Món chưa mua còn lại đi đâu? | ✅ Đã chốt: món chưa mua tự chuyển sang kế hoạch mua sắm mới (cả quét bill và thủ công). Xem câu 3 ở Phần D. |
| 4 | **Xóa kế hoạch đang gửi cho người nhà đi mua** | ✅ Đã chốt: theo luật gửi tuần tự. Chưa "Mua hàng xong" thì hủy và Minh nhận thông báo; đã xong thì chỉ xóa kế hoạch. Xem câu 4 ở Phần D. |
| 5 | **Ai được làm gì trong nhà?** (lỗ hổng 11 cũ) | ✅ Đã chốt: chủ nhà thêm, xóa thành viên; kế hoạch của ai người đó sửa, xóa, gửi; người được nhờ có quyền từ chối. Xem câu 5 ở Phần D. |
| 6 | **Biểu tượng kế hoạch và vai trong nhà chỉ lưu trên máy** | ✅ Đã chốt: biểu tượng lên máy chủ; tên gọi người nhà là sở thích riêng từng tài khoản, hệ thống dùng mã tài khoản. Xem câu 6 ở Phần D. |
| 7 | **Khoản định kỳ có bị cộng hai lần?** (lỗ hổng 12 cũ) | ✅ Đã chốt: giữ nguyên, kỳ đã khớp với hóa đơn thì không cộng nữa. |

---

## Phần C · Tóm lại

Có **49/55 dòng chạy được rồi** (trên prototype và máy chủ thật). Việc P1 còn phải làm là **4 việc**:
1. Hóa đơn trừ vào kế hoạch dự kiến diễn ra trước (2.6). ✅ Đã chốt.
2. Lịch sử mua hàng đầy đủ: máy chủ giữ mọi lần mua (3.7, lỗi V3) và màn 4c, 4d (4.6). ✅ Đã chốt.
3. Màn đi mua cho người được nhờ (5.15). ✅ Đã chốt.
4. Offline Outbox & tự đồng bộ (0.3, gồm lỗi N3). ✅ Đã chốt.


**Bước tiếp theo** (4 việc và 7 câu đã chốt hết):
- Mimi viết Spec 1.5 gồm 4 việc này và luật của câu 1–7.
- Sau đó làm từng việc theo quy trình cổng: Cổng 1 bằng lời → code → ảnh → PR.

---

## Phần D · Giải thích từng vấn đề (kèm ví dụ)

### 4 việc P1 còn phải làm

**Việc 1 · Hóa đơn trừ vào kế hoạch nào** ✅ Đã chốt (01/11 01:50)

**Vấn đề:** hiện app ngầm đối chiếu hóa đơn với kế hoạch mở gần nhất.
- Ví dụ: mẹ mở "Đồ cho bé" hôm qua, hôm nay đi mua đồ cuối tuần về rồi quét bill.
- Kết quả sai: "Đồ cho bé" bị đánh dấu đã mua, còn "Mua đồ cuối tuần" vẫn ghi chưa mua.

**Luật:** món trong hóa đơn được **trừ dần vào kế hoạch có ngày dự kiến đi mua sớm nhất trước**. Còn dư thì trừ tiếp sang kế hoạch kế tiếp.
- Kế hoạch chưa đặt ngày: xếp sau các kế hoạch có ngày, cái nào tạo trước thì trừ trước.
- Kế hoạch đã "Đã mua xong": không trừ vào nữa.
- Món không có trong kế hoạch nào: vẫn ghi vào Sổ chi tiêu và Đồ dùng như thường.
- Món có trong 2 kế hoạch: theo đúng luật trên. Kế hoạch sớm hơn được trừ trước.
- Người được nhờ mua quét bill: trừ vào kế hoạch được gửi cho họ (xem việc 3).

**Ví dụ:**
- "Mua đồ cuối tuần" (02/11) cần 2 hộp sữa và 10 quả trứng.
- "Đồ cho bé" (05/11) cần 3 hộp sữa.
- Mẹ quét bill có 4 hộp sữa và 10 quả trứng.
- Kết quả:
  - "Mua đồ cuối tuần" đủ món, chuyển "Đã mua xong".
  - "Đồ cho bé" nhận 2 hộp, còn thiếu 1 hộp.

**Màn xác nhận hóa đơn:**
- Mỗi món ghi "→ [tên kế hoạch]" để mẹ thấy món sẽ trừ vào đâu.
- Bấm vào thì đổi được kế hoạch, hoặc chọn "Không thuộc kế hoạch nào".
- Tên, số lượng, giá vẫn sửa được như hiện có.

**Việc 2 · Lịch sử mua hàng đầy đủ** ✅ Đã chốt (01/11 02:11)

Gồm 2 phần làm cùng nhau.

**Phần máy chủ:** giữ đủ mọi lần mua.
- Hiện app chỉ tính lại từ 20 lần mua gần nhất, nên lần cũ hơn bị mất.
- Hậu quả: "Giá thấp nhất", "Giá trung bình" và tồn kho có thể sai.
- Cách sửa: lưu từng đợt hàng (lot) thành bản ghi riêng, xem `data-model-1-5.md`.

**Phần màn hình:** Sổ chi tiêu có màn Lịch sử mua hàng riêng. Ảnh bố cục: `assets/mau-thiet-ke/4c-4d-lich-su-chi-tiet-hoa-don.png`, kiểu dáng theo style đang có.

- **Màn 4a Sổ chi tiêu:** giữ phần tóm tắt Lịch sử mua hàng như hiện có. Bấm tiêu đề thì mở 4c, bấm một hóa đơn thì mở 4d.
- **Màn 4c · Lịch sử mua hàng** (mọi tháng):
  1. Nút ‹ và tên màn.
  2. Ô tìm kiếm "Tìm nơi mua, món hàng…": tìm cả tên cửa hàng lẫn tên món. Nút lọc theo cửa hàng và nhóm hàng.
  3. Ô chọn tháng, mặc định "Tất cả tháng".
  4. Mỗi tháng một khối:
     - Tiêu đề "Tháng 10/2026 (8 hóa đơn)".
     - 3 hóa đơn mới nhất: biểu tượng, tên cửa hàng, ngày · giờ, tổng tiền, dấu ›.
     - Dòng "Xem thêm N hóa đơn… ›".
- **Màn 4d · Chi tiết hóa đơn** (chỉ xem, không sửa, không có ảnh bill gốc):
  1. Nút ‹ và tên màn.
  2. Thẻ cửa hàng: tên, ngày · giờ.
  3. Thẻ "Đã trừ vào kế hoạch: [tên] ›".
     - Trừ vào 2 kế hoạch thì ghi cả hai tên.
     - Không thuộc kế hoạch nào thì ẩn thẻ.
  4. "Các món đã mua (N)". Mỗi món gồm biểu tượng nhóm, tên kèm quy cách, "số lượng × đơn giá", thành tiền. Có giảm giá thì thêm dòng xanh lá.
  5. Khối tổng: Tạm tính, Giảm giá, **Tổng tiền**.
  6. Hóa đơn mua hộ: người đi mua thấy đủ món kèm tag "Mua hộ mẹ / Của tôi"; người nhờ chỉ thấy phần món của mình (xem việc 3).
  7. Ghi chú cuối màn: "Thông tin cá nhân (tên khách, số điện thoại, địa chỉ, số thẻ, mã thành viên…) không được lưu."
- **Thông tin cá nhân trên bill:**
  - Gồm tên khách, số điện thoại, địa chỉ, số thẻ, mã thành viên, mã giao dịch thẻ.
  - Phần này được lọc ngay khi đọc hóa đơn, máy chủ không giữ.
  - Ảnh bill không lưu lại sau khi đọc xong.

**Việc 3 · Màn đi mua cho người được nhờ** ✅ Đã chốt (01/11 03:48)

Ảnh demo: `assets/mau-thiet-ke/viec-3-di-mua-ho-chot.png`. Kiểu dáng theo style đang có của app.

**Vấn đề:** mẹ gửi kế hoạch cho Minh đi mua, nhưng hiện Minh chỉ thấy một thẻ, không bấm vào được. Minh không đánh dấu được món đã mua, mẹ không biết Minh mua tới đâu.

**Luồng 5 màn**

1. **Minh nhận việc:**
   - Tab "Được nhờ mua" có số đỏ báo việc mới.
   - Dòng "Mẹ nhờ Minh mua".
   - Thẻ kế hoạch gồm tên, số món, nơi mua, giờ, "Người gửi: Mẹ" và dấu ›. Bấm thẻ thì mở màn 2.
2. **Danh sách mua** (đầu màn là tên kế hoạch):
   - Thẻ tóm tắt: số món, nơi mua, giờ.
   - **Hiện đủ mọi món, không thu gọn.**
     - Mỗi dòng gồm ô tick, tên, số lượng −/+, **thành tiền** (dòng nhỏ dưới ghi "2 × 14.000đ") và nút ✎ sửa.
     - Món đã tick vẫn hiện.
     - Không có "Không mua được": món không tick là chưa mua.
   - **Sau khi quét bill,** danh sách chia 2 khối, mỗi khối có tổng tiền riêng:
     - **"Mua hộ mẹ (N món)":** các món trong kế hoạch. Chữ "mẹ" lấy theo vai trong nhà của người nhờ, ví dụ "Mua hộ chị Lan".
     - **"Của tôi (N món)":** các món khác trên hóa đơn, cũng có ô tick và sửa được.
   - **Làm thủ công:** không chia khối. Đồ Minh mua cho mình thì Minh tự vào tab Đồ dùng của mình để thêm.
   - Ô **ghi chú** (ví dụ "Thiếu 1 món, thay bằng loại khác").
   - Hai nút: **Quét hóa đơn** và **✓ Mua hàng xong**.
   - Dòng nhắc: "Có thể quét hóa đơn để tự tick món khớp, hoặc tick thủ công rồi bấm Mua hàng xong".
3. **Quét hóa đơn:** camera chụp bill.
   - Chụp xong tự quay về màn 2, tự tick các món khớp và hiện khối "Của tôi".
   - Không có màn xác nhận riêng. Minh kiểm và sửa ngay trên danh sách.
4. **Đã ghi nhận:**
   - Nhãn "Có hóa đơn (đã quét)" hoặc "Thủ công".
   - Dòng "Đã gắn vào kế hoạch [tên]".
   - Tổng tiền. Khi có bill thì thêm 2 dòng "Mua hộ mẹ: 120.000đ" và "Của tôi: 15.000đ".
   - "Đã gửi kết quả cho mẹ" và nút **Về kế hoạch**.
5. **Mẹ xem kết quả trên màn chi tiết kế hoạch 5b**, không thêm tab:
   - Thanh tiến độ "Đã mua 5/8 món".
   - Nhãn "Đã mua" ở từng món.
   - Thẻ "Ghi chú của Minh ›".
   - Chuông báo khi Minh bấm "Mua hàng xong".
   - **Không có thẻ hóa đơn của Minh**, vì hóa đơn là chuyện riêng của người đi mua.

**Lưu khi nào**
- **Chưa bấm "Mua hàng xong" thì chưa ghi gì** vào Đồ dùng, Sổ chi tiêu.
  - Tick, sửa và quét bill chỉ lưu tạm.
  - Thoát app rồi mở lại vẫn còn.
- **Bấm "Mua hàng xong"** thì ghi một lần theo luật bên dưới, đồng thời gửi kết quả cho mẹ.

**Sửa tay nội dung món**
- **Sửa ở đâu:** ở mọi kế hoạch (Kế hoạch của tôi và Được nhờ mua), trước khi bấm Mua hàng xong, kể cả sau khi quét bill.
- **Cách sửa:** bấm ✎ thì mở ô sửa gồm tên, số lượng −/+ và **đơn giá**. Bấm ✓ để lưu, 🗑 để bỏ món.
- **Giá:**
  - **Nhập theo đơn giá** (giống bill in đơn giá × số lượng). Đổi số lượng thì thành tiền tự tính lại.
  - Lịch sử giá so theo đơn giá.
  - Hàng cân ký thì nhập đơn giá theo kg.
- **Hiển thị:** thành tiền to, dòng nhỏ dưới ghi "số lượng × đơn giá".
- **Tên:**
  - Món **đã chuẩn hóa** (có trong Đồ dùng hoặc nhóm chuẩn): không sửa tên, dùng tên có sẵn.
  - Món **mới thêm bằng tay:** sửa được tên.
- **Số lượng và giá:** món nào cũng sửa được.

**Ghi vào Đồ dùng và Sổ chi tiêu** (khi bấm "Mua hàng xong")
- **Có quét bill:**
  - Món đã tick ở khối "Mua hộ mẹ": vào Đồ dùng và Sổ chi tiêu của **người nhờ** (mẹ), không vào của Minh.
  - Món đã tick ở khối "Của tôi": vào Đồ dùng và Sổ chi tiêu của **người đi mua** (Minh).
  - Món không tick: không ghi vào đâu, ví dụ dòng AI đọc nhầm hoặc túi nilon.
  - Giảm giá chung cả bill: chia theo tỉ lệ tiền của mỗi bên.
- **Làm thủ công (không bill):** món đã tick vào Đồ dùng và Sổ chi tiêu của mẹ.
  - Số lượng theo danh sách.
  - Tiền theo giá đã nhập. Chưa nhập thì lấy giá mua gần nhất, nhãn "Ước tính".
  - Món chưa từng có giá thì ghi 0đ và nhắc mẹ nhập sau.
- **Ví dụ:** hóa đơn 135.000đ gồm sữa và trứng cho mẹ (120.000đ), cộng 1 lon nước Minh mua cho mình (15.000đ).
  - Mẹ: +120.000đ, nhãn "Minh mua hộ".
  - Minh: +15.000đ.

**Hóa đơn mua hộ: ai thấy gì**
- **Minh (người đi mua):**
  - Màn 4d hiện đủ món của hóa đơn, mỗi món có tag "Mua hộ mẹ" hoặc "Của tôi".
  - Dưới Tổng tiền có 2 dòng phần của mỗi bên.
- **Mẹ (người nhờ):**
  - Không thấy hóa đơn của Minh, cũng không thấy món Minh mua cho mình.
  - Lịch sử mua hàng của mẹ có một khoản "Minh mua hộ · [tên kế hoạch]". Bấm vào thì thấy ngày, nơi mua theo kế hoạch, các món của mẹ và tổng tiền.
- Sổ chi tiêu của mỗi người chỉ cộng phần của mình.

**Việc cần làm ở máy chủ**
- Trạng thái từng món đã có sẵn.
- Cần thêm:
  - Chia hóa đơn theo món cho 2 người. Hiện cả bill vào sổ của người quét.
  - Ghi chi tiêu ước tính khi làm thủ công.
  - Lưu danh sách đang mua (tick, chỗ sửa, các dòng đọc được từ bill) trước khi bấm Mua hàng xong: lưu trên máy ngay, có mạng thì đồng bộ lên máy chủ (việc 4). Ảnh bill không lưu.
  - Lưu ghi chú của người đi mua.
  - Lưu tên và đơn giá sửa tay của món trong kế hoạch.
  - Gửi thông báo cho người nhờ.

**Việc 4 · Offline Outbox & tự đồng bộ** ✅ Đã chốt (01/11 04:11, bổ sung tới 04:36)

**Mục tiêu:** người dùng thao tác bình thường trong siêu thị dù mạng yếu, mất mạng hay hết phiên đăng nhập.
- Mimi luôn lưu việc vào máy trước.
- Có mạng lại thì app tự gửi hoặc tự đồng bộ những gì đang chờ, giống tin nhắn SMS chờ gửi.
- Dữ liệu chỉ rời hàng đợi khi máy chủ xác nhận đã nhận.
- Người dùng không phải làm lại.

**1. Lưu trên máy và đồng bộ là hai việc tách riêng**
- **Lưu trên máy:** khi người dùng sửa số lượng, sửa giá, tick món, ghi chú, bấm Mua hàng xong hay gửi kế hoạch, dữ liệu được lưu **ngay vào app, vào bộ nhớ máy**, có mạng hay không cũng vậy.
- **Đồng bộ / gửi:** app tự làm sau đó. Có mạng chỉ là để đồng bộ nội dung, không phải điều kiện để lưu.
- **Không bao giờ xóa dữ liệu trên máy** trước khi máy chủ xác nhận đã nhận.
- **Lưu trạng thái cuối, không lưu từng thao tác nhỏ.** Ví dụ chuyến mua hộ lưu một bản `ShoppingResult` gồm `plan_id`, các món, số lượng, giá, ghi chú, `completed_at`, `sync_status`. Không lưu chuỗi "giảm sữa 1, tăng trứng 2…".

**2. Hai loại hàng đợi (Outbox)**

| Loại | Gồm | Luồng |
|---|---|---|
| **Data Outbox** | Sửa số lượng, sửa giá, tick món, ghi chú, cập nhật kế hoạch, thay đổi tồn kho, Mua hàng xong | Lưu trên máy → có mạng → đồng bộ lên máy chủ → "Đã đồng bộ" |
| **Message Outbox** | Mẹ gửi / cập nhật kế hoạch cho Minh; Minh gửi kết quả mua hộ cho mẹ | Lưu trên máy → "Đang chờ gửi" → có mạng → tự gửi → "Đã gửi cho Minh" / "Đã gửi kết quả cho mẹ" (người nhận được thông báo) |

**3. Gửi kế hoạch và cập nhật: tuần tự, không sửa cùng lúc**
- **Chọn người mua và nút Lưu** ✅ (chốt 01/11 04:36, ảnh hiện tại `assets/mau-thiet-ke/5c-chon-thanh-vien-hien-tai.png`):
  - Trong kế hoạch, bảng "Chọn thành viên gia đình" **chỉ để chọn người mua**, không gửi ngay. Nút cuối bảng là **"Chọn"** thay cho "Gửi kế hoạch". Chọn xong, kế hoạch hiện "Người mua: Minh".
  - Kế hoạch **chỉ được gửi khi bấm Lưu kế hoạch**. Nút Lưu có **biểu tượng máy bay giấy** (giống nút gửi tin nhắn).
  - Đã chọn người mua: bấm máy bay giấy là lưu và gửi (lần đầu là gửi, các lần sau là "Gửi / Cập nhật").
  - **Bắt buộc chọn người mua** thì mới bấm được máy bay giấy. Chưa chọn thì nút mờ đi. Muốn tự đi mua thì chọn "Tôi" (chốt 01/11 05:02).
  - **Rời kế hoạch mà chưa bấm Lưu:** những gì đã sửa vẫn được giữ trên máy, kế hoạch ở trạng thái **"Chưa hoàn thành"** (giống thư nháp), chưa gửi cho ai. Vào lại kế hoạch thì sửa tiếp và bấm máy bay giấy để gửi (chốt 01/11 05:02).
  - Kế hoạch đã gửi rồi mà mẹ sửa tiếp rồi rời đi chưa bấm Lưu: Minh vẫn giữ bản đã gửi trước đó. Phần sửa mới chỉ nằm trên máy mẹ, ở trạng thái "Chưa hoàn thành". ✅ (duyệt 01/11 05:03)
- **Kế hoạch đã gửi là cố định.** Mẹ bấm Lưu (máy bay giấy) thì Minh nhận đúng bản lúc gửi. Không có chuyện hai người sửa cùng lúc theo thời gian thực.
- **Minh chưa bấm "Mua hàng xong":**
  1. Mẹ sửa kế hoạch rồi bấm **"Gửi / Cập nhật"**.
  2. Minh thấy bản mới, kèm thông báo có giờ, ví dụ "Mẹ đã cập nhật kế hoạch lúc 10:32".
  3. Minh đi mua theo bản mới.
  
  Nếu Minh đã tick vài món: món còn trong bản mới giữ dấu tick và chỗ đã sửa, món mẹ bỏ thì mất khỏi danh sách.
- **Minh đã bấm "Mua hàng xong"** (có bill hay chưa chụp bill đều vậy): lần nhờ mua đó **đã hoàn thành**, không cập nhật được nữa. Mẹ muốn nhờ tiếp thì phải **tạo lần nhờ mua mới**.
- **Một trong hai người mất mạng** ✅ (chốt 01/11 04:27): bản cập nhật của mẹ mà Minh **chưa nhận được** trước khi bấm "Mua hàng xong" thì **coi như một kế hoạch mua sắm mới**.
  - Minh vẫn hoàn thành theo bản Minh đang có, và kết quả đó gửi cho mẹ như bình thường.
  - Bản cập nhật của mẹ được tách thành kế hoạch mới trong danh sách của mẹ. Mẹ thấy thông báo "Minh đã mua xong theo bản trước. Bản cập nhật lúc 10:32 được lưu thành kế hoạch mới", rồi gửi lại nếu muốn.
  - Áp dụng cả khi mẹ mất mạng (bản cập nhật đến máy chủ sau khi Minh đã xong) lẫn khi Minh mất mạng (Minh chưa tải được bản mới).

**4. Quét bill khi mất mạng** ✅ (chốt 01/11 04:33)
- Quét bill cần AI đọc, nên **phải có mạng**. Mất mạng thì **không chụp chờ**, cũng không lưu ảnh tạm.
- Bấm Quét hóa đơn lúc mất mạng thì báo ngay: **"Hiện không có mạng nên Mimi chưa đọc được hóa đơn. Mẹ nhập thủ công nhé."** Kèm nút **"Nhập thủ công"** quay về danh sách để tick, sửa số lượng và giá.
- Nhập thủ công xong, bấm "Mua hàng xong" thì lưu trên máy và chờ gửi khi có mạng, như mục 1 và 2.
- **Có mạng:** quét bill như bình thường. AI không tự ghi thẳng vào dữ liệu. Người dùng kiểm tên món, số lượng, đơn giá, thành tiền, kế hoạch liên quan rồi mới xác nhận. Ảnh bill không lưu lại sau khi đọc xong.

**5. Hết phiên đăng nhập**
- Máy chủ báo lỗi đăng nhập thì app làm mới phiên rồi gửi lại.
- **Làm mới được:** tiếp tục gửi.
- **Làm mới không được:**
  - Không xóa dữ liệu, giữ trạng thái chờ.
  - Báo: "Có 2 thay đổi chưa đồng bộ. Đăng nhập lại để tiếp tục gửi."

**6. Chống gửi trùng**
- Mỗi bản ghi đồng bộ có ID riêng: `sync_id`, `shopping_result_id`.
- Trường hợp cần chặn: máy chủ đã nhận, nhưng mạng đứt trước khi app nhận phản hồi, nên app gửi lại.
- Máy chủ nhận ra ID đã xử lý thì bỏ qua. Không cộng chi tiêu hai lần, không trừ tồn hai lần, không tạo hai hóa đơn, không gửi kết quả mua hộ hai lần.

**7. Phạm vi:** chỉ các thao tác quan trọng.
- Sửa số lượng, sửa giá, tick món, ghi chú.
- Cập nhật kế hoạch, Mua hàng xong.
- Gửi / cập nhật kế hoạch, gửi kết quả mua hộ, từ chối yêu cầu.
- Quét bill **không** nằm trong phạm vi offline: mất mạng thì nhập thủ công.

Không biến cả app thành offline hoàn toàn.

**8. Trạng thái trên màn hình:** dòng nhỏ, không bật popup liên tục.
- "✓ Đã lưu trên máy", "🕓 Chờ đồng bộ", "🕓 Đang chờ gửi".
- "Hiện không có mạng, mẹ nhập thủ công nhé" (khi bấm quét bill lúc mất mạng).
- "✓ Đã đồng bộ", "✓ Đã gửi cho Minh", "✓ Đã gửi kết quả cho mẹ".
- "⚠ Chưa đồng bộ".

**9. Việc cần làm trong code**
- **Đã có:**
  - Hàng đợi ghi khi mất mạng (`frontend/src/lib/offlineQueue.ts`) cho sửa tồn và tick giỏ. Hàng đợi tự gửi khi có mạng hoặc khi mở lại app.
  - Mỗi thao tác đã mang `Idempotency-Key`.
- **Sửa lỗi N3:** hiện hết phiên thì hàng đợi coi là "bị từ chối hẳn" và xóa thao tác. Sửa thành làm mới phiên rồi gửi lại, không xóa.
- **Máy chủ:**
  - Nhớ `Idempotency-Key` để chống gửi trùng. Hiện chỉ phần đơn hàng online có.
  - Lưu **bản kế hoạch đã gửi** cho người được nhờ, thay vì cho đọc trực tiếp bản mới nhất như hiện nay.
- **Thêm mới:**
  - Message Outbox.
  - Thông báo "không có mạng, nhập thủ công" ở nút Quét hóa đơn.
  - Các dòng trạng thái.

**10. Các điểm chốt thêm**
- **Không chia đợt:** vì mất mạng thì nhập thủ công, không có phần chụp bill chờ, nên việc 4 làm một lần.
- **Mẹ lập kế hoạch lúc mất mạng** ✅ (chốt 01/11 04:34): cũng như email.
  - Tạo, sửa kế hoạch (tên, món, số lượng, ngày, nơi mua) thì lưu ngay trên máy, giống thư nháp.
  - Bấm Lưu (máy bay giấy) khi đã chọn Minh thì nằm ở hộp thư đi, có mạng thì tự gửi, rồi chuyển "Đã gửi".
  - Lúc mất mạng, chọn món lấy từ danh sách Đồ dùng đã có trên máy, còn Dự kiến chi tính theo giá đã lưu lần trước.
- **Việc chờ gửi lâu** ✅ (chốt 01/11 04:28):
  - Bấm "Mua hàng xong" lúc mất mạng thì lưu trên máy. Có mạng thì tự gửi tiếp.
  - Sau **vài tiếng** vẫn chưa gửi được thì có thông báo "Chưa gửi được kết quả mua hàng cho mẹ".
  - ✅ (chốt 04:30) Nhắc sau **3 tiếng**, mỗi việc nhắc 1 lần.
  - *Kỹ thuật:* app vẫn xin iPhone quyền "lưu bền" để iPhone không tự xóa dữ liệu chờ gửi. Thông báo hiện được cả khi app đang đóng nếu người dùng đã thêm Mimi ra màn hình chính và cho phép thông báo. Nếu không thì thông báo hiện lúc mở app.
- **Chỗ xem việc đang chờ** ✅ (chốt 01/11 04:29): **gửi kế hoạch hoạt động như gửi email.**
  - Bấm gửi lúc mất mạng thì việc nằm trong "hộp thư đi". Có mạng thì tự gửi, rồi chuyển sang "Đã gửi".
  - Tab Cài đặt có chấm đỏ nhỏ khi còn việc chưa gửi.
  - Mục **"Đang chờ đồng bộ (N)"** trong Cài đặt liệt kê từng việc, ví dụ "Gửi kế hoạch cho Minh · 10:32", "Kết quả mua hàng cho mẹ · 11:05".
  - Mỗi dòng có nút **Thử lại**.

### 7 câu hỏi (đã chốt hết)

**Câu 1, câu 2** ✅ Đã chốt, gộp vào việc 1 ở trên.

**Câu 3 · Món chưa mua khi kế hoạch xong** ✅ Đã chốt (01/11 04:13)

**Luật:** món chưa mua **tự động chuyển sang một kế hoạch mua sắm mới**. Luật này áp dụng cho cả hai cách:
- **Quét bill:** món trong kế hoạch không có trên hóa đơn.
- **Làm thủ công:** món không được tick khi bấm "Mua hàng xong".

**Ví dụ:** "Mua đồ cuối tuần" có 5 món, mua được 4, còn Dầu ăn hết hàng.
- "Mua đồ cuối tuần" chuyển "Đã mua xong", chỉ còn 4 món đã mua.
- Kế hoạch mới được tạo, có 1 món Dầu ăn.

**Bổ sung** ✅ (duyệt 01/11 05:03):
- **Tên kế hoạch mới:** "[tên cũ] · còn lại", ví dụ "Mua đồ cuối tuần · còn lại". Đổi tên được.
- **Kế hoạch mới giữ** biểu tượng và nơi mua của kế hoạch cũ. **Chưa có ngày đi mua**, mẹ tự đặt.
- **Mua hộ:** kế hoạch mới nằm trong danh sách của **người nhờ** (mẹ), không tự gửi lại cho Minh. Mẹ muốn nhờ tiếp thì bấm gửi như một lần nhờ mua mới.
- **Lúc nào thì tách:** khi kế hoạch được chốt xong.
  - Có người bấm "Mua hàng xong", hoặc mẹ xác nhận hóa đơn trừ vào kế hoạch đó.
  - Theo luật trừ dần ở việc 1, kế hoạch đứng sau chỉ **nhận phần dư** của bill thì chưa tính là xong, nên không tách.
- **Thông báo:** "Đã chuyển 1 món chưa mua sang kế hoạch mới ›". Bấm vào thì mở kế hoạch mới.
- **Mua đủ hết món:** không tạo kế hoạch mới.
- **Kế hoạch "Đã mua xong":** thẻ xuống cuối danh sách, mờ đi. Mẹ xóa khi không cần.

**Câu 4 · Xóa kế hoạch đã gửi cho người nhà** ✅ Đã chốt (01/11 04:17)

**Nguyên tắc:** app không biết Minh có đang ở siêu thị hay không. Chỉ khi Minh bấm **"Mua hàng xong"** và gửi cho mẹ thì mới tính là Minh đã đi mua. Xóa đi theo đúng luật gửi tuần tự ở việc 3 và việc 4, không cần hỏi lại kiểu "Minh đang mua".

- **Minh chưa bấm "Mua hàng xong":**
  - Mẹ xóa (bấm 2 lần như nút Xóa hiện có) thì lần nhờ mua bị hủy.
  - Minh nhận thông báo có giờ, ví dụ "Mẹ đã hủy kế hoạch lúc 10:40".
  - Thẻ kế hoạch biến mất khỏi tab Được nhờ mua. Các dấu tick, chỗ sửa Minh đang lưu tạm cũng bỏ theo.
- **Minh đã bấm "Mua hàng xong":** lần nhờ mua đã hoàn thành. Món đã mua đã vào Đồ dùng, Sổ chi tiêu của mẹ. Mẹ xóa kế hoạch thì chỉ xóa kế hoạch, **không xóa** những gì đã ghi.
- **Trường hợp mất mạng** ✅ (duyệt 01/11 05:03): Minh bấm "Mua hàng xong" lúc offline, nhưng mẹ đã hủy trước đó. Máy chủ xét theo thứ tự đến trước.
  - Lần nhờ mua đã hủy nên kết quả của Minh không gửi cho mẹ được.
  - Các món Minh đã tick được ghi vào Đồ dùng, Sổ chi tiêu của Minh, kèm thông báo "Kế hoạch đã bị mẹ hủy. Các món đã mua được ghi vào tài khoản của bạn".

**Câu 5 · Ai được làm gì trong nhà** ✅ Đã chốt (01/11 04:18)

| Việc | Ai được làm |
|---|---|
| Thêm (mời), xóa thành viên | **Chủ nhà** |
| Sửa, xóa kế hoạch mua sắm, gửi kế hoạch nhờ người khác mua | **Người lập kế hoạch đó.** Chủ nhà cũng không sửa, xóa kế hoạch của người khác |
| Nhận yêu cầu mua: đi mua, hoặc **từ chối** | **Người được nhờ** |
| Quét hóa đơn, sửa Đồ dùng và Sổ chi tiêu của mình | Mọi người |

**Khi người được nhờ từ chối:**
- **Bên người được nhờ (Minh):** yêu cầu tự biến mất khỏi tab Được nhờ mua.
- **Bên người nhờ (mẹ):** kế hoạch ở trạng thái **chờ**, chưa có người nhận mua.

**Bổ sung** ✅ (duyệt 01/11 05:03):
- **Mẹ nhận thông báo** có giờ: "Minh đã từ chối kế hoạch Mua đồ cuối tuần lúc 10:45".
- **Thẻ kế hoạch bên mẹ** có nhãn "Chưa có người mua". Mẹ gửi cho người khác, hoặc chọn "Tôi" để tự đi mua.
- **Nút "Từ chối"** nằm ở màn danh sách mua của Minh, cạnh tên kế hoạch. Bấm 2 lần mới từ chối, giống nút Xóa.
- **Đã bấm "Mua hàng xong" thì không từ chối được nữa.**
- **Từ chối lúc mất mạng:** đi theo hàng đợi gửi cho người khác ở việc 4.

**Câu 6 · Dữ liệu chỉ lưu trên máy** ✅ Đã chốt (01/11 04:20)

- **Biểu tượng kế hoạch:** chuyển lên máy chủ, để người được nhờ thấy cùng biểu tượng và đổi máy không mất.
- **Đơn vị hiển thị (chai, hộp…) và nhóm đang thu gọn:** sở thích riêng, để trên máy.
- **Tên gọi, vai trong nhà mình đặt cho người khác** (Anh, Chị, Bố, Mẹ, "Minh"…): là **sở thích riêng của từng tài khoản**, không đồng bộ cho người khác.
  - Mẹ gọi Minh là "Con", Minh gọi mẹ là "Mẹ". Mỗi người chỉ thấy tên mình đặt.
- **Hệ thống làm việc bằng mã số tài khoản:** mỗi tài khoản có một mã riêng (uid). Máy chủ chỉ dùng mã này để gửi kế hoạch, thông báo, chia hóa đơn. Tên gọi chỉ để hiển thị.
- **Lưu ở đâu** ✅ (chốt 01/11 04:22): **trong tài khoản của người đặt.** Người khác không thấy. Đổi máy chỉ cần đăng nhập đúng tài khoản là có lại.

**Câu 7 · Khoản định kỳ có bị cộng hai lần?** ✅ Đã chốt (01/11 04:21): giữ nguyên cách code đang làm.
- Ví dụ: tiền điện 500.000đ là khoản định kỳ. Mẹ trả ở cửa hàng tiện lợi rồi quét hóa đơn đó.
- Nếu không xử lý thì Sổ chi tiêu cộng 500.000đ hai lần: một lần từ khoản định kỳ, một lần từ hóa đơn.
- Code hiện tại đã xử lý: kỳ đã khớp với hóa đơn thì không cộng nữa. Giữ nguyên cách này.

---

## Phần E · Góp ý sau khi xem prototype Spec 1.5 (01/11 06:18)

Ảnh mẹ gửi: `assets/mau-thiet-ke/gop-y-0111-1-dinh-ky.png`, `-2-chon-nhom.png`, `-3-goi-y-gia.png`. Đã sửa prototype theo góp ý 1, 2, 4 (PR #5, 01/11 07:05; ảnh `assets/mimi15-phan-e/`). Góp ý 3 treo. Ô Nhóm ở màn khoản định kỳ chỉ hiện "Khoản chi định kỳ", không sửa, vì 300 nhóm chuẩn là nhóm sản phẩm.

**Góp ý 1 · Chưa xem được chi tiết khoản chi định kỳ đã chi** ✅ Đã chốt (01/11 06:30)

**Hiện tại:** bảng "Theo nhóm hàng" ở Sổ chi tiêu (Đã chi) chỉ là thanh số liệu. Bấm vào dòng nào cũng không mở gì.

**Luật:**
1. **Bấm dòng "Khoản chi định kỳ"** thì chuyển sang tab **Đồ dùng, phần Khoản chi định kỳ** (3c) đã có. Không mở danh sách riêng ở Sổ chi tiêu.
2. **Lịch sử mua hàng (4c) có thêm các khoản chi định kỳ đã trả**, nằm chung với hóa đơn, xếp theo ngày.
   - Mỗi dòng: biểu tượng lịch, tên khoản (ví dụ "Tiền điện"), nhãn "Định kỳ", ngày trả, số tiền.
   - Bấm vào thì mở khoản đó trong Đồ dùng (3c).
   - Tìm kiếm tìm được theo tên khoản. Số đếm của tháng tính cả khoản định kỳ, ví dụ "Tháng 10/2026 (8 hóa đơn, 3 khoản định kỳ)".
   - Kỳ đã trả qua hóa đơn (câu 7) chỉ hiện một lần, là hóa đơn đó, để không trùng.
   - Tóm tắt Lịch sử mua hàng ở Sổ chi tiêu cũng tính cả khoản định kỳ.

**Bổ sung** ✅ (duyệt 01/11 06:39): các dòng nhóm hàng khác (Sữa, Thịt cá trứng…) giữ như hiện tại, chỉ để xem số.

**Góp ý 2 · Ô "Nhóm sản phẩm"** ✅ Đã chốt (01/11 06:33)

**Nhóm dùng để làm gì:** mỗi món được gắn vào một trong 300 nhóm chuẩn (file Taxonomy mẹ làm, ví dụ "Sữa tươi", "Nước rửa chén"). Nhờ nhóm, Mimi cộng món vào đúng món trong **Đồ dùng** (tồn kho, dự đoán ngày hết), so **giá** với lần mua trước, xếp vào **danh mục chi tiêu** và trừ vào **kế hoạch**. Món chưa có nhóm vẫn ghi tiền (mục "Chưa phân loại") nhưng không vào Đồ dùng, không có dự đoán.

**Hiện tại:** ở chi tiết món (Đồ dùng → Hàng tiêu dùng → bấm món, 3b), ô "Nhóm sản phẩm" trông như danh sách thả xuống. Bấm vào mới mở ô tìm trống, phải gõ mới ra gợi ý, có thêm nút Hủy.

**Luật:**
- Ô **Nhóm sản phẩm là ô nhập chữ**, không phải danh sách thả xuống.
- Ô hiện sẵn tên nhóm đang gắn. Mẹ gõ tên (ví dụ "sữa") thì ngay dưới ô hiện **4–5 nhóm gợi ý** lấy trong 300 nhóm chuẩn, tìm cả khi gõ không dấu.
- Bấm một gợi ý là chọn, ô hiện tên nhóm mới. Không chọn gợi ý nào thì giữ nhóm cũ, không tạo nhóm mới ngoài 300 nhóm.

**Bổ sung** ✅ (chốt 01/11 06:41):
- Màn **xác nhận hóa đơn** dùng chung ô này với Đồ dùng: mỗi món có ô ghi tên nhóm, gõ thì gợi ý 4–5 nhóm. Bỏ nút "Đổi nhóm" và ô tìm cũ.
- Không thêm dòng giải thích dưới ô.

**Góp ý 3 · Chưa có phần Mimi gợi ý giá tốt trong Kế hoạch mua sắm** ⏸ Treo, làm sau (chốt 01/11 06:39)

**Hiện tại:** gợi ý giá chỉ nằm trong chi tiết kế hoạch, ở ô "Dự kiến chi" (ví dụ "Bách Hóa Xanh · Đủ 3/3 món" và nút "Xem so sánh giá"). Danh sách kế hoạch (ảnh mẹ gửi) không có.

**Mimi đề xuất:**
- Mỗi thẻ kế hoạch thêm **một dòng gợi ý của Mimi**, có biểu tượng Mimi, ví dụ:
  - "Mimi gợi ý: mua ở Bách Hóa Xanh, rẻ hơn khoảng 8.000đ."
  - Khi chia 2 nơi rẻ hơn: "Mimi gợi ý: chia 2 nơi, tiết kiệm khoảng 15.000đ."
  - Nơi mua mẹ đã chọn đã là rẻ nhất: "Nơi mua đã là giá tốt nhất."
- Bấm dòng gợi ý thì mở chi tiết kế hoạch, mở sẵn phần **So sánh giá**.
- Trong chi tiết, đưa ô "Dự kiến chi" lên ngay dưới danh sách món, đổi tiêu đề thành **"Mimi gợi ý giá tốt"** cho dễ thấy.
- Chưa đủ lịch sử giá thì không hiện dòng gợi ý (không đoán bừa).

**Góp ý 4 · Khoản chi định kỳ có màn chi tiết riêng** ✅ Đã chốt (01/11 06:34)

**Hiện tại:** ở Đồ dùng → Khoản chi định kỳ (3c), bấm "Sửa" thì ô sửa mở ngay trong danh sách.

**Luật:** bấm vào một khoản định kỳ thì **chuyển sang màn chi tiết riêng**, giống màn chi tiết món ở Đồ dùng (3b).

**Bố cục** ✅ (duyệt 01/11 06:39), theo kiểu màn 3b:
1. Nút ‹ và tên khoản, kèm biểu tượng nhóm.
2. Ô **Tên khoản**, ô **Số tiền**, ô **Chu kỳ** (hằng tháng, hằng quý…), ô **Ngày đến hạn tiếp**, ô **Nhóm** (dùng ô gõ có gợi ý như góp ý 2).
3. Thẻ **kỳ này**: đến hạn ngày nào, trạng thái (Chờ trả / Đã trả / Trả qua hóa đơn / Bỏ qua), nút **Đánh dấu đã trả**.
4. **Lịch sử các kỳ**: mỗi dòng gồm ngày, số tiền, cách trả. Kỳ trả qua hóa đơn bấm được, mở Chi tiết hóa đơn (4d).
5. Cuối màn: nút **Tạm dừng / Tiếp tục**, nút **Xóa** (bấm 2 lần như các nút Xóa khác), nút **Lưu**.

Màn này cũng là nơi mở tới khi bấm một khoản định kỳ ở Lịch sử mua hàng (góp ý 1). Thêm khoản mới vẫn dùng nút "+ Thêm khoản định kỳ", mở màn này ở dạng trống.
