# Lịch sử cập nhật

Mỗi lần cập nhật ghi: ngày, nhánh / PR, phần đã làm theo từng file spec, cách kiểm tra, điều còn thiếu.
Mới nhất ở trên cùng.

---

## 2026-10-30 · Mimi 1.0: bù các chỗ còn thiếu sau khi đối chiếu lại 10 file spec

- Nhánh: `feat/mimi1-specs` → `feat/mimi2-core-demo`
- PR: https://github.com/qlinh142/Mimi-Test/pull/1
- Commit: `647e216` (màn xác nhận hóa đơn), `d9593f0` (Kế hoạch), `f0bff5d` (chi tiết món, test)
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
