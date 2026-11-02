# Mimi 1.5 · Data model (bước 8)

Ngày: 31/10/2026 17:10. Đối chiếu prototype đã duyệt (`notes/mimi-1-5-tong-the.md`) với dữ liệu backend đang lưu trên Firestore (`backend/internal/gcp/fsstore`).

**Kết luận:** backend đã có gần đủ dữ liệu cho mọi màn. Cần thêm **13 thay đổi**: 3 thay đổi từ prototype (dòng 1–3) và 10 thay đổi từ các quyết định ở `buoc-6-de-xuat.md` ngày 01/11 (dòng 4–13). Cập nhật 01/11 05:15. Luật chi tiết ở `spec-1-5-viec-moi.md`.

## A. Dữ liệu đang lưu

Dữ liệu chia theo **từng người** (`users/{uid}/…`) và theo **cả nhà** (`households/{hid}/…`).

| Nhóm dữ liệu | Lưu ở | Chứa gì | Màn dùng |
|---|---|---|---|
| users | `users/{uid}` | tên Mimi gọi, ngôn ngữ, múi giờ, giờ nhắc | 6 Cài đặt |
| households | `households/{hid}` | tên nhà, chủ nhà, thành viên (tên, vai trò chủ nhà / thành viên), mã mời 48 giờ | 6b Thành viên |
| receipts | `users/{uid}/receipts` | hóa đơn đã lưu: cửa hàng, ngày, tổng tiền, các dòng, kế hoạch đã đối chiếu | 2 Quét, 4 Lịch sử, Địa điểm mua hàng |
| drafts, page_cache | `users/{uid}/…` | hóa đơn đang quét dở, kết quả đọc ảnh | 2 Quét |
| purchases | `users/{uid}/purchases` | **từng lần mua** của từng món: ngày, số lượng, giá, cửa hàng | 3b Lịch sử mua hàng, giá, dự đoán |
| items | `users/{uid}/items` | món đồ dùng: tên, nhóm, tồn kho, dự đoán ngày hết | 3 Đồ dùng, 1 Home |
| months | `users/{uid}/months` | tổng chi theo tháng và theo nhóm | 4 Sổ chi tiêu |
| recurring, recurring_occurrences | `users/{uid}/…` | khoản chi định kỳ và từng kỳ (chờ / đã trả / trả qua hóa đơn / bỏ qua) | 3c, 4b Sắp chi |
| plans | `users/{uid}/plans` | kế hoạch: tên, ngày giờ, nơi mua, ngày đã đối chiếu hóa đơn | 5 Kế hoạch |
| plans/{id}/items, plan_items | `users/{uid}/…` | món trong từng kế hoạch (Giỏ mua sắm dùng `plan_items`) | 5b |
| shopping_assignments | `households/{hid}/…` | việc nhờ mua: người gửi, người mua, kế hoạch, **trạng thái từng món** | 5c, Được nhờ mua |
| asks, digests | | xin mua, email tóm tắt | Thông báo |
| orders, payments, fx_rates | | mua online, PayPal | **Treo** |

## B. Các thay đổi cần làm

| # | Thay đổi | Vì sao | Màn | Ưu tiên |
|---|---|---|---|---|
| 1 | **Lưu lot (từng đợt hàng còn trong nhà) thành nhóm riêng** `users/{uid}/lots`. Mỗi lần lưu hóa đơn thì thêm lot; mỗi lần dùng hoặc giảm số lượng thì trừ theo thứ tự mua trước dùng trước. | Hiện lot được tính lại mỗi lần mở món, chỉ từ 20 lần mua gần nhất, nên lot cũ bị mất (lỗi V3, việc P1 số 2) | 3 Đồ dùng, 3b | P1 |
| 2 | **Kế hoạch thêm ô `icon`** (bag, family, kid, gift, home, food) | Hiện chỉ lưu trên máy (câu 6) | 5a, 5b' | P2 |
| 3 | **Tên gọi người nhà là riêng từng tài khoản** (chốt 01/11 04:20): không lưu chung ở thành viên. Lưu trong tài khoản người đặt (chốt 04:22), ví dụ `users/{uid}/contacts/{uid_khác}.relation`. Hệ thống làm việc bằng mã tài khoản (uid) | Mỗi người tự đặt tên gọi cho người khác | 6c, 5c | P2 |
| 4 | **Kế hoạch đã gửi có số phiên bản**: mỗi lần bấm Lưu (máy bay giấy) thì lưu một bản chụp kèm giờ gửi | Kế hoạch đã gửi là cố định, cập nhật tuần tự; bản Minh chưa nhận trước khi Mua hàng xong thì tách thành kế hoạch mới (việc 4) | 5b, 5c, Được nhờ mua | P1 |
| 5 | ✅ **Lưu `Idempotency-Key` ở máy chủ cho mọi thao tác ghi**: `users/{uid}/idempotency/{key}`, TTL 24 giờ (đã làm, PR #6, 02/11) | Chống gửi trùng khi mạng đứt (việc 4 mục 6) | Mọi màn | P1 |
| 6 | **Hóa đơn chia theo từng món cho nhiều người**: món trong kế hoạch vào Đồ dùng và Sổ chi tiêu của người nhờ, món "Của tôi" vào của người mua; giảm giá chung chia theo tỉ lệ | Hiện cả bill cộng vào sổ của người quét (việc 3) | 2, 4, Được nhờ mua | P1 |
| 7 | **Hóa đơn trừ vào nhiều kế hoạch**: mỗi dòng hóa đơn ghi `plan_id` riêng, trừ dần theo kế hoạch có ngày sớm nhất | Hiện một hóa đơn chỉ gắn một kế hoạch (việc 1) | 2, 4d, 5 | P1 |
| 8 | **Chi tiêu ước tính** khi mua thủ công: theo giá lần trước, hoặc 0đ kèm nhắc nhập giá | Việc 3, nhập thủ công | 4, Được nhờ mua | P1 |
| 9 | **Món trong kế hoạch lưu thêm** tên sửa tay (món chưa chuẩn hóa), đơn giá, dấu tick, ghi chú người đi mua; **kế hoạch lưu người mua đã chọn** (bắt buộc trước khi gửi). Trạng thái "Chưa hoàn thành" chỉ nằm trên máy, chưa lên máy chủ | Việc 3; chọn người mua không gửi ngay (chốt 04:36, 05:02) | 5b, Được nhờ mua | P1 |
| 10 | **Trạng thái "Từ chối" và "Đã hủy"** cho việc nhờ mua (`shopping_assignments`) | Người được nhờ từ chối (câu 5); mẹ hủy khi Minh chưa mua xong (câu 4) | Được nhờ mua, 5a | P1 |
| 11 | **Tự tạo kế hoạch mới cho món chưa mua** khi kế hoạch xong, nằm ở danh sách người nhờ | Câu 3 | 5a | P1 |
| 12 | **Lịch sử mua hàng: khoản "X mua hộ · [kế hoạch]"** hiện ở lịch sử người nhờ, không lộ hóa đơn gốc của người mua | Việc 2, 3 (riêng tư) | 4c, 4d | P1 |
| 13 | **Thông báo** cho người nhận: đã gửi / đã cập nhật kế hoạch, kết quả mua hộ, từ chối | Việc 3, 4, câu 5 | Chuông | P1 |

Kèm theo:
- API: thêm `icon` vào `PATCH /plans/{id}`, chỗ lưu tên gọi riêng, và các ô mới ở dòng 4–13.
- Cập nhật `backend/api/openapi.yaml` (bước 9).
- Firestore rules và index cho `lots` (bước 11).

## C. Những gì đã có sẵn, dùng lại được

- **Trạng thái từng món khi nhờ mua:** `shopping_assignments` đã lưu (đã giao / đã mua / trả lại). Cần thêm trạng thái Từ chối (dòng 10).
- **Hóa đơn gắn với kế hoạch:** API lưu hóa đơn đã nhận `plan_id`, nhưng chỉ một kế hoạch cho cả hóa đơn. Cần chuyển xuống từng dòng (dòng 7).

## D. Một điểm ảnh hưởng tới câu 5 (ai được làm gì)

- Kế hoạch đang lưu **riêng theo từng người** (`users/{uid}/plans`). Chỉ người lập mới thấy, sửa và xóa được.
- Người nhà chỉ thấy kế hoạch khi được gửi (bấm Lưu bằng nút máy bay giấy). Họ thấy bản đã gửi gần nhất. Họ sửa được số lượng, giá, tick trong danh sách đi mua của mình, nhưng không sửa kế hoạch gốc. Người được nhờ có quyền từ chối (câu 5, chốt 01/11 04:18).
- Như vậy câu 5 ("kế hoạch của ai thì người đó sửa, xóa, gửi") **khớp với cách lưu hiện tại**.
- Nếu mẹ muốn cả nhà cùng sửa chung một kế hoạch, thì phải chuyển kế hoạch sang lưu theo cả nhà. Thay đổi đó lớn hơn nhiều, Mimi đề xuất để bản 2.0.

## E. Không đổi trong bản này

- `consumption_key` cho các biến thể của cùng một món (lỗi V4): P2, làm sau.
- Các nhóm dữ liệu của phần Treo (orders, payments, Mandate): giữ nguyên.
