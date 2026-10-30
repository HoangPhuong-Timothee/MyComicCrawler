# 🚀 Comic Crawler Engine (Golang High-Speed & Loop)

Hệ thống cào truyện tranh / manga / comic tự động thế hệ mới được viết lại toàn bộ tầng lõi (Core Engine) bằng **Golang**, đóng gói thành **file thực thi độc lập (`comic-crawler.exe`)** không cần cài đặt môi trường, tự động xử lý bảo mật (Cloudflare Turnstile / TLS Fingerprinting / Mirror Failover), tải ảnh siêu tốc qua **Worker Pool Goroutines**, nén trực tiếp trong **RAM thành file `.cbz`** và chạy theo **vòng lặp tương tác liên tục**.

---

## 🌟 Tính năng nổi bật & Cải tiến cốt lõi

1. **Hiệu năng cực cao với Golang:**
   - Quản lý Goroutines tải song song đa luồng từ CDN hình ảnh với Connection Pool tối ưu (`net/http.Transport` kết hợp `tls-client`).
   - Xử lý ảnh hoàn toàn trong bộ nhớ (In-Memory buffer) và nén trực tiếp thành định dạng `.cbz` với metadata chuẩn `ComicInfo.xml`, không lưu rác/ảnh tạm ra đĩa cứng.

2. **Khả năng mang vác di động (Zero-Dependency Portability):**
   - Đã được biên dịch sẵn thành file nhị phân tĩnh duy nhất `comic-crawler.exe` (~13MB).
   - Chỉ cần chép thư mục hoặc file `.exe` sang bất kỳ máy tính Windows nào là chạy được ngay, **không cần cài đặt Python, Go, Node.js hay bất kỳ thư viện nào khác**.

3. **Vòng lặp tương tác liên tục (Continuous Interactive Loop):**
   - Tool chạy vòng lặp liên tục: cào xong 1 bộ truyện sẽ hiển thị hướng dẫn chép vào máy đọc sách và tự động hỏi tiếp URL của bộ truyện tiếp theo.
   - Khi muốn dừng, chỉ cần gõ `exit`, `quit` hoặc nhấn tổ hợp phím `Ctrl + C`.

4. **Kiến trúc Sidecar Solver thông minh (Fallback Cloudflare Solver):**
   - Mặc định: Go Engine tự giải quyết bằng `tls-client` giả lập chữ ký trình duyệt Chrome 120 kết hợp cơ chế **Mirror Failover**.
   - Mở rộng: Nếu gặp thử thách Turnstile/Captcha phức tạp ở cấp độ JS challenge, Go tự động kích hoạt Sidecar Helper (FlareSolverr tại `http://localhost:8191` hoặc `solver.py`) để lấy token/cookies rồi tiếp tục tải tốc độ cao.

5. **Tối ưu chuẩn máy đọc sách Android:**
   - Đánh số thứ tự chương và trang ảnh chuẩn hóa zero-padded (`Chapter 001.cbz`, trang `001.jpg`, `002.jpg`).
   - Tương thích 100% với các ứng dụng đọc truyện phổ biến trên Android / E-Ink: **Mihon**, **Tachiyomi**, **Kotatsu**, **Moon+ Reader**, **Perfect Viewer**, **Boox NeoReader**.

---

## 🏛️ Cấu trúc dự án

```text
comic-craw-tool/
├── comic-crawler.exe        # Ứng dụng lõi độc lập viết bằng Go (Chạy trực tiếp)
├── run.bat                  # Script khởi động nhanh 1-click trên Windows
├── solver.py                # Helper Sidecar giải Captcha/Turnstile dự phòng
│
├── cmd/
│   └── comic-crawler/
│       └── main.go          # CLI Entrypoint, Interactive Loop, Signal handling
│
├── pkg/
│   ├── models/              # Định nghĩa Structs (Manga, Chapter, PageData)
│   ├── network/             # Dual-Client Fetcher (tls-client + HTTP pool) & Sidecar Caller
│   ├── parsers/             # Trích xuất dữ liệu, phát hiện Dynamic AJAX API & DOM scanning
│   ├── downloader/          # Goroutines Worker Pool tải ảnh song song với Progress Bar
│   ├── packager/            # Đóng gói file .cbz trực tiếp trong RAM & ComicInfo.xml
│   └── utils/               # Chuẩn hóa tên file, zero-padding, nhận diện MIME type
│
└── comic_crawler/           # (Legacy) Module Python dự phòng
```

---

## 🛠️ Hướng dẫn sử dụng

### 1. Khởi chạy nhanh (1-Click)
Nhấp đúp vào file [`run.bat`](run.bat) hoặc mở PowerShell/CMD:
```powershell
.\comic-crawler.exe
```

### 2. Thao tác trên giao diện dòng lệnh:
1. **Nhập URL:** Dán link truyện (ví dụ: `https://.../truyen-tranh/homunculus` hoặc bất kỳ trang truyện nào).
2. **Thư mục lưu:** Nhấn **Enter** để chọn thư mục mặc định `comic/` tự động nhận diện nằm cùng cấp với tool (không nằm trong source code), hoặc nhập đường dẫn tùy chỉnh (tương đối hoặc tuyệt đối).
3. **Chọn chương tải:**
   - Nhấn **Enter** (hoặc gõ `all`): Tải tất cả các chương.
   - Gõ khoảng: `1-10` (tải từ chap 1 đến chap 10).
   - Gõ danh sách: `1, 3, 5` hoặc `20`.
4. Sau khi hoàn thành, tool sẽ in đường dẫn thư mục và hỏi tiếp URL truyện kế tiếp.
5. Gõ `exit` hoặc bấm `Ctrl + C` để thoát.

---

## 📱 Hướng dẫn chép vào máy đọc sách Android

Sau khi tải xong, trong thư mục đích chỉ chứa duy nhất các file `.cbz` nén sạch sẽ:
```text
[Thư mục cha]/comic/[Tên Truyện]/
├── Chapter 001.cbz
├── Chapter 002.cbz
└── Chapter 003.cbz
```

1. **Với Mihon / Tachiyomi / Kotatsu:**
   - Cắm cáp USB vào máy tính (chọn chế độ Chuyển tệp MTP).
   - Chép cả thư mục truyện vào đường dẫn: `Bộ nhớ máy/Mihon/local/<Tên Truyện>` (hoặc `Tachiyomi/local/`).
   - Mở app Mihon -> chuyển qua tab **Duyệt (Browse)** -> **Nguồn cục bộ (Local source)** để đọc.
2. **Với Moon+ Reader / Perfect Viewer / Máy Boox / Meebook:**
   - Chép vào thư mục `Books/` hoặc `Comics/` trên máy đọc sách.
   - Mở ứng dụng và chọn đọc trực tiếp file `.cbz`.
