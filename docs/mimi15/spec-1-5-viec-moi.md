# Spec 1.5 · Phần việc mới (4 việc P1 và luật 7 câu)

Ngày: 01/11/2026 05:10. Viết từ `buoc-6-de-xuat.md` (mẹ chốt hết lúc 05:03). File này mô tả **app sẽ chạy thế nào sau khi làm xong 4 việc**. Phần app đang chạy vẫn ở `mimi-1-5-tong-the.md`. Dữ liệu máy chủ ở `data-model-1-5.md` (dòng 1–13).

Cách đọc: mỗi phần gồm **Màn hình**, **Luật**, **Dữ liệu** và **Kiểm tra** (các câu để test, mỗi câu phải đúng thì mới tính là xong).

Trong ví dụ: **mẹ** là người lập kế hoạch và nhờ mua, **Minh** là người được nhờ.

---

## 0. Trạng thái của một kế hoạch

| Trạng thái | Khi nào | Hiện ở thẻ kế hoạch |
|---|---|---|
| Chưa hoàn thành | Đang sửa mà rời đi chưa bấm Lưu (máy bay giấy). Giống thư nháp | Nhãn "Chưa hoàn thành" |
| Đang chờ gửi | Đã bấm Lưu lúc mất mạng, nằm trong hộp thư đi | "🕓 Đang chờ gửi" |
| Đã gửi / Đang đi mua | Máy chủ đã nhận, người mua đã có kế hoạch | "Người mua: Minh" |
| Chưa có người mua | Người được nhờ đã từ chối | Nhãn "Chưa có người mua" |
| Đã mua xong | Người mua bấm "Mua hàng xong", hoặc hóa đơn trừ đủ món | Thẻ xuống cuối danh sách, mờ đi |
| Đã hủy | Mẹ xóa khi Minh chưa mua xong | Biến mất ở cả hai bên |

Mỗi lần gửi tạo ra **một bản đã gửi có giờ**. Người mua luôn làm theo bản đã gửi gần nhất mà họ nhận được.

---

## 1. Lập và gửi kế hoạch (màn 5b, 5c)

**Màn hình**
- Trong chi tiết kế hoạch có mục **Người mua**. Bấm vào thì mở bảng "Chọn thành viên gia đình" gồm **Tôi** và các thành viên. Nút cuối bảng là **"Chọn"**. Bảng này **chỉ chọn, không gửi**.
- Nút **Lưu kế hoạch** có **biểu tượng máy bay giấy**, giống nút gửi tin nhắn.
  - Chưa chọn người mua: nút mờ, không bấm được.
  - Đã chọn: bấm là lưu và gửi. Lần đầu là gửi, các lần sau là cập nhật.
- Chọn **Tôi** thì kế hoạch là của mẹ tự đi mua, không gửi cho ai.

**Luật**
- Rời kế hoạch mà chưa bấm Lưu: mọi chỗ sửa vẫn giữ trên máy, kế hoạch ở trạng thái **Chưa hoàn thành**. Vào lại thì sửa tiếp.
- Kế hoạch đã gửi mà mẹ sửa tiếp nhưng chưa bấm Lưu: Minh vẫn giữ bản đã gửi. Phần sửa mới chỉ nằm trên máy mẹ.
- **Gửi tuần tự, không sửa cùng lúc.** Bản đã gửi là cố định.
  - Minh **chưa** bấm Mua hàng xong: mẹ sửa rồi bấm máy bay giấy. Minh thấy bản mới kèm thông báo "Mẹ đã cập nhật kế hoạch lúc 10:32". Món còn trong bản mới giữ dấu tick và chỗ sửa của Minh, món mẹ bỏ thì mất.
  - Minh **đã** bấm Mua hàng xong: lần nhờ mua đã hoàn thành. Mẹ muốn nhờ tiếp thì tạo lần nhờ mua mới.
  - Bản cập nhật Minh **chưa nhận được** trước khi bấm Mua hàng xong (do một trong hai người mất mạng): tách thành **kế hoạch mới** bên mẹ. Mẹ thấy "Minh đã mua xong theo bản trước. Bản cập nhật lúc 10:32 được lưu thành kế hoạch mới".
- Lập kế hoạch lúc mất mạng: lưu trên máy như thư nháp. Món chọn từ Đồ dùng có sẵn trên máy, Dự kiến chi tính theo giá đã lưu lần trước.

**Kiểm tra**
- [ ] Chọn Minh trong bảng rồi bấm "Chọn": Minh **chưa** nhận gì.
- [ ] Chưa chọn người mua: nút máy bay giấy mờ.
- [ ] Bấm máy bay giấy: Minh nhận kế hoạch, có thông báo.
- [ ] Sửa rồi thoát app chưa Lưu, mở lại: chỗ sửa còn, nhãn "Chưa hoàn thành", Minh vẫn thấy bản cũ.
- [ ] Mẹ cập nhật khi Minh đã tick 2 món: 2 dấu tick còn (nếu món còn trong bản mới).
- [ ] Mẹ cập nhật lúc Minh mất mạng, Minh bấm Mua hàng xong theo bản cũ: bên mẹ có kế hoạch mới từ bản cập nhật.

---

## 2. Người được nhờ đi mua (việc 3)

Ảnh bố cục: `assets/mau-thiet-ke/viec-3-di-mua-ho-chot.png`.

**Màn hình**
1. **Tab Được nhờ mua:** số đỏ báo việc mới. Dòng "Mẹ nhờ Minh mua". Thẻ gồm tên, số món, nơi mua, giờ, "Người gửi: Mẹ", dấu ›.
2. **Danh sách mua:**
   - Đầu màn: tên kế hoạch và nút **Từ chối** (bấm 2 lần mới từ chối).
   - Thẻ tóm tắt: số món, nơi mua, giờ.
   - Hiện đủ mọi món, không thu gọn. Mỗi dòng: ô tick, tên, số lượng −/+, **thành tiền** (dòng nhỏ "2 × 14.000đ"), nút ✎.
   - Sau khi quét bill: chia 2 khối, mỗi khối có tổng riêng.
     - **"Mua hộ [tên gọi người nhờ]"** (ví dụ "Mua hộ mẹ", "Mua hộ chị Lan"): món trong kế hoạch.
     - **"Của tôi"**: món khác trên hóa đơn, cũng tick và sửa được.
   - Làm thủ công: không chia khối.
   - Ô ghi chú. Hai nút **Quét hóa đơn** và **✓ Mua hàng xong**.
3. **Quét hóa đơn:** chụp xong tự về màn 2, tự tick món khớp, hiện khối "Của tôi". Không có màn xác nhận riêng.
4. **Đã ghi nhận:** nhãn "Có hóa đơn (đã quét)" hoặc "Thủ công", "Đã gắn vào kế hoạch [tên]", tổng tiền (có bill thì thêm 2 dòng phần mỗi bên), "Đã gửi kết quả cho mẹ", nút **Về kế hoạch**.
5. **Mẹ xem trên màn 5b:** thanh "Đã mua 5/8 món", nhãn "Đã mua" ở từng món, thẻ "Ghi chú của Minh ›". **Không có thẻ hóa đơn của Minh.**

**Sửa tay món** (ở mọi kế hoạch, trước khi bấm Mua hàng xong, kể cả sau khi quét)
- ✎ mở ô sửa: tên, số lượng −/+, **đơn giá**. ✓ lưu, 🗑 bỏ món.
- Nhập **đơn giá**, hiển thị **thành tiền**. Hàng cân ký nhập đơn giá theo kg.
- Món **đã chuẩn hóa** không sửa được tên. Món thêm tay sửa được tên.

**Luật ghi** (chỉ khi bấm Mua hàng xong, ghi một lần)
- Trước đó: tick, sửa, các dòng đọc từ bill chỉ lưu tạm (trên máy, có mạng thì đồng bộ). Ảnh bill không lưu.
- Có bill:
  - Món tick ở khối "Mua hộ": vào Đồ dùng và Sổ chi tiêu của **người nhờ**.
  - Món tick ở khối "Của tôi": vào của **người mua**.
  - Món không tick: không ghi vào đâu.
  - Giảm giá chung: chia theo tỉ lệ tiền mỗi bên.
- Thủ công: món tick vào của **người nhờ**. Giá chưa nhập thì lấy giá mua gần nhất (nhãn "Ước tính"). Chưa từng có giá thì ghi 0đ và nhắc nhập.
- Đồ Minh tự mua cho mình khi làm thủ công: Minh tự thêm ở tab Đồ dùng của mình.
- Món không tick khi xong: chuyển sang kế hoạch mới (mục 5).

**Riêng tư**
- Minh: màn 4d thấy đủ hóa đơn, mỗi món có tag "Mua hộ mẹ" / "Của tôi", và 2 dòng phần mỗi bên.
- Mẹ: không thấy hóa đơn của Minh, không thấy món của Minh. Lịch sử mua hàng có khoản **"Minh mua hộ · [kế hoạch]"**, bấm vào thấy ngày, nơi mua, các món của mẹ và tổng.

**Kiểm tra**
- [ ] Hóa đơn 135.000đ (sữa, trứng của mẹ 120.000đ; nước của Minh 15.000đ): mẹ +120.000đ nhãn "Minh mua hộ", Minh +15.000đ.
- [ ] Bấm Mua hàng xong 2 lần do mạng chập chờn: chỉ ghi một lần.
- [ ] Sửa tên món đã chuẩn hóa: không cho sửa.
- [ ] Đổi số lượng từ 2 lên 3: thành tiền tự tính lại theo đơn giá.
- [ ] Mẹ không thấy món "Của tôi" của Minh ở bất kỳ màn nào.

---

## 3. Hóa đơn trừ vào kế hoạch (việc 1)

**Luật**
- Món trong hóa đơn **trừ dần vào kế hoạch có ngày đi mua sớm nhất trước**, dư thì trừ tiếp kế hoạch sau.
- Kế hoạch chưa có ngày: xếp sau kế hoạch có ngày, tạo trước thì trừ trước.
- Kế hoạch Đã mua xong: không trừ nữa.
- Món không thuộc kế hoạch nào: vẫn ghi Đồ dùng và Sổ chi tiêu như thường.
- Người được nhờ quét: trừ vào kế hoạch được gửi cho họ (mục 2).

**Màn xác nhận hóa đơn** (khi tự quét cho mình)
- Mỗi món ghi "→ [tên kế hoạch]". Bấm vào để đổi kế hoạch hoặc chọn "Không thuộc kế hoạch nào".
- Tên, số lượng, giá sửa được như hiện có.

**Kiểm tra**
- [ ] "Mua đồ cuối tuần" (02/11) cần 2 sữa, 10 trứng; "Đồ cho bé" (05/11) cần 3 sữa. Quét bill 4 sữa, 10 trứng: kế hoạch 02/11 Đã mua xong; kế hoạch 05/11 nhận 2, còn thiếu 1, chưa xong.
- [ ] Đổi "→" của một món sang kế hoạch khác: món trừ đúng vào kế hoạch đã chọn.

---

## 4. Lịch sử mua hàng (việc 2)

Ảnh bố cục: `assets/mau-thiet-ke/4c-4d-lich-su-chi-tiet-hoa-don.png`.

**Màn hình**
- **4a Sổ chi tiêu:** giữ tóm tắt Lịch sử mua hàng. Bấm tiêu đề mở 4c, bấm hóa đơn mở 4d.
- **4c Lịch sử mua hàng:** ô tìm "Tìm nơi mua, món hàng…" (tìm cả cửa hàng và món), lọc theo cửa hàng và nhóm hàng, chọn tháng (mặc định "Tất cả tháng"). Mỗi tháng một khối "Tháng 10/2026 (8 hóa đơn)", 3 hóa đơn mới nhất, dòng "Xem thêm N hóa đơn… ›".
- **4d Chi tiết hóa đơn:** chỉ xem, không có ảnh bill. Thẻ cửa hàng, thẻ "Đã trừ vào kế hoạch: [tên] ›" (ẩn nếu không có), danh sách món "số lượng × đơn giá" và thành tiền, giảm giá dòng xanh lá, khối Tạm tính, Giảm giá, **Tổng tiền**. Ghi chú cuối: "Thông tin cá nhân (tên khách, số điện thoại, địa chỉ, số thẻ, mã thành viên…) không được lưu."

**Luật**
- Máy chủ giữ **mọi lần mua** (lot riêng). Giá thấp nhất, giá trung bình, tồn kho tính trên toàn bộ.
- Thông tin cá nhân trên bill bị lọc ngay khi đọc. Ảnh bill không giữ.

**Kiểm tra**
- [ ] Món đã mua 25 lần: lịch sử và giá trung bình tính đủ 25 lần.
- [ ] Tìm "sữa": ra các hóa đơn có món sữa.
- [ ] Bill có số điện thoại khách: không lưu ở đâu cả.

---

## 5. Món chưa mua khi kế hoạch xong (câu 3)

- Món chưa mua **tự chuyển sang kế hoạch mới**, cả khi quét bill (món không có trên bill) lẫn thủ công (món không tick).
- Tên "[tên cũ] · còn lại", đổi được. Giữ biểu tượng và nơi mua, **chưa có ngày**.
- Mua hộ: kế hoạch mới nằm bên **người nhờ**, không tự gửi lại.
- Tách khi kế hoạch chốt xong (Mua hàng xong, hoặc mẹ xác nhận hóa đơn trừ vào nó). Kế hoạch chỉ nhận phần dư của bill thì chưa tách.
- Thông báo "Đã chuyển 1 món chưa mua sang kế hoạch mới ›". Mua đủ hết thì không tạo.

**Kiểm tra**
- [ ] Kế hoạch 5 món, mua 4: kế hoạch cũ Đã mua xong với 4 món, có kế hoạch "· còn lại" 1 món.

---

## 6. Xóa kế hoạch đã gửi (câu 4)

- Minh chưa Mua hàng xong: mẹ xóa (bấm 2 lần) là hủy. Minh nhận "Mẹ đã hủy kế hoạch lúc 10:40", thẻ biến mất, phần lưu tạm của Minh bỏ theo.
- Minh đã Mua hàng xong: xóa chỉ xóa kế hoạch, không xóa những gì đã ghi.
- Mất mạng: máy chủ xét thứ tự đến trước. Mẹ hủy trước thì kết quả của Minh ghi vào tài khoản Minh, kèm "Kế hoạch đã bị mẹ hủy. Các món đã mua được ghi vào tài khoản của bạn".

---

## 7. Ai được làm gì (câu 5)

| Việc | Ai |
|---|---|
| Mời, xóa thành viên | Chủ nhà |
| Sửa, xóa, gửi kế hoạch | Người lập kế hoạch đó (chủ nhà cũng không sửa kế hoạch người khác) |
| Đi mua hoặc từ chối | Người được nhờ |
| Quét hóa đơn, sửa Đồ dùng và Sổ chi tiêu của mình | Mọi người |

**Từ chối:** thẻ biến mất bên Minh. Bên mẹ có nhãn "Chưa có người mua" và thông báo "Minh đã từ chối kế hoạch [tên] lúc 10:45". Đã Mua hàng xong thì không từ chối được. Từ chối lúc mất mạng đi theo hàng đợi gửi.

---

## 8. Mất mạng và tự đồng bộ (việc 4)

**Luật chung**
- Mọi thao tác quan trọng **lưu vào máy trước**, có mạng thì tự đồng bộ. Không bao giờ xóa dữ liệu trên máy trước khi máy chủ xác nhận.
- Lưu trạng thái cuối, không lưu từng thao tác nhỏ.
- Hai hàng đợi:
  - **Data Outbox:** sửa số lượng, giá, tick, ghi chú, cập nhật kế hoạch, tồn kho, Mua hàng xong.
  - **Message Outbox:** gửi / cập nhật kế hoạch, gửi kết quả mua hộ, từ chối. Hoạt động như email: hộp thư đi → Đã gửi.
- Phạm vi chỉ các thao tác trên. Không làm cả app chạy offline.

**Quét bill lúc mất mạng:** báo ngay "Hiện không có mạng nên Mimi chưa đọc được hóa đơn. Mẹ nhập thủ công nhé." kèm nút **Nhập thủ công**. Không chụp chờ, không lưu ảnh.

**Hết phiên đăng nhập:** làm mới phiên rồi gửi lại. Không làm mới được thì giữ nguyên hàng đợi, báo "Có 2 thay đổi chưa đồng bộ. Đăng nhập lại để tiếp tục gửi."

**Chống gửi trùng:** mỗi bản ghi có ID riêng (`Idempotency-Key`, `shopping_result_id`). Máy chủ gặp lại ID đã xử lý thì bỏ qua.

**Trên màn hình** (dòng nhỏ, không popup): "✓ Đã lưu trên máy", "🕓 Chờ đồng bộ", "🕓 Đang chờ gửi", "✓ Đã đồng bộ", "✓ Đã gửi cho Minh", "✓ Đã gửi kết quả cho mẹ", "⚠ Chưa đồng bộ".

**Chờ lâu**
- Sau **3 tiếng** chưa gửi được thì nhắc một lần cho mỗi việc, ví dụ "Chưa gửi được kết quả mua hàng cho mẹ".
- Tab Cài đặt có chấm đỏ khi còn việc chờ. Mục **"Đang chờ đồng bộ (N)"** liệt kê từng việc kèm giờ và nút **Thử lại**.
- App xin quyền lưu bền để điện thoại không tự xóa dữ liệu chờ. Thông báo khi app đóng chỉ có nếu đã thêm Mimi ra màn hình chính và cho phép thông báo.

**Kiểm tra**
- [ ] Tắt mạng, tick 3 món, bấm Mua hàng xong, tắt app. Bật mạng, mở app: kết quả tự gửi, mẹ nhận đủ.
- [ ] Hết phiên giữa chừng: không mất thao tác nào (lỗi N3).
- [ ] Mạng đứt ngay sau khi máy chủ nhận: gửi lại không cộng tiền hai lần.
- [ ] Bấm Quét hóa đơn lúc mất mạng: hiện thông báo và nút Nhập thủ công.
- [ ] Để mất mạng 3 tiếng: có đúng một lời nhắc.

---

## 9. Dữ liệu chỉ lưu trên máy (câu 6, câu 7)

- **Biểu tượng kế hoạch** lên máy chủ.
- **Đơn vị hiển thị, nhóm đang thu gọn:** để trên máy.
- **Tên gọi người khác** (Con, Mẹ, chị Lan…): riêng từng tài khoản, lưu trong tài khoản người đặt. Hệ thống chỉ dùng mã tài khoản (uid).
- **Khoản định kỳ trả qua hóa đơn:** giữ cách hiện tại, kỳ đã khớp hóa đơn thì không cộng lần nữa.

---

## 10. Thứ tự làm đề xuất

1. Việc 4 (nền: hàng đợi, sửa N3, chống gửi trùng, bản đã gửi) vì các việc khác dựa vào nó.
2. Mục 1 (chọn người mua, nút máy bay giấy, trạng thái kế hoạch).
3. Việc 3 (màn đi mua hộ, chia hóa đơn) cùng câu 3, 4, 5.
4. Việc 1 (trừ dần theo ngày).
5. Việc 2 (lot, màn 4c, 4d).

Mỗi việc đi qua: sửa prototype → mẹ xem → code → ảnh chạy thử → PR.
