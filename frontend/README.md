# Mimi web (React + Vite, PWA trên Firebase Hosting)

Khung frontend theo ADR-003: PWA tĩnh trên Firebase Hosting, mọi lời gọi đi qua `/api/**`
(Hosting rewrite sang Cloud Run `mimi-api`), không CORS.

## Chạy thử

```bash
npm install
cp .env.example .env      # để VITE_API_MODE=mock: chạy bằng dữ liệu mẫu, không cần Firebase
npm run dev               # http://localhost:5173
```

Ở chế độ mock: nhập email bất kỳ, bấm "Nhận link đăng nhập", rồi bấm "Mở link (bản thử)".

Chạy với backend Go ở máy: `VITE_API_MODE=live`, điền cấu hình Firebase, chạy
`firebase emulators:start --only auth,firestore` và `go run ./cmd/mimi serve-api` (cổng 8080).
Vite proxy `/api` sang 8080.

## Mimi Online (PayPal sandbox)

Kế hoạch → "Để Mimi mua online" → giỏ ở cửa hàng sandbox, kiểm tra Ủy quyền (ALLOW / ASK / BLOCK)
→ thanh toán PayPal → Đơn hàng (mô phỏng giao) → chụp ảnh hàng nhận, xác nhận số lượng → Refund nếu thiếu.

- Có `VITE_PAYPAL_CLIENT_ID` (sandbox): dùng PayPal Buttons thật, backend phải chạy `PAYPAL_MODE=sandbox`
  với cùng ứng dụng PayPal.
- Để trống: nút mô phỏng, dùng với `PAYPAL_MODE=fake` ở backend hoặc với chế độ mock.

Ủy quyền mua online nằm trong Cài đặt.

## Phân tích chi tiêu (AG Studio)

Sổ chi tiêu → "Phân tích chi tiêu" (`/spending/analytics`): bảng phân tích dựng bằng AG Studio 3.0
(cùng AG Grid 36.2 của màn Lịch sử mua hàng), tải riêng khi mở màn. Mã nằm trong `src/analytics/`:

- `data.ts`: gom lịch sử hóa đơn, chi tiết từng hóa đơn (60 hóa đơn gần nhất), đồ dùng và bảng giá
  đang bán của Kế hoạch thành bốn bảng quan hệ cho Studio (Hóa đơn, Dòng hóa đơn, Đồ dùng, Giá đang bán).
- `report.ts`: báo cáo dựng sẵn ba trang Tổng quan, So sánh giá, Hóa đơn. Người dùng chỉnh được
  (nút Chỉnh bảng), bản chỉnh nhớ trên máy, "Về bảng gốc" xóa bản đã nhớ.
- `widgets.tsx`: hai widget riêng của Mimi, Run-out Radar và Tiết kiệm được, lấy dữ liệu qua
  `widgetApi.getData` nên nhận bộ lọc như widget có sẵn.
- `theme.ts`: theme Studio theo màu Mimi (`styles/tokens.css`).
- `askMimi.ts`, `gemini.ts`: agent "Hỏi Mimi" (Studio Agent Framework) với bốn tool riêng, giao việc
  vẽ biểu đồ cho agent widget/page/data có sẵn của Studio. Mọi agent gọi Gemini qua `POST /api/ai/turn`
  của máy chủ (không có khóa ở trình duyệt). Ở chế độ mock, `src/api/mockAi.ts` trả lời mẫu.

Khóa AG Studio: `VITE_AG_STUDIO_LICENSE` (xem `.env.example`). Thiếu khóa thì chỉ hiện watermark.

## Ngôn ngữ giao diện

Mặc định tiếng Anh (bản dự thi). Toàn bộ chữ nằm trong `src/i18n/en.ts` và `src/i18n/vi.ts`;
kiểu `Messages` bảo đảm hai bản luôn đủ khóa như nhau. Xem bản tiếng Việt: thêm `?ui=vi` vào URL
(lựa chọn được nhớ trên máy), quay lại tiếng Anh bằng `?ui=en`.
Công tắc VI/EN ở màn đăng nhập vẫn là ngôn ngữ dịch hóa đơn, không phải ngôn ngữ giao diện.

## Triển khai

```bash
npm run build
firebase deploy --only hosting           # hoặc hosting:channel:deploy pr-123 cho PR
```

`firebase.json` đã có: rewrite `/api/**` → Cloud Run `mimi-api` (asia-southeast1), SPA fallback,
cache `immutable` cho `/assets/**`, `no-cache` cho `index.html` và `sw.js`, header bảo mật.

## Cấu trúc

```
src/
  app/          App (route, cổng đăng nhập), queryClient (cache IndexedDB)
  api/          client (token, App Check, Idempotency-Key, hàng đợi), hooks, types, mock
  lib/          firebase, auth (email link + Google redirect), offlineQueue, image (nén, bỏ EXIF), text (bỏ dấu, VND)
  components/   AppShell (thanh trên, thanh dưới, dải offline, Hoàn tác), ItemRow, MimiMark
  screens/      Welcome, Home, Items, ItemDetail, Scan (+ Xác nhận), Stub
  styles/       tokens.css, base.css, screens.css
```

## Quy ước

- Client không tính tiền hay số ngày. Mọi con số do API trả về; client chỉ định dạng.
- Màu luôn đi kèm chữ. `#FF8A3D` chỉ dùng cho viền và vòng focus; chữ màu cam dùng `--st-soon` (#B4530A) để đủ tương phản.
- Ô nhập tối thiểu 16px (iOS không zoom), vùng chạm tối thiểu 48px.
- Service worker chỉ cache app shell; dữ liệu API nằm trong cache TanStack Query (IndexedDB),
  bị xóa khi đăng xuất và khi xóa dữ liệu.
- Thao tác ghi nhẹ (sửa tồn, tick giỏ) xếp hàng khi mất mạng, xả lại khi app mở hoặc có mạng.
  Lưu hóa đơn luôn gửi trực tiếp, dùng `receipt_id` làm khóa idempotent.
- Kiểu dữ liệu trong `src/api/types.ts` là bản viết tay; khi có `api/openapi.yaml` chạy `npm run gen:api`.