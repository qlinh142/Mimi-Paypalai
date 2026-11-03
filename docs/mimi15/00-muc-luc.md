# Mục lục tài liệu Mimi: file nào cần cập nhật khi sửa

Ngày: 31/10/2026 17:15, cập nhật 01/11/2026 05:15. Memory của dự án đang tắt, nên **các file dưới đây chính là trí nhớ của dự án**. Khi sửa app, Mimi cập nhật đúng các file ở **Bảng 1**. Các file ở Bảng 2 chỉ để tra lại lịch sử, không sửa nữa.

## Bảng 1 · File sống (sửa app thì sửa theo)

| File | Chứa gì | Cập nhật khi nào |
|---|---|---|
| `notes/mimi-1-5-tong-the.md` | **Spec 1.5:** app đúng như đang chạy, từng màn. Mục 11 là bảng góp ý theo giờ | Mỗi lần sửa một màn: sửa mô tả màn đó và thêm một dòng ở mục 11 |
| `notes/ban-do-frontend.md` | Màn nào nằm ở file code nào, file sẽ đổi theo Spec 1.5 (mục 2b), danh sách mốc, quy trình sửa | Thêm màn mới hoặc có mốc mới |
| `notes/data-model-1-5.md` | Dữ liệu lưu trên máy chủ, 13 thay đổi cần làm | Đổi dữ liệu hoặc thêm ô mới |
| `notes/spec-1-5-viec-moi.md` | **Spec 1.5 phần việc mới:** app sẽ chạy thế nào sau 4 việc P1 và luật 7 câu, kèm các câu kiểm tra | Đổi luật của việc mới. Làm xong một phần thì chuyển mô tả sang `mimi-1-5-tong-the.md` |
| `notes/buoc-6-de-xuat.md` | Danh sách tính năng, ưu tiên P1–P3, 4 việc và 7 câu đã chốt (01/11 05:03), giải thích kèm ví dụ | Thêm hoặc bớt tính năng, chốt câu hỏi |
| `notes/pham-vi-luong.md` | Phần nào LÀM, phần nào TREO | Đổi phạm vi |
| `assets/quy-trinh/checklist-mimi.html` (+ `.png`) | Tiến độ 13 bước | Xong một việc |
| `assets/mimi15-man-*/` | Ảnh chạy thử từng màn | Sửa giao diện màn đó |
| `prototype/mimi-prototype.html` + link prototype | Bản bấm thử | Mỗi lần sửa giao diện |
| `exports/frontend/mimi-frontend-1-5.zip` | Bản nén code frontend | Sau mỗi lần gộp PR |
| `notes/loi-trien-khai.md` | **Sổ lỗi triển khai:** lỗ hổng đối chiếu theo bảng 100 vấn đề, đã sửa ở PR nào, còn mở gì | Sửa hoặc tìm thấy một lỗi triển khai |
| `notes/CHANGELOG.md` (bản sao của `CHANGELOG.md` trong repo) | Lịch sử theo đợt, mỗi PR một mục | Mỗi PR |
| `notes/kich-ban-demo.md` | Kịch bản demo đã chốt | Đổi kịch bản |
| `notes/luat-gia.md` (bản sao của `docs/mimi15/luat-gia.md`) | **Luật giá:** hai loại giá, chọn giá hiện, theo nước, sổ nguồn, job đêm, nút Cập nhật giá, lịch sử thay đổi | Đổi bất kỳ luật nào về giá |
| `notes/huong-dan-chay-demo.md` | Cách chạy app với dữ liệu mẫu | Đổi cách chạy |
| Repo GitHub `qlinh142/Mimi-Test` | Code, lịch sử commit, `backend/api/openapi.yaml` (API) | Mọi thay đổi code |

## Bảng 2 · File lịch sử (giữ lại, không sửa)

| File | Vì sao giữ |
|---|---|
| `man-hinh-0-4-cong-1.md`, `man-hinh-5-cong-1.md`, `man-hinh-6-cong-1.md`, `gio-mua-sam-cong-1.md` | Bản kể bằng lời trước khi code, kèm câu hỏi và câu trả lời của bạn. Nội dung đã gộp vào `mimi-1-5-tong-the.md` |
| `danh-sach-tinh-nang.md` | Danh sách 46 dòng cũ theo spec 2.0. Đã thay bằng `buoc-6-de-xuat.md` |
| `tien-do-mimi.md` | Bảng tiến độ cũ. Đã thay bằng checklist HTML |
| `kiem-tra-spec-v2-cho-prototype.md` | 12 lỗ hổng gốc của spec 2.0 |
| `khung-co-dinh.md`, `quy-trinh-tinh-chinh-mimi.md`, `huong-dan-hoan-thien-mimi2-theo-quy-trinh.md`, `buoc-1-y-tuong-va-yeu-cau.md` | Các bước đầu: khung, quy trình, ý tưởng |
| `ra-soat-mimi1-buoc0.md`, `doi-chieu-spec-mimi1.md`, `mimi1-tong-hop-vs-code.md`, `mimi2-spec-vs-code.md`, `so-sanh-mimi2-spec-v1-v2.md` | Rà soát code và so sánh spec. Mã lỗi N1–N4, V1–V5 lấy từ đây |
| `merged-md/mimi-tong-hop.md`, `mimi2/` | Spec gốc 1.0 và 2.0 |
| `uploads/hearth/` (file bạn tải lên): 0.1 Sản phẩm & tồn kho, 0.2 Taxonomy 300 nhóm, 1.1 Chuẩn hóa tên & đa ngôn ngữ, 7.1 Cách viết tên sản phẩm, 8.1 Lịch sử giá… | Luật gốc về tên sản phẩm và nhóm. Code đã làm theo (`backend/internal/taxonomy`, đối chiếu ở `doi-chieu-spec-mimi1.md`). Sửa luật thì sửa cả file này và code |
| `assets/mau-thiet-ke/` | Ảnh mẫu bạn gửi, dùng để đối chiếu. Mới: `4c-4d-lich-su-chi-tiet-hoa-don.png`, `viec-3-di-mua-ho-chot.png` (bản chốt; `viec-3-di-mua-ho.png`, `-v2.png` là bản nháp), `5c-chon-thanh-vien-hien-tai.png` |

## Còn thiếu (sẽ làm khi tới bước đó)

- **Bản đồ backend** (gói Go nào lo tính năng nào): làm khi bắt đầu sửa backend ở bước 10b.
- **Bản sao trong repo:** các file md này mới nằm ở thư mục dự án, chưa đưa vào repo. Sẽ đưa vào `docs/` cùng PR sửa prototype.
