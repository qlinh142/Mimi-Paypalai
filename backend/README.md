# Mimi backend (Go)

Một binary, ba vai trò trên Cloud Run: `serve-api`, `serve-worker`, `job <name>`.
Thiết kế chi tiết: `docs/backend-design.md`. Hợp đồng API: `api/openapi.yaml`.

## Bắt đầu

```bash
# Đổi module path trong go.mod cho khớp repo thật, rồi:
make tidy          # tải cloud.google.com/go/firestore, firebase.google.com/go/v4, google.golang.org/genai
make test-domain   # các package chỉ dùng thư viện chuẩn, chạy được ngay cả khi chưa tidy
make test          # toàn bộ
make emulators     # cửa sổ khác: Firestore + Auth emulator
make run-api
```

## Biến môi trường cho Mimi Online

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `PAYPAL_MODE` | `fake` | `sandbox`, `live`, `fake` (không gọi PayPal, cấm ở prod) hoặc `off` (tắt các route /orders, /mandate) |
| `PAYPAL_CLIENT_ID`, `PAYPAL_CLIENT_SECRET` | | Bắt buộc với `sandbox` và `live`. Lấy ở developer.paypal.com, mục Apps & Credentials |
| `PAYPAL_VND_PER_USD` | `25000` | PayPal không nhận VND, đơn được tính bằng USD theo tỷ giá này (1000 đến 1000000) |
| `GEMINI_LOCATION` | `global` | Vùng Vertex AI cho đọc hóa đơn và Verify ảnh giao hàng |

Luồng: `POST /orders/basket` → `POST /orders` (tạo PayPal order) → người dùng duyệt trên PayPal →
`POST /orders/{id}/capture` (ghi Spending, không cộng tồn kho) → `/advance` (cửa hàng sandbox mô phỏng giao)
→ `/delivery/read` + `/verify` (chỉ lượng xác nhận vào Confirmed Inventory) → `/refund` nếu thiếu.

## Triển khai

```bash
docker build -t asia-southeast1-docker.pkg.dev/$PROJECT/mimi/backend:$SHA .
gcloud run deploy mimi-api --image ... --args serve-api --service-account sa-api@... \
  --region asia-southeast1 --no-traffic --tag canary
gcloud run deploy mimi-worker --image ... --args serve-worker --ingress internal --no-allow-unauthenticated
gcloud run jobs deploy mimi-job-digest --image ... --args job,digest
firebase deploy --only firestore:rules,firestore:indexes
```
