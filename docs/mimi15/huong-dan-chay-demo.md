# Hướng dẫn chạy giao diện Mimi với dữ liệu mẫu

Chế độ này chỉ chạy phần giao diện trên máy bạn. Không cần backend, Firebase hay Gemini. Mọi dữ liệu là dữ liệu mẫu, tải lại trang là về như ban đầu.

## 1. Cài 2 phần mềm (làm một lần)

- **Git**: tải ở https://git-scm.com/downloads, cài mặc định.
- **Node.js bản 20 trở lên**: tải bản LTS ở https://nodejs.org, cài mặc định.

Mở cửa sổ lệnh:
- Windows: bấm Start, gõ `PowerShell`, mở lên.
- Mac: bấm Cmd + Space, gõ `Terminal`, mở lên.

Kiểm tra đã cài được chưa, gõ từng dòng rồi Enter:

```
git --version
node --version
```

Nếu hiện số phiên bản (ví dụ `v22.x`) là được.

## 2. Tải code và chạy

Gõ lần lượt từng dòng, mỗi dòng bấm Enter và chờ xong mới gõ dòng tiếp:

```
git clone https://github.com/qlinh142/Mimi-Test.git
cd Mimi-Test
cd frontend
```

Tạo file cấu hình (chọn dòng theo máy):

```
cp .env.example .env          (Mac)
copy .env.example .env        (Windows)
```

Cài thư viện (lần đầu mất 1 đến 3 phút) rồi chạy:

```
npm install
npm run dev
```

Khi thấy dòng `Local: http://localhost:5173/` là đã chạy. Để nguyên cửa sổ lệnh này, đừng tắt.

## 3. Mở ứng dụng

1. Mở trình duyệt (Chrome), vào: **http://localhost:5173/?ui=vi** (phần `?ui=vi` để hiện tiếng Việt).
2. Nhập email bất kỳ, ví dụ `test@gmail.com`, bấm **Nhận link đăng nhập**.
3. Bấm **Mở link (bản thử)** là vào được ứng dụng.

Mẹo: bấm F12 rồi bật chế độ điện thoại (biểu tượng điện thoại) để xem giống trên di động.

## 4. Những gì nên thử

**Kế hoạch**
- Chọn hoặc tạo Kế hoạch mới ở hàng nút trên cùng.
- "Thêm món mới": gõ tên (ví dụ "sữa"), chọn nhóm có sẵn hoặc thêm tên tự nhập.
- "Được nhờ mua": có sẵn lời nhờ của Minh (Bánh mì).
- Phần ước tính giá theo lịch sử giá các nơi đã mua.

**Sổ chi tiêu**
- Dự báo 30 ngày tới.
- Khoản định kỳ (Netflix, Internet Viettel): bấm **Đã trả** hoặc **Bỏ qua kỳ này**, hoặc thêm khoản mới.

**Cài đặt → Nhà mình**
- Bấm tạo mã mời. Bản thử sẽ tự thêm thành viên mẫu "Minh".
- Quay lại **Kế hoạch**, phần "Nhờ người nhà mua": chọn Minh, tick món, bấm gửi. Món đó sẽ bị khóa trong Kế hoạch, có thể hủy.

**Chi tiết món** (bấm vào một món, ví dụ Sữa tươi)
- Các lần mua còn lại, lịch sử giá (gần nhất, thấp nhất, trung bình).
- Thêm / Dùng / Đánh dấu hết.

**Quét**
- Chọn ảnh bất kỳ trong máy. Bản thử trả về hóa đơn mẫu 4 dòng, trong đó "Bánh quy bơ" cần chọn nhóm trước khi lưu.

## 5. Tắt và chạy lại

- Tắt: quay lại cửa sổ lệnh, bấm **Ctrl + C**.
- Lần sau chạy lại chỉ cần:

```
cd Mimi-Test/frontend
npm run dev
```

- Lấy bản code mới nhất (khi mình có cập nhật): trong thư mục `Mimi-Test` gõ `git pull`.

## 6. Lỗi hay gặp

| Lỗi | Cách xử lý |
|---|---|
| `npm` / `node` / `git` is not recognized, command not found | Chưa cài hoặc cần tắt mở lại cửa sổ lệnh sau khi cài. |
| `Port 5173 is in use` | Đang có một bản chạy rồi. Tắt cửa sổ cũ, hoặc mở đúng địa chỉ mà dòng `Local:` mới hiện ra (ví dụ 5174). |
| Windows báo "running scripts is disabled" khi gõ npm | Mở PowerShell, gõ `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`, chọn Y, rồi thử lại. |
| Trang hiện tiếng Anh | Thêm `?ui=vi` vào cuối địa chỉ. |
| `npm install` báo lỗi mạng | Kiểm tra mạng rồi chạy lại `npm install`. |

## Lưu ý đăng nhập trên điện thoại (02/11/2026)

- App giữ phiên đăng nhập trong máy (Firebase, lưu ở IndexedDB). Mở app là vào thẳng, chỉ đăng nhập lại khi bấm Đăng xuất hoặc tài khoản bị thu hồi.
- Riêng "Xóa toàn bộ dữ liệu của tôi" sẽ yêu cầu đăng nhập lại ngay lúc đó để xác nhận (B10).
- iPhone: nếu mở bằng Safari như trang web và không dùng trong 7 ngày, Safari có thể xóa dữ liệu trang và phải đăng nhập lại. Khi demo, cài Mimi ra màn hình chính (Chia sẻ → Thêm vào Màn hình chính) để tránh.
