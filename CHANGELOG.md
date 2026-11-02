# Lịch sử cập nhật

Mỗi lần cập nhật ghi: ngày, nhánh / PR, phần đã làm theo từng file spec, cách kiểm tra, điều còn thiếu.
Mới nhất ở trên cùng.

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
