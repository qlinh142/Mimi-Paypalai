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
gcloud run jobs deploy mimi-job-fx-sync --image ... --args job,fx-sync
gcloud run jobs deploy mimi-job-cleanup --image ... --args job,cleanup
firebase deploy --only firestore:rules,firestore:indexes
```

Các job Cloud Scheduler gọi:

| Job | Lịch gợi ý (giờ Việt Nam) | Làm gì |
|---|---|---|
| `digest` | mỗi 15 phút | Gửi thư tóm tắt đúng giờ mỗi người chọn |
| `fx-sync` | 06:00 hằng ngày | Lấy tỷ giá từ open.er-api.com vào `fx_rates` (cần ra Internet) |
| `cleanup` | 03:00 hằng ngày | Gỡ mã mời hết hạn, xóa thông báo quá 90 ngày |
| `migrate` | chạy tay sau deploy có bước chuyển dữ liệu | Xem mục Phiên bản dữ liệu |

### Dữ liệu demo (`seed-demo`)

Ghi khoảng 5 tuần lịch sử mua sắm của nhà chị Lan (7 hóa đơn, tính lùi từ hôm nay) vào một tài khoản, đi qua đúng đường lưu hóa đơn thật:

1. Mở app, đăng nhập tài khoản demo một lần (để có `users/{uid}`). Lấy uid trong Firebase console → Authentication.
2. Chạy job với `DEMO_UID`:

```bash
gcloud run jobs execute mimi-job-seed-demo --region asia-southeast1 --update-env-vars DEMO_UID=<uid> --wait
```

Nên chạy 1 đến 2 ngày trước buổi demo: Sữa tươi khi đó còn khoảng 2 ngày, Trứng gà sắp hết, Dầu gội còn khoảng 5 ngày. Chạy lại cùng ngày không ghi trùng.

Bản mới deploy với `--no-traffic --tag canary` nên chưa ai dùng tới. Thử ở URL `canary` rồi mới chuyển traffic:

```bash
gcloud run services update-traffic mimi-api --region asia-southeast1 --to-tags canary=100
```

## Thông báo trong nhà (hàng đợi `notify`)

Gửi kế hoạch, cập nhật, hủy, mua xong, từ chối, món chưa mua sang kế hoạch mới: mỗi việc ghi một sự kiện
vào `users/{uid}/events/{id}` trong cùng transaction với thay đổi, nên không mất. App đọc ở `GET /notifications`.
Khi có hàng đợi, API đẩy thêm một task để `mimi-worker` gửi thư (tên task theo sự kiện, đẩy hai lần không gửi hai thư).

```bash
# Một lần cho mỗi project
gcloud tasks queues create notify --location asia-southeast1 \
  --max-attempts 5 --min-backoff 30s --max-backoff 10m --max-doublings 4
# API cần quyền tạo task và "đóng vai" service account gọi worker
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:sa-api@$PROJECT.iam.gserviceaccount.com --role roles/cloudtasks.enqueuer
gcloud iam service-accounts add-iam-policy-binding sa-tasks@$PROJECT.iam.gserviceaccount.com \
  --member serviceAccount:sa-api@$PROJECT.iam.gserviceaccount.com --role roles/iam.serviceAccountUser
gcloud run services add-iam-policy-binding mimi-worker --region asia-southeast1 \
  --member serviceAccount:sa-tasks@$PROJECT.iam.gserviceaccount.com --role roles/run.invoker
```

Biến môi trường của `mimi-api`: `TASKS_QUEUE=projects/$PROJECT/locations/asia-southeast1/queues/notify`,
`WORKER_URL` (URL của mimi-worker), `TASKS_SA_EMAIL=sa-tasks@...`. Không đặt `TASKS_QUEUE` thì thông báo chỉ hiện trong app.
Worker trả 503 khi lỗi tạm thời để hàng đợi thử lại theo cấu hình trên, 204 khi đã gửi hoặc lỗi vĩnh viễn.
Gửi thư thật cần nhà cung cấp email (bước 13).

## Quay lại bản cũ (rollback)

Quay lại chỉ đổi **code** đang chạy. Dữ liệu trong Firestore giữ nguyên, người dùng không mất gì.

**Khi nào quay lại:** ngay sau một lần deploy mà app không mở được, lưu hóa đơn lỗi, tỷ lệ lỗi 5xx tăng rõ
trên Cloud Monitoring, hoặc dữ liệu hiện sai. Quay lại trước, tìm nguyên nhân sau.

**Backend (Cloud Run giữ mọi revision cũ):**

```bash
# 1. Xem các revision, bản đang nhận traffic nằm trên cùng
gcloud run revisions list --service mimi-api --region asia-southeast1 --limit 5

# 2. Chuyển 100% traffic về revision trước đó (thay tên cho đúng)
gcloud run services update-traffic mimi-api --region asia-southeast1 --to-revisions mimi-api-00042-abc=100

# 3. Worker và job làm tương tự
gcloud run services update-traffic mimi-worker --region asia-southeast1 --to-revisions mimi-worker-00017-xyz=100
gcloud run jobs update mimi-job-digest --region asia-southeast1 --image <image của bản trước>
```

**Frontend (Firebase Hosting giữ các bản đã phát hành):**

```bash
firebase hosting:rollback            # về bản phát hành ngay trước
# hoặc: Firebase console → Hosting → Release history → chọn bản → Rollback
```

PWA trên điện thoại sẽ nhận bản cũ ở lần mở app kế tiếp (service worker tự cập nhật).

**Firestore rules và index:** deploy lại từ commit cũ:

```bash
git checkout <commit cũ> -- backend/firestore.rules backend/firestore.indexes.json
firebase deploy --only firestore:rules,firestore:indexes
```

**Luật để quay lại không làm hỏng dữ liệu:**

1. Mọi thay đổi dữ liệu phải để **code cũ vẫn đọc được**: chỉ thêm ô mới, không đổi tên hay xóa ô cũ trong cùng một lần deploy.
   Đổi tên thì làm hai lần: lần 1 ghi cả ô cũ lẫn ô mới, lần 2 (khi đã chắc không cần quay lại) mới bỏ ô cũ.
2. Bước chuyển dữ liệu (`mimi job migrate`) chạy **sau** khi code mới đã ổn định, không chạy cùng lúc deploy.
3. Sau khi quay lại, ghi vào `CHANGELOG.md`: giờ quay lại, revision nào, vì sao.

### Phiên bản dữ liệu (`schema_version`)

Mỗi `users/{uid}` có ô `schema_version`, cho biết dữ liệu của người đó đang ở phiên bản nào (hiện là 1).
Mỗi người dùng chỉ có một ô số này, nên dữ liệu không phình ra.
Danh sách bước chuyển nằm ở `internal/gcp/fsstore/migrate.go`.

- **Khi đổi cách lưu:** thêm một bước có `Version` tiếp theo. Bước đó phải chạy lại nhiều lần được mà không hỏng.
- **Khi bước chạy:** lúc người dùng đăng nhập mà dữ liệu còn ở bản cũ. Người lâu không đăng nhập thì chạy job:

```bash
gcloud run jobs deploy mimi-job-migrate --image ... --args job,migrate   # một lần
gcloud run jobs execute mimi-job-migrate --region asia-southeast1
```

- **Sau khi quay lại bản cũ:** code cũ gặp dữ liệu có phiên bản mới hơn thì để nguyên, không hạ phiên bản.

**Diễn tập:** làm thử một lần trên project dev trước demo: deploy bản mới, quay lại revision cũ,
kiểm tra app vẫn mở và dữ liệu còn nguyên.
