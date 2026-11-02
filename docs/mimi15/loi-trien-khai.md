# Sổ theo dõi lỗi triển khai Mimi

Mimi đối chiếu code với bảng 100 vấn đề triển khai (file mẹ gửi ngày 01/11/2026, `uploads/hearth/` "100 van de.md").
Mỗi lỗ hổng tìm thấy có một dòng ở đây. Sửa xong thì ghi PR, ngày và cách kiểm tra. Tìm thấy lỗi mới thì thêm dòng.

Báo cáo đầy đủ theo 10 nhóm (đã có / còn thiếu): https://claude.ai/artifact/KjBxdpyhR5GRJznYWWLcMn

## Bảng 1 · Đã sửa

| # | Vấn đề (số trong bảng 100) | Trước khi sửa | Đã sửa thế nào | PR, ngày gộp | Kiểm tra |
|---|---|---|---|---|---|
| 1 | Gửi trùng, retry không an toàn (2, 9, 10) | App gửi `Idempotency-Key` nhưng backend bỏ qua. Hàng đợi offline gửi lại có thể làm một thao tác chạy hai lần | Middleware giữ kết quả theo key 24 giờ ở `users/{uid}/idempotency/{key}`. Gửi lại cùng nội dung thì trả kết quả cũ, khác nội dung thì 422, đang xử lý thì 409 thử lại được | PR #6, 02/11/2026 (`87e0a36`) | Test middleware và `idem.Memory`; `go test`, `tsc` qua |
| 2 | Đăng xuất còn dữ liệu (65) | Xóa cache và hàng đợi nhưng còn nháp kế hoạch, vòng đi mua hộ, kế hoạch đang mở... trong localStorage. Đổi tài khoản trên cùng máy thì người sau thấy nháp của người trước | `wipeDeviceData()` trong `frontend/src/lib/localData.ts` xóa mọi khóa `mimi.*`, chỉ giữ ngôn ngữ giao diện. Hai nút Đăng xuất dùng chung | PR #7, 02/11/2026 (`9464b0b`) | `tsc`, `npm run build` qua |
| 3 | Loading không kết thúc, không timeout (13, 90) | App gọi API không giới hạn thời gian, mạng chập chờn thì quay mãi | Dừng sau 20 giây, ảnh hóa đơn 65 giây. Báo "Mạng chậm quá"; GET bị hết giờ được tự thử lại | PR #7 | `tsc`, `npm run build` qua |
| 4 | "Hôm nay" và job bị bỏ qua (52, 58) | Thư tóm tắt luôn gửi 8:00, bỏ qua giờ chọn trong Cài đặt. Gửi lỗi thì không bao giờ gửi lại | Gửi theo `settings.digest_time`. Lỗi tạm thời thử lại sau 15 phút, tối đa 3 lần; kẹt "sending" quá 30 phút được gửi lại; bị từ chối hẳn thì chờ ngày mai | PR #7 | 3 test mới trong `notify/digest_test.go` |
| 5 | Sai currency (44) | Tỷ giá PayPal 25.000 đ/USD cố định, đơn không ghi nguồn và ngày | Giỏ, đơn, payment ghi `rate_source`, `rate_as_of`. `PAYPAL_MODE=live` bắt buộc khai `PAYPAL_VND_PER_USD` và `PAYPAL_RATE_AS_OF` | PR #7 | Test `TestCheckoutRecordsRateInfo` |
| 6 | Secret lên Git, không có regression test tự động (71, 95) | Không có CI, test chỉ chạy tay | `.github/workflows/ci.yml`: `go vet`, `go test -race`, build frontend, gitleaks quét toàn bộ lịch sử | PR #7 | CI xanh cả 3 job; gitleaks không thấy bí mật nào |
| 7 | Hàng đợi offline chưa gắn tài khoản (63, 65) | Bị tự đăng xuất vì 401 rồi người khác đăng nhập trên cùng máy thì thao tác cũ gửi bằng tài khoản người sau, người sau còn thấy cache và nháp của người trước | Mỗi thao tác ghi kèm uid, chỉ gửi và chỉ hiện cho đúng người; đổi tài khoản thì xóa cache và nháp của người trước (`claimDevice`) | PR #8, 02/11/2026 (`b9274fd`) | CI xanh; chạy thử kịch bản A → B → A |
| 8 | Lịch sử mua bị cắt (lỗi V3) | Mọi lần mua đều được lưu, nhưng Chi tiết món chỉ đọc 20 lần gần nhất để chia đợt hàng và 30 lần để tính giá thấp nhất, giá trung bình | Theo ý mẹ dùng toàn bộ lịch sử: Chi tiết một món đọc tới 500 lần mua; màn chỉ hiện đợt còn hàng và 20 đợt mới nhất; giỏ nhiều món giữ 30 lần | PR #9, 02/11/2026 (`f0a6e9d`) | `TestItemReadsFullHistory`, `TestVisibleLots`; CI xanh |
| 9 | Rollback (99), A6 | README chưa có các bước quay lại bản cũ | Mục "Quay lại bản cũ" trong `backend/README.md`: Cloud Run, Hosting, rules, chuyển dần lượt truy cập, quy tắc đổi dữ liệu không làm hỏng bản cũ | PR #10, 02/11/2026 (`26093a8`) | CI xanh |
| 10 | Đăng xuất làm mất icon và tên gọi (B11, lỗi do PR #7) | Icon kế hoạch và tên gọi người nhà chỉ lưu trên máy | Icon lưu cùng Kế hoạch (6 khóa của bộ chung), tên gọi lưu trong `settings.relations` riêng từng tài khoản | PR #11, 02/11/2026 (`f1d8990`) | `TestPlanIconStoredOnServer`, `TestRelations` |
| 11 | Màn 4c, 4d chỉ có dữ liệu mẫu (B1) | Thiếu `/receipts/history`, `/receipts/{id}` | Hai API gom hóa đơn tự lưu, phần mua hộ, khoản định kỳ; hiện tên gọi người mua hộ | PR #12, 02/11/2026 (`74a4f00`) | `TestHistory`, `TestDetail`; chạy thử 4c, 4d |
| 12 | Món không đọc được giá (A7, 49) | Ô giá trống bị báo lỗi, không lưu được | Lưu là chưa rõ giá, không tính 0đ, hiện mờ | PR #13, 02/11/2026 (`ffa3ec5`) | test trong `TestSave` |
| 13 | Màn đi mua hộ chưa có API (B2) | Thiếu `complete`, `decline` | `complete` ghi hóa đơn từ danh sách đã mua, gửi lại không ghi trùng; `decline` trả món về người nhờ | PR #14, 02/11/2026 (`ab4c514`) | `TestCompleteWritesReceipt`, `TestCompleteRules`, `TestDecline` |

## Bảng 1b · Đợt 2 đã sửa và gộp (02/11/2026)

Mỗi mục một PR, gộp một lượt theo thứ tự #15 → #22 ngày 02/11/2026. CI xanh cả 3 job; main sau khi gộp chạy lại `go test`, build frontend đều qua.

| # | Vấn đề (số trong bảng 100) | Trước khi sửa | Đã sửa thế nào | PR | Kiểm tra |
|---|---|---|---|---|---|
| 14 | Migration (37, 38), A4 | Dữ liệu không ghi phiên bản, không có cách chuyển dữ liệu cũ | `users/{uid}.schema_version`, danh sách bước chuyển chạy lại được, tự chạy khi đăng nhập, có job `migrate` | PR #15 (`ace1f8f`) | `TestMigrationsOrdered`, `TestPendingMigrations` |
| 15 | Hóa đơn chỉ trừ một kế hoạch (B3) | Màn Xác nhận chia món theo kế hoạch nhưng lúc lưu chỉ gửi một kế hoạch | Mỗi dòng gửi `plans: [{plan_id, qty}]`, máy chủ trừ đúng từng kế hoạch | PR #16 (`0c26fc4`) | `TestPlanBuys` |
| 16 | Cả bill vào sổ người quét (B4) | Món mua hộ cũng cộng vào Sổ chi tiêu người quét | Mỗi người chỉ cộng phần món của mình; giảm giá vào phần người trả; sổ người nhờ ghi "người nhà mua hộ" | PR #17 (`46c8834`) | `TestSplitSpend`, `TestDelegatedRows` |
| 17 | Gửi lại kế hoạch tạo thêm vòng (B5) | Mỗi lần bấm gửi tạo thêm một lần nhờ mua | Mỗi kế hoạch chỉ có bản đang lưu và bản đã gửi; gửi lại thì ghi đè, người mua thấy giờ cập nhật | PR #18 (`3eb17a4`) | `TestAssignKeepsTwoVersions` |
| 18 | Món chưa mua nằm lại kế hoạch cũ (B6) | Chưa làm theo câu 3 đã chốt | Kế hoạch chốt xong thì món còn mở sang "[tên cũ] · còn lại", giữ icon, nơi mua, không ngày, không người mua | PR #19 (`41eb922`) | `TestRemainName`, `TestRemainLine` |
| 19 | Món không có giá khi đi mua không bill (B7, 50) | Không nhập giá thì lưu trống dù đã từng mua | Lấy giá gần nhất, ghi "Ước tính"; chưa từng có giá thì để chưa có giá; mỗi dòng ghi `price_source`, có chỗ cho giá online | PR #20 (`9f8dbc0`) | `TestCompletePriceSources` |
| 20 | Thông báo trong nhà (B9) | Worker `/tasks/notify` trả 501, không ai được báo | Sự kiện ghi cùng transaction; hàng đợi Cloud Tasks gửi thư, lỗi tạm thì thử lại, không gửi trùng; màn Thông báo hiện và đánh dấu đã đọc | PR #21 (`34a30e7`) | `TestDeliver`, `TestTaskBody`, `TestListEvents` |
| 21 | Loading, lỗi từng màn (12, 13, 90), A10 | Vài màn lỗi trông như trống, quay mãi, hoặc báo nhầm | `LoadState` dùng chung: đang tải, lỗi theo loại, Thử lại; áp dụng cho 8 màn còn thiếu | PR #22 (`636d15a`) | Chạy thử chế độ mẫu |

Còn lại cho sau: nhập giá sau cho món "chưa có giá" (màn sửa giá trong Chi tiết hóa đơn); gửi thư thật cần nhà cung cấp email (bước 13).

## Bảng 1c · Đợt 3 đã sửa và gộp (02/11/2026)

Mỗi mục một PR, gộp một lượt theo thứ tự #23 → #28 ngày 02/11/2026. CI xanh cả 3 job; main sau khi gộp chạy lại `go test`, `gen:api`, build frontend đều qua.

| # | Vấn đề (số trong bảng 100) | Trước khi sửa | Đã sửa thế nào | PR | Kiểm tra |
|---|---|---|---|---|---|
| 22 | Hai máy cùng sửa kế hoạch (4), A1 | Máy lưu sau ghi đè ngày, giờ, ghi chú của máy lưu trước mà không ai biết | Kế hoạch có số phiên bản `rev`; lệch thì máy chủ trả 409, app tải lại bản mới và báo mẹ. Sửa từng món vẫn là lần ghi sau thắng | PR #23 (`bcbfc77`) | `TestSetStateRevConflict` |
| 23 | Unicode NFC (28), A2 | Chữ dán từ Mac hoặc AI đọc có thể ở dạng dấu rời, nhìn giống mà máy so khác | Mọi chữ gửi lên về dạng NFC trước khi lưu; tìm không dấu bỏ dấu ở mọi dạng | PR #24 (`c0edceb`) | `TestNFCAndFoldDecomposed`, `TestDecodeJSONNormalizesNFC` |
| 24 | API contract (23, 82), A3 | Kiểu frontend viết tay; `openapi.yaml` thực ra đã hỏng (mất dòng `Item:`) mà không ai biết | Commit kiểu sinh từ OpenAPI, CI báo khi lệch; tsc so tên trường 22 kiểu; test Go so route với OpenAPI. Sửa 9 chỗ lệch tìm ra | PR #25 (`09b99a5`) | `TestOpenAPIMatchesRoutes`, `contract.ts` |
| 25 | Rate limit theo instance (79), A5 | 60 lần ghi mỗi phút đếm riêng từng instance | Đếm chung trên Firestore trong transaction; Firestore lỗi thì không chặn người dùng | PR #26 (`dae4b25`) | `TestAllowWriteSharedAcrossInstances` |
| 26 | N+1 khi đọc giá (89), A9 | Đọc giá từng món nối tiếp nhau | Đọc song song tối đa 8 lượt, giữ thứ tự kết quả | PR #27 (`7ef99ea`) | `TestEachGroupParallel` |
| 27 | Xóa tài khoản, B10 | `DELETE /me` trả 501, app chưa có chỗ xóa | Đăng nhập lại trong 5 phút để xác nhận; chủ nhà giải tán nhà, thành viên rời nhà; xóa hết dữ liệu rồi xóa tài khoản đăng nhập | PR #28 (`7e23d39`) | `TestDeleteMeNeedsRecentSignIn`, `TestDeleteOwnerDissolvesHousehold` |

Để sau: xếp lại thư mục frontend theo tính năng và thay hẳn `types.ts` bằng kiểu sinh ra (cần OpenAPI ghi `required`); thử xóa tài khoản với Firebase thật (bước 13).

## Bảng 1d · Đợt 4 đã sửa và gộp (02/11/2026)

Mỗi mục một PR, gộp một lượt theo thứ tự #29 → #34 ngày 02/11/2026. CI xanh cả 4 job; main sau khi gộp chạy lại `go test`, `gen:api`, build và test E2E đều qua.

| # | Vấn đề | Trước khi sửa | Đã sửa thế nào | PR | Kiểm tra |
|---|---|---|---|---|---|
| 28 | Test emulator không chạy ở đâu | Test Firestore thật tự bỏ qua vì CI không có emulator; một test đã cũ mà không ai biết | CI có job `emulator` chạy Firestore emulator; sửa test cũ theo cách gửi lại cho cùng người (B5) | PR #29 (`b8fd721`) | CI job `emulator` |
| 29 | Job cleanup, rules/index | Chưa có job; mã mời hết hạn và thông báo cũ nằm mãi | Job `cleanup` gỡ mã mời hết hạn, xóa thông báo quá 90 ngày (kèm TTL); rà index cho mọi truy vấn | PR #30 (`2a23551`) | `TestCleanupEmulator` |
| 30 | Job fx-sync | Chưa có job, hóa đơn ngoại tệ không quy đổi được | Job lấy tỷ giá hằng ngày từ open.er-api.com cho 12 ngoại tệ, ghi nguồn và giờ lấy | PR #31 (`4a82282`) | `TestSyncFromERAPI`, `TestSaveRatesThenRateEmulator` |
| 31 | Dữ liệu demo (V5) | Ngày cố định 16/10; chưa có cách nạp lịch sử cho tài khoản demo | Job `seed-demo` ghi 7 hóa đơn tính lùi từ hôm nay. Test bắt thêm 2 lỗi: job thiếu danh sách danh mục nên món không vào Đồ dùng; rau, chuối, bánh quy thiếu khối lượng | PR #32 (`adb1234`) | `TestSeedLinesReachInventory`, `TestDemoSeedEmulator` |
| 32 | Danh mục chi tiêu một nguồn (V2) | 27 danh mục viết tay ở 3 nơi (backend "KHÓA TẠM", vi.ts, en.ts) | Một file `categories.json`; frontend dùng bản chép, test báo khi lệch | PR #33 (`d07403f`) | `TestCategoriesFrontendCopy` |
| 33 | Test E2E kịch bản demo | Chưa có; bản thử đang không lưu được hóa đơn ("Unknown field") mà không ai biết | Test Playwright chạy cảnh 1 → 3c trong CI; sửa danh sách trường của backend giả | PR #34 (`b0076e5`) | `e2e/demo.spec.ts` |

## Bảng 2 · Còn mở (chưa sửa)

| Vấn đề | Mô tả | Mức | Ghi chú |
|---|---|---|---|
| Sửa từng món trên hai máy (4) | Số lượng, ưu tiên của một món vẫn là lần ghi sau thắng | Thấp | Ngày, giờ, ghi chú kế hoạch đã có kiểm tra (A1) |
| Chưa soi kỹ (15, 49, 86, 87) | Optimistic update, 0 và null, phân trang | Chưa rõ | Loading (12), chưa có giá (50) sửa ở Đợt 2; N+1 (89) sửa ở Đợt 3 |

## Lịch sử sổ này

- 01/11/2026: rà soát lần đầu, tìm 6 lỗ hổng chính.
- 02/11/2026: gộp PR #6 và PR #7, sửa xong 6 lỗ hổng. Mở sổ này.
- 02/11/2026: gộp PR #8 (hàng đợi gắn tài khoản) và PR #9 (đọc đủ lịch sử mua).
- 02/11/2026: làm xong Đợt 1 (A6, B11, B1, A7, B2), gộp một lượt PR #10 đến #14 theo ý mẹ.
- 02/11/2026: sửa xong Đợt 2 (A4, B3 đến B7, B9, A10), PR #15 đến #22, gộp một lượt theo ý mẹ.
- 02/11/2026: sửa xong Đợt 3 (A1, A2, A3, A5, A9, B10), PR #23 đến #28, gộp một lượt theo ý mẹ.
