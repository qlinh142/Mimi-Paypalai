# Bộ Postman cho API Mimi

Bộ này sinh từ `backend/api/openapi.yaml`. Sửa API thì sửa file yaml rồi chạy lại:

```
python3 backend/api/postman/build.py
```

| File | Dùng làm gì |
| --- | --- |
| `mimi.postman_collection.json` | Mọi endpoint, gom theo nhóm, kèm thư mục **Kịch bản thanh toán sandbox** |
| `mimi-live.postman_environment.json` | Gọi app thật `https://mimi-1-5.web.app/api` |
| `mimi-local.postman_environment.json` | Gọi backend chạy ở máy `http://localhost:8080/api` |

## Nhập vào Postman

1. Mở Postman, chọn **Import**, rồi kéo cả 3 file JSON vào.
2. Ở góc trên bên phải, chọn môi trường **Mimi live (mimi-1-5)** hoặc **Mimi local**.

## Lấy token khi gọi app thật

Máy chủ chỉ nhận lệnh có token đăng nhập Firebase. Khi App Check đang bật, lệnh còn phải có token App Check. Cách lấy hai token từ trình duyệt:

1. Mở https://mimi-1-5.web.app và đăng nhập.
2. Bấm F12, mở tab **Network**, gõ `api` vào ô lọc, rồi tải lại trang.
3. Bấm vào một dòng bất kỳ, ví dụ `home`, rồi kéo xuống **Request Headers**:
   - `authorization: Bearer eyJ...`: chép phần sau chữ `Bearer ` vào biến `idToken`.
   - `x-firebase-appcheck: ...`: chép vào biến `appCheckToken`. Nếu không thấy dòng này thì App Check đang tắt, để trống biến này.

Cả hai token hết hạn sau khoảng 1 giờ. Khi Postman báo lỗi 401, lấy lại token theo các bước trên.

## Kịch bản thanh toán sandbox

**PayPal: mua, giao, kiểm hàng, hoàn tiền.** Tài khoản cần có ít nhất một món trong Kế hoạch (thêm trong app, hoặc gọi `PUT /plan/items/{key}` trong nhóm API: plan). Sau đó chạy lần lượt 11 bước. Mỗi bước tự lưu `itemKey`, `orderId` và số tiền hoàn cho bước sau.

- Bước 4 tạo đơn và in link `approveUrl` ra Console (View, rồi Show Postman Console). Mở link đó, đăng nhập tài khoản **người mua PayPal sandbox**, bấm duyệt, rồi chạy bước 5.
- Bước 9 báo thiếu cả món. Máy chủ gợi ý số tiền hoàn, và bước 10 hoàn đúng số đó qua PayPal sandbox.

**Braintree: trả bằng thẻ.** Chạy sau bước 2 của kịch bản PayPal. Bước 3 dùng `fake-valid-nonce`, là thẻ thử của Braintree sandbox. Số thẻ thật không bao giờ đi qua máy chủ Mimi.

Ở bản live, các bước này ghi thật vào sổ chi tiêu của tài khoản đang đăng nhập (bằng tiền sandbox). Nên dùng một tài khoản thử riêng.

## Chạy ở máy, không cần PayPal thật

Bật Firestore emulator và Auth emulator, rồi chạy backend:

```
PAYPAL_MODE=fake BRAINTREE_MODE=fake APP_CHECK_REQUIRED=false \
FIRESTORE_EMULATOR_HOST=localhost:8081 FIREBASE_AUTH_EMULATOR_HOST=localhost:9099 \
GOOGLE_CLOUD_PROJECT=demo-mimi go run ./cmd/mimi serve-api
```

Môi trường **Mimi local** tự đăng nhập Auth emulator để lấy `idToken`, nên không cần dán token. Chế độ `fake` bỏ qua bước duyệt trên PayPal.
