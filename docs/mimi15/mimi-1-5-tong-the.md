# Mimi 1.5 · Bản mô tả tổng thể (đã chốt và đã code)

Cập nhật: 31/10/2026 16:15 · Code: PR #2 đã gộp vào `main` (màn 0–6); PR #3 (prototype trên điện thoại và các góp ý sau 15:30) đang chờ gộp.
Prototype bấm thử: https://claude.ai/artifact/TNXorGatmm8XEv8LbEhvGu

File này gộp các file Cổng 1 (`man-hinh-0-4-cong-1.md`, `man-hinh-5-cong-1.md`, `man-hinh-6-cong-1.md`, `gio-mua-sam-cong-1.md`) cùng mọi góp ý sau đó. Nó mô tả app **đúng như đang chạy**. Ba file Cổng 1 giữ lại làm lịch sử: đó là bản kể bằng lời trước khi code, kèm câu hỏi và câu trả lời của bạn.

Ảnh chạy thử:
- Màn 0–4: `assets/mimi15-man-0-4/00-tong-hop-lan-2.png`
- Màn 5: `assets/mimi15-man-5/00-tong-hop.png`
- Màn 6: `assets/mimi15-man-6/00-tong-hop.png`

## 1. Luật chung

- **Cách xưng hô:** Mimi xưng "Mimi", gọi người dùng là "mẹ".
- **Ngày giờ:** app dùng ngày giờ thật.
- **Ngôn ngữ:**
  - Mặc định là tiếng Việt, đổi sang English trong Cài đặt.
  - Hóa đơn được đọc và dịch theo đúng ngôn ngữ của app, nên không còn ô chọn ngôn ngữ hóa đơn riêng.
- **Không có biểu tượng tài khoản (hình người)** ở đầu màn. Đăng xuất nằm trong Cài đặt.
- **Đầu màn:**
  - Màn chính: logo Mimi bên trái (bấm vào thì về Home) và chuông bên phải.
  - Màn con: nút ‹ quay lại, tên màn ở giữa và chuông.
  - Tên màn hiện ở **Kế hoạch mua sắm** và **Cài đặt**. Đồ dùng và Sổ chi tiêu đang không hiện tên (câu hỏi 3 vẫn còn treo).
- **Thanh tab dưới** (Sổ chi tiêu · Đồ dùng · Quét · Kế hoạch · Cài đặt): có ở mọi màn trừ Home.
- **Tính năng đang treo** (giữ code nhưng khóa trên giao diện):
  - Mua online, Mimi Online, Ủy quyền mua online, Đơn mua online: làm mờ và hiện "Sắp ra mắt".
  - "Hôm nay ăn gì": hiện "Sắp ra mắt".
- **Luật chia nhóm:** dùng chung cho Đồ dùng, Sắp chi và Gợi ý mua.
  - **Hàng tiêu dùng:**
    - **Cần mua ngay:** tồn kho bằng 0, hoặc dự đoán hết trong 3 ngày.
    - **Sắp hết:** dự đoán hết trong 7 ngày.
    - **Còn nhiều:** trên 7 ngày.
  - **Khoản chi định kỳ:**
    - **Đến hạn:** hạn trong 3 ngày, tính cả khoản quá hạn.
    - **Sắp tới:** hạn trong 7 ngày.
    - **Còn thời gian:** hạn sau 7 ngày.
- **Nút giỏ hàng và nút Mua sắm** (chốt 31/10 15:58, thay luật "kế hoạch đang mở gần nhất"): bấm thì hiện bảng **"Thêm vào kế hoạch nào?"** gồm các kế hoạch đang có (ghi "Đã có" nếu món đã nằm trong đó) và **Tạo kế hoạch mới**. Nút giỏ hàng tô đậm khi món đã nằm trong bất kỳ kế hoạch nào. Trong màn chi tiết một kế hoạch, "+ Thêm" và "Thêm sản phẩm" vẫn thêm thẳng vào kế hoạch đó.
- **Giỏ mua sắm** (trước gọi là "Kế hoạch chính", đổi tên 31/10 15:50): kế hoạch mặc định của máy chủ. Ẩn khi trống. Không đổi tên được; bấm Xóa thì làm trống và nó tự ẩn.

## 2. Màn 0 · Vào app

- **Chưa đăng nhập:** màn đăng nhập như bản 1.0.
- **Đã đăng nhập:** mở thẳng Home.

## 3. Màn 1 · Home

- **Đầu màn:** "Mimi AI ✦" và robot ở giữa, chuông bên phải.
- **Bong bóng** "Hôm nay ăn gì? · Sắp ra mắt".
- **Lời chào** "Chào [tên]!".
- **4 ô màu**, mỗi ô có hình minh họa:
  - **Sổ chi tiêu:** tiền chi tháng này.
  - **Đồ dùng:** số món sắp hết.
  - **Kế hoạch:** số món cần mua.
  - **Cài đặt.**
- **Nút tròn "QUÉT HÓA ĐƠN"**, chữ uốn cong phía trên.
- **Không có thanh tab dưới.**

## 4. Màn 2 · Quét

- Giữ như bản 1.0.
- Thêm dòng nhắc "⚠️ Chú ý: không chụp thông tin cá nhân (số thẻ, số điện thoại, địa chỉ…)".

## 5. Màn 3 · Đồ dùng

Đầu màn có 2 nút chuyển (**Hàng tiêu dùng** · **Khoản chi định kỳ**), rồi ô tìm kiếm.

**3a · Hàng tiêu dùng**
- Món chia 3 nhóm: Cần mua ngay (đỏ), Sắp hết (cam), Còn nhiều (xanh lá).
- **Thu gọn nhóm:** mỗi nhóm có ký hiệu thu gọn / mở rộng và số món. Máy nhớ nhóm nào đang thu gọn.
- **Thẻ món:** tên, số lượng, "Dự kiến còn khoảng N ngày", "✎ Chỉnh sửa" và nút giỏ hàng.

**3b · Chi tiết sản phẩm**
- **Sửa thông tin món:** sửa **Tên sản phẩm** và **Nhóm sản phẩm**. Đã bỏ mục "Sửa tên hoặc nhóm" cũ.
- **Số lượng hiện có:** nút −/+ và ô chọn đơn vị. Máy nhớ đơn vị đã chọn.
- **Hai nút chính:**
  - **Cập nhật sản phẩm:** lưu tên và số lượng, hiện "Đã cập nhật.".
  - **Mua sắm:** thêm món vào kế hoạch.
- **Lịch sử mua hàng:** giá gần nhất, giá thấp nhất, giá trung bình, rồi từng lần mua.

**3c · Khoản chi định kỳ**
- Khoản chi chia 3 nhóm: Đến hạn, Sắp tới, Còn thời gian.
- **Thẻ khoản chi:** "số tiền · hạn" (đã bỏ chữ "Hằng tháng"), "✎ Chỉnh sửa" và nút giỏ hàng.
- **Sửa ngay trên thẻ:** lưu, hủy, "Đã trả kỳ này" và Xóa.
- **Nút "+ Thêm khoản chi"** ở cuối danh sách.

## 6. Màn 4 · Sổ chi tiêu

Hai nút chuyển: **Đã chi** · **Sắp chi**.

**4a · Đã chi**
- Chọn tháng.
- **Tổng tiền:** so với tháng trước (▲ hoặc ▼ kèm %).
- **Theo nhóm hàng:**
  - **Khoản chi định kỳ luôn đứng đầu**, các nhóm khác xếp theo số tiền.
  - Hiện 5 nhóm đầu, có nút "Xem tất cả".
- **Tiết kiệm nhiều nhất.**
- **Địa điểm mua hàng:** gộp theo cửa hàng.
- **Lịch sử mua hàng:** mở ra / thu lại được. Lưu **mọi hóa đơn** của tháng, mỗi dòng gồm địa điểm mua hàng, ngày và số tiền (góp ý 31/10 16:32; trước chỉ có 3 hóa đơn gần nhất).

**4b · Sắp chi**
- **Dự kiến chi** gồm 2 dòng, bấm mỗi dòng thì mở danh sách chi tiết:
  - **Khoản chi định kỳ:** Đến hạn và Sắp tới.
  - **Hàng tiêu dùng:** Cần mua ngay và Sắp hết, tính theo giá mua gần nhất.

## 7. Màn 5 · Kế hoạch mua sắm

**5a · Danh sách** (bấm tab Kế hoạch)
- **Tên màn:** "Kế hoạch mua sắm".
- **Hai nút chuyển:** **Kế hoạch của tôi** · **Được nhờ mua**.
- **Thẻ kế hoạch gồm:**
  - Biểu tượng và tên kế hoạch.
  - Số sản phẩm và **Ngân sách**, tính theo giá mua gần nhất (bản 2.0 sẽ dùng giá online).
  - Nơi mua, ngày và giờ đi mua.
  - Nhãn trạng thái, và "Người mua: X" nếu đã gửi cho người nhà.
- **Nút "+ Kế hoạch mới"** và ghi chú "Mỗi kế hoạch có danh sách mua riêng."
- **Được nhờ mua:** tạm hiện thẻ. Màn đi mua chi tiết làm sau.

**5b · Chi tiết kế hoạch**
- **Đầu màn:** logo và chuông, **không** hiện tên kế hoạch.
- **Hàng tên:** nút ‹, ô biểu tượng và ô sửa tên nằm ngang hàng. Giỏ mua sắm không đổi tên được.
- **Ngày, giờ, nơi mua:** sửa được.
- **Danh sách dự kiến mua:** bấm ✎ ở một món thì hiện −/+, nút **dấu tích** (lưu số lượng) và **thùng rác** (bỏ món). Số lượng chỉ lưu khi bấm dấu tích.
- **Gợi ý mua:** các món Cần mua ngay và Sắp hết chưa có trong danh sách, bấm "+ Thêm" để thêm.
- **Hai nút thêm món:**
  - **Kiểm tra đồ dùng:** bấm thì mở thẳng tab Đồ dùng (góp ý 31/10 16:22).
  - **Thêm sản phẩm:** tìm món theo tên.
- **Dự kiến chi:** tổng tiền, cửa hàng rẻ nhất và "Xem so sánh giá".
- **Người mua:**
  - **Tôi.**
  - **Gửi cho thành viên:** mở bảng 5c.
  - **Mimi:** đang khóa, hiện "Sắp ra mắt".
- **Nút cuối màn:** **Xóa** nằm cạnh **Lưu kế hoạch** (góp ý 31/10 15:43). Bấm Xóa thì nút đổi thành "Xóa hẳn", bấm lần nữa mới xóa. Kế hoạch nào cũng xóa được (góp ý 31/10 16:04); với Giỏ mua sắm, Xóa là bỏ hết món và ngày giờ, nơi mua, giỏ trống thì tự ẩn.

**5b' · Chọn biểu tượng cho kế hoạch**
- 6 biểu tượng: Túi xách, Gia đình, Trẻ em, Quà tặng, Nhà cửa, Ăn uống.
- Hai nút: Hủy · **Lưu biểu tượng**.

**5c · Chọn thành viên gia đình**
- Mỗi người nhà hiện tên và **vai trong nhà**. Chỉ chọn được 1 người.
- Hai nút: Hủy · **Gửi kế hoạch**.
- Gửi xong thì hiện "Đã gửi kế hoạch cho Minh".

**5e · Thêm vào kế hoạch nào?** (bấm giỏ hàng ở Đồ dùng hoặc Mua sắm ở chi tiết sản phẩm, chốt 31/10 15:58)
- Liệt kê các kế hoạch đang có: biểu tượng, tên, số món, ngày đi mua. Kế hoạch đã có món đó ghi "Đã có".
- Dòng cuối **Tạo kế hoạch mới**: tạo "Kế hoạch dd/mm hh:mm" rồi cho món vào.
- Chọn xong hiện "Đã thêm [món] vào [kế hoạch]" kèm Hoàn tác.

## 8. Màn 6 · Cài đặt

Bạn đã đồng ý cả 5 đề xuất lúc 31/10 14:46.

**6a · Cài đặt**
- **Tên màn:** "Cài đặt".
- **Thẻ hồ sơ:** "Xem hồ sơ cá nhân ›".
- **Cài đặt chung:**
  - **Thông báo & nhắc mua ›**, dòng phụ "08:00 · Trước 7 ngày".
  - **Ngôn ngữ & khu vực ›**, dòng phụ "Tiếng Việt · VND · múi giờ".
  - Ghi chú: hóa đơn theo ngôn ngữ đã chọn.
- **Gia đình:**
  - **Thành viên ›**, dòng phụ "N thành viên".
  - **Ủy quyền mua online:** làm mờ, nhãn "Đang tắt".
- **Mua sắm:** **Đơn mua online**, làm mờ, nhãn "Sắp ra mắt".
- **Tài khoản:** **Quyền riêng tư & bảo mật ›** và **Đăng xuất** (chữ đỏ).
- Bấm vào mục đang làm mờ thì hiện "Tính năng này sắp ra mắt."

**6a' · Các trang con**
- **Hồ sơ cá nhân:** sửa tên Mimi gọi. Email chỉ xem, không sửa.
- **Thông báo & nhắc mua:** bật/tắt email nhắc, giờ gửi, báo trước bao nhiêu ngày.
- **Ngôn ngữ & khu vực:**
  - Chọn Tiếng Việt / English: đổi cả giao diện lẫn ngôn ngữ hóa đơn.
  - Tiền tệ cố định là VND.
  - Có ô chọn múi giờ.
- **Quyền riêng tư & bảo mật:** tạm là trang chữ.

**6b · Thành viên**
- **Thẻ nhà:**
  - "Nhà của [chủ nhà]" và số thành viên.
  - **Mã nhà:** bấm "Mời thành viên" thì tạo mã. Mã dùng được 48 giờ và có nút **Sao chép mã** (bấm xong hiện "Đã chép mã").
- **Danh sách người nhà:** tên, vai trong nhà và vai trò (Chủ nhà / Thành viên).
- **Nút "+ Mời thành viên".**
- **Thẻ Quyền riêng tư:** chỉ kế hoạch mua sắm được chia sẻ.
- **Ủy quyền mua online:** làm mờ.
- **Rời khỏi nhà / vào nhà khác:**
  - Nút viền đỏ **Rời khỏi nhà**.
  - Ô vào nhà khác bằng mã, chỉ hiện khi đang ở một mình.

**6c · Một thành viên**
- Tên và vai trò.
- **Vai trong nhà:** Anh, Chị, Em, Bố, Mẹ, Con, Ông, Bà, Khác. Vai này hiện trong bảng Gửi kế hoạch (5c).
- Nút **Xóa khỏi nhà**, chỉ chủ nhà mới thấy.

## 9. Dữ liệu tạm lưu trên máy

Những dữ liệu sau chưa có chỗ lưu ở máy chủ, nên đổi sang máy khác thì phải chọn lại. Khi backend thêm chỗ lưu thì chuyển lên máy chủ.

- Đơn vị hiển thị của từng món.
- Biểu tượng của kế hoạch.
- Vai trong nhà của người nhà.
- Nhóm nào đang thu gọn.

## 10. Còn treo / làm sau

- Câu hỏi 3: có hiện tên màn ở đầu màn Đồ dùng và Sổ chi tiêu không?
- Màn đi mua chi tiết cho "Được nhờ mua".
- Bản 2.0: mua online, Mimi Online, Ngân sách theo giá online, "Hôm nay ăn gì".

## 11. Thay đổi sau khi gộp PR #2 (nằm trong PR #3)

| Giờ (31/10) | Góp ý của bạn | Đã đổi |
|---|---|---|
| 15:43 | Thêm nút Xóa cạnh Lưu kế hoạch | Nút Xóa → "Xóa hẳn" (bấm 2 lần) |
| 15:50 | Đổi tên Kế hoạch chính | Thành **Giỏ mua sắm** |
| 15:58 | Bấm Mua sắm thì cho chọn kế hoạch | Bảng 5e "Thêm vào kế hoạch nào?" + Tạo kế hoạch mới; Giỏ ẩn khi trống |
| 16:04 | Không còn kế hoạch nào không xóa được | Xóa Giỏ mua sắm = làm trống, giỏ tự ẩn; bỏ lời nhắc "không xóa được" |
| 16:22 | "Kiểm tra đồ dùng gia đình" đổi thành "Kiểm tra đồ dùng", bấm thì sang Đồ dùng | Đổi tên nút, bấm mở tab Đồ dùng |
| 16:23 | Thu nhỏ nút + Thêm ở ô Thêm món mới | Nút nhỏ bên phải, tên món hiện đủ |
| 16:28 | Sửa món: chỉ cần thùng rác bên phải, bên trái là nút lưu dấu tích; mọi nút lưu đĩa mềm đổi sang dấu tích | Dòng sửa món: −/+ · ✓ · 🗑. Nút "Cập nhật sản phẩm" dùng dấu tích |
| 16:32 | Lịch sử mua hàng lưu các hóa đơn: địa điểm, ngày, số tiền | API trả mọi hóa đơn của tháng (trước chỉ 3), Địa điểm mua hàng cũng tính đủ |
| 16:34 | Prototype ổn, tạm thời như vậy | Chốt bản prototype hiện tại |
| 01/11 01:50 | Hóa đơn trừ dần vào kế hoạch dự kiến diễn ra trước | Chốt câu 1, câu 2 bước 6 (chưa code) |
| 01/11 01:57 | Lịch sử mua hàng: bấm vào thì ra màn riêng, đủ nội dung hóa đơn, bỏ thông tin cá nhân | Ghi vào bước 6 (màn 4c, 4d), chưa code |
| 01/11 02:11 | Gửi bố cục màn 4c Lịch sử mua hàng và 4d Chi tiết hóa đơn | Ghi bố cục vào bước 6, lưu ảnh mẫu; chưa code |
| 01/11 02:15 | Gửi phương án bố cục việc 3 (đi mua hộ) | Ghi nhận xét và 5 đề xuất chỉnh vào bước 6, chờ chốt |
| 01/11 02:54 | Bố cục việc 3 bản 2: 6 màn, hai cách ghi nhận (thủ công / quét), có ô ghi chú, không có "Không mua được" | Ghi vào bước 6, kèm 5 chỉnh nhỏ và 2 câu hỏi; chưa code |
| 01/11 03:00 | Việc 3: nút ✓ Mua hàng xong cho cả 2 cách; giữ sửa dòng hóa đơn; món trong kế hoạch tính cho người nhờ, món ngoài kế hoạch tính cho người đi mua | Ghi vào bước 6; chưa code |
| 01/11 03:02 | Chi tiết hóa đơn mua hộ: hiện đủ món, có tag phân biệt món mua hộ / món của người đi mua | Ghi vào bước 6; chưa code |
| 01/11 03:02 | Thủ công không bill: tick rồi bấm Mua hàng xong thì cộng vào Đồ dùng và chi tiêu của mẹ | Ghi vào bước 6 (tiền theo giá dự kiến); chưa code |
| 01/11 03:05 | Bấm món trong kế hoạch thì sửa tay được tên, số lượng, giá | Ghi vào bước 6 (màn đi mua và 5b); chưa code |
| 01/11 03:07 | Sửa tay ở mọi kế hoạch trước khi Mua hàng xong và sau khi quét bill; món đã chuẩn hóa không sửa tên | Ghi vào bước 6; chưa code |
| 01/11 03:38 | Demo việc 3 tinh chỉnh: 5 màn, quét xong quay về danh sách, sửa số lượng và giá ngay trên danh sách | Cập nhật việc 3 theo demo, kèm 5 góp ý; chưa code |
| 01/11 03:42 | Món ngoài kế hoạch: thủ công thì tự chỉnh ở Đồ dùng; quét bill thì có khối "Món khác trên hóa đơn", tick để nhập vào tài khoản của người đi mua | Ghi vào việc 3; chưa code |
| 01/11 03:45 | Sau khi quét bill, danh sách chia 2 phần (Mua hộ mẹ / Của tôi); thủ công thì không chia | Ghi vào việc 3; chưa code |
| 01/11 03:46 | Không chia sẻ hóa đơn của người đi mua cho người nhờ (riêng tư) | Bỏ thẻ Hóa đơn của Minh; mẹ chỉ thấy phần món của mình; chưa code |
| 01/11 03:48 | Giá: nhập theo đơn giá, hiển thị thành tiền | Chốt; gộp việc 3 thành bản cuối; chưa code |
| 01/11 03:56 | Việc 4 · Offline Outbox & tự đồng bộ (3 hàng đợi, ảnh bill tạm, chống gửi trùng) | Ghi vào bước 6 kèm hiện trạng code; chưa code |
| 01/11 04:02 | Sửa của người đi mua lưu ngay trên máy, mạng chỉ để đồng bộ; kế hoạch đã gửi là cố định, sửa thì gửi lại (tuần tự, không thời gian thực) | Ghi vào việc 4; chưa code |
| 01/11 04:05 | Chưa Mua hàng xong: mẹ sửa rồi Gửi/Cập nhật, Minh thấy thông báo có giờ. Đã Mua hàng xong: lần nhờ mua hoàn thành, muốn nhờ tiếp phải tạo lần mới | Ghi vào việc 4; chưa code |
| 01/11 04:11 | Tổng hợp việc 4 thành bản cuối | Gộp các quyết định 03:56–04:09; chưa code |
| 01/11 04:13 | Món chưa mua (quét bill hoặc thủ công) tự chuyển sang kế hoạch mua sắm mới | Chốt câu 3; chưa code |
| 01/11 04:17 | Xóa kế hoạch đã gửi: theo luật tuần tự, chỉ "Mua hàng xong" mới tính là đã mua | Chốt câu 4; chưa code |
| 01/11 04:18 | Quyền: chủ nhà thêm/xóa thành viên; kế hoạch của ai người đó xóa, gửi; người được nhờ có quyền từ chối (bên nhờ chuyển trạng thái chờ) | Chốt câu 5; chưa code |
| 01/11 04:20 | Biểu tượng kế hoạch lên máy chủ; tên gọi người nhà là riêng từng tài khoản, hệ thống dùng mã tài khoản | Chốt câu 6, sửa data model; chưa code |
| 01/11 04:21 | Khoản định kỳ: giữ nguyên cách chống cộng hai lần | Chốt câu 7 |
| 01/11 04:22 | Tên gọi người nhà lưu trong tài khoản người đặt, đổi máy đăng nhập là có lại | Chốt; chưa code |
| 01/11 04:27 | Một trong hai người mất mạng: bản cập nhật Minh chưa nhận trước khi Mua hàng xong thì coi như kế hoạch mới | Ghi vào việc 4; chưa code |
| 01/11 04:28 | Mua hàng xong lúc mất mạng: lưu máy, tự gửi khi có mạng, sau vài tiếng chưa gửi thì thông báo | Ghi vào việc 4; chưa code |
| 01/11 04:29 | Gửi kế hoạch như gửi email: hộp thư đi, mục Đang chờ đồng bộ trong Cài đặt | Ghi vào việc 4; chưa code |
| 01/11 04:33 | Mất mạng thì nhập thủ công và chờ gửi; quét bill (cần AI) lúc mất mạng thì báo ngay, đề nghị nhập thủ công. Bỏ hàng đợi ảnh bill, không chia đợt | Chốt việc 4; chưa code |
| 01/11 04:34 | Mẹ lập kế hoạch lúc mất mạng: lưu như thư nháp, gửi qua hộp thư đi (như email) | Ghi vào việc 4; chưa code |
| 01/11 04:36 | Chọn người mua trong kế hoạch không gửi ngay; chỉ gửi khi bấm Lưu kế hoạch, nút Lưu là máy bay giấy | Ghi vào việc 4 mục 3 và dòng 5.9; chưa code |
| 01/11 04:40 | Rà soát bước 6: sửa các chỗ cũ còn sót, bổ sung 10 thay đổi máy chủ vào data model | Chỉ sửa file md |
| 01/11 05:02 | Phải chọn người mua mới bấm được máy bay giấy; rời kế hoạch chưa Lưu thì kế hoạch ở trạng thái "Chưa hoàn thành" | Ghi vào việc 4 mục 3; chưa code |
| 01/11 05:03 | Mẹ duyệt hết các phần Mimi bổ sung (câu 3, 4, 5, việc 4 mục 3). Bước 6 chốt xong | Chỉ sửa file md |
| 01/11 05:15 | Viết Spec 1.5 phần việc mới (`spec-1-5-viec-moi.md`); cập nhật mục lục, bản đồ frontend (mục 2b), data model | Chỉ sửa file md, chưa đụng prototype |
| 01/11 05:30 | Sửa prototype theo Spec 1.5 mục 1–8 (5 commit trên nhánh `feat/mimi15-spec-viec-moi`, PR #4) | Ảnh ở `assets/mimi15-spec-moi/`; đăng lại link prototype 06:14 |
| 01/11 06:17 | Gộp PR #3 và PR #4 vào main; xuất lại bản nén frontend | |
| 01/11 06:18 | Mẹ góp ý 3 điểm: xem chi tiết khoản định kỳ đã chi, ô chọn nhóm để làm gì, gợi ý giá tốt ở danh sách kế hoạch | Ghi đề xuất vào bước 6 Phần E; chờ mẹ chốt |
| 01/11 06:30 | Chốt góp ý 1: bấm Khoản chi định kỳ ở Sổ chi tiêu thì sang Đồ dùng (3c); Lịch sử mua hàng thêm khoản định kỳ đã trả | Ghi vào bước 6 Phần E; chưa sửa prototype |
| 01/11 06:33 | Chốt góp ý 2: ô Nhóm sản phẩm ở chi tiết món là ô nhập chữ, gõ thì gợi ý 4–5 nhóm trong 300 nhóm chuẩn | Ghi vào bước 6 Phần E; chưa sửa prototype |
| 01/11 06:34 | Chốt góp ý 4: khoản chi định kỳ có màn chi tiết riêng, giống chi tiết món ở Đồ dùng | Ghi vào bước 6 Phần E; chưa sửa prototype |
| 01/11 06:39 | Góp ý 3 (gợi ý giá ở danh sách kế hoạch) treo làm sau; duyệt bổ sung góp ý 1 và bố cục góp ý 4; góp ý 2 phần bổ sung chờ giải thích | Chỉ sửa md |
| 01/11 06:41 | Màn xác nhận hóa đơn dùng chung ô nhóm gõ có gợi ý với Đồ dùng; không thêm dòng giải thích. Phần E chốt hết (trừ góp ý 3 treo) | Bắt đầu sửa prototype |
| 01/11 07:05 | Sửa prototype theo Phần E góp ý 1, 2, 4; ảnh ở `assets/mimi15-phan-e/` | PR #5 chờ duyệt; chưa đăng lại link xem thử |
| 01/11 07:32 | Mẹ đồng ý; đăng lại link xem thử với bản Phần E | PR #5 chờ gộp |
| 01/11 08:09 | Gộp PR #5 (Phần E) vào main | Bản nén frontend cập nhật |
| 01/11 15:29 | Đưa các phần Google Cloud còn thiếu vào checklist: 10b (Cloud Tasks, 3 job, Calendar), 13 (dựng project, Secret Manager, App Check, Scheduler). Gợi ý giá tốt vẫn treo | Chỉ sửa checklist |
| 02/11 10:33 | Gộp PR #6: Idempotency-Key cho mọi thao tác ghi (data model thay đổi 5) | Chờ chốt cách sửa lot (V3) |
