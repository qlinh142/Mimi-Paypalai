# Bản đồ frontend Mimi 1.5 (để sửa từng phần, không làm lại toàn bộ)

Ngày: 31/10/2026 17:10. Cập nhật 01/11/2026 05:30: mục 2b ghi các file đã sửa theo Spec 1.5.

## 1. Code và bản lưu nằm ở đâu

| Thứ | Ở đâu | Dùng để |
|---|---|---|
| Code gốc, đủ lịch sử | GitHub `qlinh142/Mimi-Test`, thư mục `frontend/` | Nơi chính thức. Mỗi lần sửa là một commit, quay lại bản cũ được bất cứ lúc nào |
| Bản nén code frontend | `exports/frontend/mimi-frontend-1-5.zip` | Bản chụp main lúc 01/11 08:09, sau khi gộp PR #3, #4 và #5 |
| Prototype một file | `prototype/mimi-prototype.html` · link https://claude.ai/artifact/TNXorGatmm8XEv8LbEhvGu | Bấm thử, gửi người khác xem |
| Mô tả app đúng như đang chạy | `notes/mimi-1-5-tong-the.md` | Bản spec 1.5. Mục 11 là bảng mọi góp ý và thay đổi theo giờ |
| Ảnh chạy thử từng màn | `assets/mimi15-man-0-4/`, `mimi15-man-5/`, `mimi15-man-6/` | Đối chiếu trước và sau khi sửa |

## 2. Mỗi màn nằm ở file nào

Đường dẫn tính từ `frontend/src/`. Khi mẹ góp ý một màn, Mimi chỉ sửa file của màn đó và phần chữ của nó.

| Màn | File chính | Phần dùng chung | Mục trong `mimi-1-5-tong-the.md` |
|---|---|---|---|
| 0 · Vào app, đăng nhập | `screens/Welcome.tsx`, `lib/auth.tsx` | | 2 |
| 1 · Home | `screens/Home.tsx` | `components/MimiMark.tsx` | 3 |
| 2 · Quét, xác nhận hóa đơn | `screens/Scan.tsx` | `components/ReceiptLines.tsx` | 4 |
| 3a · Hàng tiêu dùng | `screens/Items.tsx` | `components/ItemRow.tsx`, `Group15.tsx`, `CartButton.tsx` | 5 · 3a |
| 3b · Chi tiết sản phẩm | `screens/ItemDetail.tsx` | `components/ItemExtras.tsx`, `lib/displayUnit.ts` | 5 · 3b |
| 3c · Khoản chi định kỳ | `components/RecurringList.tsx`, `Recurring.tsx` | | 5 · 3c |
| 4 · Sổ chi tiêu (Đã chi, Sắp chi, tóm tắt lịch sử) | `screens/Spending.tsx` | | 6 |
| 5a · Danh sách kế hoạch, Được nhờ mua | `screens/PlanList.tsx` | | 7 · 5a |
| 5b · Chi tiết kế hoạch, sửa món, xóa | `screens/PlanDetail.tsx` | `components/PlanExtras.tsx`, `Sheet15.tsx` | 7 · 5b |
| 5b' · Chọn biểu tượng | `screens/PlanDetail.tsx` | `lib/planIcon.ts` | 7 · 5b' |
| 5c · Gửi cho thành viên | `screens/PlanDetail.tsx` | `lib/relation.ts` | 7 · 5c |
| 5e · Thêm vào kế hoạch nào? | `components/PlanPicker.tsx` | | 7 · 5e |
| 6 · Cài đặt và trang con | `screens/Settings.tsx` | `components/Household.tsx`, `lib/relation.ts` | 8 |
| Thông báo (chuông) | `screens/Notifications.tsx` | | |
| Khung chung: đầu màn, thanh tab | `components/AppShell.tsx`, `app/App.tsx` | | 1 |

**Phần dùng chung cho mọi màn:**
- **Chữ hiển thị (tiếng Việt / English):** `i18n/m15.ts` (màn 1.5), `i18n/m1.ts`, `vi.ts`, `en.ts`. Đổi chữ thì chỉ sửa ở đây.
- **Màu, cỡ chữ, bố cục:** `styles/m15.css` (màn 1.5), `tokens.css` (màu chung), `base.css`, `screens.css`.
- **Lấy dữ liệu từ máy chủ:** `api/hooks.ts`, `api/client.ts`. Kiểu dữ liệu nằm ở `api/types.ts`.
- **Dữ liệu mẫu cho prototype:** `api/mock.ts`.

## 2b. Màn và file theo Spec 1.5 (đã làm trên prototype 01/11)

Luật chi tiết ở `notes/spec-1-5-viec-moi.md`. Ảnh chạy thử ở `assets/mimi15-spec-moi/`.

| Phần trong spec | Màn | File | Ghi chú |
|---|---|---|---|
| 1 · Chọn người mua, nút máy bay giấy, "Chưa hoàn thành" | 5b, 5c, 5a | `screens/PlanDetail.tsx`, `screens/PlanList.tsx`, **`lib/planDraft.ts`** (mới) | Người mua đã chọn và trạng thái nháp tạm lưu trên máy |
| 2 · Đi mua hộ: danh sách, quét, đã ghi nhận, Từ chối | `/plan/run/:id` | **`screens/ShoppingRun.tsx`** (mới), `screens/PlanList.tsx` (thẻ bấm được, số đỏ) | Danh sách đang mua lưu tạm trên máy; quét bill trên prototype là hóa đơn mẫu |
| 2 · Mẹ xem tiến độ, nhãn Đã mua, ghi chú | 5b | `screens/PlanDetail.tsx` (`Progress`) | |
| 3 · Mỗi món ghi "→ [kế hoạch]" | 2 Xác nhận | `screens/Scan.tsx`, `components/ReceiptLines.tsx` (`extra`), **`lib/allocate.ts`** (mới) | Máy chủ vẫn nhận một `plan_id`; chia theo dòng là việc backend (data model dòng 7) |
| 4 · Lịch sử mua hàng, Chi tiết hóa đơn | `/spending/history`, `/spending/receipt/:id` | **`screens/PurchaseHistory.tsx`** (mới, gồm 4c và 4d), `screens/Spending.tsx` (`HistorySummary`) | API `/receipts/history`, `/receipts/{id}` mới chỉ có ở dữ liệu mẫu |
| 5, 6, 7 · "· còn lại", hủy, "Chưa có người mua" | Đã ghi nhận, 5a, 5b | `screens/ShoppingRun.tsx`, `screens/PlanList.tsx`, `screens/PlanDetail.tsx` | |
| 8 · Hộp thư đi, sửa N3, "Đang chờ đồng bộ (N)", chấm đỏ | Mọi màn, 6 | `lib/offlineQueue.ts` (`useQueue`, `label`), `api/client.ts` (`apiSendQueued`), `screens/Settings.tsx` (`SyncBox`), `components/AppShell.tsx` | Nhắc sau 3 tiếng và thông báo đẩy chưa làm (cần backend, bước 10b) |
| 8 · Báo mất mạng khi bấm quét | 2, đi mua hộ | `screens/Scan.tsx`, `screens/ShoppingRun.tsx` | |
| Chữ mới | Mọi màn | `i18n/m15.ts` (`run`, `history`, `scanPlan`, `sync`) | |
| Dữ liệu mẫu | | `api/mock.ts` | Lời nhờ mẫu 5 món, lần nhờ Minh đang dở, 11 hóa đơn mẫu 4 tháng |

## 3. Các mốc đã qua (lịch sử commit)

Mỗi dòng là một bước sửa. Quay lại bất kỳ mốc nào được, không mất các mốc khác.

**Đã gộp vào main (PR #2, 31/10):**
- `a053c57` Màn 0–4 theo ảnh mẫu.
- `bb22317` Đồ dùng: thu gọn nhóm, sửa tên và nhóm trong chi tiết món.
- `a89213e` Định kỳ: bỏ chữ "Hằng tháng".
- `0e88daa` Sổ chi tiêu: định kỳ đứng đầu.
- `704ecf9` Màn 5 Kế hoạch.
- `d19d067` Kế hoạch: nút quay lại cùng hàng tên.
- `cefa24d` Kế hoạch: chữ tiếng Việt.
- `c3167e9` Màn 6 Cài đặt.

**Đã gộp vào main (PR #3, 01/11 06:17):**
- `69fd1a7` Prototype một file.
- `40b55f4` Nút Xóa kế hoạch.
- `9962c9f` Đổi tên thành Giỏ mua sắm.
- `34e7c97` Bảng chọn kế hoạch.
- `e445a2e` Xóa được Giỏ mua sắm.
- `68cf2ea` Kiểm tra đồ dùng mở tab Đồ dùng.
- `3b675e1` Nút + Thêm nhỏ.
- `a04476c` Dấu tích và thùng rác.
- `668b0b0` Lịch sử đủ hóa đơn.

**Đã gộp vào main (PR #4, 01/11 06:17) · Prototype theo Spec 1.5:**
- `84426eb` Chọn người mua, nút máy bay giấy, Chưa hoàn thành.
- `2f3f327` Màn đi mua hộ, mẹ xem tiến độ.
- `2c0cfd0` Hóa đơn ghi → kế hoạch, quét lúc mất mạng.
- `36f4531` Lịch sử mua hàng 4c, Chi tiết hóa đơn 4d.
- `00d9184` Hộp thư đi, Đang chờ đồng bộ, sửa N3, câu 3–5.

## 4. Quy trình khi mẹ muốn sửa

1. Mẹ nói màn nào, chỗ nào (có ảnh càng tốt).
2. Nếu sửa lớn thì qua **Cổng 1**: Mimi kể lại bằng lời. Sửa nhỏ thì làm luôn.
3. Mimi chỉ sửa đúng các file của màn đó (bảng mục 2), commit thành **một bước riêng**.
4. Mimi chụp ảnh màn trước và sau, cập nhật prototype.
5. Mimi ghi một dòng vào `mimi-1-5-tong-the.md` (mục 11), rồi sửa mô tả màn đó trong cùng file.
6. Gom vào PR. Mẹ nói "gộp" thì vào main.

Nếu một bước sửa không ưng ý, Mimi bỏ riêng bước đó (`git revert`). Các bước khác giữ nguyên.

## 2c. Phần E (góp ý 01/11, PR #5, đã gộp 08:09)

| Việc | File |
|---|---|
| Màn chi tiết khoản chi định kỳ (`/items/recurring/:id`, `/items/recurring/new`) | `screens/RecurringDetail.tsx` (mới), `components/RecurringList.tsx`, `app/App.tsx` |
| Dòng Khoản chi định kỳ ở Sổ chi tiêu sang Đồ dùng | `screens/Spending.tsx`, `styles/m15.css` (`.bars15__link`) |
| Lịch sử mua hàng có khoản định kỳ đã trả | `screens/PurchaseHistory.tsx`, `api/mock.ts` (`/receipts/history`), `api/types.ts` (`kind: "recurring"`) |
| Ô nhóm gõ có gợi ý (5 nhóm chuẩn) | `components/GroupInput.tsx` (mới), dùng ở `screens/ItemDetail.tsx` và `components/ReceiptLines.tsx` |
| Chữ | `i18n/m15.ts` (`recd`, `groupInput`, `history.recurringTag`) |
