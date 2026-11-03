# Lịch sử cập nhật

Mỗi lần cập nhật ghi: ngày, nhánh / PR, phần đã làm theo từng file spec, cách kiểm tra, điều còn thiếu.
Mới nhất ở trên cùng.

---

## 2026-11-03 · Nước lấy giá online: Việt Nam và Mỹ

Nhánh `feat/nuoc-vn-us`. Mẹ chốt 03/11: Cài đặt → Quốc gia (giá online) cho chọn Việt Nam và Mỹ; giữ Singapore trong hệ thống vì sau này sẽ thêm nhiều nước.

- `market.Countries` thêm Mỹ (USD; sổ nguồn ban đầu Walmart, Target, Costco, Kroger, Whole Foods), Singapore giữ nguyên. `market.Selectable` là danh sách nước hiện trong Cài đặt; thêm nước mới chỉ cần thêm một dòng.
- Chưa chọn nước thì đoán theo múi giờ: múi giờ Mỹ là US, Asia/Singapore là SG, còn lại VN.
- Cài đặt nhận VN, US, SG; màn Cài đặt hiện Việt Nam, Mỹ. Người đã chọn Singapore trước đó vẫn thấy Singapore trong danh sách.
- Bản thử: chọn Mỹ thì bảng giá online giả giá USD ở Walmart, Target, Costco.
- Kiểm tra: `go test ./...` (thêm `TestSelectableCountries`, ca US trong CountryFor và Cài đặt), `tsc -b`, bản thử chọn Mỹ ra "Rẻ nhất ở Target · 1.15 USD".

## 2026-11-03 · Thư thông báo theo ngôn ngữ của người nhận

Nhánh `feat/thu-theo-ngon-ngu`. Theo `notes/mimi-v2-chot-ngon-ngu-tien-te.md` mục 2 (mẹ chốt 03/11): lời nhắc, link mua hộ, email theo ngôn ngữ trong Cài đặt của người nhận.

- Thư sự kiện (nhờ mua, cập nhật, hủy, mua xong, không mua được, chuyển món) và bản tin "món sắp hết" có thêm bản tiếng Anh; chọn theo `settings.lang` của người nhận, vắng là tiếng Việt.
- Thông báo trong app và màn mua hộ vốn đã dựng chữ theo ngôn ngữ của người đang xem, không cần sửa.
- Kiểm tra: `TestEventMailLang`, `go test ./internal/notify`.
- Lưu ý: chưa có dịch vụ gửi email thật (để sau demo), hiện thư chỉ ghi log.

## 2026-11-03 · Chia giảm giá cả hóa đơn về từng dòng

Nhánh `feat/chia-giam-gia`. Theo `notes/mimi-v2-chot-ngon-ngu-tien-te.md` mục 4 (mẹ đồng ý 03/11): giảm giá tổng bill chia về từng dòng theo tỷ lệ tiền, để sau này giá vốn mỗi món là giá đã trừ giảm.

- `domain.AllocateDiscount`: chia theo tỷ lệ, tổng các phần đúng bằng số giảm (phần dư làm tròn cho dòng có phần lẻ lớn nhất); dòng chưa rõ giá không nhận phần nào.
- Lưu hóa đơn có dòng khuyến mãi (số âm): mỗi dòng ghi thêm `discount_alloc_vnd` trong receipts. Tổng đã chi, tiết kiệm, lịch sử giá không đổi.
- Kiểm tra: `TestAllocateDiscount`, `TestSaveDiscounts` (12.000đ chia 7.529đ + 4.471đ).
- Chưa dùng: giá vốn từng lô vẫn đang treo; dữ liệu này để sẵn cho lúc làm.

## 2026-11-03 · Giao diện hai ngôn ngữ không tràn trên màn nhỏ

Nhánh `feat/giao-dien-song-ngu`. Theo `notes/mimi-v2-chot-ngon-ngu-tien-te.md` mục 2.2 (mẹ chốt 03/11): chữ cố định tinh chỉnh riêng cho từng ngôn ngữ, tên sản phẩm theo luật hiển thị.

- Tab và ô Trang chủ tiếng Anh "Shopping List" đổi thành "Plans" (khớp "Kế hoạch", không xuống 2 dòng).
- Màn mua hộ (tiếng Anh, 320px): nút "Add to Google Calendar" và hai nút cuối màn được xuống dòng thay vì đẩy màn rộng ra; tên món dài được ngắt.
- Lịch sử mua hàng (cả hai ngôn ngữ, 320px): ô tìm co lại, nút Lọc giữ nguyên cỡ.
- Tên sản phẩm ở danh sách (Đồ dùng, Kế hoạch, đi mua): tối đa 2 dòng rồi cắt "…" (trước đây Đồ dùng chỉ 1 dòng); màn chi tiết vẫn hiện đủ.
- Kiểm tra tự động mới `e2e/layout.spec.ts`: mở 13 màn chính ở tiếng Việt và tiếng Anh trên màn 320px, báo lỗi nếu màn nào tràn ngang (chạy trong CI cùng test demo). Bỏ phần sửa CSS thì test báo lỗi, đã thử.

## 2026-11-03 · Dịch tên món theo ngôn ngữ app, giữ tên theo bill để đối chiếu

Nhánh `feat/dich-ten-mon`. Theo `notes/mimi-v2-chot-ngon-ngu-tien-te.md` mục 2.1 (mẹ chốt 03/11).

- Lúc quét: app gửi ngôn ngữ đang dùng (`lang`, trước đây AI bỏ qua). Bill khác ngôn ngữ đó thì AI trả thêm `name_translated` trong cùng lượt đọc (gọn ~40 ký tự, giữ tên hãng), không gọi thêm.
- Máy chủ: dòng có tên dịch thì `name` là tên dịch, `original_name` là tên theo bill đã chuẩn hóa; `raw_name` vẫn là chữ in nguyên văn. Lưu `original_name` vào receipts, trả ra ở chi tiết Lịch sử mua hàng. Bản đọc nhớ tạm theo ảnh + ngôn ngữ.
- App: màn xác nhận hóa đơn và chi tiết Lịch sử mua hàng hiện tên dịch, tên theo bill chữ nhỏ bên dưới. Bill cùng ngôn ngữ thì chỉ một dòng như cũ.
- Kiểm tra: test mới trong `TestRead` (dòng có/không có tên dịch), `go test ./internal/receipt ./internal/history ./internal/httpapi`, `tsc -b`, Playwright bản tiếng Anh (ảnh `screens/ten-dich-lich-su.png`).
- Chưa làm: tên món ở Đồ dùng vẫn lấy theo tên trên bill; hóa đơn đã lưu trước đây không có tên dịch.

## 2026-11-03 · Tiền tệ hiển thị theo từng người (VND, USD)

Nhánh `feat/tien-te-hien-thi`. Theo `notes/mimi-v2-chot-ngon-ngu-tien-te.md` mục 4–5 (mẹ chốt 03/11): mỗi người tự chọn tiền tệ hiển thị, tạm VND và USD; số tiền quy đổi theo tỷ giá của ngày mua.

- Cài đặt › Ngôn ngữ & khu vực: ô Tiền tệ chọn được VND hoặc USD, lưu vào tài khoản (`settings.currency`). Câu ghi chú sửa lại: ngôn ngữ và tiền tệ chọn riêng.
- Máy chủ: `GET /fx/rates?currency=USD&from&to` trả tỷ giá từng ngày (kèm ngày gần nhất trước `from`), dùng chung index với `Store.Rate`. Số tiền vẫn lưu bằng đồng, không đổi dữ liệu nào.
- App (`lib/money.ts`, `app/MoneySync.tsx`): nạp tỷ giá USD 800 ngày, nhớ trên máy; `formatVnd(số, ngày)` quy đổi theo ngày mua, ngày chưa có tỷ giá dùng ngày gần nhất trước đó.
  - Theo ngày mua: Lịch sử mua hàng (từng hóa đơn, từng dòng), lịch sử giá của món, giá lần mua gần nhất / thấp nhất.
  - Tổng theo tháng (Sổ chi tiêu): tháng đã qua theo tỷ giá ngày cuối tháng (cố định), tháng này theo hôm nay.
  - Giá hiện tại, kế hoạch, dự báo: theo tỷ giá hôm nay.
- Sau khi lưu cài đặt, app ghi ngay vào bản nhớ `/me`; lúc mở app chỉ tin `/me` vừa tải từ máy chủ (bản khôi phục từ IndexedDB có thể còn giá trị cũ).
- Kiểm tra: `go test ./internal/httpapi ./internal/user` (test mới `TestFXRates`), test emulator `TestRateHistoryEmulator`, `tsc -b`, Playwright đổi sang USD: Trang chủ 1.284.000đ → 50,46 US$, Sổ chi tiêu và Lịch sử đổi theo.
- Chưa làm: ô nhập tiền (sửa giá, khoản định kỳ) vẫn nhập bằng đồng.

## 2026-11-03 · Ngôn ngữ theo tài khoản, người mới theo ngôn ngữ điện thoại

Nhánh `feat/ngon-ngu-theo-tai-khoan`. Theo `notes/mimi-v2-chot-ngon-ngu-tien-te.md` mục 2 (mẹ chốt 03/11): một ngôn ngữ chung của app, lưu theo tài khoản; người mới theo ngôn ngữ máy.

- Người mới mở app: máy đặt tiếng Việt thì vào tiếng Việt, còn lại tiếng Anh (trước đây luôn tiếng Việt).
- Ô VI/EN ở màn đăng nhập trước là "Ngôn ngữ dịch hóa đơn" và không nối vào đâu; giờ là ngôn ngữ của app, bấm là đổi ngay.
- Sau khi đăng nhập (`app/LangSync.tsx`): tài khoản chưa có ngôn ngữ thì ghi ngôn ngữ đang dùng lên tài khoản; tài khoản đã có ngôn ngữ khác máy thì app đổi theo tài khoản (đăng nhập máy mới vẫn đúng ngôn ngữ). Mở bằng `?ui=` thì ưu tiên `?ui=` và ghi lên tài khoản.
- Máy chủ: tài khoản mới không còn mặc định `lang: vi`; mô tả `lang` trong API đổi thành ngôn ngữ của tài khoản (giao diện, tên món dịch, lời nhắc, email).
- Kiểm tra: Playwright với máy vi-VN, en-US, fr-FR ra đúng ngôn ngữ và nút VI/EN đổi được; `go test ./internal/user ./internal/httpapi`, `tsc -b`.

## 2026-11-03 · Tỷ giá đã chốt của một ngày không bị ghi đè

Nhánh `feat/ty-gia-khong-ghi-de`. Theo `notes/mimi-v2-chot-ngon-ngu-tien-te.md` mục 5 (mẹ chốt 03/11): quét tỷ giá 1 lần/ngày, ngày đã có thì giữ nguyên, ngày chưa có dùng tỷ giá gần nhất trước đó.

- `fsstore.SaveRates` dùng Create thay cho Set: chạy lại fx-sync trong ngày không đổi tỷ giá đã ghi, nên hóa đơn quy đổi theo ngày đó luôn ra cùng một số.
- Phần "ngày chưa có dùng tỷ giá gần nhất trước đó" đã đúng từ trước (`Store.Rate`); lịch fx-sync vẫn 6h sáng mỗi ngày.
- Kiểm tra: test emulator mới `TestSaveRatesKeepsFirstOfDayEmulator` (ghi 2 lần cùng ngày, đọc ra lần đầu).

## 2026-11-03 · Mã nhà cố định, có nút Đổi mã

Nhánh `feat/ma-nha-co-dinh`. Mẹ chốt 03/11: mỗi nhà một mã cố định, dùng như ID nhà; chủ nhà có nút "Đổi mã" khi lỡ lộ.

- Máy chủ: mã nhà không còn hết hạn sau 48 giờ. Bỏ `expires_at` khỏi API, vào nhà chỉ kiểm tra mã đúng và nhà chưa đủ 8 người. Job cleanup không gỡ mã mời nữa; mã cũ đang có (kể cả còn trường `invite_expires_at`) giữ nguyên và dùng tiếp.
- `POST /households/invites` giờ là "tạo hoặc đổi mã": mã cũ hết hiệu lực, người đã ở trong nhà không ảnh hưởng.
- Cài đặt → Thành viên: chưa có mã thì nút "Mời thành viên" tạo mã; có mã rồi thì hiện "Mã cố định, dùng như ID của nhà", nút "Sao chép mã" và nút "Đổi mã" (bấm 2 lần để xác nhận).
- Kiểm tra: `go test ./...`, test emulator cleanup sửa lại (mã cũ không bị gỡ), `tsc`, build, bản thử đổi mã trên iPhone 13.

## 2026-11-02 · Thêm kế hoạch vào lịch (một lần)

Nhánh `feat/them-vao-lich`. ADR-004 mục Lịch, mẹ chốt: chỉ tạo lịch một lần, không nối tài khoản Google Calendar, không tự sửa lịch về sau.

- `lib/calendar.ts`: link Google Calendar điền sẵn (tên, giờ, nơi mua, danh sách món, múi giờ Việt Nam, dài 1 giờ; không đặt giờ thì cả ngày).
- Chi tiết kế hoạch: khi đã có ngày đi chợ, hiện nút "Thêm vào Google Calendar", kèm nhắc đổi giờ thì thêm lại.
- Kiểm tra: tsc, build; trên bản thử link mở đúng ngày giờ 04/11 09:30 đến 10:30.
- Đã bỏ nút "Lịch trên iPhone" (file .ics): trên iPhone file chỉ bị tải về, không mở app Lịch, kể cả trong Safari (mẹ thử 02/11). Người dùng iPhone dùng nút Google Calendar.
- Màn đi mua hộ: người được nhờ cũng có nút "Thêm vào Google Calendar" ngay dưới ô tóm tắt, khi kế hoạch đã có ngày (tên, giờ, nơi mua, danh sách món lấy từ kế hoạch gốc). Nút tách ra `components/AddToCalendar.tsx` dùng chung.

## 2026-11-02 · Quét giá theo nước và sổ nguồn

Nhánh `feat/gia-theo-nuoc` (trên `feat/gia-nha-minh`). Mẹ chốt 02/11. Luật đầy đủ: `docs/mimi15/luat-gia.md`.

- Cài đặt có ô "Quốc gia (giá online)": Việt Nam, Singapore. Chưa chọn thì đoán theo múi giờ (Asia/Singapore là Singapore).
- Bảng giá chung tách theo nước: khóa `market_prices/{nước}_{món}` (ví dụ `vn_sua-tuoi`). Giá ngoài Việt Nam lưu số tiền gốc (`local`, `currency`) và quy ra VND theo tỷ giá job fx-sync; chưa có tỷ giá thì bỏ, không đoán. Giá vô lý kiểm theo tiền gốc từng nước.
- Hỏi AI gộp tối đa 5 món cùng nước, cùng ngành hàng một lần (job đêm và nút Cập nhật giá).
- Sổ nguồn `price_sources/{nước}_{web}`: lưu link ngành hàng và mẫu link tìm kiếm, không lưu link từng sản phẩm. Lần sau đưa link cho AI đọc trước (URL context), thiếu mới tìm Google. Web mới vào sổ khi ra giá 2 lần; đọc trang 2 lần liền không ra giá thì đánh dấu, quay về Google, một tháng sau thử lại.
- Shopee, Lazada chỉ nhận gian hàng chính hãng (Mall, Official).
- Dòng giá Singapore hiện thêm tiền gốc, ví dụ "1.49 SGD".
- Lưu ý: các dòng bảng giá cũ (khóa chưa có nước) không dùng nữa, job đêm tra lại. Gemini đọc trang kèm tìm Google chưa thử trên Vertex thật; nếu lỗi thì tự hỏi lại chỉ bằng Google.
- Kiểm tra: `go test -race ./...` (test mới cho Normalize SGD, batches, sổ nguồn, câu hỏi AI), test emulator `TestPriceSourcesEmulator`, `TestWantedItemsCountryEmulator`; bản thử chọn Singapore thấy giá SGD.

---

## 2026-11-02 · Giá tốt nhất: so giá online với giá nhà mình từng mua

Nhánh `feat/gia-nha-minh` (trên `feat/cap-nhat-gia`). Mẹ chốt 02/11: mỗi tài khoản có lịch sử giá riêng; giá online bằng 0 (chưa có) hoặc cao hơn giá nhà mình từng mua thì lấy giá trong lịch sử mua sắm.

- Khối "Giá đang bán" đổi tên "Giá tốt nhất". Mỗi món lấy giá thấp hơn giữa giá online rẻ nhất và giá nhà mình từng mua (`unit_price`, `best_store` của Kế hoạch, đọc từ hóa đơn của chính nhà).
- Dòng dùng giá nhà mình ghi rõ "Giá nhà mình từng mua ở … (chỉ nhà mình thấy)", kèm giá online để so hoặc "chưa có giá online".
- Chỉ đổi giao diện: giá nhà mình không gửi lên, không ghi vào bảng giá chung.
- Lưu ý: giá online theo quy cách nguồn ghi (ví dụ 4x180ml), giá nhà mình theo đơn vị đã mua, nên so chỉ là tương đối.

---

## 2026-11-02 · Nút "Cập nhật giá" trong Kế hoạch

Nhánh `feat/cap-nhat-gia` (trên `feat/gia-online`). Mẹ hỏi 02/11: không phải chờ job đêm, chỉ tra các món của kế hoạch đang mở.

- `POST /plan/market-prices/refresh`: tra lại giá các món của kế hoạch, cùng nguồn công khai với job. Bỏ qua món đã tra trong 24 giờ, tối đa 12 món mỗi lần, tra song song 6 món để kịp giới hạn 60 giây của Hosting, 3 lần mỗi người mỗi ngày (`quota/price-{ngày}`, tách khỏi lượt đọc hóa đơn). Mẹ chốt 24 giờ và 3 lần (02/11). Không có món nào cần tra thì không tốn lượt.
- Nút "Cập nhật giá" dưới khối "Giá đang bán", báo "Đã tra N món, có giá M món" hoặc lỗi hết lượt. Dưới nút hiện "Hôm nay còn x/3 lần" và "Lần cập nhật gần nhất" (lưu ở `quota/price-last`, `GET /plan/market-prices` trả kèm `refresh`).
- Kiểm tra: `go test ./internal/market ./internal/httpapi`, test emulator `TestMarketTouchedAndRefreshQuotaEmulator`, bản thử bấm nút trên iPhone 13.

---

## 2026-11-02 · Giá online: bảng giá chung và job price-sync

Nhánh `feat/gia-online`. Theo ADR-004 (mẹ duyệt 02/11): bảng giá chung chỉ từ nguồn công khai; giá trên hóa đơn của nhà nào chỉ dùng cho nhà đó.

- `internal/market`: bảng giá chung, `Sync` gom món đang nằm trong Kế hoạch của mọi nhà (chỉ tên món), gộp trùng, bỏ món đã có giá trong 3 ngày, món nhiều nhà cần tra trước, tối đa `PRICE_SYNC_LIMIT` (mặc định 200) món mỗi đêm, dừng sớm khi nguồn lỗi 5 lần liền. Nguồn cắm thêm theo thứ tự ưu tiên; đối tác thêm vào trước Gemini khi có.
- `gemini.PriceSearch`: Gemini + Google Search, chỉ nhận giá thấy trong kết quả tìm kiếm; `market.Clean` bỏ giá vô lý, link lạ, gộp trùng nơi bán.
- Firestore `market_prices/{key}` (+ `price_history/{ngày}`), không trường nào trỏ tới người dùng. Lịch sử giữ 30 ngày (mẹ chốt), TTL `expire_at` của Firestore tự xóa.
- `GET /plan/market-prices` và khối "Giá đang bán" ở Kế hoạch (cạnh "Ước tính theo lịch sử giá").
- Deploy: job `mimi-job-price-sync` (60 phút), lịch 2 giờ sáng, `sa-jobs` thêm quyền Vertex AI.
- Kiểm tra: test `market`, `gemini.parsePrices`, handler, OpenAPI; giao diện chạy trên bản thử.
- Chưa có: mã món chuẩn (hiện khóa là tên món bỏ dấu).
## 2026-11-02 · Quét 2 bill trong một lần

Nhánh `feat/quet-2-bill` (dựa trên `fix/quet-hoa-don-that`).

- Người dùng hay để dồn vài ngày mới quét. Mẹ chốt: tối đa 2 bill mỗi lần quét, mỗi bill đọc riêng (không gộp cửa hàng, ngày, tổng tiền).
- Màn chụp có nút "Bill tiếp": bill đang chụp được gửi đọc ngay (chạy ngầm), màn chụp mở lại cho bill 2. Góc khung camera hiện trạng thái bill 1.
- Bấm "Xong": xem lại và lưu bill 1, rồi "Xem bill tiếp theo" để lưu bill 2. Bill 1 đọc lỗi: chọn "Đọc lại bill 1" (ảnh đã đọc được lấy từ bộ nhớ đệm) hoặc "Bỏ bill 1".
- Mỗi bill vẫn tính 1 lượt quét như cũ. Không đổi backend.
- Thử trên iPhone: bị hỏi quyền camera mỗi lần sang bill 2 hoặc gửi lỗi. Camera giờ mở một lần cho cả màn quét (`useCamera`), không tắt khi gửi đọc, tắt khi rời màn.
- Thử `GEMINI_THINKING=MINIMAL`: mọi bill báo "tạm thời chưa đọc được" (AI từ chối). Script deploy giữ LOW, ghi chú không dùng MINIMAL.
- Tạm thời (mẹ chốt): trên iPhone/iPad, nút chụp mở camera có sẵn của máy (`<input capture>`), không gọi camera trong trang nên không bị hỏi quyền. Khung hiện ảnh vừa chụp. Android giữ camera trong trang. Có app cài trên máy thì bỏ nhánh này.
- Kiểm tra: tsc, build, chạy luồng 2 bill trên bản thử (giả lập iPhone 13, camera giả).

## 2026-11-02 · Quét hóa đơn thật báo "chụp lại" dù ảnh rõ

Nhánh `fix/quet-hoa-don-that`.

- Thử trên mimi-1-5 với bill GO! rõ nét, app báo "Mimi chưa đọc được hóa đơn này. Chụp lại rõ hơn".
- Nguyên nhân nghi nhiều nhất: schema gửi Gemini có `category_key` với enum chứa chuỗi rỗng, Vertex AI từ chối (400). Mọi lỗi 4xx của AI lại bị xếp thành "ảnh không đọc được".
- Sửa: bỏ chuỗi rỗng khỏi enum; lỗi 400/403/404 từ AI thành `ai_unavailable` ("thử lại sau hoặc nhập tay"), không bảo chụp lại, không thử lại; log `receipt read` ghi thêm `cause` (lời Google trả về) để lần sau thấy ngay.
- Nguyên nhân thật (log mới chỉ ra): Google trả 404 cho `gemini-2.5-flash` vì dòng Gemini 2.5 trên Vertex AI đã bị tắt ngày 20/10/2026. Script deploy đổi mặc định sang `gemini-3.8-flash` (thử trên mimi-1-5: 3.8, 3.7, 3.5 flash trả 200).
- Sau khi đổi model, quét 1 ảnh được nhưng chậm; chụp 2 đến 3 ảnh thì ảnh sau báo `read_failed`. Thời gian chờ mỗi ảnh tăng từ 20 lên 40 giây (request vẫn trong 55 giây).
- Màn chụp trên iPhone: khung camera co thành dải mỏng khi có ảnh. Video giờ phủ kín khung (vị trí tuyệt đối), khung giữ tối thiểu 40% chiều cao màn hình, màn cuộn được nếu chật.
- Thử lại trên iPhone: khung vẫn chỉ hiện video ở một dải dưới đáy sau ảnh 1. Khung camera giờ cao cố định (44% màn hình), không đổi cỡ khi thêm ảnh hay hiện thông báo; ảnh thu nhỏ và nút không bị ép bẹp; lời dặn ẩn sau ảnh đầu để nút "Xong" vẫn trong màn hình. Gửi lỗi thì camera tự bật lại thay vì để khung đen.
- Gemini 3.x mặc định "suy nghĩ" trước khi trả lời, làm đọc bill chậm. Thêm `GEMINI_THINKING` (mặc định `LOW`; đặt rỗng để model tự chọn).
- Kiểm tra: `TestSchemaEnumsHaveNoEmptyValue`, `TestClassify`, test service mới; chạy lại trên mimi-1-5 với bill thật.

## 2026-11-02 · Bật App Check (reCAPTCHA Enterprise)

Nhánh `deploy/app-check`, xếp chồng lên PR #36.

- `deploy/appcheck.sh <project>`: bật API, tạo khóa reCAPTCHA `mimi-web` cho `web.app` và `firebaseapp.com`, đăng ký khóa với App Check của app web, build lại frontend có `VITE_RECAPTCHA_ENTERPRISE_KEY` và đưa lên Hosting. Máy chủ chưa bắt buộc.
- `deploy/appcheck.sh <project> on`: máy chủ bắt buộc App Check (`APP_CHECK_REQUIRED=true`). `off` để quay lại.
- `deploy/cloudshell-setup.sh` chạy lại vẫn giữ App Check đang bật hay tắt, và tự thêm khóa reCAPTCHA vào frontend nếu đã có.
- Kiểm tra: `bash -n`; chạy thật trên mimi-1-5 rồi thử đăng nhập, lưu hóa đơn trước khi `on`.
- Còn thiếu: `MIMI_ENV=prod` chờ có nhà cung cấp email.

## 2026-11-02 · Lệnh dựng và deploy bản demo từ Cloud Shell

- `deploy/cloudshell-setup.sh <project-id>`: tạo tài khoản dịch vụ (sa-api, sa-jobs, sa-scheduler) và cấp quyền, deploy `mimi-api` lên Cloud Run từ mã nguồn, tạo các job (fx-sync, cleanup, migrate, seed-demo), lịch chạy fx-sync 06:00 và cleanup 03:00, lấy cấu hình web Firebase qua API (không commit khóa), build frontend và đưa rules, index, Hosting lên Firebase.
- `backend/firebase.json` để `firebase deploy --only firestore` dùng đúng rules và index.
- Bản demo chạy `MIMI_ENV=dev`: thư chỉ ghi log, App Check tắt. Bật thật khi đã chọn dịch vụ email và cấu hình reCAPTCHA Enterprise.

### Cách kiểm tra
- `bash -n deploy/cloudshell-setup.sh` qua. Chạy thật trên project `mimi-1-5`.

---

## 2026-11-02 · Test tự động cho kịch bản demo

- Thêm test Playwright `frontend/e2e/demo.spec.ts` chạy kịch bản demo trên bản thử: đăng nhập, quét bill, xem lại và lưu, Đồ dùng (Cần mua ngay, Sắp hết, Dầu gội còn 5 ngày), Sổ chi tiêu (tổng tháng, nhãn danh mục), Kế hoạch (gợi ý mua, dự kiến chi, ba cách mua với Mimi đang khóa).
- CI chạy test này sau bước build. Khi lỗi thì tải được báo cáo `playwright-report`.
- Lỗi test bắt được: bản thử báo "Unknown field" khi lưu hóa đơn, vì danh sách trường của backend giả thiếu `plans` và `price_source`. Đã sửa, bây giờ lưu hóa đơn trên bản thử chạy lại được.

### Cách kiểm tra
- `npm run build && npm run test:e2e` qua (trên máy có sẵn Chromium thì đặt `PW_CHROMIUM_PATH`).

---

## 2026-11-02 · Danh mục chi tiêu một nguồn (V2)

- 27 danh mục chi tiêu (khóa và nhãn tiếng Việt, tiếng Anh) cùng hai danh mục hệ thống `hoa-don-dinh-ky`, `chua-phan-loai` nay nằm ở một file: `backend/internal/taxonomy/categories.json`.
- Backend đọc danh sách cho AI từ file này (`taxonomy.SpendingKeys`), bỏ danh sách "KHÓA TẠM" trong `cmd/mimi`. Hai danh mục hệ thống dùng hằng số thay cho chuỗi viết tay.
- Frontend lấy nhãn từ bản chép `frontend/src/i18n/categories.json`, bỏ hai bảng nhãn lặp trong `vi.ts` và `en.ts`. Chữ hiển thị không đổi.
- Thêm hoặc đổi danh mục: sửa file backend rồi chép sang frontend. Test sẽ báo nếu quên chép.

### Cách kiểm tra
- `go test ./...` qua; test mới `TestSpendingKeys`, `TestSpendingByParentKnown` (danh mục mặc định theo nhóm cha đều có thật), `TestCategoriesFrontendCopy` (hai file giống hệt).
- Frontend `tsc -b` và `npm run build` qua.

---

## 2026-11-02 · Dữ liệu mẫu cho buổi demo (V5)

- `mimi job seed-demo` ghi 7 hóa đơn mẫu của nhà chị Lan vào tài khoản demo (`DEMO_UID`): Co.opmart, Bách Hóa Xanh, WinMart trong khoảng 5 tuần, tính lùi từ ngày chạy.
- Hóa đơn đi qua đúng đường lưu thật nên Đồ dùng, dự đoán ngày hết, Sổ chi tiêu, lịch sử giá đều có số liệu như khi quét bill.
- Lịch mua được chọn để ngày demo khớp kịch bản cảnh 3a: Sữa tươi còn khoảng 2 ngày, Trứng gà sắp hết, Dầu gội còn khoảng 5 ngày. Giá chênh nhẹ giữa các cửa hàng để màn so sánh giá có nơi rẻ hơn.
- Không còn ngày cố định 16/10 ở đâu nữa (bản chạy thử trên máy đã dùng ngày thật từ trước).
- README: cách chạy job, và bảng lịch cho các job `digest`, `fx-sync`, `cleanup`.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; test mới `TestSeedDatesFollowToday`, `TestDemoSeedEmulator` (lưu thật trên emulator, kiểm tra dự đoán Sữa tươi và chạy lại không ghi trùng).
- `TestSeedLinesReachInventory` chạy qua service lưu hóa đơn thật: mọi dòng demo đều đủ danh mục, nhóm và khối lượng để vào tồn kho. Test này bắt được job thiếu danh sách danh mục (mọi dòng bị coi là chưa phân loại) và rau, chuối, bánh quy thiếu khối lượng.

---

## 2026-11-02 · Job fx-sync hằng ngày: tỷ giá cho hóa đơn ngoại tệ (Đợt 4)

- `mimi job fx-sync` lấy tỷ giá mỗi ngày từ open.er-api.com (miễn phí, không cần khóa) và ghi `fx_rates/{ngoại tệ}_{ngày}` cho 12 ngoại tệ hay gặp (USD, EUR, JPY, KRW, CNY, THB, SGD, GBP, AUD, TWD, HKD, MYR).
- Mỗi tỷ giá ghi rõ nguồn và lúc lấy; hóa đơn ngoại tệ dùng tỷ giá gần nhất không sau ngày mua, như đã thiết kế. Trước đây chưa có job này nên hóa đơn ngoại tệ không quy đổi được.
- Tỷ giá tính bằng số hữu tỉ từ chuỗi gốc, không qua số thực, lưu 4 chữ số thập phân.
- Nguồn lỗi hoặc thiếu VND thì job báo lỗi, không ghi gì; thiếu một ngoại tệ thì bỏ qua ngoại tệ đó.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; test mới `TestSyncFromERAPI`, `TestSyncRejectsBadTable` (máy chủ giả), `TestSaveRatesThenRateEmulator`.

---

## 2026-11-02 · Job cleanup hằng ngày (Đợt 4)

- `mimi job cleanup` (Cloud Scheduler gọi mỗi ngày) giờ chạy thật thay vì báo "chưa làm":
  - Gỡ mã mời đã hết hạn khỏi nhà. Chủ nhà vừa tạo mã mới trong lúc job chạy thì bỏ qua, không gỡ nhầm.
  - Xóa thông báo trong app quá 90 ngày (màn Thông báo chỉ hiện 30 ngày gần nhất).
- Thông báo mới có `expire_at` = lúc tạo + 90 ngày, `firestore.indexes.json` thêm TTL cho `events`; job dọn nốt thông báo cũ chưa có trường này.
- Bộ đếm hạn mức, bản nháp hóa đơn, chống gửi trùng, bộ đếm giới hạn vẫn do TTL của Firestore tự xóa.
- Rà lại index: mọi truy vấn nhiều điều kiện đều đã có index, không cần thêm.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; test mới `TestCleanupEmulator` chạy trong job emulator của CI.

---

## 2026-11-02 · Xóa tài khoản, đăng nhập lại để xác nhận (B10)

- Cài đặt → Quyền riêng tư & bảo mật có mục Xóa tài khoản. Bấm Xóa vĩnh viễn: nếu mẹ chưa đăng nhập lại trong 5 phút, Mimi mời đăng nhập lại (Google, hoặc link gửi về email rồi quay lại đúng trang này), sau đó mẹ bấm Xóa vĩnh viễn lần nữa.
- App không dùng mật khẩu, nên "nhập lại mật khẩu" ở đây là đăng nhập lại. Máy chủ kiểm tra token chưa bị thu hồi và thời điểm đăng nhập (`auth_time`) trong 5 phút; không thì trả 403 `reauth_required` (không phải 401, để app không tự đăng xuất).
- Máy chủ xóa theo thứ tự: chủ nhà thì mời từng người nhà ra (vòng mua hộ còn mở bị hủy, món trả về kế hoạch người nhờ) rồi xóa nhà; thành viên thì rời nhà. Sau đó xóa toàn bộ `users/{uid}`, rồi xóa tài khoản đăng nhập sau cùng. Lỗi giữa chừng thì gọi lại sẽ chạy tiếp.
- Món mẹ đã mua hộ người nhà nằm trong sổ của họ nên vẫn giữ.
- Xóa xong, app xóa dữ liệu trên máy và về màn Chào mừng.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; test mới `TestDeleteOwnerDissolvesHousehold`, `TestDeleteMemberOnlyLeaves`, `TestDeleteResumesAndStopsBeforeAuth`, `TestDeleteMeNeedsRecentSignIn`, `TestEraseUserEmulator` (chạy khi có emulator).
- `npm run build` qua; chạy thử chế độ mẫu: bấm xóa thì được mời đăng nhập lại, đăng nhập lại rồi xóa thì về màn Chào mừng, không lỗi console.
- Chưa thử với Firebase thật (chờ dựng dự án Firebase ở bước 13).

---

## 2026-11-02 · Đọc giá nhiều món cùng lúc (A9)

- Trước đây máy chủ đọc giá gần nhất và lịch sử giá lần lượt từng món: giỏ 20 món là 20 lượt đọc Firestore nối nhau, màn so sánh giá và giá ước tính chậm theo số món.
- Giờ các lượt đọc chạy song song, tối đa 8 lượt cùng lúc; giỏ 20 món chỉ mất khoảng thời gian của 3 lượt. Món trùng trong danh sách chỉ đọc một lần.
- Áp dụng cho giá theo nơi mua (Giỏ, Giá tốt, ô Trang chủ, giá ước tính khi đi mua) và lịch sử giá 90 ngày. Kết quả giữ nguyên thứ tự như trước.

### Cách kiểm tra
- `go vet`, `go test -race ./...` qua; test mới `TestEachGroupParallel`.

---

## 2026-11-02 · Giới hạn số lần ghi dùng chung giữa các máy chủ (A5)

- Trước đây mỗi máy chủ (instance Cloud Run) tự đếm 60 thao tác ghi mỗi phút cho mỗi người. Chạy 3 instance thì một người ghi được tới 180 lần.
- Giờ bộ đếm nằm trên Firestore (`users/{uid}/rate/{phút}`), mọi instance cùng đọc ghi trong một transaction, nên tổng vẫn là 60.
- Bộ đếm tại chỗ vẫn chạy trước để chặn ngay lượt vượt rõ ràng, không tốn lượt ghi Firestore. Firestore lỗi thì không chặn người dùng, bộ đếm tại chỗ vẫn giữ giới hạn.
- `firestore.indexes.json` thêm TTL cho `rate.expire_at` (10 phút sau phút đếm). Lúc deploy cần `firebase deploy --only firestore:indexes` để bật TTL.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; test mới `TestAllowWriteSharedAcrossInstances`, `TestRateLimiterSharedEmulator` (chạy khi có emulator).

---

## 2026-11-02 · Hợp đồng API: sinh kiểu từ OpenAPI, CI báo khi lệch (A3)

- `frontend/src/api/schema.d.ts` sinh từ `backend/api/openapi.yaml` (`npm run gen:api`) và được commit. CI chạy lại lệnh này; file sinh ra khác bản đã commit thì CI đỏ.
- `frontend/src/api/contract.ts`: lúc build, so tên trường của 22 kiểu viết tay trong `types.ts` với OpenAPI. Thiếu hay thừa một trường thì `tsc` báo đúng kiểu và tên trường.
- Backend có test `TestOpenAPIMatchesRoutes`: mọi route trên máy chủ phải có trong OpenAPI và ngược lại.
- Ba thứ tìm ra khi bật kiểm tra:
  - `openapi.yaml` bị hỏng từ đợt trước (mất dòng tên `Item`, khóa trùng), nên `gen:api` không chạy được. Đã sửa.
  - OpenAPI thiếu trường máy chủ vẫn gửi: `Cart.plan_id`, `Cart.phase`, `Assignment.trip`, `BasketPrices.window_days`, `Stock.parts`. Đã thêm.
  - App thiếu kiểu cho `HomeSummary.receipt_count`, `Item.interval_days`, `PlanPrices.sale_offers`; OpenAPI ghi `GET /ready` mà máy chủ chưa phục vụ. Đã thêm.
- Chưa làm: xếp lại thư mục frontend theo tính năng, và thay hẳn `types.ts` bằng kiểu sinh ra. Cần OpenAPI ghi đủ `required` trước, làm sau.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; `npm run gen:api` chạy hai lần ra cùng file; `npm run build` qua. Thử đổi tên một trường trong `types.ts` thì `tsc` báo lỗi tại `contract.ts`.

---

## 2026-11-02 · Tìm tên món không phân biệt cách gõ dấu (A2)

- Cùng chữ "ầ" có thể đến ở hai dạng: một ký tự (bàn phím điện thoại) hoặc "a" kèm dấu rời (dán từ máy Mac, chữ AI đọc từ ảnh). Trước đây hai dạng nhìn giống nhau nhưng máy chủ coi là khác nhau.
- Máy chủ đưa mọi chữ gửi lên (tên món, tên kế hoạch, ghi chú, tên gọi người nhà) về một dạng chuẩn (NFC) trước khi lưu. Tên món AI đọc từ hóa đơn cũng vậy.
- Tìm không dấu tách hẳn dấu ra rồi bỏ, nên "dau goi" khớp "Dầu gội" ở cả hai dạng; dữ liệu cũ đã lưu ở dạng dấu rời vẫn tìm ra.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; test mới `TestNFCAndFoldDecomposed`, `TestDecodeJSONNormalizesNFC`.

---

## 2026-11-02 · Báo khi hai máy cùng sửa một kế hoạch (A1)

- Mỗi kế hoạch có số phiên bản `rev` (lưu ở `plan_rev`), tăng mỗi lần lưu ngày, giờ, ghi chú.
- App gửi kèm `base_rev` là số đã đọc. Máy khác đã lưu trước thì máy chủ trả 409 "Kế hoạch vừa được sửa trên máy khác…", không ghi đè; app tự tải lại bản mới để mẹ xem rồi sửa lại.
- Kiểm tra và ghi nằm trong một transaction Firestore, nên hai máy bấm lưu cùng lúc cũng chỉ một máy thắng.
- Bản app cũ không gửi `base_rev` vẫn lưu được như trước. Sửa từng món (số lượng, ưu tiên) vẫn lưu theo lần ghi sau cùng.
- OpenAPI: `PlanState` thêm `buy_minute`, `reconciled_on`, `rev`, `base_rev`; `PUT /plan/state` có 409 và tham số `plan_id`.

### Cách kiểm tra
- `go vet`, `go test ./...` qua; test mới `TestSetStateRevConflict` và `TestSetStateRevEmulator` (chạy khi có emulator).
- `npm run build` qua; chạy thử chế độ mẫu: lưu kế hoạch hai lần liền không báo nhầm, dữ liệu cũ trên máy thì hiện câu báo xung đột và tải lại.

---

## 2026-11-02 · Rà màn đang tải và báo lỗi ở mọi màn (A10)

- Thêm `LoadState` dùng chung: lần tải đầu hiện "Đang tải"; lỗi mà chưa có dữ liệu thì báo đúng lý do (mất mạng, mạng chậm, lỗi máy chủ) kèm nút Thử lại; đang có dữ liệu đã lưu thì không che màn.
- Các chỗ đã sửa:
  - Chi tiết hóa đơn (4d): hóa đơn không còn hoặc lỗi thì trước đây quay "Đang tải" mãi, giờ báo "Mục này không còn nữa" hoặc lỗi kèm Thử lại.
  - Lịch sử mua hàng (4c), danh sách kế hoạch, chi tiết kế hoạch, lịch sử giá trong Chi tiết món: thêm báo lỗi và Thử lại (trước đây lỗi trông như danh sách trống).
  - Màn đi mua hộ: mất mạng không còn báo nhầm "Lần nhờ mua này không còn nữa".
  - Sổ chi tiêu, phần Sắp chi: lỗi tải không còn hiện 0đ như thể chưa có khoản nào.
  - Chi tiết khoản định kỳ: phân biệt "không còn" với lỗi tải.
  - Trang chủ: không tải được số liệu thì báo một dòng kèm Thử lại, các ô vẫn bấm được.
- Các màn khác (Đồ dùng, Thông báo, Quét, Cài đặt, Online) đã có đủ trạng thái tải và lỗi.

### Cách kiểm tra
- `tsc`, `npm run build` qua; chạy thử ở chế độ mẫu các màn Trang chủ, Kế hoạch, Lịch sử, Chi tiết hóa đơn không tồn tại, Đồ dùng, Thông báo, không có lỗi trên console.

---

## 2026-11-02 · Thông báo trong nhà qua hàng đợi, có thử lại (B9)

- Mẹ gửi, cập nhật hay hủy kế hoạch thì người mua nhận thông báo; người mua bấm "Mua hàng xong" hay từ chối thì mẹ nhận thông báo; món chưa mua sang kế hoạch mới cũng báo mẹ.
- Mỗi thông báo được ghi trong cùng một lần lưu với thay đổi (`users/{uid}/events`), nên không mất kể cả khi gửi thư lỗi. Màn Thông báo hiện chúng ở đầu, chưa đọc thì đậm, bấm vào mở đúng kế hoạch hoặc lần đi mua và đánh dấu đã đọc.
- Có cấu hình hàng đợi Cloud Tasks `notify` thì máy chủ đẩy task cho `mimi-worker` gửi thư: lỗi tạm thời thì hàng đợi thử lại có giãn cách, tối đa 5 lần; bị từ chối hẳn thì dừng; task bị gửi lại không gửi thư lần hai. Người tắt nhắc nhở hoặc chưa có email thì không nhận thư, vẫn thấy trong app.
- Thư không có giá tiền. Gửi thư thật cần nhà cung cấp email ở bước 13; hiện bản dev ghi log thay vì gửi.
- README có mục tạo hàng đợi và cấp quyền.

### Cách kiểm tra
- `go test ./...` qua. Test mới: `TestDeliver`, `TestEventMail`, `TestTaskBody`, `TestEndEventFor`, `TestListEvents`.
- `npm run build` qua; chạy thử màn Thông báo ở chế độ mẫu.

---

## 2026-11-02 · Món chưa có giá và giá ước tính (B7)

- Mỗi dòng hóa đơn ghi thêm nguồn giá `price_source`: `receipt` (in trên hóa đơn), `manual` (người đi mua tự nhập), `estimate` (ước tính), `missing` (chưa có giá). Giá trị `online` đã có chỗ trong dữ liệu, để dành cho phần quét giá online ở bước 13.
- Đi mua hộ không có hóa đơn mà không nhập giá: lấy giá mua gần nhất của món đó, ghi "Ước tính". Món của người nhờ theo giá người nhờ từng mua, món của mình theo giá của mình.
- Món chưa từng có giá thì để là chưa có giá: không tính 0đ vào tổng, màn Chi tiết hóa đơn hiện "chưa có giá" và đếm số món cần nhập sau.
- Có quét hóa đơn thì không ước tính, giá theo hóa đơn.
- Hóa đơn cũ không có `price_source`: đọc như trước.

### Cách kiểm tra
- `go test ./...` qua. Test mới: `TestCompletePriceSources`, `TestLatestUnitPrices`, "price source is validated" trong `TestSave`.
- `npm run build` qua.

---

## 2026-11-02 · Món chưa mua tự sang kế hoạch "· còn lại" (B6)

- Khi một kế hoạch được chốt (người mua bấm "Mua hàng xong", hoặc mẹ xác nhận hóa đơn trừ vào kế hoạch đó), món còn mở chuyển sang kế hoạch mới tên "[tên cũ] · còn lại".
- Kế hoạch mới giữ biểu tượng và nơi mua, chưa có ngày đi mua, chưa có người mua, nằm trong danh sách của người nhờ. Món mua thiếu chỉ mang theo phần còn thiếu.
- Mua đủ hết thì không tạo gì. Kế hoạch chính không tách vì là danh sách dùng lâu dài. Kế hoạch đứng sau chỉ nhận phần dư của hóa đơn thì chưa tính là xong.
- Màn lưu hóa đơn báo "Đã chuyển 1 món chưa mua sang kế hoạch mới ›", bấm vào mở kế hoạch mới. Kế hoạch "Đã mua xong" xuống cuối danh sách và mờ đi.
- Bước tách chạy ngay sau khi lưu hóa đơn. Nếu lỗi thì hóa đơn vẫn lưu, món vẫn ở kế hoạch cũ; kế hoạch cũ ghi `split_to` nên không bao giờ tách hai lần.

### Cách kiểm tra
- `go test ./...` qua. Test mới: `TestRemainName`, `TestRemainLine`, `TestSplitRemainingEmulator` (chạy khi có Firestore emulator).
- `npm run build` qua.

---

## 2026-11-02 · Kế hoạch giữ tối đa 2 bản (B5)

- Trước đây mỗi lần bấm máy bay giấy lại tạo thêm một lần nhờ mua chỉ gồm món mới, nên người mua thấy nhiều thẻ cho cùng một kế hoạch, còn món mẹ bỏ đi vẫn nằm trong bản cũ.
- Giờ mỗi kế hoạch chỉ có hai bản: bản mẹ đang lưu và bản đã gửi người mua. Gửi lại thì ghi đè bản đã gửi:
  - món mới thêm vào, món mẹ bỏ trả về kế hoạch, món đã mua giữ nguyên;
  - tên và số lượng lấy theo kế hoạch hiện tại;
  - người mua thấy "Mẹ đã cập nhật kế hoạch lúc 10:32". Món còn trong bản mới giữ dấu tick và chỗ đã sửa.
- Đổi người mua thì bản đã gửi người cũ bị hủy, món về lại kế hoạch rồi gửi người mới. Người mua đã bấm "Mua hàng xong" thì lần gửi sau là lần nhờ mua mới.

### Cách kiểm tra
- `go test ./...` qua. Test mới: `TestAssignKeepsTwoVersions`.
- `npm run build` qua.

---

## 2026-11-02 · Chia hóa đơn theo từng người (B4)

- Trước đây cả hóa đơn cộng vào Sổ chi tiêu của người quét, kể cả món mua hộ.
- Giờ mỗi người chỉ cộng phần của mình. Ví dụ hóa đơn 135.000đ có 120.000đ món của chủ giỏ và 15.000đ món của Minh: sổ chủ giỏ +120.000đ, sổ Minh +15.000đ.
- Giảm giá cả hóa đơn trừ vào phần người trả tiền. Nếu giảm giá lớn hơn phần đó, phần người được mua hộ thu nhỏ theo tỉ lệ để tổng vẫn khớp hóa đơn.
- Sổ chi tiêu của chủ giỏ có thêm dòng "Trong đó người nhà mua hộ …" (`delegated_spent`). Danh sách cửa hàng cũng gồm các lần được mua hộ (`delegated: true`), chỉ tính tiền món của mình.
- Tháng cũ không tự sửa lại: chỉ hóa đơn lưu từ bản này mới chia.

### Cách kiểm tra
- `go test ./...` qua. Test mới: `TestSplitSpend`, `TestSplitCategories`, `TestDelegatedRows`. Test emulator mua hộ đổi kỳ vọng: sổ Owner +70.000đ, sổ Buyer 0đ.
- `npm run build` qua.

---

## 2026-11-02 · Mỗi món trong hóa đơn trừ vào đúng kế hoạch của nó (B3)

- Trước đây màn Xác nhận đã hiện "→ Mua đồ cho bé" cho từng món, nhưng lúc lưu chỉ gửi một kế hoạch cho cả hóa đơn. Món thuộc kế hoạch khác nên không được đánh dấu đã mua.
- Giờ mỗi dòng gửi kèm `plans: [{plan_id, qty}]`, đúng như phần chia đang hiện: tự chia theo kế hoạch đi mua sớm nhất, hoặc kế hoạch người dùng chọn, hoặc "không trừ". Máy chủ đối chiếu từng kế hoạch theo đúng số lượng được chia.
- Dòng hóa đơn lưu thêm ô `plans`. Chi tiết hóa đơn (4d) và Lịch sử (4c) hiện mọi kế hoạch hóa đơn đã trừ vào.
- Hóa đơn cũ không cần chuyển dữ liệu: dòng không có `plans` vẫn đọc theo `source_plan_id` như trước.

### Cách kiểm tra
- `go test ./...` qua. Test mới: `TestPlanBuys`, cùng "per-line plans are validated and kept" trong `TestSave`.
- `npm run build` qua.

---

## 2026-11-02 · Số phiên bản dữ liệu và bước chuyển dữ liệu (A4)

- Mỗi `users/{uid}` có ô `schema_version`. Người dùng mới được ghi phiên bản hiện tại (1). Dữ liệu cũ chưa có ô này được coi là bản 0 và được nâng lên 1 ở lần đăng nhập tới. Mỗi người chỉ có một ô số, nên dữ liệu không phình ra.
- `internal/gcp/fsstore/migrate.go` giữ danh sách bước chuyển. Bước 1 chỉ làm mốc, không đổi dữ liệu. Các thay đổi sau này (ví dụ B3, gắn kế hoạch cho từng dòng hóa đơn) thêm bước 2, 3...
- Bước chuyển chạy lúc đăng nhập, và chạy bằng job `mimi job migrate` cho người lâu không đăng nhập. Không bao giờ hạ phiên bản, nên quay lại bản cũ vẫn an toàn.
- README có mục "Phiên bản dữ liệu".

### Cách kiểm tra
- `go vet ./...`, `go test ./...` qua. Test mới: `TestMigrationsOrdered`, `TestPendingMigrations`.

---

## 2026-11-02 · README: các bước quay lại bản cũ (A6)

- `backend/README.md` thêm mục "Quay lại bản cũ": khi nào quay lại, lệnh cho Cloud Run (api, worker, job), Firebase Hosting, Firestore rules, và luật thay đổi dữ liệu để quay lại không hỏng dữ liệu.
- Thêm lệnh chuyển traffic sang bản `canary` sau khi thử.

---

## 2026-11-02 · Người được nhờ báo mua xong hoặc từ chối (B2)

- `POST /households/shopping-assignments/{id}/complete`: ghi các món đã tick thành một hóa đơn của người mua, đi qua cùng đường lưu hóa đơn có `assignment_id`. Kết quả:
  - Món nhờ mua đã tick thành "done" và vào tồn kho của người nhờ.
  - Món chưa tick trả về Kế hoạch của người nhờ.
  - Món "của mình" vào tồn kho của người mua.
  - Chi tiêu tính cho người trả tiền.
  - Mã hóa đơn là `run-{mã vòng}`, nên hàng đợi offline gửi lại cũng không ghi trùng.
- Món chưa nhập giá được lưu là chưa rõ giá (A7). Không tick món nào thì vòng kết thúc ở trạng thái "mua một phần" và mọi món trả về Kế hoạch.
- `POST /households/shopping-assignments/{id}/decline`: người được nhờ từ chối. Vòng chuyển sang trạng thái `declined`, mọi món trả về Kế hoạch của người nhờ.
- Chỉ người được nhờ mới báo xong hoặc từ chối được. Vòng đã kết thúc thì báo `invalid_state`.
- App gửi đủ danh sách, chỉ gồm các trường máy chủ cần.

### Cách kiểm tra
- `go test ./...` qua. Test mới: `TestCompleteWritesReceipt`, `TestCompleteRules`, `TestDecline`.
- `npm run build` qua.

---

## 2026-11-02 · Món chưa rõ giá không bịa số, không tính 0đ (A7)

- Lưu hóa đơn (`POST /receipts`): dòng có ô giá trống được lưu là "chưa rõ giá" (giá null). Món vẫn vào tồn kho và lịch sử, nhưng không cộng vào tổng, chi tiêu tháng hay lịch sử giá. Giá có ghi mà sai dạng thì vẫn báo lỗi như cũ.
- Màn Xác nhận hóa đơn: món chưa rõ giá hiện mờ với chữ "Chưa rõ giá", kèm dòng "n món chưa rõ giá, không tính vào tổng".
- Chi tiết hóa đơn (4d): món chưa rõ giá hiện mờ, có ghi chú dưới tổng tiền.

### Cách kiểm tra
- `go test ./...` qua. Test mới: "blank amount is saved as unknown price" trong `TestSave`.
- `npm run build` qua.

---

## 2026-11-02 · Lịch sử mua hàng và Chi tiết hóa đơn chạy bằng dữ liệu thật (B1)

- `GET /receipts/history` (màn 4c) trả mọi hóa đơn, mới nhất trước. Gồm hóa đơn tự lưu (tối đa 500), phần món người nhà mua hộ (gom theo hóa đơn của người mua, mã `d-...`, chỉ các món của mình) và khoản định kỳ trả tay.
- `GET /receipts/{id}` (màn 4d) trả các món, tạm tính, giảm giá và kế hoạch đã trừ. Hóa đơn có món mua hộ thì chia phần "của người nhờ" và "của mình".
- Người mua hộ có kèm `buyer_uid`, nên app hiện đúng tên gọi người dùng đã đặt (Anh, Bố...), không có thì hiện tên hiển thị.
- Món chưa rõ giá có `amount` là null, không cộng vào tổng, và được đếm ở `missing_price`. App ghi "chưa có giá" thay cho 0đ. Phần làm mờ thuộc A7.
- Chỉ đọc dữ liệu đã có (receipts, purchases, recurring_occurrences), không thêm collection hay index.

### Cách kiểm tra
- `go vet ./...`, `go test ./...` qua. Test mới: `TestHistory`, `TestDetail`.
- `npm run build` qua. Đã chạy thử màn 4c và 4d ở chế độ mock, console không có lỗi.

---

## 2026-11-02 · Icon kế hoạch và tên gọi người nhà lưu trên máy chủ (B11)

- Biểu tượng của Kế hoạch lưu cùng Kế hoạch trên máy chủ (`icon` trong `GET/POST /plans`, `PATCH /plans/{id}`). Kế hoạch chính chỉ đổi được biểu tượng, lưu ở `default_plan_icon` trên users/{uid}. Máy chủ chỉ nhận 6 khóa trong bộ biểu tượng chung của app: bag, family, kid, gift, home, food.
- Tên gọi người nhà (Anh, Chị, Bố...) lưu trong `settings.relations` theo uid, riêng cho từng tài khoản. `PUT /settings {relations: {uid: "anh"}}` chỉ gửi phần đổi, `""` là xóa, tối đa 50 tên.
- App không còn giữ hai thứ này trên máy, nên đăng xuất rồi đăng nhập lại, hoặc đổi máy, vẫn còn nguyên. Đây là phần sửa cho lỗi đăng xuất làm mất icon và tên gọi (từ PR #7).

### Cách kiểm tra
- `go vet ./...`, `go test ./...` qua. Test mới: `TestPlanIconStoredOnServer`, `TestRelations`.
- `npm run build` qua.

### Còn thiếu
- Icon và tên gọi đã đặt trên máy trước bản này không được chuyển lên máy chủ. Từ PR #7 trở đi, đăng xuất đã xóa chúng rồi.

---

## 2026-11-02 · Chi tiết món đọc đủ lịch sử mua (lỗi V3)

- Mọi lần mua vẫn được lưu như trước (không bao giờ xóa). Sửa ở chỗ đọc: Chi tiết một món đọc tới 500 lần mua thay cho 20 (chia đợt hàng) và 30 (giá thấp nhất, giá trung bình), nên lần mua cũ không còn bị bỏ qua.
- Chi tiết món vẫn chỉ hiện các đợt còn hàng và 20 đợt mới nhất.
- Giỏ nhiều món (Kế hoạch, so giá cửa hàng) giữ 30 lần mua gần nhất mỗi món để mở nhanh.

### Cách kiểm tra
- `go vet ./...`, `go test ./...` qua. Test mới: `TestItemReadsFullHistory`, `TestVisibleLots`.

---

## 2026-11-02 · Hàng đợi offline và dữ liệu trong máy gắn với tài khoản

- Mỗi thao tác chờ gửi ghi kèm uid. Người khác đăng nhập trên cùng máy thì không thấy và không gửi thao tác của người trước; người trước đăng nhập lại thì gửi tiếp. (vấn đề 63, 65)
- Đăng nhập bằng tài khoản khác với tài khoản đang giữ dữ liệu trong máy (người trước bị tự đăng xuất vì hết phiên): xóa cache API và nháp của người trước trước khi dùng.
- Đăng xuất chủ động chỉ xóa thao tác chờ gửi của chính mình.
- Xả hàng đợi đọc lại kho mỗi vòng, nên thao tác thêm vào trong lúc đang gửi không bị mất.

### Cách kiểm tra
- `tsc -b`, `npm run build` qua. Chạy thử logic hàng đợi bằng kho giả trong bộ nhớ: đổi A → B → A, đúng người gửi đúng thao tác.

---

## 2026-11-02 · Sửa 6 lỗ hổng từ bảng 100 vấn đề triển khai

- Đăng xuất xóa sạch dữ liệu của người dùng trong máy: cache API, hàng đợi ghi và mọi khóa `mimi.*` trong localStorage (nháp kế hoạch, vòng đi mua hộ...). Chỉ giữ ngôn ngữ giao diện. (vấn đề 65)
- App tự cắt request sau 20 giây (ảnh hóa đơn 65 giây), báo "Mạng chậm quá" thay vì quay mãi; lỗi hết giờ của GET được tự thử lại. (vấn đề 13, 90)
- Thư tóm tắt gửi theo giờ người dùng chọn trong Cài đặt; gửi lỗi tạm thời thì thử lại sau 15 phút, tối đa 3 lần. (vấn đề 52, 58)
- Mỗi giỏ và đơn PayPal ghi kèm nguồn và ngày của tỷ giá (`rate_source`, `rate_as_of`); `PAYPAL_MODE=live` bắt buộc khai báo tỷ giá. (vấn đề 44)
- GitHub Actions: `go vet`, `go test -race`, build frontend, quét bí mật bằng gitleaks. (vấn đề 71, 95)
- Chống gửi trùng cho mọi thao tác ghi nằm ở PR #6, cần gộp riêng.

### Cách kiểm tra
- `go vet ./...`, `go test ./...`, `npm run build` đều qua. Có test mới cho giờ gửi thư, gửi lại khi lỗi và thông tin tỷ giá.

---

## 2026-10-31 · Mimi 1.5: màn 0–6 theo ảnh mẫu và các góp ý sau Cổng 3

- PR #2 (đã gộp, `8ecf6a8`): màn 0–6 theo ảnh mẫu đã duyệt.
- PR #3 (chờ gộp): prototype một file, Giỏ mua sắm, bảng "Thêm vào kế hoạch nào?", xóa kế hoạch, sửa dòng món bằng dấu tích và thùng rác, Lịch sử mua hàng đủ mọi hóa đơn.
- Mô tả app đúng như đang chạy: `notes/mimi-1-5-tong-the.md` (Project files). Mục 11 của file này ghi từng góp ý và thay đổi theo giờ.
- Bản đồ màn → file: `notes/ban-do-frontend.md` (Project files).

### Cách kiểm tra
- `go test ./...` và `npm run build` đều qua.
- Chạy thử từng màn bằng trình duyệt tự động trên dữ liệu mẫu, không có lỗi console. Ảnh nằm ở `assets/mimi15-man-0-4`, `man-5`, `man-6`.

### Còn lại
- Bước 6: 4 việc P1 (`notes/buoc-6-de-xuat.md`).
- Bước 8: 3 thay đổi dữ liệu (`notes/data-model-1-5.md`).

---

## 2026-10-30 · Mimi 1.0: bù các chỗ còn thiếu sau khi đối chiếu lại 10 file spec

- Nhánh: `feat/mimi1-specs` → `feat/mimi2-core-demo`
- PR: https://github.com/qlinh142/Mimi-Test/pull/1
- Commit: `647e216` (màn xác nhận hóa đơn), `d9593f0` (Kế hoạch), `f0bff5d` (chi tiết món, test), `dcc1034` (ngưỡng To Buy)
- Báo cáo đối chiếu: `doi-chieu-spec-mimi1.md` (trong Project files)

### Theo từng file spec

| Spec | Đã bổ sung |
|---|---|
| 6.1 BUG-06 Lưu hóa đơn | Nút "Chọn nhóm" chạy được; sửa tên / số lượng / giá, bỏ hoặc giữ lại từng dòng; tổng tính lại sau khi sửa; lỗi khi lưu tô đúng ô ngày hoặc đúng dòng, lỗi máy chủ giữ nguyên phần đã sửa |
| 7.1 Tên sản phẩm | Màn xác nhận hiện 2 dòng: "Brand Tên riêng" rồi "Nhóm · lượng" |
| 5.1 BUG-05 Nhiều ảnh | Nút "Chụp lại ảnh N" thay đúng vị trí ảnh đó; tối đa 10 ảnh |
| 0.2 Taxonomy | Cờ `active` cho nhóm (nhóm ngừng dùng không hiện trong gợi ý); danh mục chi tiêu lấy theo nhóm khi AI không trả |
| 3.1 Kế hoạch | Mục "Check tồn" có nút + thêm nhanh; "Thêm vào Kế hoạch" ở chi tiết món, hỏi chọn Kế hoạch đích khi có nhiều Kế hoạch; đổi tên Kế hoạch; giờ mua dạng 09:30; trạng thái planning / shopping / reconciled; cảnh báo "Còn nhiều" cho món tự thêm; ngưỡng To Buy theo số ngày "báo trước" trong Cài đặt; người được nhờ thấy tên, giờ mua, ghi chú của Kế hoạch |
| 0.1 Sản phẩm & tồn kho | Chi tiết món: sửa tên chuẩn hóa, đổi nhóm khi AI phân loại sai (`PATCH /items/{key}`; lịch sử mua, tồn, dự đoán chuyển theo, gộp nếu nhóm đích đã có món); test bộ 100 ca mục 19 đối chiếu với taxonomy 0.2 |

### Cách kiểm tra
- `go vet ./...` và `go test ./...` (kèm Firestore emulator) đều qua; `npm run build` qua.
- Chạy thử bản dữ liệu mẫu bằng trình duyệt tự động: màn xác nhận, Kế hoạch, chi tiết món, không có lỗi console.

### Còn lại
- Tách `display_group_key` / `consumption_group_key` (0.1 mục 18): chính spec ghi là điểm còn mở (mục 23), chưa làm. Hiện các biến thể cùng nhóm (Sure Diacerna, Sure Prevent) vẫn chung một dòng tồn.

---

## 2026-10-30 · Mimi 1.0: bổ sung nội dung theo 10 file spec

- Nhánh: `feat/mimi1-specs` → `feat/mimi2-core-demo`
- PR: https://github.com/qlinh142/Mimi-Test/pull/1
- Commit: `7153d59` (backend), `e825122` (frontend)

### Theo từng file spec

| Spec | Nội dung | Vị trí chính |
|---|---|---|
| 0.1 Product & Inventory | Quy cách đóng gói (2 x 900g, combo), đơn vị chuẩn g / ml / m / item, purchase lot chia FIFO, tồn theo từng chiều đo (không cộng g với ml), thao tác set / increase / decrease / mark_out | `backend/internal/domain/packaging.go`, `lots.go`, `quantity.go`; `internal/item/service.go` |
| 0.2 Taxonomy | Bộ 300 nhóm sản phẩm, 20 nhóm cha, nhãn vi / en, chiều đo, đơn vị; từ điển thương hiệu; `GET /taxonomy` | `backend/internal/taxonomy/` |
| 1.1 Đọc hóa đơn | Chuẩn hóa tên một lần (AI không dịch tên), mã lỗi theo spec kèm `retryable`, lỗi chỉ đúng ảnh cần chụp lại, nhớ kết quả từng ảnh để chụp lại một ảnh không đọc lại cả hóa đơn | `internal/receipt/service.go`, `internal/gcp/gemini/reader.go` |
| 6.1 / 7.1 Tên sản phẩm | Brand + tên + quy cách, viết hoa chuẩn, bỏ chữ quảng cáo; dòng thiếu nhóm vẫn lưu nhưng `needs_review`, không vào tồn kho / dự đoán | `internal/domain/productname.go` |
| Lưu hóa đơn | Một transaction: tồn kho theo người nhận hàng, chi tiêu theo người trả, đối chiếu đúng Kế hoạch, đóng / trả món của vòng mua hộ, khớp khoản định kỳ (không cộng hai lần) | `internal/gcp/fsstore/save.go` |
| 2.1 Định kỳ & Dự báo | Khoản định kỳ, Đã trả (cộng chi tiêu), Bỏ qua kỳ, tự tạo kỳ sau, `GET /forecast` 30 ngày | `internal/recurring/`, `internal/gcp/fsstore/recurring.go` |
| 3.1 Kế hoạch | Nhiều Kế hoạch (`/plans`), `?plan_id=` cho mọi route `/plan`, thêm món từ taxonomy hoặc tự nhập, dòng đang nhờ mua bị khóa | `internal/plan/plans.go`, `internal/gcp/fsstore/plan.go` |
| 4.1 Mua hộ | Nhà: mã mời 48 giờ, vào, rời, mời ra; gửi món cho người nhà mua, hủy, `GET /plan/delegated`. Mỗi thành viên tự quản lý dữ liệu của mình | `internal/household/`, `internal/gcp/fsstore/household.go`, `internal/authz/` |
| 8.1 Lịch sử giá | Đơn giá chuẩn hóa (đ / ml, đ / g...), cửa sổ 90 ngày, giá gần nhất / thấp nhất / trung bình, ước tính giỏ theo độ phủ từng nơi và phương án chia nơi | `internal/pricing/` |

### Frontend
- Kế hoạch: chọn / tạo Kế hoạch, thêm món mới, nhờ người nhà mua, "Được nhờ mua", ước tính theo lịch sử giá.
- Sổ chi tiêu: Dự báo 30 ngày, Khoản định kỳ.
- Cài đặt: Nhà mình (thành viên, mã mời, vào / rời nhà).
- Chi tiết món: các lần mua còn lại (lot), lịch sử giá, Thêm / Dùng / Đánh dấu hết.
- Quét: chỉ ra ảnh cần chụp lại, dòng cần chọn nhóm, chọn hóa đơn cho lời nhờ mua.
- Mock (`VITE_API_MODE=mock`) có đủ các API mới.

### API, dữ liệu
- `backend/api/openapi.yaml`: thêm các route và schema mới.
- `backend/firestore.indexes.json`: index cho `recurring_occurrences`, `shopping_assignments`; TTL cho `page_cache`.

### Kiểm tra
- `go vet ./...`, `go test ./...` qua, gồm test với Firestore emulator (lưu hóa đơn cá nhân và mua hộ, khoản định kỳ, luồng nhà và mua hộ).
- `npm run typecheck`, `npm run build` qua; chạy thử 4 màn ở chế độ mock không lỗi.
- Sửa 2 test hỏng sẵn trên nhánh gốc: route `/health`, thứ tự kiểm tra ủy quyền khi đặt đơn online.

### Chưa làm / cần lưu ý
- Chưa chạy với Gemini thật và Firestore đã deploy; cần deploy index mới trước khi dùng.
- Đổi quyền: thành viên trong nhà giờ có hóa đơn, tồn kho, kế hoạch, chi tiêu riêng (trước chỉ chủ nhà).
- `POST /recurring/scan`, `/households/asks` vẫn trả 501 như trước.
